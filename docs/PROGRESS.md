# Lunar Tear — Private Server for NieR Re[in]carnation

## Goal
Minimal client-side patches + full server implementation.  
End state: repackaged APK with host redirect → connect to our server. No Frida on real devices.

## Architecture

```
Android Device/Emulator (patched APK)
  ├── Frida hooks (frida/hooks.js) — dev-time only, for debugging
  ├── gRPC server (server/, port 7777) — UserService, DataService
  ├── HTTP server (server/, port 8080/80) — Octo assets, TOS/privacy pages
  └── HTTPS server (server/, port 443) — master data, web assets
```

## What Works

| Component | Status |
|-----------|--------|
| DNS redirect (getaddrinfo hook) | OK |
| HTTPS→HTTP downgrade (UnityWebRequest) | OK |
| SSL/TLS bypass (ToNativeCredentials→null) | OK |
| Encryption bypass (Encrypt/Decrypt passthrough) | OK |
| gRPC UserService (GetAndroidArgs, Auth, RegisterUser) | OK |
| gRPC DataService (GetLatestMasterDataVersion, GetUserDataNameV2, GetUserData) | OK |
| Octo v2 asset server (/v2/pub/a/.../list/0 → empty protobuf) | OK |
| Octo AES DecryptAes bypass (dynamic memory scan) | OK |
| MasterData download + decrypt (database.bin.e) | OK |
| TOS/Privacy HTML pages | OK |
| TOS + Age verification dialogs displayed and accepted | OK |
| FSM: OnFirstStep → OnTitleScreen → OnApplicationVersion → OnBanAccount → OnTermOfService | OK |
| OnTermOfService internal flow (network init, auth, asset bundles) | OK |
| **SyncMasterDataAndUserData bypass → instant true** | **OK (PATCHED)** |
| **FSM transition: OnTermOfService → OnFirstDownload (RequestUpdate 7)** | **OK** |
| **OnFirstDownload → GetFirstDownloadSizeAsync** | **OK** |
| **FSM transition: OnFirstDownload → OnRegistUserName (RequestUpdate 9)** | **OK** |

## Current State: User Name Registration Screen

Game reaches `OnRegistUserName` — the user name input screen. This is the first interactive gameplay screen past the loading flow.

### How We Got Here

The blocker was `SyncMasterDataAndUserData` returning `false` to OnTermOfService state 12, which skipped `RequestUpdate(7)`. The fix: patch the wrapper function at RVA `0x30A8760` to return a completed `UniTask<bool>(true)` immediately (same pattern as LoadTextData and FetchTermsOfServiceVersion patches).

With this patch:
1. TOS state 12 gets `bool=TRUE` from the awaiter
2. `RequestUpdate(7)` fires naturally
3. FSM transitions to `OnFirstDownload`
4. `GetFirstDownloadSizeAsync` returns 0 (Octo empty list)
5. `RequestUpdate(9)` fires
6. FSM transitions to `OnRegistUserName`

### What the Patch Skips

The `SyncMasterDataAndUserData` patch skips:
- `SyncMasterData` — gRPC GetLatestMasterDataVersion, database.bin.e download
- `SyncUserData` — gRPC GetUserDataNameV2, GetUserData

Master data download ALSO happens in state 10-11 (InitializeAssetBundles, MasterData.DownloadAsync), so the database IS downloaded and decrypted. The sync step was loading it into runtime structures.

For production, need to either:
- Fix the server responses so SyncMasterDataAndUserData returns true naturally
- Or keep the binary patch (static patch in libil2cpp.so)

## Resolved: Previous Blocker (State 12 Branch)

### Root Cause (CONFIRMED)

`SyncMasterDataAndUserData` returned `false` despite both `SyncMasterData` and `SyncUserData` completing with `isError=0`. The actual bool result propagated through the `UniTask<bool>` was FALSE, likely due to an additional validation in the state machine body beyond the isError flags.

### What We Tried Before the Fix

| Approach | Result | Why It Failed |
|----------|--------|---------------|
| Binary patch tbnz→b at 0x28fab60 | Bytes in memory but never executed | Mid-function patches unreliable with Frida Interceptor |
| Binary patch mov w0,#1; b (8 bytes) | Same | Same |
| Remove Interceptor.attach, patch only | Patch still not executed | Issue NOT just Interceptor trampoline |
| NativeFunction RequestUpdate(7) from JS | Call succeeds, FSM fields updated | DoUpdate/UpdateEvent loop dead |
| Mid-function Interceptor.attach probes | Probes never fire | Frida limitation |
| **Patch SyncMasterDataAndUserData wrapper → true** | **SUCCESS** | **Bypasses the broken bool entirely** |

### Key Insight

Mid-function `Memory.patchCode` patches inside async state machine `MoveNext` methods are unreliable in the Frida environment. Patching function ENTRY points (entire wrapper replacement) works reliably. All three working patches (LoadTextData, FetchTermsOfServiceVersion, SyncMasterDataAndUserData) replace function entry with `mov x0, #1; mov x1, #0; ret`.

## Next Steps

### Step 1: Handle OnRegistUserName
The game is at the user name registration screen. Need to implement the gRPC endpoint for user name registration and see what the next FSM states require.

