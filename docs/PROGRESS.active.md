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
  - `DiffUserData[*].DeleteKeysJson` must be explicitly set to `"[]"`
  - The currently trusted `GameStart` table set is:
    - `IUserProfile`
    - `IUserCharacter`
    - `IUserCostume`
    - `IUserWeapon`
    - `IUserCompanion`
    - `IUserDeckCharacter`
    - `IUserDeck`
    - `IUserMission`
    - `IUserMainQuestFlowStatus`
    - `IUserMainQuestMainFlowStatus`
    - `IUserMainQuestProgressStatus`
    - `IUserMainQuestSeasonRoute`
    - `IUserQuest`
    - `IUserTutorialProgress`
  - All currently enabled `GameStart` tables that are consumed through runtime `Dictionary<string, object>` paths must use lower-camel keys.
  - All datetime-like fields proven so far in these rows must stay plain unix-millis scalars.
- Runtime proof from Frida:
  - `DiffEnumerator.MoveNext -> true`
  - `ToImmutableBuilder -> DarkUserImmutableBuilder`
  - `EntityIUserProfile.ctor(dict)` completes
  - `Diff(IUserProfile[])` completes
  - `Diff(IUserCharacter[])` completes
  - `Diff(IUserCostume[])` completes
  - `Diff(IUserWeapon[])` completes
  - `Diff(IUserCompanion[])` completes
  - `Diff(IUserDeckCharacter[])` completes
  - `Diff(IUserDeck[])` completes
  - `Diff(IUserMission[])` completes
  - `Diff(IUserMainQuestFlowStatus[])` completes
  - `Diff(IUserMainQuestMainFlowStatus[])` completes
  - `Diff(IUserMainQuestProgressStatus[])` completes
  - `Diff(IUserMainQuestSeasonRoute[])` completes
  - `Diff(IUserQuest[])` completes
  - `Diff(IUserTutorialProgress[])` completes
  - `DiffEnumerator.MoveNext -> false`
  - `TaskAwaiter<TResult>.GetResult -> GameStartResponse`
  - title FSM then continues into `CheckResolutionSetting`, `OnGraphicQualitySetting`, and `OnFinish`

## Current Server Shape
- `RegisterUser` / `Auth` still seed baseline diff data.
- `GetUserDataNameV2` includes the account/core tables again with corrected JSON shapes.
- `GameStart()` no longer returns full `StartedDiff()`.
- `GameStart()` now always returns the trusted starter diff via `StartedGameStartDiff()`.
- `GameStart()` currently sends the 14-table starter/outgame set proven above.
- Current row detail retained from the earlier safe boundary:
  - `IUserProfile.favoriteCostumeId = 0`
- Current diff detail:
  - every enabled `DiffData` row sets `DeleteKeysJson = "[]"`
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

The likely issue is now the first post-title continuation after `Title.OnFinish`, not missing `GameStart` starter rows.

Working hypothesis:
- The currently trusted 14-table `GameStart` diff is sufficient to get through diff application and title completion.
- The process still crashes after `Title.OnFinish -> UniTaskCompletionSource`, during the first outgame startup step that follows title completion.
- The next investigation should focus on the post-`OnFinish` handoff rather than adding more `GameStart` tables.

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
- `updatedUserData.updateMapCount` still stays `0` during these `GameStart` runs, but that is no longer the active blocker for the current full trusted `GameStart` diff.
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
Instrument the first post-title continuation after `Title.OnFinish`.

Goal of that step:
- identify the first gameplay/outgame method that runs after `Title.OnFinish -> UniTaskCompletionSource`
- determine whether the crash is in `Gameplay.OnRunApplicationAsync`, `Gameplay.OnTitleAsync`, `Gameplay.OnMainStoryAsync`, or the first continuation they schedule
