# Lunar Tear — Private Server for NieR Re[in]carnation

## Goal
Minimal client-side patches + full server implementation.  
End state: repackaged APK with host redirect → connect to our server. No Frida on real devices.

## Architecture

```
Android Device/Emulator (patched APK)
  ├── Frida hooks (frida/hooks.js) — dev-time only, for debugging
  ├── gRPC server (server/, port 7777) — UserService, DataService, GamePlayService, QuestService, NotificationService
  ├── HTTP server (server/, port 8080/80) — Octo assets, TOS/privacy pages
  └── HTTPS server (server/, port 443) — master data, web assets
```

## What Works

| Component | Status |
|-----------|--------|
| DNS redirect (getaddrinfo hook) | OK |
| HTTPS→HTTP downgrade (UnityWebRequest) | OK |
| SSL/TLS bypass (ToNativeCredentials→null) | OK |
| Encryption bypass (Encrypt/Decrypt passthrough for gRPC) | OK |
| gRPC UserService (GetAndroidArgs, Auth, RegisterUser, GameStart, SetUserName) | OK |
| gRPC DataService (GetLatestMasterDataVersion, GetUserDataNameV2, GetUserData) | OK |
| gRPC GamePlayService (CheckBeforeGamePlay) | OK |
| gRPC QuestService (UpdateMainFlowSceneProgress, StartMainQuest, FinishMainQuest, FinishAutoOrbit) | OK |
| gRPC NotificationService (GetHeaderNotification) | OK |
| Octo v2 asset server (/v2/pub/a/.../list/0 → empty protobuf) | OK |
| Octo AES DecryptAes bypass (dynamic memory scan) | OK |
| MasterData download (database.bin.e via CDN, HEAD+GET) | OK |
| MasterMemory binary parsing (MessagePack map16 format, 607 tables) | OK |
| DecryptMasterData natural (native AES decryption of real archive.org data) | OK |
| DarkClient deadline extension (0s→300s via Frida) | OK |
| TOS/Privacy HTML pages | OK |
| TOS + Age verification dialogs displayed and accepted | OK |
| SyncMasterData — natural flow (gRPC + CDN download + parse) | OK |
| OnRegistUserName bypass → event=10 + instant complete | OK (PATCHED) |
| OnGraphicQualitySetting bypass → event=11 + instant complete | OK (PATCHED) |
| OnFinish → natural flow (server GamePlayService responds OK) | OK |
| **Full Title FSM chain (all 11 states → IsCompleted=true)** | **OK** |
| **Gameplay FSM full flow: StartGameplay → OnTitle → RunTitle → OnMainStory** | **OK** |
| **Story FSM initialization (Generate → Setup → InternalInitialize)** | **OK** (no Frida hooks!) |
| **Real master data (archive.org, 607 tables, native AES decrypt)** | **OK** |
| **Story.ApplyFirstScene + ApplyNewestScene called** | **OK** (crashes if hooked) |
| **il2cpp_raise_exception hook** | **OK** (instance method API fix) |
| **SyncUserData hybrid bypass (real load + forced completion)** | **OK** |
| **Full Story FSM quest flow (UpdateSceneProgress → StartQuest → FinishQuest → FinishAutoOrbit)** | **OK** |
| **Battle bypass (CalculatorQuest.StartQuest replaced → skip to FinishMainQuest)** | **OK** |
| **Asset download / loading screen reached (33%→60%+)** | **REACHED** |

## Current State: Asset Download / Outgame Loading

### What's Happening
After the full quest flow completes successfully, the game enters an asset download/loading phase.
The loading bar shows progress (33.3% → 60%+), suggesting the game is downloading or loading
asset bundles for the outgame (home screen / Mama area).

### Boot Flow (Confirmed Working)
```
Title FSM → all 11 states complete
Gameplay FSM → OnMainStoryAsync
Story FSM → UpdateMainFlowSceneProgress(sceneId=2)
           → StartMainQuest(questId=1, isMainFlow=true) × 2
           → FinishMainQuest(questId=1, storySkipType=1)
           → FinishAutoOrbit
           → NotificationService/GetHeaderNotification
           → Asset download / loading screen
```

