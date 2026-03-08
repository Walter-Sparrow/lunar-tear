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
- Seen on client: `<databaseBuilder>5__3` becomes non-null inside `UserDataGet.<RequestAsync>d__11.MoveNext`
- Seen on client: the captured `<>c__DisplayClass11_0.bin` remains `<null>` through the sampled failing branch
- Seen on client: `UserDataGet.<RequestAsync>d__11.MoveNext` finish with state `-2`
- Seen on client: `UnitySynchronizationContext2.Post` from caller `0x361BD40`
- Seen on client: `SendOrPostCallback.ctor(target=UserDataGet, method=<stable per run>)`
- Seen on client: `SendOrPostCallback.Invoke(target=UserDataGet, state=<null>)`
- Seen on client: `req=<n>` ties callback construction/post/invoke to the same `UserDataGet.RequestAsync` attempt
- Seen on client: `InitializeDefault` installs non-null fetch handlers but leaves `OnSuccess` / `OnError` null at that point
- Seen on client: by callback-construction time, `OnSuccess=<null>` and `OnError=HandleError`
- Not seen on client: `FetchTargetTableMethod.Invoke`
- Not seen on client: `FetchRecordMethod.Invoke`
- Seen on client: focused result counts show `1` record for all currently suspected core user tables, including `IUserQuest` and `IUserMission`
- Seen on client: `UserDataGet.<RequestAsync>b__11_3` with matching `lastPostedCallback` / `lastPostedMethod`
- Seen on client: `UserDataGet.HandleError.Invoke`
- Seen on client: `CalculatorNetworking.GetUserDataGetDataSource(onSuccess=<null>, onError=HandleError)`
- Seen on client: `Title.<SyncUserData>b__0`
- Not seen on client: `UserDataGet.HandleSuccess.Invoke`
- Not seen on client: `DatabaseBuilderBase.Build`
- Not seen on client: `Task.Run<DarkUserMemoryDatabase>`
- Not seen on client: `UserDataGet.<RequestAsync>b__0`
- Seen on client with risky hook: `DarkUserDataDatabaseBuilderAppendHelper.Append(builder, "IUser", records)` is entered once before the process crashes
- Not yet seen on client: user DB build path (`DatabaseBuilderBase.Build`, `DarkUserMemoryDatabase`, `DatabaseDefine.set_User`)
- Seen on client: `UserDataGet.InitializeDefault`
- Seen on client: `UserDataGet.add_OnSuccess(value=<null>)`
- Seen on client: `UserDataGet.add_OnError(value=HandleError)`

