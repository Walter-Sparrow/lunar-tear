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
- Confirmed by runtime tracing: `SendOrPostCallback.ctor` is called with `target=UserDataGet` and a stable method pointer for the sampled run.
- Confirmed by runtime tracing: the exact same `SendOrPostCallback` instance later invokes on the main thread with `target=UserDataGet` and `state=<null>`.
- Confirmed by runtime tracing: in the latest correlated run, the same callback object created in `SendOrPostCallback.ctor` is:
  - posted by `UnitySynchronizationContext2.Post`
  - invoked by `SendOrPostCallback.Invoke`
  - then immediately followed by `UserDataGet.<RequestAsync>b__11_3`
- Confirmed by runtime tracing: `UserDataGet.<RequestAsync>b__11_3` now logs the same `lastPostedCallback` / `lastPostedMethod` that were seen at callback construction and post time.
- Confirmed by runtime tracing: the same `req=1` correlation now ties the whole failing branch together:
  - `RequestAsync req=1`
  - successful awaits / `WhenAll`
  - `SendOrPostCallback.ctor req=1`
  - `UnitySynchronizationContext2.Post req=1`
  - `SendOrPostCallback.Invoke req=1`
  - `UserDataGet.<RequestAsync>b__11_3 req=1`
- Corrected field-offset tracing now shows `InitializeDefault` really does install non-null fetch handlers:
  - `_fetchTableHandler=FetchTargetTableMethod`
  - `_fetchRecordHandler=FetchRecordMethod`
- Corrected field-offset tracing now shows `OnSuccess` remains `<null>` while `OnError` becomes `HandleError` by callback-construction time.
- New handler-invocation tracing does not show `FetchTargetTableMethod.Invoke` or `FetchRecordMethod.Invoke` firing in the failing branch.
- Confirmed by runtime tracing: the focused per-table result summary now shows non-empty records for all currently suspected core user tables:
  - `IUser=1`
  - `IUserStatus=1`
  - `IUserGem=1`
  - `IUserProfile=1`
  - `IUserLogin=1`
  - `IUserLoginBonus=1`
  - `IUserTutorialProgress=1`
  - `IUserCharacter=1`
  - `IUserCostume=1`
  - `IUserWeapon=1`
  - `IUserCompanion=1`
  - `IUserDeck=1`
  - `IUserDeckCharacter=1`
  - `IUserQuest=1`
  - `IUserMission=1`
- Confirmed by runtime/server tracing: forcing a real `RegisterUser -> Auth -> GetUserData` sequence does not change the final failure branch; registration/auth ordering is not the current blocker.
- Confirmed by runtime tracing: when the first-entrance payload is reduced to all-empty user tables, the client takes the healthy path:
  - `DatabaseBuilderBase.Build`
  - non-null `bin`
  - `DarkUserMemoryDatabase.ctor`
  - `UserDataGet.<RequestAsync>b__11_1`