### Step 2: Server-first fix for SyncMasterDataAndUserData
Investigate why the server responses cause a FALSE result despite isError=0. Disassemble `SyncMasterDataAndUserData.MoveNext` body to find the exact bool computation. Fix server responses so the sync returns true naturally.

### Step 3: Continue FSM progression
After OnRegistUserName, the Title FSM has more states (OnGraphicQualitySetting, OnFinish, etc.). Map and implement each.

### Step 4: Asset serving
Implement proper Octo asset serving. Currently returning empty lists, which works for bypassing downloads but won't work for actual gameplay.

## Title FSM Flow (confirmed via runtime tracing)

```
Event 1: OnFirstStep
Event 3: OnTitleScreen
Event 3: OnApplicationVersion (reuses event 3?)
Event 4: OnBanAccount
Event 5: ??? (internal)
Event 8: OnTermOfService
  └── TOS states: init → dialog → age verify → network → auth → assets → sync → RequestUpdate(7)
Event 7: OnFirstDownload
  └── GetFirstDownloadSizeAsync → size=0 → RequestUpdate(9)
Event 9: OnRegistUserName  ← CURRENT
Event ??: OnGraphicQualitySetting
Event ??: OnFinish
```

## Essential Client Patches (for production APK)

These are needed even with a perfect server:
- **Host redirect** — point all API/asset URLs to our server
- **SSL bypass** — accept our self-signed cert or use plaintext
- **Crypto bypass** — `HandleNet.Encrypt`/`Decrypt` passthrough (or implement matching encryption)
- **Octo AES bypass** — `OctoAPI.DecryptAes` return passthrough
- **FetchTermsOfServiceVersion** — return version=1 instantly (client HTTP parsing bug)
- **LoadTextData** — return true instantly (no text assets available)
- **SyncMasterDataAndUserData** — return true instantly (server response validation issue)

## Key RVAs (libil2cpp.so)
```
Title.OnTermOfService                     0x30A9A00
Title.<OnTermOfService>d__40.MoveNext     0x28F99A8
Title.OnFirstDownload                     0x30A9350
Title.<OnFirstDownload>d__29.MoveNext     0x28F8278
Title.OnRegistUserName                    0x30A97BC
Title.InitializeAssetBundles              0x30A94DC
Title.FetchTermsOfServiceVersion          0x30A9AAC (PATCHED → instant v=1)
Title.LoadTextData                        0x30A9B80 (PATCHED → instant true)
Title.SyncMasterDataAndUserData           0x30A8760 (PATCHED → instant true)
Title.SyncMasterDataAndUserData.MoveNext  0x28FECBC
Title.SyncMasterData.MoveNext             0x28FEA2C
Title.SyncUserData.MoveNext               0x28FF204
Title.GetFirstDownloadSizeAsync           0x30A93FC
FSM.DoUpdate.MoveNext                     0x423B594
FSM.UpdateEvent.MoveNext                  0x423C018
FSM.RequestUpdate                         0x423D24C
OctoManager.StartDbUpdate                 0x4C041B8
OctoAPI.DecryptAes                        0x4BFA3E4 (found via memory scan)
```

## OnTermOfService State Machine
```
-1 → 1:  init, IsValidTermOfService check
 1 → 2:  TOS dialog + AdditionalWorldWide
 2 → 3:  age verification dialog
 3 → 4:  FetchTermsOfServiceVersion (patched → instant)
 4 → 5:  InitializeNetworkAsync, gRPC channel setup
 5 → 10: auth (GetAndroidArgs, AuthAsync)
10 → 11: InitializeAssetBundles (Octo), MasterData.DownloadAsync
11 → 12: SyncMasterDataAndUserData → patched to instant true
12 → -2: bool=TRUE → RequestUpdate(7) → OnFirstDownload
```

## FiniteStateMachineTask Fields (from dump.cs)
```csharp
abstract class FiniteStateMachineTask<TState, TEvent> {
    TState CurrentState;                         // Title+0x10
    TState NextState;                            // Title+0x14
    TState FirstState;                           // +0x18
    List<TState> _states;                        // +0x20
    Dictionary<TEvent, TransactionEvent> _events;// +0x28
    List<TransactionContextAsync> _transitions;  // +0x30
    bool _firstTime;                             // +0x38 byte 0
    bool _inUpdate;                              // +0x39 byte 1
    bool _doUpdateEvent;                         // +0x3A byte 2
    TEvent _requestUpdateEvent;                  // +0x3C
    ITransactionUserData _requestUserData;       // +0x40
    CancellationTokenSource _cts;                // +0x48
}
```

## File Locations
- `frida/hooks.js` — all Frida hooks and patches
- `server/internal/service/octo.go` — HTTP server (assets, TOS pages, Octo v2)
- `server/internal/service/user.go` — gRPC UserService
- `server/internal/service/data.go` — gRPC DataService
- `server/cmd/lunar-tear/main.go` — server entrypoint
- `client/il2cpp_dump/dump.cs` — **full IL2CPP dump** (1.2M lines, all C# signatures + RVAs)
- `docs/PROGRESS.md` — this file

## Reference Rule
**Always consult `client/il2cpp_dump/dump.cs` when unsure about any method, class, field, or RVA.**
Use Grep to search it — never guess when the dump is available.
