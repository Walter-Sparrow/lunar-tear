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
| **WaitCompletionScene unblock (force flag at Gameplay+0x158)** | **OK** |
| **IsNeedsChapterAssetDownload forced false** | **OK** |
| **OnMainStoryAsync full bypass (Memory.patchCode → completed UniTask)** | **OK** (but OnTitleAsync stuck) |
| **PlayTitleFlowMovieAsync bypass (Memory.patchCode → completed UniTask)** | **OK** |
| **OnTitleAsync.MoveNext disassembly (309 insns, all BL targets mapped)** | **DONE** |
| **AsyncUniTaskMethodBuilder.SetException tracing (zero exceptions)** | **DONE** |
| **DiffUserData plain JSON format (quest.go fix)** | **OK** |
| **ARM64 BL disassembly of ApplyNewestScene call tree (4 methods)** | **DONE** |
| **Runtime tracing hooks for all Story sub-methods** | **DONE** |
| **Live tracing revealed NULL ActivePlayerToEntityMainQuestStatus** | **CRITICAL FINDING** |
| **Complete call chain mapped via runtime logs** | **DONE** |

## Strategic Reassessment: Too Many Patches, Wrong Focus

### The Problem with Current Approach
We've spent extensive time trying to "heal" the game through Frida hooks — forcing completions, bypassing methods, patching return values. This is **not sustainable** for the actual goal:

**Goal**: Minimal APK patches (host redirect only) + server implementation → working home screen  
**Current**: Dozens of Frida hooks, deep reverse engineering of FSM internals, trying to make broken async flows work

### Reality Check
1. **Frida is dev-time only** — we cannot ship Frida with the APK
2. **Every patch is technical debt** — harder to maintain, breaks with updates
3. **We're debugging the wrong layer** — instead of understanding what the server *should* provide, we're patching client symptoms

### Root Cause Analysis (Correct)
The `ActivePlayerToEntityMainQuestStatus() -> NULL` issue isn't about data format — **server returns correct JSON**. The real issue:
- `SyncUserData` Task→UniTask bridge is broken (client-side async infrastructure)
- `DarkUserDatabaseBuilder` never builds the database
- This is a **client-side bug with our setup**, not a server data issue

### Correct Path Forward
1. **Minimal APK patches only**:
   - Host redirect (getaddrinfo or URL patch)
   - SSL bypass (if needed)
   - That's it — no method patching, no FSM manipulation

2. **Server provides natural responses**:
   - Real master data (working ✓)
   - User data in format client expects (JSON, working ✓)
   - gRPC services that make sense for game flow

3. **Accept natural game flow**:
   - If game expects certain data → provide it
   - If game expects certain sequence → implement it
   - Don't bypass — implement

### What We Should NOT Be Doing
- ❌ Hooking Story.ApplyFirstScene/ApplyNewestScene
- ❌ Patching OnMainStoryAsync completion
- ❌ Forcing FSM flags
- ❌ Bypassing WaitCompletionScene
- ❌ Complex async state machine manipulation

### What We SHOULD Be Doing
- ✅ Understanding exact server→client contract
- ✅ Implementing proper QuestService responses
- ✅ Ensuring user data populates MemoryDatabase naturally
- ✅ Testing with minimal patches only

### Call Chain (Confirmed via Disassembly)
```
OnRunApplicationAsync (d__524):
  state 0-2: WaitInitializedScene, setup
  state 3: await OnTitleAsync → STUCK HERE (OnTitleAsync never completes)
  state 4: (never reached) → would call CreateAsyncTitleEndContents

OnTitleAsync (d__528) — disassembled, only direct BL calls:
  InitializeAudioAsync (0x2737488)
  PlaySplashAsync (0x274A5FC)
  InitializeUserStateAsync (0x274C5C8)
  CreateTouch (0x274BB80)
  RunTitle (0x274B29C)        ← runs Title FSM
  FSM.RequestUpdate (0x423D24C) ← enqueues MainStory event for Gameplay FSM
  SetResult (0x408C7D4)       ← SHOULD complete OnTitleAsync (in success path)
  
  NOTE: OnTitleAsync does NOT directly call OnMainStoryAsync or CreateAsyncTitleEndContents!
  OnMainStoryAsync is triggered by the FSM machinery via transition handlers.
  CreateAsyncTitleEndContents is called from OnRunApplicationAsync.
```