- Current server experiment: `FirstEntranceUserDataJSONClientTables()` now sends only a minimal `IUser` row while leaving the other user tables empty, to binary-search which `IUser` field/value triggers the stall.
- Latest result: a run with `IUser=1` and every other currently watched user table at `0` still takes `b__11_3` before `DatabaseBuilderBase.Build`, so `IUser` alone is sufficient to reproduce the failure.
- Follow-up result: setting `PlayerId = UserId` does not change the branch; the client still takes `b__11_3` before `DatabaseBuilderBase.Build`.
- Follow-up result: restoring `OsType = 2` and `PlatformType = 2` also does not change the branch; the client still takes `b__11_3` before `DatabaseBuilderBase.Build`.
- Current live experiment after that result: keep the isolated `IUser` row, keep `PlayerId = UserId`, keep `OsType = 2` / `PlatformType = 2`, set `RegisterDatetime` to `nowMillis`, and keep `GameStartDatetime = 0` for first-entrance semantics.
- Follow-up result: setting `RegisterDatetime = nowMillis` also does not change the branch; the client still takes `b__11_3` before `DatabaseBuilderBase.Build`.
- Current live experiment after that result: keep the isolated `IUser` row, keep `PlayerId = UserId`, keep `OsType = 2` / `PlatformType = 2`, keep `RegisterDatetime = nowMillis`, and now also set `GameStartDatetime = nowMillis` to test whether the runtime rejects a pre-start `IUser` row specifically.
- Follow-up result: setting `GameStartDatetime = nowMillis` also does not change the branch; the client still takes `b__11_3` before `DatabaseBuilderBase.Build`.
- New conclusion: the isolated `IUser` field search is exhausted enough to deprioritize single-field `IUser` mismatches as the primary explanation.
- Current live experiment after that result: keep a plausible non-empty `IUser` row and add back only a minimal `IUserStatus` row, while leaving the other user tables empty, to test whether non-empty `IUser` requires a companion singleton row.
- Follow-up result: `IUser + IUserStatus` still takes `b__11_3`, so `IUserStatus` alone is not the missing companion row.
- New alternate path from the sibling project/dev note: enable the hidden title menu, enter the transfer flow via Square Bridge, and watch whether the runtime moves past `set_User` but still stalls at 40%.
- Transfer-flow result with the forced title menu: the client does reach `GetBackupToken` and `TransferUser`, then continues into `Auth -> GetLatestMasterDataVersion -> GetUserDataNameV2 -> GetUserData`, but still falls into the same `UserDataGet.<RequestAsync>b__11_3` path.
- In that transfer-flow run, `DatabaseDefine.set_User` is still not seen before the stall, so transfer does not currently move us past the existing user-data build boundary.
- Current live experiment after that result: keep `IUser` and `IUserStatus`, and add back only `IUserProfile` as the next singleton companion row.
- Follow-up result: `IUser + IUserStatus + IUserProfile` still takes `b__11_3`, so `IUserProfile` alone is not the missing singleton companion either.
- Current live experiment after that result: keep `IUser`, `IUserStatus`, and `IUserProfile`, and add back only `IUserLogin` as the next singleton companion row.
- Follow-up result: adding `IUserLogin` still ends in `Title.<SyncUserData>b__0` flipping `isError` to `true`, so `IUserLogin` alone is not the missing singleton companion either.
- Current live experiment after that result: keep `IUser`, `IUserStatus`, `IUserProfile`, and `IUserLogin`, and add back only `IUserSetting` as the next singleton companion row.
- Follow-up result: adding `IUserSetting` still ends in `Title.<SyncUserData>b__0` flipping `isError` to `true`, so `IUserSetting` alone is not the missing singleton companion either.
- Current live experiment after that result: keep `IUser`, `IUserStatus`, `IUserProfile`, `IUserLogin`, and `IUserSetting`, and add back only `IUserLoginBonus` as the next singleton companion row.
- Assembly result: the hot call at `0x361B930` in `UserDataGet.<RequestAsync>d__11.MoveNext` resolves to `DarkUserDataDatabaseBuilderAppendHelper.Append(...)` at RVA `0x28F06C8`.
- Assembly result: the later call at `0x361B968` resolves to `DatabaseBuilderBase.Build()` at RVA `0x3BDBD90`.
- New conclusion from those branch-target dumps: the failure boundary is now narrowed to the append-helper path itself, before `DatabaseBuilderBase.Build()` starts.
- New conclusion from the earlier `WhenAll` dump: outer/inner array shape checks pass before the append-helper call, so this is no longer a `WhenAll`/transport/null-array problem.

## Current Blocker
The client still stalls inside `UserDataGet.RequestAsync`, but the latest assembly work narrowed the failing boundary further: the request gets through `WhenAll` and reaches the per-table append-helper call, then later ends up posting the error callback before `DatabaseBuilderBase.Build()` ever runs.

Observed boundary in failing runs:
- Seen on client: `UserDataGet.RequestAsync`
- Seen on client: `GetUserDataNameV2Api.RequestAsyncMethod`
- Seen on client: `GetUserDataApi.RequestAsyncMethod(list)`
- Seen on client: `GetUserDataApi.<RequestAsyncMethod>d__1.MoveNext` resume/complete
- Seen on server: `GetUserData` request arrives and response is sent
- Seen on client: `TaskAwaiter<List<List<string>>>.GetResult` succeed with one group / `106` table names
- Seen on client: `TaskAwaiter<ValueTuple<string, List<Dictionary<string, object>>>[][]>.GetResult` succeed with one inner array / `106` table results
- Seen on client: `Task.WhenAll<TResult[]>` create a `WhenAllPromise`
- Seen on client: `<databaseBuilder>5__3` becomes non-null inside `UserDataGet.<RequestAsync>d__11.MoveNext`
- Seen in assembly/runtime correlation: the `WhenAll` result branch at `0x361B8AC` only performs array/null/length guards before entering the tuple loop
- Seen in assembly/runtime correlation: the tuple loop calls `DarkUserDataDatabaseBuilderAppendHelper.Append(...)` at `0x28F06C8` via the callsite at `0x361B930`
- Seen on client in failing runs: `<>c__DisplayClass11_0.bin` remains `<null>`
- Not seen on client in failing runs: `DatabaseBuilderBase.Build`
- Seen in assembly/runtime correlation: `DatabaseBuilderBase.Build()` would be called later via `0x361B968`, but the failing path never reaches that point
- Seen on client in failing runs: `UserDataGet.<RequestAsync>b__11_3`
- Seen on client in failing runs: `UserDataGet.HandleError.Invoke`
- Seen on client in failing runs: posted callback correlation still ties `SendOrPostCallback.ctor -> Post -> Invoke -> b__11_3`

