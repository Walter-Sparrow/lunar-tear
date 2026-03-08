# Lunar Tear Progress

Last updated: 2026-03-08

`docs/PROGRESS.md` is deprecated. This file tracks only the currently trusted state.

## Goal
Reach the first playable/home flow with minimal client patching and a server-first implementation.

## Proven State
- Asset serving works with revision `0` fallback.
- Master data download and parse complete.
- The foreground Go server receives the real client gRPC traffic.
- `GetUserData` is no longer blocked at the old 40% stall.
- The critical `GetUserData` table shapes now proven by runtime are:
  - `IUser`: lower-camel keys, unix-millis scalar datetimes
  - `IUserStatus`: lower-camel keys, unix-millis scalar `staminaUpdateDatetime`
  - `IUserProfile`: lower-camel keys, unix-millis scalar `*UpdateDatetime` fields
- With those fixes in place, the client now progresses through:
  - ToS completion
  - name entry dialog
  - `SetUserName`
  - `GameStart`
  - graphic quality setting dialog
  - title `Completion`
  - `OnFinish`

## What Was Proven About `GameStart`
- The old immediate `GameStart` crash was inside `UserDiffUpdateInterceptor` while applying `DiffUserData`.
- The following `GameStart` payload details are now required for the current passing boundary:
  - `DiffUserData["IUserProfile"].UpdateRecordsJson` must contain a real profile row, not `"[]"`
  - `IUserProfile` JSON must use lower-camel keys:
    - `userId`
    - `name`
    - `nameUpdateDatetime`
    - `message`
    - `messageUpdateDatetime`
    - `favoriteCostumeId`
    - `favoriteCostumeIdUpdateDatetime`
    - `latestVersion`
  - `IUserProfile` datetime fields must be plain unix-millis scalars
  - `DiffUserData["IUserProfile"].DeleteKeysJson` must be explicitly set to `"[]"`
- Runtime proof from Frida:
  - `DiffEnumerator.MoveNext -> true`
  - `ToImmutableBuilder -> DarkUserImmutableBuilder`
  - `EntityIUserProfile.ctor(dict)` completes
  - `Diff(IUserProfile[])` completes
  - `DiffEnumerator.MoveNext -> false`
  - `TaskAwaiter<TResult>.GetResult -> GameStartResponse`
  - title FSM then continues into `CheckResolutionSetting`, `OnGraphicQualitySetting`, and `OnFinish`

## Current Server Shape
- `RegisterUser` / `Auth` still seed baseline diff data.
- `GetUserDataNameV2` includes the account/core tables again with corrected JSON shapes.
- `GameStart()` no longer returns full `StartedDiff()`.
- `GameStart()` currently returns `StartedMinimalDiff()` in a tight bisect configuration:
  - selected group: `profileOnly`
  - current table set: `IUserProfile`
  - current row detail: `favoriteCostumeId = 0`
  - current diff detail: `DeleteKeysJson = "[]"`
- `GameStart()` currently sends common-response trailers:
  - `x-apb-response-datetime`
  - `x-apb-update-user-data-names`

## Current Boundary
`GameStart` diff application is no longer the blocker.

What is now proven:
- The client fully consumes the current reduced `GameStart` diff.
- The title flow exits `GameStart` and reaches:
  - `CheckResolutionSetting`
  - `OnGraphicQualitySetting`
  - `OnComplete`
  - `Completion`
  - `OnFinish`
- The process now crashes only after `OnFinish`, during the first post-title / outgame startup step.

## Current Concern
The new blocker is no longer JSON shape or interceptor diff application.

The likely issue is now missing required starter outgame state after title completion.

Working hypothesis:
- `profileOnly` is enough to get past `GameStart` apply logic.
- The first post-title flow still needs a minimal gameplay/outgame set that is not yet present.
- The next safest tables to add back are the smallest identity/loadout tables first:
  - `IUserCharacter`
  - `IUserCostume`
- After that, if needed:
  - `IUserWeapon`
  - `IUserCompanion`
  - `IUserDeckCharacter`
  - `IUserDeck`

## Active Instrumentation
Primary script: `frida/hooks_userdata_focus.js`

Current useful probes:
- title FSM progression
- `GameStart` interceptor path
- `MapField<string, DiffData>` enumeration
- `DarkUserDataDatabaseBuilderAppendHelper.Diff(...)`
- `EntityIUserProfile.ctor(dict)`
- `MPDateTime.ConvertMPDateTime`
- local player registration / title gates

Notes:
- `updatedUserData.updateMapCount` still stays `0` during these `GameStart` runs, but that is no longer the active blocker for the current reduced profile-only diff.
- `dump.cs` remains the source of truth whenever checked-in source disagrees with runtime behavior.

## Reference Paths
Open these first when continuing this investigation:
- `docs/PROGRESS.active.md`
- `client/dump_output/dump.cs`
- `frida/hooks_userdata_focus.js`
- `server/internal/mock/diff.go`
- `server/internal/userdata/userdata.go`
- `server/internal/service/user.go`
- `server/internal/service/data.go`

Most relevant symbols / areas:
- `UserDiffUpdateInterceptor.<SendAsync>d__1.MoveNext`
- `ResponseContextExtensions.GetCommonResponse(...)`
- `DarkUserDataDatabaseBuilderAppendHelper.Diff(...)`
- `DarkUserDataDatabaseBuilderAppendHelper.Remove(...)`
- `EntityIUserProfile.ctor(Dictionary<string, object>)`
- `EntityIUserStatus.ctor(Dictionary<string, object>)`
- `MPDateTime.ConvertMPDateTime`
- `Title.<OnRegistUserName>d__37.MoveNext`
- `Title.<OnGraphicQualitySetting>d__39.MoveNext`
- `Title.OnFinish`

## Immediate Next Step
Move the `StartedMinimalDiff()` bisect forward from `profileOnly` to the next smallest starter bundle:
- `characterCostumeOnly`

Goal of that step:
- keep the now-working `GameStart` diff apply path
- determine whether adding `IUserCharacter` + `IUserCostume` moves the crash later or unlocks the first post-title flow