### Next Steps
1. Wait for loading to complete and see what happens next
2. If new gRPC calls fail, add missing service stubs
3. If asset downloads fail (Octo CDN), may need to provide asset bundles or skip

## Previous Blocker (RESOLVED): QuestService + Battle Scene Loading

### Problem
After Title FSM → Story FSM, the game called `QuestService/UpdateMainFlowSceneProgressAsync`
which failed with "unknown service". After implementing QuestService, `StartMainQuest` succeeded
but the game tried to load battle scene assets (which we don't have), showing a retry dialog.

### Fix
1. Implemented `QuestService` with RPCs: UpdateMainFlowSceneProgress, StartMainQuest, FinishMainQuest, FinishAutoOrbit, SetQuestSceneChoice
2. Registered under `apb.api.quest.QuestService` namespace (client expects full package path)
3. Replaced `CalculatorQuest.StartQuest` (RVA 0x27276A4) with NativeCallback returning completed UniTask — skips battle scene loading entirely
4. Patched `ShowDialogQuestRetry` (RVA 0x304953C) to return false (retire) as safety fallback
5. Added `NotificationService` with `GetHeaderNotification` stub
6. Result: game auto-completes quest 1 and advances to loading screen

## Previous Blocker (RESOLVED): SyncUserData Completion Chain

### Root Cause (CONFIRMED — NOT exceptions)
`il2cpp_raise_exception` hook now works (fix: use `Process.getModuleByName().findExportByName()` instead of broken static `Module.findExportByName`). **No managed exceptions thrown** during SyncUserData — eliminates the "silent exception" hypothesis.

The UserDataGet pipeline completes fully:
1. `DarkUserDatabaseBuilder` created ✓
2. `BuildDB` lambda fires (DB bytes built) ✓
3. `b__11_1` success continuation fires ✓
4. BUT `HandleSuccess.Invoke` never fires
5. The Task from `RequestAsync` never signals completion to the UniTask awaiter

**Actual root cause**: Task→UniTask async bridge is broken. The C# `Task` completes but the `UniTask` wrapper in `SyncUserData` never receives the completion signal.

### Workaround (WORKING)
Hybrid `Interceptor.attach` on `SyncUserData` (RVA `0x30A8940`):
- `onEnter`: original function runs (starts data pipeline — gRPC calls fire in background)
- `onLeave`: override `x0=1` (result=true), `this.context.x1=ptr(0)` (source=null → completed UniTask)
- FSM advances immediately while user data loads in background

### il2cpp_raise_exception Hook Fix
`Module.findExportByName("libil2cpp.so", "il2cpp_raise_exception")` threw "TypeError: not a function" in Frida 17.x. The STATIC `Module.findExportByName` is broken. Fix: use instance method:
```javascript
const il2cppMod = Process.getModuleByName("libil2cpp.so");
const raiseEx = il2cppMod.findExportByName("il2cpp_raise_exception");
```

### Previous Blocker (FIXED — older): Hook Installation Chain Broken
The `il2cpp_raise_exception` IIFE inside the `awaitLibil2cpp` callback threw an exception (TypeError: not a function), which was silently caught by the outer try/catch. Since `globalThis._hooksInstalled` was already set to true, the retry mechanism returned immediately, leaving critical hooks uninstalled:
- **PATCH1** (tbnz force TRUE) — never applied
- **URL downgrade** hooks (HTTPS→HTTP for UnityWebRequest) — never installed
- **DNS redirect** (getaddrinfo hook) — never installed

Without URL/DNS hooks, the master data download (`database.bin.e`) failed, and without PATCH1, the error dialog "Failed to connect. Retrying." appeared.

**Fix**: Wrapped the `il2cpp_raise_exception` hook in try/catch so failures don't abort the callback. Also removed dangerous MoveNext hooks (`UserDataGet.RequestAsync.MoveNext`, `OnFirstDownload.MoveNext`) that broke async completion chains.

### Previous Blocker (RESOLVED — see above): SyncUserData Completion Chain Broken
The natural `SyncUserData` flow completes the data pipeline fully (BuildDB + success continuation fire), but the Task→UniTask async bridge is broken. **Not caused by exceptions** (confirmed via working `il2cpp_raise_exception` hook — zero exceptions thrown).

**Workaround**: Hybrid `Interceptor.attach` — original runs (data loads in background), onLeave overrides return to completed UniTask. `SyncPurchase` also bypassed (Unity IAP unavailable in emulator).

## Boot Sequence (fully traced)

```
Generator.OnEntrypoint()
Generator.SetupApiSystem()
StateMachine.SetupStateMachine()
  ├── HandleNet.Setup()
  └── Gameplay.Generate() + Setup()

Title.Generate(delegator)              ← standalone (NOT via CreateTitleAsync)
Title.Setup()

Title FSM: FirstStep → TitleScreen → ApplicationVersion → BanAccount →
  TermOfService (captures _titleInstance) →
    ├── InitializeNetworkAsync (auth, channel setup)
    ├── InitializeAssetBundles (Octo DB update + DecryptAes bypass)
    ├── SyncMasterDataAndUserData:
    │     ├── SyncMasterData:
    │     │   ├── gRPC DataService/GetLatestMasterDataVersionAsync → "20240404193219"
    │     │   ├── HTTP HEAD+GET database.bin.e (DNS redirect + HTTPS→HTTP downgrade)
    │     │   └── DecryptMasterData (NATURAL — native AES decrypt of real archive.org data)
    │     │   └── DarkMasterMemoryDatabase.ctor parses 607 tables
    │     ├── SyncUserData: *** HANGS HERE *** (gRPC succeeds, DB built, completion never fires)
    │     │   Alternatively: BYPASSED (instant true — see workaround below)
    │     └── SyncPurchase: BYPASSED (instant true — no Unity IAP in emulator)
    └── natural flow (no PATCH1 needed — sync succeeds with URL/DNS hooks)
  → FirstDownload → RegistUserName(PATCHED) →
  GraphicQualitySetting(PATCHED) → Finish (natural, CheckBeforeGamePlay OK)

Title.IsCompleted = true
FSM-POLL: cs=11(Finish) ns=11 ft=0 iu=0 due=0 rue=11 ic=1

Gameplay FSM detects completion:
  Gameplay.StartGameplayStateMachine()
  Gameplay.GetFirstGameplayEvent() → 4 (StartGameplay)
  Gameplay.OnRunApplicationAsync()
  Gameplay.OnTitleAsync()          ← waits for Title.IsCompleted
  Gameplay.RunTitle()              ← Title FSM runs here
  Gameplay.OnMainStoryAsync()      ← transitions to MainStory
    Story.Generate()               ← creates Story FSM instance
    Story.InternalInitialize()     ← calls SetupTransitions(), sets CurrentSeasonId
    FSM.Setup()                    ← Story FSM now running (no crash!)
    Story.ApplyFirstScene()        ← called (confirmed, crashes if hooked)
    Story.ApplyNewestScene()       ← called (confirmed, crashes if hooked)
    gRPC >>> QuestService/UpdateMainFlowSceneProgressAsync(sceneId=2)    → OK
    gRPC >>> QuestService/StartMainQuestAsync(questId=1, isMainFlow=true) × 2  → OK
    CalculatorQuest.StartQuest → BYPASSED (battle skip, return completed UniTask)
    gRPC >>> QuestService/FinishMainQuestAsync(questId=1, storySkipType=1)  → OK
    gRPC >>> QuestService/FinishAutoOrbitAsync                               → OK
    gRPC >>> NotificationService/GetHeaderNotificationAsync                  → OK
    → Asset download / loading screen (33% → 60%+)
```

## Key Findings

### Master Data Format (CONFIRMED — 607 tables)
- `database.bin.e` is **MasterMemory** format (MessagePack), NOT SQLite
- Source: nier-rein-apps (`DarkMasterMemoryDatabase : MemoryDatabaseBase`)
- **Binary format**: starts directly with a MessagePack map (`0xde` = map16 for 607 tables)
  - NOT prefixed with a 4-byte header length (previous assumption was wrong)
  - `dataStart.readU8()` == `0xde` (map16), next 2 bytes = big-endian count (607 = `0x025F`)
- Real archive.org data decrypts to 7,801,092 bytes with 607 tables
- ~600 table properties in `DarkMasterMemoryDatabase` (EntityMAbilityTable, EntityMQuestTable, etc.)
- Constructor RVA: `MemoryDatabaseBase.ctor` = `0x3B5B7B0`

### DarkClient Deadline Issue (FIXED)
- `DarkClient` has a `deadline` field (TimeSpan, offset `0x30`) set in constructor
- Constructor: `.ctor(CancellationToken, Nullable<TimeSpan> timeout, INetworkInterceptor[])`
- For UserService calls: deadline = 10s (enough for quick gRPC calls)
- For DataService calls after master data download: deadline = **0s** (instant timeout!)
- Root cause: the DarkClient created for post-sync gRPC calls gets `TimeSpan.Zero` as timeout
- Fix: Frida hook on `DarkClient.ctor` (RVA `0x27A3134`) extends deadline to 300s in onLeave

### Frida Limitations with IL2CPP
- **Interceptor.attach on MoveNext** — NEVER do this. Async state machine MoveNext methods use jump tables (computed branches) that Frida's trampoline insertion corrupts. This was the cause of both FSM.DoUpdate/UpdateEvent issues AND the SyncMasterData completion chain breakage.
- **Interceptor.attach on small/ADRP functions** — dangerous if first instructions contain PC-relative addressing (ADRP, ADR, B.cond).
- **Interceptor.attach on shared generic methods** — the hidden MethodInfo* parameter (x1→x20) is corrupted by the trampoline, causing crashes for specific generic instantiations.
- **Interceptor.attach ANYWHERE in FSM.Setup call chain** — ANY hook on functions called from or calling FSM.Setup (including Story.Generate→FSM.Setup→InternalInitialize→SetupTransitions) corrupts the MethodInfo* register. The abort() at post-InternalInitialize dereference chain: `x20(MethodInfo*)→+0x18(rgctx)→+0xc0→+8→vtable→call`.
- **NativeCallback UniTask return** — only controls x0 register. For void UniTask, set both x0=0, x1=0 via Memory.patchCode.
- **Module.findExportByName (static)** — throws "TypeError: not a function" in Frida 17.x. Use instance method: `Process.getModuleByName("libil2cpp.so").findExportByName("...")`.
- **UniTask struct return override** — UniTask<bool> returns in (x0=result, x1=source). Use `Interceptor.attach` onLeave with `retval.replace(ptr(1))` + `this.context.x1 = ptr(0)` for forced completion.
- **BL to Interceptor'd targets** — crash if MethodInfo* missing.

### Frida Script Re-evaluation Guard (FIXED)
- `awaitLibil2cpp()` was called up to **92 times** per launch — each `Interceptor.attach` stacked, causing 92x callback overhead per function call
- Root cause: module-scoped `let hooksInstalled = false` resets on each script re-evaluation by Frida
- Fix: use `globalThis._hooksInstalled` (persists across re-evaluations in same Frida agent)
- Same fix applied to `octoAesBypassed` → `globalThis._octoAesBypassed`
- After fix: 2 installations (acceptable, down from 92)

### Sync MoveNext Hooks Break Completion Chain
`Interceptor.attach` on `SyncMasterDataAndUserData.MoveNext`, `SyncMasterData.MoveNext`, `SyncUserData.MoveNext` prevented `OnTermOfService.MoveNext` from being resumed after sync. Removing these hooks fixed the Title FSM stall at TOS state.

### User Data Format (PARTIALLY WORKING)
- **Wire format**: gRPC `UserDataGetResponse` has `map<string, string> user_data_json` (field 1) where key = table name, value = JSON string
- **Client-side**: `FetchRecordMethod` returns `Task<List<Dictionary<string, object>>>` — client expects JSON arrays of objects
- **Current format**: plain JSON with named keys (`{"UserId":1001,"PlayerId":1001,...}`) → gRPC succeeds, DB builds, pipeline completes
- **Status**: data loads successfully (confirmed via diagnostic hooks on DarkUserDatabaseBuilder + BuildDB lambda), async completion is the only issue (worked around via hybrid bypass)
- **Open question**: whether DB content is correctly populated (0-table vs populated) — Story FSM reaches QuestService call regardless, suggesting it reads enough data to proceed

## Title FSM Flow (COMPLETE — all states traced)

```
TitleEvent → TitleState → Handler
─────────────────────────────────
Start(1)                 → FirstStep(1)        → OnFirstStep
StartPreApplication(2)   → InPreApplication(2)  (system FSM)
StartFormalApplication(3)→ TitleScreen(3)       → OnTitleScreen
CheckApplicationVersion(4)→ApplicationVersion(4)→ OnApplicationVersion
CheckBanAccount(5)       → BanAccount(5)        → OnBanAccount
CheckTermOfService(8)    → TermOfService(7)     → OnTermOfService
CheckFirstDownload(7)    → FirstDownload(8)     → OnFirstDownload
RegisterUserName(9)      → RegistUserName(9)    → OnRegistUserName (PATCHED)
CheckResolutionSetting(10)→ResolutionSetting(10)→ OnGraphicQualitySetting (PATCHED)
Completion(11)           → Finish(11)           → OnFinish (natural flow)
```

## Client Patches (current dev-time via Frida)

| Patch | RVA | What It Does |
|-------|-----|-------------|
| FetchTermsOfServiceVersion | 0x30A9AAC | Return UniTask\<int>(1) — skip HTTP parse bug |
| LoadTextData | 0x30A9B80 | Return UniTask\<bool>(true) — no text assets |
| OnRegistUserName | 0x30A97BC | Interceptor.replace → write event=10, return completed |
| OnGraphicQualitySetting | 0x30A9958 | Interceptor.replace → write event=11, return completed |
| ShowDialogEnterUserName | 0x304939C | Return UniTask\<bool>(true) — skip UI dialog |
| ShowDialogGraphicQualitySetting | 0x304946C | Return UniTask\<bool>(true) — skip UI dialog |
| SyncUserData | 0x30A8940 | Interceptor.attach onLeave: x0=1, x1=0 (hybrid: real load + forced completion) |
| SyncPurchase | 0x30A8A10 | Memory.patchCode → return UniTask\<bool>(true) instantly |
| DecryptMasterData | 0x2775A0C | Interceptor.attach (logging only) — native AES decryption runs |
| DarkClient.ctor deadline | 0x27A3134 | onLeave: extend deadline 0s→300s (field at +0x30) |

### Hooks REMOVED (proven dangerous)
| Hook | RVA | Why Removed |
|------|-----|-------------|
| FSM.DoUpdate.MoveNext | 0x423B594 | Corrupts async state machine jump tables |
| FSM.UpdateEvent.MoveNext | 0x423C018 | Corrupts async state machine jump tables |
| Title.OnComplete | 0x30A9260 | Interceptor.attach corrupts PC-relative ADRP instruction |
| FSM.Setup (shared generic) | 0x423D048 | Corrupts MethodInfo* hidden param for Story instantiation |
| Story.Generate | 0x2788E28 | Calls FSM.Setup internally → same MethodInfo* corruption |
| Story.InternalInitialize | 0x2788FD4 | Called FROM FSM.Setup → Frida trampoline corrupts caller's x20 |
| Story.SetupTransitions | 0x2789000 | Called from InternalInitialize inside FSM.Setup chain |
| SyncMasterDataAndUserData.MoveNext | 0x28FECBC | Corrupts async completion chain → TOS handler never resumes |
| SyncMasterData.MoveNext | 0x28FEA2C | Same — MoveNext hooks break completion callbacks |
| SyncUserData.MoveNext | 0x28FF204 | Same — MoveNext hooks break completion callbacks |
| SyncMasterDataAndUserData (bypass) | 0x30A8760 | REMOVED — let sync run naturally via gRPC + CDN |
| UserDataGet.RequestAsync.MoveNext | 0x2C5AFB4 | MoveNext hook breaks async completion chain |
| OnFirstDownload.MoveNext | 0x28F8278 | MoveNext hook breaks async completion chain |
| tbnz PATCH1 | 0x28fab60 | REMOVED — natural sync flow works with URL/DNS hooks |

## Class Hierarchy

```
Dark.Kernel.StateMachine.SetupStateMachine()   (RVA 0x2AA440C)
  ├── HandleNet : FiniteStateMachineTask<HandleNetState, HandleNetEvent>
  └── Gameplay : FiniteStateMachineTask<GameplayState, GameplayEvent>
        ├── _title : Title                    (offset 0x1E0)
        ├── CreateTitleAsync()                (RVA 0x274A8D4) — for re-entry, NOT cold boot
        └── GameplayState: Unknown=0, DevelopConfig=1, FirstStep=2,
            LockApplication=3, MainStory=4, Title=5

Title : FiniteStateMachineTask<TitleState, TitleEvent>
  ├── _isResistSuccess : bool           (offset 0x50)
  ├── IsCompleted : bool                (offset 0x51)
  ├── _delegator : IDelegator           (offset 0x58)  → TitleStubDelegator
  ├── _actForTitle : ActForTitle        (offset 0x60)
  ├── _loadedAssets : LoadedAssets      (offset 0x68)
  ├── Generate(IDelegator)              (RVA 0x30A9DC0) — standalone creation
  └── OnComplete() = RequestUpdate(Completion=11)  (RVA 0x30A9260, tail call)

Story : FiniteStateMachineTask<StoryState, StoryEvent>
  ├── StoryState: Unknown=0, Idle=1, Playing=2
  ├── StoryEvent: Unknown=0, CompleteStory=1, PlayLastStory=2, PlayStory=3, ...
  ├── Generate()                     (RVA 0x2788E28, calls FSM.Setup internally!)
  ├── InternalInitialize()           (RVA 0x2788FD4, calls SetupTransitions + sets CurrentSeasonId)
  ├── SetupTransitions()             (RVA 0x2789000)
  ├── ApplyFirstScene()              (RVA 0x2785888, needs master data)
  ├── CurrentSeasonId                (offset 0x5C)
  ├── CurrentChapterId               (offset 0x64)
  ├── _currentQuestId                (offset 0x68)
  ├── CurrentSceneId                 (offset 0x70)
  └── NO Interceptor.attach hooks allowed (corrupts FSM.Setup MethodInfo*)
```

## FiniteStateMachineTask Fields (from dump.cs)
```csharp
abstract class FiniteStateMachineTask<TState, TEvent> {
    TState CurrentState;                         // +0x10
    TState NextState;                            // +0x14
    TState FirstState;                           // +0x18
    List<TState> _states;                        // +0x20
    Dictionary<TEvent, TransactionEvent> _events;// +0x28
    List<TransactionContextAsync> _transitions;  // +0x30
    bool _firstTime;                             // +0x38
    bool _inUpdate;                              // +0x39
    bool _doUpdateEvent;                         // +0x3A
    TEvent _requestUpdateEvent;                  // +0x3C
    ITransactionUserData _requestUserData;       // +0x40
    CancellationTokenSource _cts;                // +0x48
}
```

## Key RVAs (libil2cpp.so)
```
Title.OnTermOfService                     0x30A9A00
Title.OnFirstDownload                     0x30A9350
Title.OnRegistUserName                    0x30A97BC (PATCHED)
Title.OnGraphicQualitySetting             0x30A9958 (PATCHED)
Title.OnFinish                            0x30A92B4 (natural flow)
Title.OnComplete                          0x30A9260 (= RequestUpdate(Completion=11))
Title.get_IsCompleted                     0x30A9DAC
Title.set_IsCompleted                     0x30A9DB4
Title.Generate                            0x30A9DC0
Title..ctor                               0x30A9E44
Title.SetupTransitions                    0x30A9F58
Title.FetchTermsOfServiceVersion          0x30A9AAC (PATCHED → v=1)
Title.LoadTextData                        0x30A9B80 (PATCHED → true)
Title.SyncMasterDataAndUserData           0x30A8760 (natural flow, no bypass)
Title.IsNeedGameStartApi                  0x30A9868
HandleNet.DecryptMasterData               0x2775A0C (PATCHED → pass-through)
HandleNet.DecryptMasterDataInternal       0x2775A1C
FSM.DoUpdate (Int32Enum)                  0x423D1D0
FSM.DoUpdate.MoveNext                     0x423B594 (NO HOOK)
FSM.UpdateEvent.MoveNext                  0x423C018 (NO HOOK)
FSM.RequestUpdate                         0x423D24C
FSM.Setup (shared generic)                0x423D048
Gameplay.Generate                         0x274DCB8
Gameplay.CreateTitleAsync                  0x274A8D4 (for re-entry only)
Gameplay.CreateTitleAsync.MoveNext         0x289588C
Gameplay.DisposeTitle                      0x274A77C
Gameplay.StartGameplayStateMachine         0x274E478
Gameplay.GetFirstGameplayEvent             0x274E780
Gameplay.OnRunApplicationAsync             0x274E634
Gameplay.OnTitleAsync                      0x274E788
Gameplay.OnMainStoryAsync                  0x274E4D4
Gameplay.RunTitle                          0x274B29C
Gameplay.<OnMainStoryAsync>d__522.MoveNext 0x2885CF8
StateMachine.SetupStateMachine             0x2AA440C
Story.Generate                            0x2788E28 (NO HOOK — calls FSM.Setup)
Story.InternalInitialize                  0x2788FD4 (NO HOOK)
Story.SetupTransitions                    0x2789000 (NO HOOK)
Story.ApplyFirstScene                     0x2785888
Story.ApplyNewestScene                    0x27858E8
Story.ApplyPortalOrMainScene              0x2786508
DialogHelper.ShowDialogEnterUserName      0x304939C (PATCHED → true)
DialogHelper.ShowDialogGraphicQualitySetting 0x304946C (PATCHED → true)
OctoManager.StartDbUpdate                 0x4C041B8
SceneManager.LoadSceneAsyncNameIndexInternal 0x4D4A45C
Generator.OnEntrypoint                    0x2E966A8
Generator.SetupApiSystem                  0x2E96DBC
MasterDataDownloader.DownloadAsync         0x32F0A3C
MemoryDatabaseBase.ctor                    0x3B5B7B0
MemoryDatabaseBase.ExtractTableData        0x3D0AA44 (shared generic)
DarkClient.ctor                            0x27A3134 (PATCHED → deadline 300s)
DarkClient.deadline                        offset 0x30 (TimeSpan, ticks)
DarkClient.InvokeAsync                     0x38743FC (shared generic)
```

## File Locations
- `frida/hooks.js` — all Frida hooks and patches
- `server/assets/20240404193219.bin.e` — master data file (MasterMemory/MessagePack format)
- `server/internal/service/octo.go` — HTTP server (assets, TOS pages, Octo v2)
- `server/internal/service/user.go` — gRPC UserService
- `server/internal/service/data.go` — gRPC DataService
- `server/internal/service/gameplay.go` — gRPC GamePlayService
- `server/cmd/lunar-tear/main.go` — server entrypoint
- `client/il2cpp_dump/dump.cs` — **full IL2CPP dump** (1.2M lines, all C# signatures + RVAs)
- `client/unpacked/` — unpacked APK (assets, lib, dex files)
- `docs/PROGRESS.md` — this file

## Reference Rule
**Always consult `client/il2cpp_dump/dump.cs` when unsure about any method, class, field, or RVA.**
Use Grep to search it — never guess when the dump is available.
