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
- The critical `IUser` `GetUserData` shape is now known:
  - keys must be lower camel case (`userId`, `playerId`, ...)
  - `registerDatetime` / `gameStartDatetime` must be plain unix-millis scalars
  - nested `{ unixTime: ... }` maps crash the runtime `EntityIUser.ctor(Dictionary<string, object>)`
- With that `IUser` fix in place, the client now progresses through:
  - ToS completion
  - name entry dialog
  - `SetUserName`
  - `GameStart`
  - graphic quality setting dialog
  - title `Completion`
  - `OnFinish`

## What Was Proven About `IUser`
- `IUser` was the toxic table in `GetUserData`.
- The failure was not “table presence only”; it was a JSON-shape mismatch.
- Runtime evidence from `dump.cs` + Frida showed:
  - `EntityIUser.ctor(Dictionary<string, object>)` reads `userId`, `playerId`, `osType`, `platformType`, `userRestrictionType`, `registerDatetime`, `gameStartDatetime`, `latestVersion`
  - it expects lower-camel keys
  - `registerDatetime` is read as a raw object and passed directly into `MPDateTime.ConvertMPDateTime(object)`
  - passing a nested dictionary there crashes immediately

## Current Server Shape
- `RegisterUser` / `Auth` still seed baseline diff data.
- `GetUserDataNameV2` currently advertises `IUser` again.
- `FirstEntranceUserDataJSONClientTables()` uses the corrected `IUser` JSON shape.
- `GameStart()` no longer returns full `StartedDiff()`.
- `GameStart()` currently returns a reduced `StartedMinimalDiff()` with only starter outgame rows:
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

## New Current Concern
The old blocker is gone. The new blocker is post-title / post-`GameStart`.

What is proven:
- Empty `GameStart` diff avoids the old immediate post-name crash.
- With empty `GameStart` diff, the title flow reaches:
  - `CheckResolutionSetting`
  - `OnComplete`
  - `Completion`
  - `OnFinish`
- The client still crashes after the graphic quality dialog / title finish, so some required post-title runtime state is still missing.

Current hypothesis:
- Full `StartedDiff()` is too aggressive and contains one or more toxic rows for the post-title phase.
- Empty `GameStart` diff is too small and leaves the client without required starter outgame state.
- `StartedMinimalDiff()` is the current compromise and needs validation table-by-table.

## Active Instrumentation
Primary script: `frida/hooks_userdata_focus.js`

Current useful probes:
- title FSM progression
- `SetUserName` / `GameStart` path
- focused `GetUserData` builder path
- `EntityIUser` constructor dictionary reads
- local player registration / title gates

`dump.cs` is the source of truth whenever checked-in source disagrees with runtime behavior.

## Immediate Next Step
Validate the current `StartedMinimalDiff()` run.

If it still crashes:
- identify which `GameStart` diff table is the first toxic post-title row
- add/remove started tables one at a time
- keep `IUser` out of `GameStart` diff unless specifically needed again