Observed boundary in the all-empty success run:
- Seen on client: focused summary `IUser=0`, `IUserStatus=0`, `IUserGem=0`, `IUserProfile=0`, `IUserLogin=0`, `IUserLoginBonus=0`, `IUserTutorialProgress=0`, `IUserCharacter=0`, `IUserCostume=0`, `IUserWeapon=0`, `IUserCompanion=0`, `IUserDeck=0`, `IUserDeckCharacter=0`, `IUserQuest=0`, `IUserMission=0`
- Seen on client: `DatabaseBuilderBase.Build`
- Seen on client: non-null `<>c__DisplayClass11_0.bin` with `binLen=4006`
- Seen on client: `UserDataGet.<RequestAsync>b__0`
- Seen on client: `DarkUserMemoryDatabase.ctor`
- Seen on client: `TaskAwaiter<TResult>.GetResult caller=0x361ba58 -> DarkUserMemoryDatabase`
- Seen on client: `UserDataGet.<RequestAsync>b__11_1`
- Not seen on client in that run: `b__11_3` / `HandleError.Invoke`

Binary-search conclusion:
- all-empty user-data baseline -> success path (`Build` -> worker -> `b__11_1`)
- `IUser` present alone -> failure path (`b__11_3`)
- `IUser` is now the highest-priority suspect by far

That makes the likely fault window:
- payload-dependent behavior inside `DarkUserDataDatabaseBuilderAppendHelper.Append(...)`
- after `WhenAll.GetResult`
- before `DatabaseBuilderBase.Build()`
- not a generic transport problem
- not a generic main-thread callback-posting bug
- not broad `WhenAll` array-shape/null guarding
- likely either:
  - append-helper dispatch on `tableName`
  - per-table JSON-to-entity conversion inside append helper
  - append-helper exception/fallback path before build

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
- We need to understand what `DarkUserDataDatabaseBuilderAppendHelper.Append(...)` is doing with the first tables in the tuple loop.
  The new highest-value question is whether append-helper dispatch/conversion throws or rejects one of the early tables before build starts.
- We still may need to test more singleton companions later, but that is now lower priority than append-helper control flow.
- We also need to test the transfer-flow path with the forced title menu visible.
  If transfer still stalls after `set_User`, that would shift the investigation beyond the current first-entrance payload boundary.
- We should treat the isolated `IUser` field binary search as largely exhausted for now:
  - `PlayerId`
  - `OsType`
  - `PlatformType`
  - `RegisterDatetime`
  - `GameStartDatetime`
  all failed to change the branch when tested in isolation against `IUser`.
- We should keep the safe hooks that distinguish the failing `b__11_3` path from the healthy `b__11_1` path.
- We should avoid returning to broad table pruning unless append-helper disassembly stops producing new information.

## Why This Helps
- The all-empty run proved the build/runtime callback path is healthy in this build.
- The latest runs reduced the payload suspect set to `IUser` alone.
- The latest assembly result proves the branch decision is later and more concrete than before: the runtime reaches append-helper dispatch and fails before build.
- Since the isolated `IUser` field experiments no longer move the branch, append-helper control flow is now a better target than more blind row combinations.

## Immediate Next Checks
- Dump and inspect more of `DarkUserDataDatabaseBuilderAppendHelper.Append(...)` around RVA `0x28F06C8`.
- Determine whether the helper is:
  - dispatching by `tableName` and failing lookup
  - converting JSON dictionaries into typed entities and throwing there
  - or branching into a dedicated exception/fallback path before build
- Keep singleton-row experiments deprioritized until the append-helper disassembly is understood.
- Use the forced title-menu hook to try the transfer flow and compare:
  - whether `DatabaseDefine.set_User` fires
  - whether the client still hangs afterward
- Keep the safe hooks that show:
  - `DatabaseBuilderBase.Build`
  - `binLen`
  - `b__11_1` vs `b__11_3`