That makes the likely fault window:
- after successful fetch and successful `WhenAll` result consumption inside the compiler-generated `UserDataGet.RequestAsync` post-fetch pipeline
- after the async method body itself reaches completion, but before any success-side callback / worker-start path becomes visible
- likely in the callback-selection / callback-construction path that decides which `UserDataGet`-bound `SendOrPostCallback` gets posted to the main thread
- likely before or at `SendOrPostCallback` construction time, not inside `UnitySynchronizationContext2.Post` itself
- the currently posted callback is strongly correlated with the error-side path: the same callback object flows into `b__11_3`
- the next concrete unknown is whether that callback is the only `SendOrPostCallback` ever constructed for the request, or whether a success-side callback is also constructed and then discarded
- the installed fetch-handler delegates currently look less central than before: they are present, but they do not appear to be invoked in the failing branch we are tracing
- the strongest remaining symptom is that `OnSuccess` stays null while `OnError` is present when the single posted callback is constructed
- the worker path (`Task.Run<DarkUserMemoryDatabase>` / `UserDataGet.<RequestAsync>b__0`) now looks absent in the failing branch, so the remaining unknown is what condition prevents `databaseBuilder.Build()` / `bin` production before the error callback is chosen
- a risky append-helper probe shows the natural flow likely reaches at least the first append dispatch for `IUser`, but append/helper hooks remain too crash-prone to trust as ongoing instrumentation
- the next narrow boundary is whether `UserDataGet` constructs any new success/error delegates internally during the post-`WhenAll` phase before the main-thread callback is posted
- likely using `UserDataGet`'s own error handling rather than the generic `DarkServerAPI.OnErrorRequest` path
- likely routed through whichever caller wires the `HandleError` delegate into `GetUserDataGetDataSource`
- title is downstream, not the root cause: `Title.<SyncUserData>b__0` appears to consume the `UserDataGet` failure by flipping its local error flag
- `InitializeDefault` not wiring fetch handlers suggests those fields may be unused in this runtime build, or are not the source of the failure we are seeing
- the remaining unknown is which post-await callback-selection rule causes `UserDataGet` to post the error-side callback despite successful awaits and despite currently suspected core tables being populated
- ruled out enough to deprioritize: the simple hypothesis that the failure is caused only by obviously empty core user tables such as `IUserStatus`, `IUserProfile`, `IUserLogin`, `IUserTutorialProgress`, `IUserCharacter`, `IUserCostume`, `IUserWeapon`, `IUserCompanion`, `IUserDeck`, `IUserDeckCharacter`, `IUserQuest`, or `IUserMission`
- ruled out enough to deprioritize: registration/auth sequencing as the primary cause of this branch

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
- We need to determine why `OnSuccess` remains null in this title-driven `UserDataGet` flow while `OnError` is present.
  This should tell us whether the failure is due to missing success subscriber wiring rather than the fetched payload itself.
- We need to determine whether `Task.Run<DarkUserMemoryDatabase>` is ever called in this branch.
  This should tell us whether the success path fails before worker scheduling or only after the worker is queued.
- We need to determine whether the captured `<>c__DisplayClass11_0.bin` ever becomes non-null/non-empty.
  This should tell us whether `databaseBuilder.Build()` is skipped entirely or produces a null/empty buffer before callback selection.
- We need to determine whether `UserDataGet` constructs any new `HandleSuccess` or `HandleError` delegates during the post-`WhenAll` phase.
  This should tell us whether callback selection is being made by internal delegate construction before the posted `SendOrPostCallback` appears.
- We need to map the `SendOrPostCallback` method pointer back to the corresponding managed method/thunk.
  This should tell us whether the posted callback is directly the error-side lambda or a wrapper that later dispatches to it.
- We need to correlate `UnitySynchronizationContext2.Post`, `SendOrPostCallback.Invoke`, and `UserDataGet.<RequestAsync>b__11_1` / `b__11_3` in the same run.
  This should tell us whether the posted callback deterministically leads to `b__11_3` and whether a success-side callback is ever constructed at all.
- We need to count how many `SendOrPostCallback` objects are constructed per `UserDataGet.RequestAsync` request and record their method pointers.
  This should tell us whether the runtime only ever constructs the error-side callback in the failing branch.
- We still may need to determine whether any non-obvious tables outside the currently seeded core set participate in the branch decision.
  But this is now lower priority than understanding why only the error-side callback path is wired/constructed.
- We need to determine whether `CalculatorNetworking.DisposeUserDataGetDataSource(...)` runs after the posted error callback path.
  This should tell us whether the datasource wrapper is tearing itself down only after the failure decision is already made.
- We need to avoid crash-prone introspection in `HandleError.Invoke`.
  That means preferring targeted delegate/callback hooks and dump analysis over native backtraces or deep append/build hooks in this branch.

