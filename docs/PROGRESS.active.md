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
- Focused `OnFinish` tracing now proves the client constructs and serializes `CheckBeforeGamePlayRequest` successfully.
- The post-`OnFinish` failure was not missing request bytes or protobuf shaping.
- The client was receiving a real gRPC `RpcException`:
  - `StatusCode = Unimplemented`
  - `detail = "unknown service apb.api.gameplay.GamePlayService"`
- Root cause: server-side service-name mismatch.
  - Client calls `apb.api.gameplay.GamePlayService`
  - Server had registered `apb.api.gameplay.GameplayService`
- The gameplay proto and Go server registration have now been corrected to `GamePlayService`.
- The active-mission interruption dialog after `CheckBeforeGamePlay` was caused by seeded running-main-quest state in `GameStart`.
- Clearing the initial `IUserMainQuestProgressStatus` / `IUserMainQuestFlowStatus` running-state fields removed that dialog and allowed natural main-story startup to continue.

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
- `CheckBeforeGamePlay` is implemented in `server/internal/service/gameplay.go`.
- `server/proto/gameplay.proto` now declares `service GamePlayService`.
- Regenerated Go gRPC bindings now advertise:
  - `ServiceName: "apb.api.gameplay.GamePlayService"`
  - full method `/apb.api.gameplay.GamePlayService/CheckBeforeGamePlay`
- `server/cmd/lunar-tear/main.go` now registers `pb.RegisterGamePlayServiceServer(...)`.
- `QuestService.UpdateMainFlowSceneProgress()` now sends a consistent lower-camel diff bundle for:
  - `IUserMainQuestFlowStatus`
  - `IUserMainQuestMainFlowStatus`
  - `IUserMainQuestProgressStatus`
- `QuestService.StartMainQuest()` now sends lower-camel `IUserQuest` with unix-millis `latestStartDatetime` and explicit `DeleteKeysJson = "[]"`.

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
- `Title.OnFinish` issues `GamePlayService/CheckBeforeGamePlayAsync`.
- The request is created, serialized, and wrapped in `ResponseContext` successfully.
- The previously observed post-`OnFinish` failure was a server `Unimplemented` reply caused by the wrong gRPC service name.
- That naming mismatch is now fixed.
- The client now progresses further through natural main-story startup:
  - `QuestService/UpdateMainFlowSceneProgress`
  - `QuestService/StartMainQuest`
  - `NotificationService/GetHeaderNotification`
  - `QuestService/UpdateMainQuestSceneProgress`
- The next confirmed blocker is:
  - `GimmickService/InitSequenceScheduleAsync`
  - server reply: `Unimplemented`
  - detail: `unknown service apb.api.gimmick.GimmickService`

## Current Concern
The active blocker is no longer JSON shape, diff application, request-body transport, `CheckBeforeGamePlay`, or the initial main-quest startup RPCs.

The prior uncertainty about `CheckBeforeGamePlay` transport has been resolved:
- the client did serialize the body
- the server was responding with `Unimplemented`
- the cause was service-name mismatch, not payload corruption

The current trusted boundary is later:
- the client now enters the mission start sequence naturally
- quest-start progress diffs are being consumed far enough to reach later outgame/gameplay initialization
- the next hard blocker is a missing `GimmickService`

Working hypothesis:
- The currently trusted 14-table `GameStart` diff is sufficient to get through diff application and title completion.
- With `GamePlayService` and the early quest-state diffs corrected, the remaining blocker is now missing server implementation for the next post-quest initialization service call.
- The next investigation should focus on `apb.api.gimmick.GimmickService`, starting with `InitSequenceScheduleAsync`, rather than revisiting `GameStart` starter rows.

## Active Instrumentation
Primary scripts:
- `frida/hooks_onfinish_handoff.js`
- `frida/hooks_userdata_focus.js`

Current useful probes:
- title FSM progression
- `GameStart` interceptor path
- post-`OnFinish` `CheckBeforeGamePlay` lifecycle
- `QuestService/UpdateMainFlowSceneProgressAsync`
- `QuestService/StartMainQuestAsync`
- `QuestService/UpdateMainQuestSceneProgressAsync`
- late post-quest service dispatches (now including `GimmickService/InitSequenceScheduleAsync`)
- `RequestContext..ctor`
- `ErrorHandlingInterceptor.SendAsync`
- `ErrorHandlingInterceptor.ErrorHandling`
- `ResponseContext<T>.WaitResponseAsync`
- `AsyncUnaryCall<T>.get_ResponseAsync`
- `CheckBeforeGamePlayRequest.CalculateSize`
- `CheckBeforeGamePlayRequest.WriteTo`

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
- `IGamePlayService.CheckBeforeGamePlayAsync`
- `IQuestService.UpdateMainFlowSceneProgressAsync`
- `IQuestService.StartMainQuestAsync`
- `IQuestService.UpdateMainQuestSceneProgressAsync`
- `IGimmickService.InitSequenceScheduleAsync`
- `RequestContext..ctor`
- `ErrorHandlingInterceptor.SendAsync`
- `ErrorHandlingInterceptor.ErrorHandling`
- `ResponseContext<T>.WaitResponseAsync`

## Immediate Next Step
Implement or stub `apb.api.gimmick.GimmickService`, starting with `InitSequenceScheduleAsync`.

Goal of that step:
- satisfy the next missing service after natural quest startup
- determine whether the client proceeds past `GimmickService/InitSequenceScheduleAsync`
- identify the next real blocker after gimmick initialization if one remains