### What's Been Tried (THIS SESSION)

#### 1. DiffUserData format fix (DONE)
QuestService was returning MessagePack-encoded base64 in DiffUserData. Client expects plain JSON.
Fixed `quest.go` to use `fmt.Sprintf` with JSON format. **Confirmed working** (server logs show correct JSON).

#### 2. AsyncUniTaskMethodBuilder.SetException tracing (DONE)
Hooked RVA 0x408C594 to catch silent async exceptions. **Result: ZERO exceptions.** The hangs are NOT caused by unhandled exceptions.

#### 3. WaitCompletionScene unblock (DONE, partial success)
After quest RPCs complete, `OnMainStoryAsync` enters `WaitCompletionScene` which polls `CompletedWaitSceneRequestReplace` (Gameplay+0x158). Our battle bypass skips the scene conductor, so the flag is never set.
**Fix**: Force flag=true on OnMainStoryAsync.MoveNext count=2 (after quest RPCs return).
**Result**: Unblocked WaitCompletionScene, but revealed the next hang (DownloadChapterAsync).

#### 4. IsNeedsChapterAssetDownload bypass (DONE, partial success)
After WaitCompletionScene, OnMainStoryAsync calls `DownloadChapterAsync` because `IsNeedsChapterAssetDownload` (0x273C598) returns true. We don't serve chapter assets.
**Fix**: Force retval → false in onLeave hook.
**Result**: DownloadChapterAsync skips download, but OnMainStoryAsync hits YET ANOTHER hang (unknown await after download check). The story flow has too many sequential hangs to fix individually.

#### 5. OnMainStoryAsync full bypass via Memory.patchCode (DONE, OnTitleAsync stuck)
Patched OnMainStoryAsync (0x274E4D4) to return completed UniTask: `mov x0, #0; mov x1, #0; ret`.
Also patched PlayTitleFlowMovieAsync (0x274E580) same way.
**Result**: OnMainStoryAsync returns instantly but OnTitleAsync STILL stuck. Disassembly revealed OnTitleAsync doesn't call OnMainStoryAsync directly — the FSM does. The synchronous completion may break the FSM's event processing loop.

#### 6. OnMainStoryAsync bypass via Interceptor.replace (FAILED — Abort)
Tried replacing Memory.patchCode with `Interceptor.replace(addr, new NativeCallback(...))`.
**Result**: Game aborted on launch. Likely MethodInfo* corruption (known IL2CPP/Frida issue with virtual method dispatch).

#### 7. OnTitleAsync.MoveNext disassembly (DONE, key insight)
Scanned 309 instructions (0x4D4 bytes) from RVA 0x2886950.
Found all BL targets. **Key discovery**: OnTitleAsync only calls RunTitle, FSM.RequestUpdate, and SetResult — it does NOT call OnMainStoryAsync or CreateAsyncTitleEndContents.
**Bug found**: Initial ARM64 BL detection failed because JS bitwise ops use signed 32-bit. Fixed: use `(word >>> 26) === 0x25` for BL, `(word >>> 10) === 0x358FC0` for BLR.

### What Has Been Tried (LATEST SESSION)
8. **NOP RequestUpdate(StartMainStory) in OnTitleAsync** — Gameplay stayed in Title (gcs=5), OnTitleAsync completed, but CreateAsyncTitleEndContents NEVER called. Conclusion: CreateAsyncTitleEndContents is NOT the next step after OnTitleAsync; it's inside/after OnMainStoryAsync.
9. **Tail-call OnMainStoryAsync → CreateAsyncTitleEndContents** — CRASHED: MethodAccessException (IAwaiter.get_IsCompleted on Gameplay failed) + SIGSEGV. CreateAsyncTitleEndContents needs full OnMainStoryAsync context (Story FSM, quest flow, etc.). Cannot jump to it directly.
10. **OnMainStoryAsync natural flow (no bypass)** — WaitCompletionScene unblock via poller. **Bug**: poller only forced cwsr when gcs=4, but during OnMainStoryAsync we see gcs=5 (transition not committed). **Fix**: also force when gcs=5 && gns=4 (transitioning to MainStory). DownloadChapterAsync patched → return completed UniTask<bool>(true). **Next test**: verify flow reaches CreateAsyncTitleEndContents.

