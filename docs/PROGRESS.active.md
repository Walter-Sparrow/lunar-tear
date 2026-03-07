# Lunar Tear Progress

Last updated: 2026-03-07

This is the current progress file.
`docs/PROGRESS.md` is deprecated because it mixes in older heavy-hook assumptions and no longer reflects the most trustworthy runtime state.

## Goal
Minimal client patches, server-first implementation, and a clean path to the home screen without relying on Frida-only gameplay fixes.

## Trusted Current State
- Asset serving is working with revision `0` as the fallback source for resources we actually have.
- Master data download and parse complete on the client.
- The main foreground server is now receiving gRPC traffic correctly.
- `UserService` and `DataService` requests are reaching the real server process:
  - `RegisterUser`
  - `GetAndroidArgs`
  - `Auth`
  - `GetLatestMasterDataVersion`
  - `GetUserDataNameV2`
  - `GetUserData`
- `GetUserDataAsync` returns a real `UserDataGetResponse` on the client.
- `GetUserDataApi.RequestAsyncMethod(list)` resumes and completes on the client.
- The active `GetUserData` path is plain JSON object arrays, not the older base64+MessagePack experiment.
- Server-side `IUser` JSON generation is now typed instead of hand-built.
- Server-side user identity is now stable across `RegisterUser`, `Auth`, and `GetUserData`.
- Baseline diffs and quest diffs now use client table names like `IUser*` instead of snake_case keys.
- Previous crashes seen around user DB append were hook-induced; risky append-helper hooks should be treated as suspect first.
- New tracing shows `UserDataGet.<RequestAsync>b__11_3` and `UserDataGet.HandleError.Invoke` fire after the fetch path completes.
- New tracing does not show `DarkServerAPI.OnErrorRequest`, so this does not currently look like the generic API error-conversion path.
- New tracing shows `CalculatorNetworking.GetUserDataGetDataSource(...)` is called with `onSuccess=<null>` and a non-null `onError` delegate.
- The checked-in source and the built APK diverge here: source shows a simple direct `await userDataGet.RequestAsync()`, while the runtime build clearly uses `CalculatorNetworking.GetUserDataGetDataSource(...)` and callback wiring around `UserDataGet`.
- More specifically, the checked-in `CalculatorNetworking.cs` only contains auth/setup helpers, while the built APK dump exposes extra datasource orchestration methods such as `GetUserDataGetDataSource`, `DisposeUserDataGetDataSource`, and generic datasource callback helpers. This path must be treated as dump-first, not source-first.
- Confirmed by runtime tracing: the `UserDataGet` error delegate really does target `Title.<SyncUserData>b__0`. The captured closure pointer inside `HandleError` matches the `self` pointer of `Title.<SyncUserData>b__0`.
- Confirmed by runtime tracing: `Title.<SyncUserData>b__0` flips its captured `isError` flag from `false` to `true`, so title is consuming the failure rather than originating it.
- Dump-first confirmation: the runtime `UserDataGet` class really has event fields (`OnSuccess`, `OnError`), handler fields (`_fetchTableHandler`, `_fetchRecordHandler`), and runtime-only callback lambdas not represented by the simple checked-in source implementation.
- Dump-first confirmation: the generic `Task.WhenAll<TResult[]>` instantiation used in this branch is at RVA `0x38AF1B4`, shared by both `Task.WhenAll<object>` and `Task.WhenAll<ValueTuple<string, List<Dictionary<string, object>>>[]>`.
- Confirmed by runtime tracing: `TaskAwaiter<List<List<string>>>.GetResult` succeeds inside `UserDataGet.RequestAsync`, yielding one group with `106` requested user-data table names.
- Confirmed by runtime tracing: `TaskAwaiter<ValueTuple<string, List<Dictionary<string, object>>>[][]>.GetResult` succeeds inside `UserDataGet.RequestAsync`, yielding one inner array with `106` per-table results.
- Confirmed by runtime tracing: the `WhenAll` result is structurally populated, not empty; for example, `IUser` has `1` record while many optional tables are `0`.
- Confirmed by runtime tracing: `UserDataGet.<RequestAsync>d__11.MoveNext` reaches async terminal state `-2` before the posted error callback fires.
- Confirmed by runtime tracing: after those successful awaits, `UnitySynchronizationContext2.Post` is called from inside the `UserDataGet` flow at caller `0x361BD40`, with a callback bound to the `UserDataGet` instance.

