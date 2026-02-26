# Lunar Tear — Private Server for NieR Re[in]carnation

## Goal
Minimal client-side Frida patches + full server implementation.  
End state: patch host + rebuild APK, everything else handled server-side.

## Architecture

```
Android Emulator (game APK)
  ├── Frida hooks (frida/hooks.js) — DNS redirect, SSL bypass, minimal flow fixes
  ├── gRPC server (server/, port 7777) — UserService, DataService
  └── HTTP server (server/, port 8080) — Octo assets, TOS/privacy pages
```

## What Works

| Component | Status |
|-----------|--------|
| DNS redirect (getaddrinfo hook) | ✅ |
| HTTPS→HTTP downgrade (UnityWebRequest) | ✅ |
| SSL/TLS bypass (ToNativeCredentials→null) | ✅ |
| Encryption bypass (Encrypt/Decrypt passthrough) | ✅ |
| gRPC UserService (GetAndroidArgs, Auth) | ✅ |
| gRPC DataService (GetLatestMasterDataVersion, GetUserDataNameV2, GetUserData) | ✅ |
| Octo asset server (database.bin.e) | ✅ |
| MasterData download + decrypt | ✅ |
| TOS/Privacy HTML pages | ✅ |
| TOS + Age verification dialogs displayed and accepted | ✅ |
| FSM progression: OnFirstStep → OnTitleScreen → OnApplicationVersion → OnBanAccount → OnTermOfService | ✅ |
| OnTermOfService internal flow (network init, auth, master data, user data) | ✅ |

## Current Blocker: FSM Transition After OnTermOfService

### Symptom
Game loads to ~40%, OnTermOfService async state machine completes (state=-2), but the FSM never transitions to OnFirstDownload. Screen stays frozen at 40%.

### Root Cause Analysis

1. **OnTermOfService completes but does NOT call RequestUpdate(7)** — the async method finishes without queuing the next FSM event. This is likely because our patches (InitializeAssetBundles, LoadTextData returning instant) cause the method to take a code path that skips the RequestUpdate call.

2. **DoUpdate.MoveNext stuck at state=0** — the FSM's polling loop enters state=0 (callback-based await pattern) and never advances because no event is queued.

3. **Manual RequestUpdate(7) was crashing** with `access violation accessing 0x18`. Root cause: IL2CPP methods require 3 args `(this, arg, MethodInfo*)` but we were only passing 2. **Fixed** by capturing MethodInfo* from a legitimate RequestUpdate(8) call.

4. **RequestUpdate(7) now succeeds** (no crash) but produces no visible FSM transition. Possible causes:
   - Reentrancy: called from within TOS MoveNext onLeave (inside FSM update loop)
   - Event queued but DoUpdate needs another cycle to process it
   - The event=7 is correct but FSM routing doesn't map it to OnFirstDownload in current state

### Vectors for Next Steps

**Vector A — Fix reentrancy:** Call RequestUpdate(7) via `setTimeout(fn, 0)` from the game's main thread (not from within the hook). Previous setTimeout attempts used wrong pointer (Title instead of FSM) and missing MethodInfo. Retry with correct args.

**Vector B — Understand why OnTermOfService doesn't call RequestUpdate naturally:** Disassemble OnTermOfService state=12 code to find the branch that should call RequestUpdate and why it's being skipped. Could be a condition check on data loaded by InitializeAssetBundles or LoadTextData (which we patched to no-ops).

**Vector C — Remove InitializeAssetBundles and LoadTextData patches:** Let these methods run naturally against our server. This requires the server to properly serve asset bundles and text data. Most correct long-term approach but requires significant server work.

**Vector D — Patch OnTermOfService directly:** ARM64 patch at state=12 exit to inject the RequestUpdate call. Fragile but immediate.

### Recommended Priority
B → A → C (long-term) → D (last resort)

## Key Technical Details

### FSM Event Map (observed)
```
event=1 → OnFirstStep
event=2 → (system FSM, separate instance at 0x7385b106c0)
event=3 → transition within OnFirstStep flow
event=4 → OnApplicationVersion  
event=5 → OnBanAccount
event=7 → OnFirstDownload (CheckFirstDownload) — NEVER REACHED
event=8 → OnTermOfService
```

### Two FSM Instances
- `0x7385b106c0` — system-level FSM (events 2, 4)
- `0x6e...` (varies per run) — Title FSM (events 1, 3, 4, 5, 8, 7)

### IL2CPP Calling Convention
All IL2CPP methods: `ReturnType Method(this*, arg1, arg2, ..., MethodInfo*)`  
MethodInfo* is the LAST argument. Some methods access it (generic methods, virtual calls). Always capture and pass it.

### Key RVAs (in libil2cpp.so)
```
Title.OnTermOfService              0x30A8B68
Title.<OnTermOfService>d__40.MoveNext  0x28F99A8
Title.OnFirstDownload              0x30A9350
Title.<OnFirstDownload>d__29.MoveNext  0x28F8278
Title.InitializeAssetBundles       0x30A94DC (PATCHED → instant UniTask)
Title.LoadTextData                 0x30A9B80 (PATCHED → instant UniTask<bool>(true))
Title.FetchTermsOfServiceVersion   0x30A9AAC (PATCHED → instant UniTask<int>(1))
FSM.DoUpdate.MoveNext              0x423B594
FSM.UpdateEvent.MoveNext           0x423C018
FSM.RequestUpdate                  0x423D24C
OctoManager.StartDbUpdate          0x4C041B8 (hookReplace, invokes callback)
IsValidTermOfService               0x28FFAF8 (hooked → force return 1)
```

### OnTermOfService State Machine States
```
-1 → 1:  init, IsValidTermOfService check
 1 → 2:  OnTermOfServiceAdditionalWorldWideAsync (age/ads dialogs)
 2 → 3:  dialog wait
 3 → 4:  FetchTermsOfServiceVersion (patched instant)
 4 → 4:  polling fetch (~20 iterations)
 4 → 5:  InitializeNetworkAsync
 5 → 10: auth (GetAndroidArgs, Auth gRPC)
10 → 12: InitializeAssetBundles, LoadTextData, MasterData, UserData
12 → -2: COMPLETES (but no RequestUpdate called!)
```

## File Locations
- `frida/hooks.js` — all Frida hooks and patches
- `server/internal/service/octo.go` — HTTP server (assets, TOS pages)
- `server/internal/service/user.go` — gRPC UserService
- `server/internal/service/data.go` — gRPC DataService
- `server/cmd/lunar-tear/main.go` — server entrypoint
