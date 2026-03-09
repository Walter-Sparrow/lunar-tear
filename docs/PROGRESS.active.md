# Lunar Tear Progress

Last updated: 2026-03-09

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
- `GimmickService` is now implemented server-side well enough for `InitSequenceScheduleAsync` to return `OK`.
- `GachaService` is now registered server-side and no longer fails with `unknown service apb.api.gacha.GachaService`.
- `GiftService` is now registered server-side and no longer fails with `unknown service apb.api.gift.GiftService`.
- `BattleService` is now registered server-side and no longer fails with `unknown service apb.api.battle.BattleService`.

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
- A shared in-memory store now backs the current bootstrap/runtime state:
  - player/session state
  - starter party and deck state
  - main-quest progress state
  - tutorial, mission, and gimmick bootstrap state
- `RegisterUser` / `Auth` now read baseline state from that store and project it into client `IUser*` tables.
- `GetUserDataNameV2` includes the account/core tables again with corrected JSON shapes.
- `GetUserData()` now serves requested tables from store-backed projections instead of direct fixture assembly.
- `GameStart()` no longer returns full `StartedDiff()`.
- `GameStart()` now sends the trusted 14-table starter/outgame set from the current in-memory user snapshot.
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
- Quest scene/finish diffs now also include `IUserMainQuestSeasonRoute`, matching the main-quest hierarchy tables already projected by `GetUserData`.
- `QuestService.StartMainQuest()` now sends lower-camel `IUserQuest` with unix-millis `latestStartDatetime` and explicit `DeleteKeysJson = "[]"`.
- `QuestService`, `TutorialService`, `GimmickService`, and `NotificationService` now mutate/read the shared in-memory store rather than relying only on stateless mock diffs.
- `GachaService` is now registered and store-backed for:
  - catalog/list reads
  - single-gacha lookups
  - reward-gacha availability counters
  - converted-gacha-medal response state
- `GiftService` is now store-backed for:
  - mailbox gift list reads
  - receiving gifts into gift history
  - notification badge count derived from pending gifts
  - one seeded default gift for each user
- `BattleService` is now store-backed for:
  - tracking active/inactive battle state
  - recording per-user start/finish counts
  - recording the latest party counts, battle-binary size, and elapsed frame count
- Main-quest progression is now owned by a dedicated master-data-driven engine in `server/internal/questflow/engine.go`.
  - Scene descriptors now distinguish bootstrap/background, running, transition, battle-entry, terminal, and post-clear-tail phases.
  - `QuestService` scene/start/finish RPCs now delegate to that engine instead of mutating quest state ad hoc in handlers.
  - Store bootstrap profiles are now applied through the same engine, rather than hardcoded `Quests` / `QuestMissions` snapshots in `store.go`.
- `FinishMainQuest()` no longer hardcodes a final scene id.
  - It preserves the latest main-quest scene pointer already established by `UpdateMainQuestSceneProgress(...)`.
  - It only clears the active/running quest markers in `IUserMainQuestFlowStatus` / `IUserMainQuestProgressStatus`.
- `GimmickService` is now registered and currently stubs:
  - `InitSequenceSchedule`
  - `UpdateSequence`
  - `UpdateGimmickProgress`
  - `Unlock`

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
  - `QuestService/FinishMainQuest`
  - follow-up `QuestService/UpdateMainQuestSceneProgress`
  - `GimmickService/InitSequenceSchedule`
- The previous `GimmickService/InitSequenceScheduleAsync` `Unimplemented` blocker is resolved.
- The previous `GachaService` `Unimplemented` blocker is also resolved.
- The previous `GiftService` `Unimplemented` blocker is also resolved.
- The previous `BattleService` `Unimplemented` blocker is also resolved.
- Current observed runtime behavior:
  - the client reaches mission startup, quest completion, and later gimmick initialization without transport or service-name failure
  - after that, the client now reaches the first gacha-service boundary instead of aborting on a missing service
  - mailbox/gift service calls can now be answered from store-backed state instead of failing at dispatch
  - battle-wave RPCs can now be answered and recorded in store-backed battle state instead of failing at dispatch
  - the latest boundary is no longer the earlier intro-camera replay loop
  - the previous `scene 13` handoff stall is no longer the active blocker
  - on `UpdateMainQuestSceneProgress(questSceneId=13)`, the active `IUserQuest` row for `questId=2` now projects as a fully cleared row:
    - `questStateType=3`
    - `clearCount=1`
    - `dailyClearCount=1`
    - non-zero `lastClearDatetime`
  - runtime probe proof:
    - the client reaches `Story.ApplyInQuestLastScene`
    - `Story.IsClearedQuestWithQuestId(questId=2)` no longer blocks the handoff
    - the client now issues `QuestService/FinishMainQuest`
  - current observed sequence after that fix:
    - `UpdateMainFlowSceneProgress(questSceneId=8)`
    - `UpdateMainFlowSceneProgress(questSceneId=9)`
    - `UpdateMainQuestSceneProgress(questSceneId=11)`
    - `BattleService/StartWave`
    - `BattleService/FinishWave`
    - `GimmickService/InitSequenceSchedule`
    - `UpdateMainQuestSceneProgress(questSceneId=13)`
    - `FinishMainQuest(questId=2, isMainFlow=false, storySkipType=4)`
    - later `GimmickService/InitSequenceSchedule`
  - the new blocker is later than the old black-screen-with-music boundary:
    - the client gets through the old `scene 13` completion handoff
    - it now stalls after the post-`FinishMainQuest` loading phase