## Current Blocker
The client still stalls inside the client-side `UserDataGet.RequestAsync` flow after `GetUserDataApi.RequestAsyncMethod(list)` finishes. The flow now appears to take `UserDataGet`'s own error callback path before user DB build/publication becomes visible.

Observed boundary:
- Seen on client: `UserDataGet.RequestAsync`
- Seen on client: `GetUserDataNameV2Api.RequestAsyncMethod`
- Seen on client: `GetUserDataApi.RequestAsyncMethod(list)`
- Seen on client: `GetUserDataApi.<RequestAsyncMethod>d__1.MoveNext` resume/complete
- Seen on server: `GetUserData` request arrives and response is sent
- Seen on client: `TaskAwaiter<List<List<string>>>.GetResult` succeed with one group / `106` table names
- Seen on client: `TaskAwaiter<ValueTuple<string, List<Dictionary<string, object>>>[][]>.GetResult` succeed with one inner array / `106` table results
- Seen on client: `Task.WhenAll<TResult[]>` create a `WhenAllPromise`
- Seen on client: `UserDataGet.<RequestAsync>d__11.MoveNext` finish with state `-2`
- Seen on client: `UnitySynchronizationContext2.Post` from caller `0x361BD40`
- Seen on client: `UserDataGet.<RequestAsync>b__11_3`
- Seen on client: `UserDataGet.HandleError.Invoke`
- Seen on client: `CalculatorNetworking.GetUserDataGetDataSource(onSuccess=<null>, onError=HandleError)`
- Seen on client: `Title.<SyncUserData>b__0`
- Seen on client: `UserDataGet.InitializeDefault completed` with `_fetchTableHandler=<null>` and `_fetchRecordHandler=<null>`
- Not seen on client: `UserDataGet.HandleSuccess.Invoke`
- Not yet seen on client: user DB build path (`DatabaseBuilderBase.Build`, `DarkUserMemoryDatabase`, `DatabaseDefine.set_User`)
- Seen on client: `UserDataGet.InitializeDefault`
- Seen on client: `UserDataGet.add_OnSuccess(value=<null>)`
- Seen on client: `UserDataGet.add_OnError(value=HandleError)`

That makes the likely fault window:
- after successful fetch and successful `WhenAll` result consumption inside the compiler-generated `UserDataGet.RequestAsync` post-fetch pipeline
- after the async method body itself reaches completion, but before any success-side callback / worker-start path becomes visible
- likely in the callback-selection / validation path that decides whether to post `b__11_1` versus `b__11_3`
- likely using `UserDataGet`'s own error handling rather than the generic `DarkServerAPI.OnErrorRequest` path
- likely routed through whichever caller wires the `HandleError` delegate into `GetUserDataGetDataSource`
- title is downstream, not the root cause: `Title.<SyncUserData>b__0` appears to consume the `UserDataGet` failure by flipping its local error flag
- `InitializeDefault` not wiring fetch handlers suggests those fields may be unused in this runtime build, or are not the source of the failure we are seeing
- the remaining unknown is which acceptance rule on the returned table set causes `UserDataGet` to post the error-side callback despite successful awaits
- not ruled in yet: a client-side validation failure on one or more required tables with `0` records

## Current Instrumentation Strategy
Prefer narrow, low-risk hooks only:
- gRPC call entry/return
- specific async state-machine checkpoints around `UserDataGet` / `GetUserDataApi`
- database publication checkpoints
- nearby compiler-generated callbacks for `UserDataGet`
- `UserDataGet` success/error delegate invocation