## Why This Helps
- If `Title.<SyncUserData>b__0` is the real error target, we can stop chasing generic networking code and focus on the title state's success/failure conditions.
- If `Title.<SyncUserData>d__7` shows an `isError` flip right after `UserDataGet.HandleError.Invoke`, we will know the visible stall is just the title flow honoring that error flag.
- Since filtered `Task.WhenAll(...)` logging and both await `GetResult` hooks now fire successfully, we can stop blaming transport and basic aggregation.
- If the per-table result summary looks sane but the main-thread post still targets the error-side callback, the next investigation should stay inside runtime callback-selection logic rather than server RPC transport.
- Since the focused summary now shows non-empty counts for the currently suspected core tables, the next useful work is no longer broad server seeding but tighter callback/post-path analysis.
- Since `SendOrPostCallback` is now confirmed to target `UserDataGet` directly, the most likely remaining bug is in which callback `UserDataGet` constructs/posts after successful awaits.
- Since the same callback object now correlates all the way through `ctor -> post -> invoke -> b__11_3`, the next useful work is to identify whether a success-side callback is ever constructed at all, not to keep broadening payload experiments.
- Since fetch handlers are installed but never invoked in this failing branch, the next useful work is more likely subscriber/callback wiring analysis than delegate payload-handler analysis.
- If `Task.Run<DarkUserMemoryDatabase>` and `DatabaseBuilderBase.Build` never fire, the failure is earlier than worker scheduling and likely decided entirely in the `MoveNext` / callback-selection path.
- Since the append-helper probe crashes immediately after first entry, append/helper hooks should remain off the table except as a last resort.
- If success-side callbacks never fire, the next investigation should stay inside `UserDataGet` and `CalculatorNetworking` runtime-only methods.
- If success-side callbacks do fire, then we should move forward again toward worker scheduling and DB build/publication.

## Immediate Next Checks
- Keep the existing narrow `UserDataGet` / `Title` callback hooks, but shift the next focus to runtime orchestration rather than title ownership.
- Keep the filtered `Task.WhenAll<TResult[]>` and await-result hooks; they have now confirmed successful aggregation.
- Determine what caller `0x361BD40` is, and whether it is the site that constructs/posts the `UserDataGet` callback bound to the observed method pointer for the current run.
- Correlate `SendOrPostCallback.ctor`, `UnitySynchronizationContext2.Post`, `SendOrPostCallback.Invoke`, and `UserDataGet.<RequestAsync>b__11_3` in one trace so we can prove the posted callback flows straight into the error-side method.
- Try to identify whether any analogous `SendOrPostCallback` ever gets constructed for a success-side method / pointer in this branch.
- Keep callback correlation state scoped per `UserDataGet.RequestAsync` so later traces are easier to compare if multiple requests happen in one session.
- Count callback constructions per request and log all observed method pointers for `UserDataGet`-targeted `SendOrPostCallback` objects.
- Determine where `OnSuccess` should be assigned in this runtime path, and whether that assignment simply never happens in the title flow.
- Determine whether `Task.Run<DarkUserMemoryDatabase>` is ever entered for this request.
- Determine whether `<>c__DisplayClass11_0.bin` is ever populated before the error callback is posted.
- Watch `UserDataGet.HandleSuccess.ctor` / `UserDataGet.HandleError.ctor` as a safer way to see internal callback selection.
- Keep `DisposeUserDataGetDataSource(...)` tracing in place, but deprioritize further bulk user-table seeding unless a new focused result points to another concrete missing table.
- Trace `CalculatorNetworking.DisposeUserDataGetDataSource(...)` to see whether teardown happens only after the same failing branch has already been selected.
- Determine whether `UserDataGet.HandleSuccess.Invoke` ever fires in a healthy branch.
- Keep watching `UserDataGet.<RequestAsync>b__11_1(object _)` versus `UserDataGet.<RequestAsync>b__11_3(object _)`.
- Determine whether the user DB worker lambda `UserDataGet.<RequestAsync>b__0()` is ever entered without risky append hooks.
- If the worker starts, trace forward to `DatabaseBuilderBase.Build`, `DarkUserMemoryDatabase.ctor`, and `DatabaseDefine.set_User`.
- If the worker still never starts, keep focus on the internal `UserDataGet.RequestAsync` callback/fan-out path rather than server payload transport.
