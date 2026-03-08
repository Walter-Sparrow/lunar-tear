# Lunar Tear Progress

Last updated: 2026-03-08

`docs/PROGRESS.md` is deprecated. This file tracks only the current, low-assumption state.

## Goal
Server-first fix for the 40% stall with minimal client patches, ideally only host redirection and temporary Frida diagnostics.

## Trusted Current State
- Asset serving works with revision `0` fallback.
- Master data download and parse complete.
- The foreground server receives the real client gRPC traffic.
- `GetUserDataNameV2` and `GetUserData` both complete at the transport level.
- The runtime `GetUserData` path uses plain JSON object arrays, not the old base64+MessagePack experiment.
- The runtime APK diverges from the checked-in source around `CalculatorNetworking` and `UserDataGet`; dump/runtime evidence is more trustworthy than source here.
- `GetUserData` is no longer the active blocker.
- The current healthy `GetUserData` path now reaches:
  - `DatabaseBuilderBase.Build`
  - non-null `bin`
  - `DarkUserMemoryDatabase.ctor`
  - `UserDataGet.<RequestAsync>b__11_1`
  - `Title.<SyncUserData>d__7.MoveNext completed ... isError=false`
- The visible 40% stall still remains, but it is now after successful user-data sync.

## Why GetUserData Failed
The confirmed failure was the first-entrance core account-table append path during `GetUserData`.

What proved it:
- `GetUserDataNameV2`, `GetUserData`, and `WhenAll` all succeeded.
- Failing runs stopped before `DatabaseBuilderBase.Build()` and before `DarkUserMemoryDatabase.ctor`.
- `IUser=1` alone was enough to reproduce the bad branch.
- Removing `IUser` alone was not enough; the request still failed while `IUserStatus`, `IUserProfile`, `IUserLogin`, and `IUserLoginBonus` were still present.
- Removing that whole core account set from `GetUserData` let the builder run and the user DB finish building.

Practical conclusion:
- do not send first-entrance core account rows through `GetUserData`
- seed them through `Auth` / `RegisterUser` diff data instead

## Current Blocker
`SyncUserData` now succeeds, but the client still visually stalls at 40% afterward.

The next blocker is after the successful title sync path:
- `Title.<SyncUserData>d__7.MoveNext` completes with `isError=false`
- `UserDataGet` completes through `b__11_1`
- the app still does not advance to the next visible screen

Current focus:
- runtime `Title` flow after `SyncUserData`
- `Title.IsNeedGameStartApi()`
- `Title.OnTitleScreen(...)`
- `IUserService.GameStartAsync(...)`

`dump.cs` is the source of truth for these title-flow RVAs.

## What We Changed
Server-side changes that got `GetUserData` past the failure:
- kept first-entrance core account rows seeded through `Auth` / `RegisterUser` baseline diff
- stopped advertising these tables from `GetUserDataNameV2`:
  - `IUser`
  - `IUserStatus`
  - `IUserProfile`
  - `IUserLogin`
  - `IUserLoginBonus`
  - `IUserSetting`
- left the rest of `GetUserData` available, mostly empty, so the builder can still produce a valid user DB

## Current Instrumentation
The active Frida script is `frida/hooks_userdata_focus.js`.

Enabled logs:
- `UserDataGet.RequestAsync`
- `UserDataGet.<RequestAsync>b__11_1`
- `UserDataGet.<RequestAsync>b__11_3`
- `DatabaseBuilderBase.Build`
- `DarkUserMemoryDatabase.ctor`
- `Title.<SyncUserData>d__7.MoveNext`
- `Title.<SyncUserData>b__0`
- `Title.IsNeedGameStartApi`
- `Title.<OnTitleScreen>d__44.MoveNext`
- `IUserService.GameStartAsync`
- focused `TaskAwaiter<TResult>.GetResult` logs for user-data/title flow only

## Immediate Next Step
Run with the focused title-flow hook set and capture:
- `[Title] IsNeedGameStartApi ...`
- `[Title] IUserService.GameStartAsync ...`
- `[Title] <OnTitleScreen>d__44 ...`
- `[Flow] TaskAwaiter<TResult>.GetResult ...`

The goal now is to identify which post-`SyncUserData` title step keeps the app visually stuck at 40%.