Avoid:
- broad asset/text spam
- broad generic awaiter hooks that mix unrelated tasks
- append/helper hooks that can perturb execution or crash the process

## What We Need To Find
- We need to confirm whether `UserDataGet.HandleSuccess.Invoke` ever fires in a healthy branch for this build.
  If success never fires, the problem is in the orchestration/callback layer; if success fires but build still never starts, the next bug is after callback dispatch.
- We need to determine whether `UserDataGet.<RequestAsync>b__11_1` is a success-side callback and whether it is simply absent in our failing path.
  This helps separate "request completed but was marked failed" from "request never reached success handling at all."
- We need to identify which tables in the `WhenAll` result are treated as required by the runtime branch after aggregation.
  This should tell us whether the server is still missing mandatory user tables even though the overall tuple set looks structurally valid.
- We need to determine whether caller `0x361BD40` is explicitly posting `UserDataGet.<RequestAsync>b__11_3` to the main thread, and under what condition it chooses that callback over `b__11_1`.
  This should tell us whether the branch is content-validation driven or simply wired incorrectly in this build.
- We need to determine whether `CalculatorNetworking.DisposeUserDataGetDataSource(...)` runs after the posted error callback path.
  This should tell us whether the datasource wrapper is tearing itself down only after the failure decision is already made.
- We need to avoid crash-prone introspection in `HandleError.Invoke`.
  That means preferring targeted delegate/callback hooks and dump analysis over native backtraces or deep append/build hooks in this branch.

## Why This Helps
- If `Title.<SyncUserData>b__0` is the real error target, we can stop chasing generic networking code and focus on the title state's success/failure conditions.
- If `Title.<SyncUserData>d__7` shows an `isError` flip right after `UserDataGet.HandleError.Invoke`, we will know the visible stall is just the title flow honoring that error flag.
- Since filtered `Task.WhenAll(...)` logging and both await `GetResult` hooks now fire successfully, we can stop blaming transport and basic aggregation.
- If the per-table result summary looks sane but the main-thread post still targets the error-side callback, the next investigation should stay inside runtime callback-selection logic rather than server RPC transport.
- If a small set of supposedly required tables are the only suspicious zero-record entries, the next useful server work is to seed those tables rather than broad user-data rewrites.
- If success-side callbacks never fire, the next investigation should stay inside `UserDataGet` and `CalculatorNetworking` runtime-only methods.
- If success-side callbacks do fire, then we should move forward again toward worker scheduling and DB build/publication.

## Immediate Next Checks
- Keep the existing narrow `UserDataGet` / `Title` callback hooks, but shift the next focus to runtime orchestration rather than title ownership.
- Keep the filtered `Task.WhenAll<TResult[]>` and await-result hooks; they have now confirmed successful aggregation.
- Refine the `WhenAll` summary to highlight only non-zero tables and a shortlist of suspicious zero-record tables such as `IUserStatus`, `IUserQuest`, `IUserDeck`, `IUserCharacter`, `IUserWeapon`, `IUserCompanion`, and `IUserCostume`.
- Determine what caller `0x361BD40` is, and whether it explicitly posts `UserDataGet.<RequestAsync>b__11_3` rather than `b__11_1`.
- Trace `CalculatorNetworking.DisposeUserDataGetDataSource(...)` to see whether teardown happens only after the same failing branch has already been selected.
- Determine whether `UserDataGet.HandleSuccess.Invoke` ever fires in a healthy branch.
- Keep watching `UserDataGet.<RequestAsync>b__11_1(object _)` versus `UserDataGet.<RequestAsync>b__11_3(object _)`.
- Determine whether the user DB worker lambda `UserDataGet.<RequestAsync>b__0()` is ever entered without risky append hooks.
- If the worker starts, trace forward to `DatabaseBuilderBase.Build`, `DarkUserMemoryDatabase.ctor`, and `DatabaseDefine.set_User`.
- If the worker still never starts, keep focus on the internal `UserDataGet.RequestAsync` callback/fan-out path rather than server payload transport.