11. **Tracing run (2026-02-26)** — Added OnMainStoryAsync ENTER/LEAVE, WaitCompletionScene ENTER/LEAVE. **Result**: ApplyFirstScene + ApplyNewestScene both return **Failure(2)**. **WaitCompletionScene is NEVER called** — when Failure, the code takes a different path that never reaches WaitCompletionScene. NeedsNextPlayingQuest, RestartQuestAsync, BeginEventMap, EndEventMap, IsNeedsFinishedReturnToTitle, RunFinishedReturnTitleAsync — none fire. **Conclusion**: Blocker is upstream — OnMainStoryAsync blocks on a different await when Failure. Need to either (a) fix user data so ApplyNewestScene returns Playing, or (b) find and patch the Failure-path await.

12. **ApplyFirstScene/ApplyNewestScene patch: Failure→NotPlaying** — When return value is 2 (Failure), force 0 (NotPlaying). **Result: CRASH** — NullReferenceException + SIGSEGV. NotPlaying path also expects objects we don't have. Reverted.

13. **Disassembly of ApplyNewestScene call tree (2026-02-26)** — Frida ARM64 disassembler dumped BL targets for 4 methods. **Key discovery**: `ApplyNewestScene` does NOT call `ApplyPortalOrMainScene` or `ApplyNewestMainScene`. It only checks:
    - `IfNeedsApplyAutoPlaying()` → probably false (no auto-orbit state)
    - `ApplyNewestExtraScene()` → false (no extra quests)
    - `ApplyNewestBigHuntScene()` → false (no big hunt)
    - `ApplyNewestContentStoryScene()` → false (no content stories)
    - `ApplyNewestEventScene()` → false (no events)
    - If ALL return false → **returns Failure(2)** because no scene was applied.
    - `ApplyPortalOrMainScene` (which calls `ApplyNewestMainScene`) is NOT in the call chain.
    
    **Call tree mapped**:
    ```
    ApplyFirstScene (0x2785888):
      → ApplyNewestScene (0x27858E8)
      → SceneIdToQuestId (0x27859E0)
      → ApplyReplay (0x27826D0)
    
    ApplyNewestScene (0x27858E8): ← returns Failure(2)
      → ActivePlayerToEntityMainQuestStatus (0x2AB491C)
      → IfNeedsApplyAutoPlaying (0x2785A50)
      → ApplyNewestExtraScene (0x2785F48)
      → ApplyNewestBigHuntScene (0x2786058)
      → ApplyNewestContentStoryScene (0x2786230)
      → ApplyNewestEventScene (0x278631C)
      *** DOES NOT CALL ApplyPortalOrMainScene ***
    
    ApplyPortalOrMainScene (0x2786508): ← NEVER REACHED
      → ApplySideStory (0x2786560)
      → ApplyPortal (0x27868E8)
      → ApplyNewestMainScene (0x27869C4)
      → ApplyMainQuestRouteIdAndSeasonId (0x27865FC)
    
    ApplyNewestMainScene (0x27869C4): ← NEVER REACHED
      → InReplayedForMainStory (0x2786B18)
      → ApplyScene(sceneId, storyType) (0x2786B90)
      → ActivateMainStoryWithSceneId (0x2786C10)
      → ApplyReplay (0x27826D0)
    ```
    **Hypothesis**: `ApplyNewestScene` only handles "resume from interrupted quest" scenarios (extra/event/bighunt/contentstory). For fresh users with no in-progress quests, ALL sub-checks return false → Failure(2). The main quest path (`ApplyPortalOrMainScene`) is called from ELSEWHERE — probably from `OnMainStoryAsync.MoveNext` itself at a different state, not from `ApplyNewestScene`.
    
    Added comprehensive runtime tracing hooks for all sub-methods + `ActivePlayerToEntityMainQuestStatus` return value inspection. **Next**: run game with tracing to confirm which sub-methods fire and what values they return.