## Current Concern
The active blocker is no longer JSON shape, diff application, request-body transport, `CheckBeforeGamePlay`, the initial main-quest startup RPCs, missing `GimmickService`, or the old early gameplay re-entry loop.

The prior uncertainty about `CheckBeforeGamePlay` transport has been resolved:
- the client did serialize the body
- the server was responding with `Unimplemented`
- the cause was service-name mismatch, not payload corruption

The current trusted boundary is later:
- the client now enters the mission start sequence naturally
- quest-start progress diffs are being consumed far enough to reach later outgame/gameplay initialization
- `GimmickService/InitSequenceSchedule` now returns `OK`
- `GachaService` now returns `OK` for the currently reached calls instead of `Unimplemented`
- `GiftService` now returns `OK` for the currently reached calls instead of `Unimplemented`
- `BattleService` now returns `OK` for the currently reached calls instead of `Unimplemented`
- `UpdateMainQuestSceneProgress(questSceneId=13)` now returns a fully cleared `IUserQuest` shape for `questId=2`
- the client-side `Story.IsClearedQuestWithQuestId(2)` gate is now satisfied far enough for the client to continue into `FinishMainQuest`
- `FinishMainQuest(questId=2, isMainFlow=false, storySkipType=4)` now returns `OK`
- the current failure mode is now a later post-`FinishMainQuest` loading stall, rather than the earlier `scene 13` handoff stall

Working hypothesis:
- The currently trusted 14-table `GameStart` diff is sufficient to get through diff application and title completion.
- With `GamePlayService`, early quest-state diffs, `GimmickService`, the `scene 13` clear-state projection, and `FinishMainQuest` corrected, the remaining blocker is now a later post-quest world/load handoff.
- The likely issue is no longer "missing next service", and no longer the old `scene 13` clear gate.
- The strongest current server-side suspicion is incomplete post-`FinishMainQuest` world progression for the `questId=2` completion path:
  - the client now calls `FinishMainQuest`
  - the request currently arrives as `isMainFlow=false`
  - our engine currently only applies next-main-quest activation when `req.IsMainFlow == true`
  - this may leave the client with a finished quest row but without the next story/world transition it expects after loading
- One cleanup already made from that investigation:
  - `FinishMainQuest` no longer overwrites the scene pointer with a hardcoded `3`
  - the scene pointer is now owned by the preceding/follow-up `UpdateMainQuestSceneProgress(...)` calls
- Another cleanup already proven from the same investigation:
  - terminal quest-progress updates now materialize a fully cleared `IUserQuest` row before `FinishMainQuest`
  - this moved the boundary forward from "stuck inside `ApplyInQuestLastScene`" to "client issues `FinishMainQuest` and then stalls later"
- The newest refactor replaced the previous bootstrap/auto-clear hacks with engine-owned sequence handoff rules derived from master data.
- Local deterministic verification now proves the `main-quest-scene-9` bootstrap profile is reproduced by replaying the same engine transition (`scene 9`) from a fresh store state, including:
  - quest `1` cleared
  - quest `2` active
  - quest `2` mission rows materialized
  - main-quest scene/flow pointers at `scene 9`
- The next investigation should focus on the post-`FinishMainQuest(questId=2, isMainFlow=false)` loading path, using the existing Frida probes to identify which world/story transition the client expects after the quest-2 completion handoff.

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
- `QuestService/FinishMainQuestAsync`
- late post-quest service dispatches (now including `GimmickService/InitSequenceScheduleAsync`)
- post-`FinishMainQuest(questId=2, isMainFlow=false)` loading handoff behavior
- `GachaService` calls reached after the post-quest handoff
- `GiftService` calls reached after the post-quest handoff
- `BattleService` calls reached after the post-quest handoff
- post-quest black-screen handoff behavior
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
- `IQuestService.FinishMainQuestAsync`
- `IGimmickService.InitSequenceScheduleAsync`
- `IGachaService.GetGachaListAsync`
- `IGachaService.GetGachaAsync`
- `IGiftService.GetGiftListAsync`
- `IGiftService.ReceiveGiftAsync`
- `IBattleService.StartWaveAsync`
- `IBattleService.FinishWaveAsync`
- gameplay/world-state transition after `FinishMainQuest(questId=2, isMainFlow=false)`
- `RequestContext..ctor`
- `ErrorHandlingInterceptor.SendAsync`
- `ErrorHandlingInterceptor.ErrorHandling`
- `ResponseContext<T>.WaitResponseAsync`

## Immediate Next Step
Investigate the post-`FinishMainQuest(questId=2, isMainFlow=false)` loading stall.

Goal of that step:
- determine what story/world transition is still missing after the now-fixed `scene 13` clear handoff
- verify whether the quest-2 finish path must still activate or expose additional next-quest / main-flow state even though the request currently arrives as `isMainFlow=false`
- establish the next concrete RPC, table transition, or hierarchy update needed to leave the post-load stall
