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
- Server-side `IUser` JSON generation is now typed and marshaled instead of hand-built.
- Previous crashes seen around user DB append were hook-induced; removing the risky append hook stopped that native crash.

## Current Blocker
The client still stalls after `GetUserData` returns, but before user DB build/publication starts.

Observed boundary:
- Seen on client: `UserDataGet.RequestAsync`
- Seen on client: `GetUserDataNameV2Api.RequestAsyncMethod`
- Seen on client: `GetUserDataApi.RequestAsyncMethod(list)`
- Seen on server: `GetUserData` request arrives and response is sent
- Not yet seen on client: user DB build path (`DatabaseBuilderBase.Build`, `DarkUserMemoryDatabase`, `DatabaseDefine.set_User`)

That makes the likely fault window:
- the await/result handoff right after `darkClient.DataService.GetUserDataAsync(...)`, or
- access/deserialization of `userData.UserDataJson`

## Current Instrumentation Strategy
Prefer narrow, low-risk hooks only:
- gRPC call entry/return
- response accessor entry/return
- database publication checkpoints
- error dialog path

Avoid:
- broad asset/text spam
- aggressive async `MoveNext` instrumentation
- append/helper hooks that can perturb execution

## Immediate Next Checks
- Confirm `DataServiceClient.GetUserDataAsync` returns on the client side.
- Confirm `UserDataGetResponse.get_UserDataJson` is actually reached.
- If the getter is reached, narrow further to the first JSON deserialization boundary.
- If the getter is not reached, focus on the post-await continuation path instead of server payload shape.