## Revised Plan: Minimal Viable Product

### Phase 1: Clean Slate (Next Session)
1. **Strip Frida hooks to minimum**:
   - Keep: DNS redirect, SSL bypass (for dev testing)
   - Remove: All Story/Gameplay/FSM hooks
   - Remove: All method bypasses
   
2. **Test natural flow with just server data**:
   - Does game reach home screen with just JSON data from server?
   - If no → identify ONE blocking point
   - Fix that ONE point on server side

### Phase 2: APK Patching Strategy
**For production APK (no Frida)**:
1. **libil2cpp.so patches** (static, done once):
   - Patch getaddrinfo to return our IP for nierreincarnation.com
   - Patch ToNativeCredentials to return null (SSL bypass)
   - Optional: Patch UnityWebRequest to downgrade HTTPS→HTTP

2. **No runtime hooks** — everything else works naturally

### Phase 3: Server Implementation
**Focus on what game actually needs** (not what we think it needs):

1. **Current working** (keep):
   - UserService (Auth, Register, GameStart)
   - DataService (MasterData version, UserData)
   - GamePlayService (CheckBeforeGamePlay)
   - QuestService (UpdateSceneProgress, StartQuest, FinishQuest)

2. **Investigate and fix**:
   - **QuestService.DiffUserData** — does client expect this after quest?
   - **Real user data format** — not JSON vs msgpack, but field names/structure
   - **What happens after FinishAutoOrbit?** — what service/screen comes next?

3. **Accept if game has bugs**:
   - If client's Task→UniTask is broken with our server → that's a client bug
   - We may need ONE minimal patch to fix that specific client issue
   - Not 50 patches to work around every symptom

### Success Criteria
- Game reaches home screen (Mama area)
- No Frida running (or minimal 2-3 hooks for dev only)
- Server provides natural responses
- APK has only host redirect + SSL patches

### CRITICAL FINDING: Data Format Tests (2026-02-27 Session)

**Test 1: Plain JSON (WORKS - data delivered to client)**
- Server returns: `[{"UserId":1001,"CurrentMainQuestRouteId":1,...}]`
- Client accepts and parses the data
- But `ActivePlayerToEntityMainQuestStatus() -> NULL`

**Test 2: MessagePack + Base64 (CRASH)**
- Server returns base64(msgpack(data))
- Client crashes immediately on `ActivePlayerToEntityMainQuestStatus` call
- Stack trace: SIGSEGV at il2cpp+0x2785368 (Story.ApplyFirstScene area)

**Conclusion**: Client expects JSON format, NOT msgpack. But even with correct JSON data, the `ActivePlayerToEntityMainQuestStatus()` accessor returns NULL. This means:
1. Data is received but not stored in MemoryDatabase properly, OR
2. Table key mismatch between server and client, OR  
3. Data is stored but accessor method fails to find it

**Root Cause Analysis:**
The `ActivePlayerToEntityMainQuestStatus` method calls:
```csharp
DatabaseDefine.User.EntityIUserMainQuestMainFlowStatusTable.FindByUserId(userId)
```

This requires:
1. `EntityIUserMainQuestMainFlowStatusTable` to be populated (from GetUserData response)
2. Table to have primary index on UserId (verified in dump.cs - yes)
3. UserId in request matches UserId in data (1001)

Server logs confirm data is sent with UserId=1001. Client must be failing to populate the table.

### CRITICAL FINDING: Tracing Run Results (2026-02-26 Session)

