# Frida Guide for NieR Re[in]carnation Reverse Engineering

Полное руководство по использованию Frida для анализа и модификации игры NieR Re[in]carnation на Android.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Architecture Overview](#architecture-overview)
3. [Common Techniques](#common-techniques)
4. [Dangerous Patterns (NEVER DO)](#dangerous-patterns-never-do)
5. [Safe Patterns](#safe-patterns)
6. [Debugging hangs/crashes](#debugging-hangscrashes)
7. [Analyzing Async Code](#analyzing-async-code)
8. [Finding RVAs](#finding-rvas)
9. [Useful Scripts](#useful-scripts)

---

## Quick Start

### Setup

```bash
# 1. Start frida-server on emulator (after cold boot)
adb shell /data/local/tmp/frida-server &

# 2. Launch game with Frida script
tail -f /dev/null | frida -Uf com.square_enix.android_googleplay.nierspww -l frida/hooks.js

# 3. Check terminal output
cat ~/.cursor/projects/*/terminals/*.txt
```

### File Structure

- `frida/hooks.js` — main script with all hooks
- `client/il2cpp_dump/dump.cs` — 1.2M lines of C# signatures + RVAs
- `terminals/*.txt` — live log output from Frida

---

## Architecture Overview

### IL2CPP Specifics

NieR uses IL2CPP (C# compiled to C++ then to ARM64). Key characteristics:

1. **MethodInfo* parameter**: Generic/virtual methods have hidden 2nd parameter (x1→x20) pointing to method metadata
2. **Async state machines**: Compiler generates `MoveNext` methods with jump tables
3. **UniTask structs**: Return values in registers (x0 for void, x0+x1 for <T>)

### Critical Memory Layouts

```
// FiniteStateMachineTask (base for Title/Gameplay/Story)
+0x10: CurrentState (int)
+0x14: NextState (int)
+0x38: _firstTime (bool)
+0x39: _inUpdate (bool)
+0x3A: _doUpdateEvent (bool)
+0x3C: _requestUpdateEvent (int)
+0x51: IsCompleted (bool) — for Title FSM

// Gameplay specific
+0x158: CompletedWaitSceneRequestReplace (bool)
```

---

## Common Techniques

### 1. Basic Hook (safe for most methods)

```javascript
function hook(name, offset, callbacks) {
    const ptr = libil2cpp.add(offset);
    Interceptor.attach(ptr, callbacks);
    console.log(`[*] Hook ${name}`);
}

// Usage
hook("Story.ApplyNewestScene", 0x27858E8, {
    onEnter(args) { console.log(`[Story] ApplyNewestScene omitSideStory=${args[1]}`); },
    onLeave(retval) { console.log(`[Story] ApplyNewestScene -> ${retval}`); }
});
```

### 2. Return Value Override

```javascript
// For bool/int return
hook("IsNeedsChapterAssetDownload", 0x273C598, {
    onLeave(retval) { 
        retval.replace(ptr(0)); // force false
        console.log("[Patch] Forced false");
    }
});

// For UniTask<bool> (16-byte struct in x0+x1)
hook("StartQuest", 0x27276A4, {
    onLeave(retval) {
        retval.replace(ptr(1)); // result = true
        this.context.x1 = ptr(0); // source = null (completed)
    }
});
```

### 3. Memory Patch (safest for FSM handlers)

```javascript
// Replace function body directly — no trampoline, no MethodInfo* corruption
const addr = libil2cpp.add(0x274E4D4);
Memory.patchCode(addr, 12, code => {
    code.writeByteArray([
        0x00, 0x00, 0x80, 0xd2, // mov x0, #0
        0x01, 0x00, 0x80, 0xd2, // mov x1, #0  
        0xc0, 0x03, 0x5f, 0xd6  // ret
    ]);
});
// Result: returns completed UniTask instantly
```

### 4. ARM64 Disassembly

```javascript
function disasmMethod(name, rva, maxInsns = 512) {
    const base = libil2cpp;
    const addr = base.add(rva);
    console.log(`\n[DISASM] ${name} at ${addr}`);
    
    for (let i = 0; i < maxInsns; i++) {
        const pc = addr.add(i * 4);
        const word = pc.readU32();
        
        // RET
        if (word === 0xD65F03C0) {
            console.log(`  +${(i*4).toString(16)}: RET`);
            break;
        }
        
        // BL (branch with link) — call target
        if ((word >>> 26) === 0x25) {
            const imm26 = word & 0x03FFFFFF;
            const offset = (imm26 < 0x02000000 ? imm26 : imm26 - 0x04000000) * 4;
            const target = pc.add(offset);
            const targetRVA = target.sub(base).toInt32();
            console.log(`  +${(i*4).toString(16)}: BL 0x${targetRVA.toString(16)}`);
        }
    }
}
```

### 5. Reading Struct Fields

```javascript
// ActivePlayerToEntityMainQuestStatus returns EntityIUserMainQuestMainFlowStatus*
hook("ActivePlayerToEntityMainQuestStatus", 0x2AB491C, {
    onLeave(retval) {
        if (retval.isNull()) {
            console.log("[UserData] MainQuestStatus -> NULL");
        } else {
            const routeId = retval.add(8).readS32();
            const curScene = retval.add(12).readS32();
            const headScene = retval.add(16).readS32();
            console.log(`[UserData] routeId=${routeId} curScene=${curScene} headScene=${headScene}`);
        }
    }
});
```

### 6. Periodic Memory Polling

```javascript
// Force field value every 200ms
setInterval(() => {
    try {
        const gameplay = globalThis._gameplayPtr;
        if (gameplay) {
            const flagAddr = gameplay.add(0x158);
            if (flagAddr.readU8() === 0) {
                flagAddr.writeU8(1); // Force cwsr = true
            }
        }
    } catch(e) {}
}, 200);
```

---

## Dangerous Patterns (NEVER DO)

### ❌ NEVER Hook MoveNext Methods

```javascript
// THIS WILL CORRUPT STATE MACHINES
Interceptor.attach(libil2cpp.add(0x423B594), { // FSM.DoUpdate.MoveNext
    onEnter() { console.log("MoveNext"); }
});
```

**Why**: Async state machines use jump tables (computed branches). Frida's trampoline corrupts the jump table addresses.

**Symptoms**: Hangs, infinite loops, "stuck at state X".

### ❌ NEVER Hook FSM.Setup Chain

```javascript
// CRASH: MethodInfo* corruption
Interceptor.attach(libil2cpp.add(0x423D048), { // FSM.Setup
    onEnter() {}
});
```

**Why**: Shared generic methods pass MethodInfo* in x1→x20. Frida overwrites x20 with return address.

**Symptoms**: Immediate SIGSEGV, abort(), "vtable call on null".

### ❌ NEVER Use Interceptor.replace on FSM Handlers

```javascript
// CRASH: Abort on launch
Interceptor.replace(libil2cpp.add(0x274E4D4), 
    new NativeCallback(() => {}, 'void', ['pointer']));
```

**Why**: NativeCallback only controls x0, but UniTask needs x0+x1. Also corrupts MethodInfo*.

**Use instead**: `Memory.patchCode`.

### ❌ NEVER Hook Functions with ADRP First Instruction

```javascript
// DANGEROUS if first instruction is ADRP (PC-relative)
Interceptor.attach(libil2cpp.add(0x30A9260), { // Title.OnComplete
    onEnter() {}
});
```

**Why**: Frida replaces first instruction with jump. ADRP needs PC-relative addressing which changes.

**Symptoms**: SIGILL, SIGSEGV, wrong data reads.

---

## Safe Patterns

### ✅ Hook Non-Generic Instance Methods

```javascript
// Safe: no MethodInfo* parameter
hook("Story.ApplyNewestScene", 0x27858E8, {
    onLeave(retval) { console.log(`-> ${retval}`); }
});
```

### ✅ Patch Code for Instant Return

```javascript
// Safest way to bypass FSM handlers
Memory.patchCode(addr, 12, code => {
    code.writeByteArray([
        0x00, 0x00, 0x80, 0xd2, // mov x0, #0
        0xc0, 0x03, 0x5f, 0xd6  // ret
    ]);
});
```

### ✅ Attach onLeave Only

```javascript
// Safer than onEnter (less trampoline interference)
Interceptor.attach(ptr, {
    onLeave(retval) {
        retval.replace(ptr(newVal));
    }
});
```

### ✅ Use globalThis for State

```javascript
// Persist across script re-evaluations
globalThis._hooksInstalled = true;
globalThis._gameplayPtr = null;
```

---

## Debugging Hangs/Crashes

### Step 1: Identify Which State Machine Hangs

```javascript
// Check Title FSM
setInterval(() => {
    const title = globalThis._titlePtr;
    if (title) {
        const cs = title.add(0x10).readS32();
        const ic = title.add(0x51).readU8();
        console.log(`Title cs=${cs} ic=${ic}`);
    }
}, 1000);
```

### Step 2: Trace Method Entry/Exit

```javascript
// Add ENTER/LEAVE logging
hook("OnMainStoryAsync", 0x274E4D4, {
    onEnter() { console.log("[TRACE] ENTER"); },
    onLeave() { console.log("[TRACE] LEAVE"); }
});
```

### Step 3: Check for NULL Returns

```javascript
// Critical: many methods return null when data missing
hook("ActivePlayerToEntityMainQuestStatus", 0x2AB491C, {
    onLeave(retval) {
        if (retval.isNull()) {
            console.log("[CRITICAL] MainQuestStatus is NULL - user data missing!");
        }
    }
});
```

### Step 4: Async Exception Tracing

```javascript
// Hook SetException to catch silent async failures
hook("AsyncUniTaskMethodBuilder.SetException", 0x408C594, {
    onEnter(args) {
        const exc = args[1];
        const klass = exc.readPointer();
        const namePtr = klass.add(0x10).readPointer();
        const typeName = namePtr.readCString();
        console.log(`[ASYNC-EXC] ${typeName}`);
    }
});
```

---

## Analyzing Async Code

### Understanding MoveNext State Machines

Compiler transforms `async UniTask Foo()` into:

```csharp
struct <Foo>d__X : IAsyncStateMachine {
    int <>1__state;        // Current await index
    AsyncUniTaskMethodBuilder <>t__builder;
    
    void MoveNext() {
        switch (<>1__state) {
            case 0: /* start */ break;
            case 1: /* after 1st await */ break;
            case 2: /* after 2nd await */ break;
        }
    }
}
```

### State Counting Technique

```javascript
let moveNextCount = 0;
hook("OnMainStoryAsync.MoveNext", 0x2885CF8, {
    onEnter() {
        moveNextCount++;
        console.log(`[MoveNext] count=${moveNextCount}`);
        
        // Access state field (offset varies, typical +0x10)
        const state = this.state.add(0x10).readS32();
        console.log(`[State] state=${state}`);
    }
});
```

### Finding BL Targets in MoveNext

Use disassembly to see what methods are awaited:

```javascript
// RVA from dump.cs: "// RVA: 0x2885CF8"
disasmMethod("OnMainStoryAsync.MoveNext", 0x2885CF8);
```

Look for `BL` instructions — these are the awaited method calls.

---

## Finding RVAs

### From dump.cs

```bash
# Find method RVA
grep -n "ApplyNewestScene" client/il2cpp_dump/dump.cs | head -5

# Output: "// RVA: 0x27858E8 Offset: 0x27858E8"
```

### Search Patterns

```bash
# Search for method with context
grep -B2 -A2 "Story.ApplyNewestScene" client/il2cpp_dump/dump.cs

# Search by offset pattern
grep "0x27858E8" client/il2cpp_dump/dump.cs
```

### Key RVA Patterns

| Pattern | Example |
|---------|---------|
| Method | `// RVA: 0x27858E8` |
| Field | `private int <CurrentQuestSceneId>k__BackingField; // 0x20` |
| Offset | `Offset: 0x27858E8` (same as RVA in libil2cpp.so) |

---

## Useful Scripts

### Template: New Hook File

```javascript
const SERVER_ADDRESS = "10.0.2.2";
let libil2cpp;

function awaitLibil2cpp(callback) {
    if (globalThis._hooksInstalled) return;
    try {
        libil2cpp = Process.getModuleByName("libil2cpp.so").base;
        globalThis._hooksInstalled = true;
        console.log("[*] libil2cpp.so loaded at:", libil2cpp);
        callback();
    } catch (error) {
        setTimeout(() => awaitLibil2cpp(callback), 100);
    }
}

function hook(name, offset, callbacks) {
    const ptr = libil2cpp.add(offset);
    Interceptor.attach(ptr, callbacks);
    console.log(`[*] Hook ${name}`);
}

awaitLibil2cpp(() => {
    // YOUR HOOKS HERE
    hook("Target.Method", 0x1234567, {
        onEnter(args) { console.log("Called"); },
        onLeave(retval) { console.log(`Returned ${retval}`); }
    });
});
```

### Quick Test Script

```javascript
// test_hook.js — test single method
function hook(name, offset) {
    const base = Process.getModuleByName("libil2cpp.so").base;
    const ptr = base.add(offset);
    Interceptor.attach(ptr, {
        onEnter(args) { console.log(`[${name}] ENTER`); },
        onLeave(retval) { console.log(`[${name}] LEAVE -> ${retval}`); }
    });
    console.log(`[*] Hooked ${name} at ${ptr}`);
}

// Test: ApplyNewestScene
hook("ApplyNewestScene", 0x27858E8);
```

### Memory Scanner (find pointers)

```javascript
function findGameplayInstance() {
    const ranges = Process.enumerateRanges({protection: 'rw-'});
    for (const range of ranges) {
        Memory.scan(range.base, range.size, '00 00 00 00 00 00 00 00', {
            onMatch(addr, size) {
                try {
                    // Check if looks like Gameplay object
                    const vtable = addr.readPointer();
                    if (vtable.toInt32() > 0x6b00000000) {
                        console.log(`Possible object at ${addr}`);
                    }
                } catch(e) {}
            }
        });
    }
}
```

---

## Common Troubleshooting

### "TypeError: not a function"

```javascript
// BROKEN (static method)
Module.findExportByName("libil2cpp.so", "il2cpp_raise_exception")

// FIXED (instance method)
Process.getModuleByName("libil2cpp.so").findExportByName("il2cpp_raise_exception")
```

### Game Aborts on Launch

1. Check for FSM.Setup hooks — REMOVE THEM
2. Check for MoveNext hooks — REMOVE THEM
3. Check for Interceptor.replace on FSM handlers — USE Memory.patchCode

### Infinite Hang

1. Check `AsyncUniTaskMethodBuilder.SetException` — any exceptions?
2. Check FSM polling — which state is stuck?
3. Check MoveNext counts — which await never returns?
4. Check for NULL returns from data accessors

### Wrong Return Values

Remember struct ABI:
- `UniTask` (void): only x0
- `UniTask<bool>`: x0=result, x1=source
- `UniTask<int>`: x0=result, x1=source

---

## Reference: Key Files

- `client/il2cpp_dump/dump.cs` — All C# signatures, RVAs, offsets
- `docs/PROGRESS.md` — Current state, what works/doesn't
- `frida/hooks.js` — Production hooks
- `~/.cursor/projects/*/terminals/*.txt` — Live log output

---

## Summary: Decision Tree

```
Need to intercept method?
├── Is it MoveNext? → NO HOOK (use tracing via other methods)
├── Is it FSM.Setup chain? → NO HOOK (corrupts MethodInfo*)
├── Is it FSM handler? → Memory.patchCode (safest)
├── Is it generic virtual? → Careful (MethodInfo* issue)
└── Regular instance method? → Interceptor.attach (safe)

Need to return early?
├── Void UniTask? → mov x0, #0; ret (8 bytes)
├── UniTask<bool>? → mov x0, #1; mov x1, #0; ret (12 bytes)
└── Complex struct? → Interceptor.attach onLeave, modify retval + x1

Need to trace flow?
├── Hook entry/exit of parent methods
├── Disassemble to find BL targets
├── Add logging to onLeave
└── NEVER hook MoveNext directly
```
