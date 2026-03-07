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

## Current Blocker
The client still stalls inside the client-side `UserDataGet.RequestAsync` flow after `GetUserDataApi.RequestAsyncMethod(list)` finishes. The flow now appears to take `UserDataGet`'s own error callback path before user DB build/publication becomes visible.

Observed boundary:
- Seen on client: `UserDataGet.RequestAsync`
- Seen on client: `GetUserDataNameV2Api.RequestAsyncMethod`
- Seen on client: `GetUserDataApi.RequestAsyncMethod(list)`
- Seen on client: `GetUserDataApi.<RequestAsyncMethod>d__1.MoveNext` resume/complete
- Seen on server: `GetUserData` request arrives and response is sent
- Seen on client: `UserDataGet.<RequestAsync>b__11_3`
- Seen on client: `UserDataGet.HandleError.Invoke`
- Not seen on client: `UserDataGet.HandleSuccess.Invoke`
- Not yet seen on client: user DB build path (`DatabaseBuilderBase.Build`, `DarkUserMemoryDatabase`, `DatabaseDefine.set_User`)

That makes the likely fault window:
- inside the compiler-generated `UserDataGet.RequestAsync` post-fetch pipeline
- likely around its internal fan-out / callback / worker scheduling path, not the gRPC transport itself
- likely using `UserDataGet`'s own error handling rather than the generic `DarkServerAPI.OnErrorRequest` path
- not ruled in yet: a client-side validation failure on fetched user data before or during worker scheduling

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

## Immediate Next Checks
- Determine whether `CalculatorNetworking.GetUserDataGetDataSource(...)` shows the expected subscriber wiring.
- Determine whether `UserDataGet.HandleSuccess.Invoke` ever fires in a healthy branch.
- Keep watching `UserDataGet.<RequestAsync>b__11_1(object _)` versus `UserDataGet.<RequestAsync>b__11_3(object _)`.
- Determine whether the user DB worker lambda `UserDataGet.<RequestAsync>b__0()` is ever entered without risky append hooks.
- If the worker starts, trace forward to `DatabaseBuilderBase.Build`, `DarkUserMemoryDatabase.ctor`, and `DatabaseDefine.set_User`.
- If the worker still never starts, keep focus on the internal `UserDataGet.RequestAsync` callback/fan-out path rather than server payload transport.