**Log Analysis from Live Tracing:**
```
[Story] ApplyNewestScene omitSideStory=0x0
[UserData] ActivePlayerToEntityMainQuestStatus -> NULL  ← CRITICAL!
[Story]   IfNeedsApplyAutoPlaying -> 0
[Story]   ApplyNewestExtraScene -> 0
[Story]   ApplyNewestBigHuntScene -> 0
[Story]   ApplyNewestContentStoryScene -> 0
[Story]   ApplyNewestEventScene -> 0
[Story]   ApplyPortalOrMainScene omitSideStory=0x0        ← CALLED INSIDE ApplyNewestScene!
[Story]     ApplySideStory -> 0
[Story]     ApplyPortal -> 0
[Story]     ApplyNewestMainScene called
[UserData] ActivePlayerToEntityMainQuestStatus -> NULL  ← AGAIN NULL!
[Story]     ApplyNewestMainScene -> 0
[Story]   ApplyPortalOrMainScene -> 2
[Story] ApplyNewestScene -> 0x2 (Failure)
[Story] ApplyFirstScene -> 0x2 (Failure)
[Story]   NeedsStampFirstChapter state=2 -> 1            ← TRUE despite Failure!
[Story]       ActivateMainStoryWithSceneId sceneId=2     ← FIRES!
[gRPC] QuestService/UpdateMainFlowSceneProgressAsync    ← OK
[gRPC] QuestService/StartMainQuestAsync ×2                ← OK
[gRPC] QuestService/FinishMainQuestAsync                ← OK
[gRPC] QuestService/FinishAutoOrbitAsync                ← OK
[gRPC] NotificationService/GetHeaderNotificationAsync   ← OK
[GP-POLL] gcs=5(Title) gns=4 giu=1 gdue=0 grue=5 cwsr=1 ← STUCK!
```

**Key Discoveries:**

1. **`ActivePlayerToEntityMainQuestStatus -> NULL`** — This is the ROOT CAUSE. The method reads `DatabaseDefine.User.EntityIUserMainQuestMainFlowStatusTable.FindByUserId(userId)` and returns NULL because the table is empty for this user.

2. **`ApplyPortalOrMainScene IS called from ApplyNewestScene`** — contrary to initial disassembly analysis. The call happens after all Extra/BigHunt/ContentStory/Event checks return false. So the flow is:
   ```
   ApplyNewestScene
     → Check all "extra" scene types → all false
     → ApplyPortalOrMainScene (if omitSideStory=false)
       → ApplySideStory → false
       → ApplyPortal → false
       → ApplyNewestMainScene
         → ActivePlayerToEntityMainQuestStatus → NULL
         → return false (no scene applied)
       → return Failure(2)
   ```

3. **`NeedsStampFirstChapter` returns true (1)** — despite Failure(2), this check passes! It must be checking different data (probably master data, not user data).

4. **`ActivateMainStoryWithSceneId(sceneId=2)` fires** — the game decides to activate scene 2 anyway, even though ApplyNewestScene returned Failure. This triggers the full quest flow (gRPC calls all succeed).

5. **FSM stuck**: `gcs=5(Title) gns=4` — Gameplay FSM is in Title (5), NextState=MainStory (4), but the transition never completes. `giu=1` (_inUpdate=true) suggests DoUpdate is still processing.

**Root Cause Theory:**
The Failure(2) from ApplyNewestScene is NORMAL for fresh users (no in-progress quests). The game should continue to home screen via the `NeedsStampFirstChapter` → `ActivateMainStoryWithSceneId` path. The hang is NOT caused by Failure(2), but by something else in the FSM transition after quest completion.

**Next Steps:**
1. Add `user_main_quest_main_flow_status` table with proper data (non-NULL routeId/sceneId)
2. Verify ActivePlayerToEntityMainQuestStatus returns valid struct
3. Check if ApplyNewestMainScene then returns true
4. If FSM still stuck, investigate post-quest FSM state (DoUpdate flags)

### Root Cause Analysis
The Gameplay FSM uses `FiniteStateMachineTask.DoUpdate.MoveNext` to process transitions. When OnTitleAsync calls `FSM.RequestUpdate`, the next event (MainStory) is enqueued. The FSM processes it asynchronously (via DoUpdate on the next frame tick). When OnMainStoryAsync is called by DoUpdate:
- **Normal flow**: OnMainStoryAsync runs asynchronously (takes many frames for quest + scenes). FSM DoUpdate awaits it. When complete, DoUpdate resumes, processes the result, and OnTitleAsync's continuation fires.
- **Patched flow**: OnMainStoryAsync returns instantly (completed UniTask). This may cause DoUpdate.MoveNext to complete synchronously in the same frame, which might confuse the FSM's internal bookkeeping (`_inUpdate`, `_doUpdateEvent` flags). Or the continuation routing breaks because the FSM expects async behavior.

We CANNOT hook DoUpdate.MoveNext (corrupts jump tables). This makes debugging the FSM's internal processing very difficult.

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
6. Result: game auto-completes quest 1 and advances past quest flow

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
    → WaitCompletionScene polling (CompletedWaitSceneRequestReplace at +0x158) → FORCED true
    → IsNeedsChapterAssetDownload → FORCED false
    → [unknown await #3 — OnMainStoryAsync has many sequential hangs]

  (WITH OnMainStoryAsync bypass: all of the above skipped, returns instantly)
  
  OnTitleAsync → calls FSM.RequestUpdate(MainStory) → triggers OnMainStoryAsync via FSM
              → OnMainStoryAsync completes (patched) → ??? → OnTitleAsync STUCK (count=5 never fires)
  
  OnRunApplicationAsync → state=3 awaiting OnTitleAsync → STUCK (state=4 never reached)
              → CreateAsyncTitleEndContents NEVER CALLED → no home screen
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
- **JS bitwise ops for ARM64 disassembly** — JS bitwise operators work on signed 32-bit integers. `(word & 0xFC000000)` produces a signed result, so comparison with `0x94000000` fails for BL instructions where bit 31 is set. Fix: use unsigned right shift `(word >>> 26) === 0x25` for BL, `(word >>> 10) === 0x358FC0` for BLR.

### UniTask Struct ABI (ARM64)
- **UniTask (void)**: 1 field — `IAwaiter source` at offset 0x0 (8 bytes). `null` source = completed.
  - Returned in `x0` register only. Set `x0=0` for completed.
- **UniTask\<T\>**: 2 fields — `T result` (returned in `x0`), `IAwaiter source` (returned in `x1`).
  - **UniTask\<bool\>**: `x0=1` (true), `x1=0` (completed) — 16 bytes, uses x0+x1.
  - For Memory.patchCode: `mov x0, #<result>; mov x1, #0; ret` (3 instructions, 12 bytes).
  - For Interceptor.attach onLeave: `retval.replace(ptr(<result>))` + `this.context.x1 = ptr(0)`.
- **CRITICAL**: `Interceptor.replace` with `NativeCallback` only controls `x0` (the return value). For 16-byte struct returns, `x1` is left uninitialized → corrupted. Use `Memory.patchCode` instead.

### OnTitleAsync Architecture (KEY DISCOVERY)
`OnTitleAsync` does NOT directly call `OnMainStoryAsync` or `CreateAsyncTitleEndContents`.
- `OnMainStoryAsync` is triggered by the Gameplay FSM's DoUpdate loop after `FSM.RequestUpdate` is called from OnTitleAsync.
- `CreateAsyncTitleEndContents` is called from `OnRunApplicationAsync` (parent) AFTER OnTitleAsync completes.
- The FSM processes transitions via `DoUpdate.MoveNext` → calls `OnMainStoryAsync` as a handler → when that completes, the FSM event loop completes → OnTitleAsync's awaiter resumes → SetResult → OnRunApplicationAsync resumes.
- **Implication**: If OnMainStoryAsync completes synchronously (patched), the FSM may not correctly route the completion back to OnTitleAsync. The FSM's internal flags (`_inUpdate`, `_doUpdateEvent`) expect async multi-frame behavior.

### CreateAsyncTitleEndContents Location (CONFIRMED)
- **CreateAsyncTitleEndContents is called from INSIDE OnMainStoryAsync**, after Story FSM, quest flow, WaitCompletionScene, etc.
- NOP RequestUpdate → OnTitleAsync completes but CreateAsyncTitleEndContents never fires (it's not in OnRunApplicationAsync's direct chain).
- Tail-call OnMainStoryAsync→CreateAsyncTitleEndContents → CRASH (needs full story context).

### Interceptor.replace vs Memory.patchCode for FSM Handlers
- **Memory.patchCode works** for functions called via FSM transitions (OnMainStoryAsync, PlayTitleFlowMovieAsync). The raw instruction patch doesn't involve Frida's trampoline/detour, so no MethodInfo* corruption.
- **Interceptor.replace CRASHES** (Aborted) for the same functions. The NativeCallback detour corrupts the hidden MethodInfo* parameter that the FSM's virtual dispatch relies on.
- **Rule**: For FSM handler methods, always use Memory.patchCode. Reserve Interceptor.replace for standalone functions not called via generic virtual dispatch.

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
| OnMainStoryAsync | 0x274E4D4 | Memory.patchCode → mov x0,#0; mov x1,#0; ret (completed UniTask) |
| PlayTitleFlowMovieAsync | 0x274E580 | Memory.patchCode → mov x0,#0; mov x1,#0; ret (completed UniTask) |
| IsNeedsChapterAssetDownload | 0x273C598 | Interceptor.attach onLeave: retval.replace(ptr(0)) → false |
| CompletedWaitSceneRequestReplace | Gameplay+0x158 | Force=true on OnMainStoryAsync.MoveNext count=2 |
| ShowDialogQuestRetry | 0x304953C | Return false (retire) — safety fallback for quest retry dialog |
| CalculatorQuest.StartQuest | 0x27276A4 | NativeCallback returning completed UniTask — skips battle scene |

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
| OnMainStoryAsync (Interceptor.replace) | 0x274E4D4 | ABORT on launch — MethodInfo* corruption with NativeCallback on FSM handler |
| PlayTitleFlowMovieAsync (Interceptor.replace) | 0x274E580 | Same — Interceptor.replace crashes on FSM-dispatched methods |

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
Gameplay.<OnRunApplicationAsync>d__524.MoveNext 0x288649C
Gameplay.<OnTitleAsync>d__528.MoveNext     0x2886950 (size=0x4D4, 309 insns)
Gameplay.PlayTitleFlowMovieAsync           0x274E580 (PATCHED)
Gameplay.IsNeedsChapterAssetDownload       0x273C598 (PATCHED → false)
Gameplay.CompletedWaitSceneRequestReplace  offset 0x158 (bool)
Gameplay.InitializeAudioAsync              0x2737488
Gameplay.PlaySplashAsync                   0x274A5FC
Gameplay.InitializeUserStateAsync          0x274C5C8
Gameplay.CreateTouch                       0x274BB80
CalculatorQuest.StartQuest                 0x27276A4 (PATCHED → skip battle)
DialogHelper.ShowDialogQuestRetry          0x304953C (PATCHED → false)
AsyncUniTaskMethodBuilder.SetException     0x408C594
AsyncUniTaskMethodBuilder.SetResult        0x408C7D4
StateMachine.SetupStateMachine             0x2AA440C
Story.Generate                            0x2788E28 (NO HOOK — calls FSM.Setup)
Story.InternalInitialize                  0x2788FD4 (NO HOOK)
Story.SetupTransitions                    0x2789000 (NO HOOK)
Story.ApplyFirstScene                     0x2785888
Story.ApplyNewestScene                    0x27858E8
Story.ApplyPortalOrMainScene              0x2786508
Story.IfNeedsApplyAutoPlaying             0x2785A50
Story.ApplyNewestExtraScene               0x2785F48
Story.ApplyNewestBigHuntScene             0x2786058
Story.ApplyNewestContentStoryScene        0x2786230
Story.ApplyNewestEventScene               0x278631C
Story.ApplySideStory                      0x2786560
Story.ApplyMainQuestRouteIdAndSeasonId    0x27865FC
Story.ApplyPortal                         0x27868E8
Story.ApplyNewestMainScene                0x27869C4
Story.InReplayedForMainStory              0x2786B18
Story.ApplyScene(sceneId, storyType)      0x2786B90
Story.ActivateMainStoryWithSceneId        0x2786C10
Story.NeedsStampFirstChapter              0x2785788
Story.SceneIdToQuestId                    0x27859E0
Story.ApplyReplay                         0x27826D0
ActivePlayerToEntityMainQuestStatus       0x2AB491C
ActivePlayerToEntityReplayFlowStatus      0x2AB4CA0
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
