# Quest Flow Client Facts

This note is a client-and-master-data reference only.

It is intended as context for writing or redesigning quest flow without relying on the current server implementation.

## Useful Paths

- `client/dump_output/dump.cs`
  - Decompiled C#-style symbols, enums, method names, and table APIs.
- `client/dump_output/script.json`
  - Method signatures from the client binary, useful for exact symbol names.
- `client/dump_output/il2cpp.h`
  - Low-level field layouts for `EntityIUser*` tables and protobuf response objects.
- `frida/hooks_story_progress_probe.js`
  - Existing runtime probes for `Story`, quest status lookups, and main-quest status lookups.
- `server/assets/master_data/EntityMQuestSceneTable.json`
  - Scene rows: scene ids, quest ids, sort order, scene type, result type, main-flow target flag.
- `server/assets/master_data/EntityMQuestTable.json`
  - Quest rows: reward groups, mission groups, release condition list ids, exp, big-win flag.
- `server/assets/master_data/EntityMMainQuestSequenceTable.json`
  - Main quest ordering.
- `server/assets/master_data/EntityMMainQuestChapterTable.json`
  - Chapter-to-route/sequence-group mapping.
- `server/assets/master_data/EntityMQuestRelationMainFlowTable.json`
  - Main-flow quest to sub-flow/replay-flow mapping.
- `server/assets/master_data/EntityMQuestReleaseConditionListTable.json`
- `server/assets/master_data/EntityMQuestReleaseConditionGroupTable.json`
- `server/assets/master_data/EntityMQuestReleaseConditionQuestClearTable.json`
  - Release gating chain.
- `server/assets/master_data/EntityMQuestMissionGroupTable.json`
- `server/assets/master_data/EntityMQuestMissionTable.json`
- `server/assets/master_data/EntityMQuestMissionRewardTable.json`
  - Quest mission membership, condition types, and mission rewards.
- `server/assets/master_data/EntityMQuestFirstClearRewardGroupTable.json`
  - First-clear reward contents.
- `server/assets/master_data/EntityMUserQuestSceneGrantPossessionTable.json`
  - Scene-based possession grants.

## Quest-Flow RPCs Present In The Client

The client binary contains request/response types for these quest-flow RPCs:

- `UpdateMainFlowSceneProgress`
- `UpdateMainQuestSceneProgress`
- `StartMainQuest`
- `RestartMainQuest`
- `FinishMainQuest`

The client binary also contains request/response types for adjacent flow RPCs commonly seen around quest progression:

- `CheckBeforeGamePlay`
- `InitSequenceSchedule`
- `SetTutorialProgress`
- `GetHeaderNotification`

## Example Client Requests

These are request-shape examples based on protobuf definitions plus observed live traffic patterns.

They are examples of what the client sends, not prescriptions for how the server should behave.

For the response side below, "should look like" means:

- what the protobuf contract allows
- what the client demonstrably reads locally from returned tables
- what a minimally coherent response shape must include for the client to keep progressing

It does not mean "copy the current server implementation."

### `GamePlayService.CheckBeforeGamePlay`

Purpose:

- Early gameplay gate before the quest/story loop proceeds.
- Seen before main-flow scene progression starts.

Proto fields:

- `tr`
- `voiceClientSystemLanguageTypeId`
- `textClientSystemLanguageTypeId`

Example:

```json
{
  "tr": "",
  "voiceClientSystemLanguageTypeId": 1,
  "textClientSystemLanguageTypeId": 1
}
```

Response contract example:

```json
{
  "isExistUnreadPop": false,
  "menuGachaBadgeInfo": [],
  "diffUserData": {}
}
```

### `QuestService.UpdateMainFlowSceneProgress`

Purpose:

- Sent when the client advances a scene that it treats as main-flow progression.
- Request contains only the target `questSceneId`.

Proto fields:

- `questSceneId`

Observed example pattern:

```json
{
  "questSceneId": 9
}
```

Response contract example:

```json
{
  "diffUserData": {
    "IUserMainQuestFlowStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestMainFlowStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestProgressStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    }
  }
}
```

Minimum client-facing responsibility:

- return a coherent `diffUserData`
- include whatever main-quest status rows are needed so local `Story` state can resolve current scene and flow type
- keep the returned tables internally consistent with the target `questSceneId`

### `QuestService.UpdateMainQuestSceneProgress`

Purpose:

- Sent when the client advances a scene that it treats as in-quest / sub-flow progression.
- Request contains only the target `questSceneId`.

Proto fields:

- `questSceneId`

Observed example patterns:

```json
{
  "questSceneId": 11
}
```

```json
{
  "questSceneId": 12
}
```

```json
{
  "questSceneId": 13
}
```

Response contract example:

```json
{
  "diffUserData": {
    "IUserQuest": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserQuestMission": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestFlowStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestMainFlowStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestProgressStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    }
  }
}
```

Minimum client-facing responsibility:

- return `diffUserData`
- include quest-local rows that the client immediately queries:
  - `IUserQuest`
  - `IUserQuestMission`
  - main-quest status tables when local flow state changes
- make the tuple-keyed rows resolvable for local lookups such as:
  - `IUserQuest(userId, questId)`
  - `IUserQuestMission(userId, questId, questMissionId)`

### `QuestService.StartMainQuest`

Purpose:

- Sent when the client starts a main/sub quest battle/story block.
- Carries quest id plus mode flags.

Proto fields:

- `questId`
- `isMainFlow`
- `userDeckNumber`
- `isBattleOnly`
- `maxAutoOrbitCount`
- `isReplayFlow`
- `cageMeasurableValues`

Observed example pattern:

```json
{
  "questId": 2,
  "isMainFlow": false,
  "userDeckNumber": 1,
  "isBattleOnly": false,
  "maxAutoOrbitCount": 0,
  "isReplayFlow": false
}
```

Notes:

- In live logs for quest `2`, the client sent `isMainFlow=false`.
- `userDeckNumber` is the deck slot chosen for the quest start.

Response contract example:

```json
{
  "battleDropReward": [],
  "diffUserData": {
    "IUserQuest": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserQuestMission": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    }
  }
}
```

Minimum client-facing responsibility:

- acknowledge start with `battleDropReward` in the response shape, even if empty
- return enough quest-state diff for the client to regard the quest as started
- materialize local quest-mission rows if the client can query them immediately after start

### `QuestService.RestartMainQuest`

Purpose:

- Restart path for an already started quest.

Proto fields:

- `questId`
- `isMainFlow`

Example:

```json
{
  "questId": 2,
  "isMainFlow": false
}
```

Response contract example:

```json
{
  "battleDropReward": [],
  "battleBinary": "",
  "deckNumber": 1,
  "diffUserData": {}
}
```

### `QuestService.FinishMainQuest`

Purpose:

- Sent when the client finishes the currently played main/sub quest.
- This is the key quest-completion request observed after `Story.FinishEachStoryTypeQuest`.

Proto fields:

- `questId`
- `isRetired`
- `isMainFlow`
- `isAnnihilated`
- `isAutoOrbit`
- `storySkipType`
- `isReplayFlow`
- `vt`

Observed example pattern for quest `2`:

```json
{
  "questId": 2,
  "isRetired": false,
  "isMainFlow": false,
  "isAnnihilated": false,
  "isAutoOrbit": false,
  "storySkipType": 3,
  "isReplayFlow": false,
  "vt": ""
}
```

Notes:

- In live logs around the current blocker, quest `2` consistently finishes with `isMainFlow=false`.
- `storySkipType=3` is the value repeatedly seen in observed finish requests.
- The client constructs this request after local `Story.IsClearedQuestWithQuestId(playedQuestId)` checks.

Response contract example:

```json
{
  "dropReward": [],
  "firstClearReward": [],
  "missionClearReward": [],
  "missionClearCompleteReward": [],
  "autoOrbitResult": [],
  "isBigWin": false,
  "bigWinClearedQuestMissionIdList": [],
  "replayFlowFirstClearReward": [],
  "userStatusCampaignReward": [],
  "autoOrbitReward": null,
  "diffUserData": {
    "IUserQuest": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserQuestMission": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestFlowStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestMainFlowStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    },
    "IUserMainQuestProgressStatus": {
      "updateRecordsJson": "[...]",
      "deleteKeysJson": "[]"
    }
  }
}
```

Minimum client-facing responsibility:

- satisfy the protobuf response shape
- return a coherent post-finish `diffUserData`
- make local post-finish checks succeed for:
  - cleared quest lookup
  - quest-mission lookup
  - main-quest status lookup
- keep reward arrays and diff state mutually consistent if reward lists are populated

### `GimmickService.InitSequenceSchedule`

Purpose:

- Seen around post-battle / story handoff points.
- Request is empty.

Example:

```json
{}
```

Response contract example:

```json
{
  "diffUserData": {}
}
```

### `TutorialService.SetTutorialProgress`

Purpose:

- Seen during the early quest-2 battle flow.
- Client reports tutorial progression separately from quest progression.

Proto fields:

- `tutorialType`
- `progressPhase`
- `choiceId`

Observed example pattern:

```json
{
  "tutorialType": 4,
  "progressPhase": 10,
  "choiceId": 0
}
```

Response contract example:

```json
{
  "tutorialChoiceReward": [],
  "diffUserData": {}
}
```

### `NotificationService.GetHeaderNotification`

Purpose:

- Adjacent out-of-quest notification refresh.
- Request is empty.

Example:

```json
{}
```

Response contract example:

```json
{
  "giftNotReceiveCount": 0,
  "friendRequestReceiveCount": 0,
  "isExistUnreadInformation": false,
  "diffUserData": {}
}
```

## Server Responsibility In Quest Flow

This section is phrased as client-contract responsibility, not as a statement about the current server code.

### Core Responsibility

The server is responsible for returning a user-data state that the client can use to rebuild its local quest/story model.

That means:

- respond with the protobuf shape expected by the called RPC
- provide coherent `diffUserData` entries for all tables the client will immediately query next
- keep returned rows internally consistent across quest, mission, and main-quest status tables

### What The Client Obviously Depends On

From dump symbols and probes, the client locally reads:

- `IUserQuest`
- `IUserQuestMission`
- `IUserMainQuestFlowStatus`
- `IUserMainQuestMainFlowStatus`
- `IUserMainQuestProgressStatus`

So when quest flow changes, server responsibility is not just "return OK".

It is:

- make those local rows resolvable by their client-side keys
- make them describe one coherent state transition
- avoid returning combinations that contradict each other

### Per-RPC Responsibility Summary

`UpdateMainFlowSceneProgress`

- move main-flow scene state forward
- refresh main-quest status tables coherently

`UpdateMainQuestSceneProgress`

- move in-quest scene state forward
- refresh quest-local and main-quest-local tables coherently

`StartMainQuest`

- mark the quest as locally started
- return enough quest state that follow-up local quest lookups succeed

`FinishMainQuest`

- mark the quest as locally finished/cleared in whatever way the client expects
- return enough post-finish state that:
  - `Story.IsClearedQuestWithQuestId`
  - `UserQuestMission` lookup
  - main-quest status lookup
  can all proceed without contradiction

`InitSequenceSchedule`, `SetTutorialProgress`, `GetHeaderNotification`, `CheckBeforeGamePlay`

- satisfy adjacent client expectations around the quest loop
- avoid blocking the next local branch by returning structurally invalid or contradictory state

### What The Server Does Not Get To Delegate

Even though the client performs local gating, the server is still responsible for providing the raw facts used by those local gates.

In practice that means:

- quest ordering comes from master data
- scene ordering comes from master data
- release conditions come from master data
- mission membership and mission condition ids come from master data
- per-user quest state must be returned in tables keyed exactly the way the client expects

## Story Methods Around Quest Progression

The existing Frida probe already hooks the client-side `Story` methods that matter for quest flow:

- `Story.SendUpdateSceneProgressAsync`
- `Story.GetUpdateStoryType`
- `Story.NeedsUpdateSceneProgress(instance)`
- `Story.NeedsUpdateSceneProgress(static)`
- `Story.OnCompleteStory`
- `Story.FinishEachStoryTypeQuest`
- `Story.SendFinishMainQuestAsync`
- `Story.IsClearedQuestWithQuestId`
- `Story.ApplyInQuestLastScene`
- `Story.ReturnMainStoryAsync`
- `Story.GetCurrentStoryHierarchy`
- `Story.ApplyCurrentGameplayStoryType`
- `Story.ApplySceneIdForStoryHierarchyTyped`
- `Story.StartQuestWithoutAnObelisk`
- `Story.EndQuestWithoutAnObelisk`

The same probe also hooks the local table lookups/status conversions used by the client during this flow:

- `ActivePlayerToUserQuestStatus`
- `ActivePlayerToEntityPlayingMainQuestStatus`
- `UserQuestMission.TryFindUniqueCore`

## Client Table Facts

### `IUserQuest`

From `client/dump_output/il2cpp.h`:

- Keyed by `(userId, questId)`
- Fields:
  - `userId`
  - `questId`
  - `questStateType`
  - `isBattleOnly`
  - `latestStartDatetime`
  - `clearCount`
  - `dailyClearCount`
  - `lastClearDatetime`
  - `shortestClearFrames`
  - `latestVersion`

From `client/dump_output/dump.cs`:

- `EntityIUserQuestTable.FindByUserIdAndQuestId(ValueTuple<long, int>)`
- `EntityIUserQuestTable.TryFindByUserIdAndQuestId(ValueTuple<long, int>, out EntityIUserQuest)`

### `IUserQuestMission`

From `client/dump_output/il2cpp.h`:

- Keyed by `(userId, questId, questMissionId)`
- Fields:
  - `userId`
  - `questId`
  - `questMissionId`
  - `progressValue`
  - `isClear`
  - `latestClearDatetime`
  - `latestVersion`

From `client/dump_output/dump.cs`:

- `EntityIUserQuestMissionTable.FindByUserIdAndQuestIdAndQuestMissionId(ValueTuple<long, int, int>)`
- `EntityIUserQuestMissionTable.TryFindByUserIdAndQuestIdAndQuestMissionId(ValueTuple<long, int, int>, out EntityIUserQuestMission)`

### `IUserMainQuestFlowStatus`

- Fields:
  - `userId`
  - `currentQuestFlowType`
  - `latestVersion`

### `IUserMainQuestMainFlowStatus`

- Fields:
  - `userId`
  - `currentMainQuestRouteId`
  - `currentQuestSceneId`
  - `headQuestSceneId`
  - `isReachedLastQuestScene`
  - `latestVersion`

### `IUserMainQuestProgressStatus`

- Fields:
  - `userId`
  - `currentQuestSceneId`
  - `headQuestSceneId`
  - `currentQuestFlowType`
  - `latestVersion`

## Quest Mission Condition Enum

From `client/dump_output/dump.cs`, `QuestMissionConditionType` includes:

- `1 = LESS_THAN_OR_EQUAL_X_PEOPLE_NOT_ALIVE`
- `7 = GREATER_THAN_OR_EQUAL_X_WEAPON_SKILL_USE_COUNT`
- `55 = LESS_THAN_OR_EQUAL_X_WEAPON_SKILL_USE_COUNT`
- `9999 = COMPLETE`

This enum is client fact and should be used instead of guessing mission semantics from ids alone.

## Concrete Example: Quest 2

### Quest Row

From `EntityMQuestTable.json`:

- `QuestId = 2`
- `QuestReleaseConditionListId = 1`
- `QuestFirstClearRewardGroupId = 10001`
- `QuestMissionGroupId = 2`
- `UserExp = 40`
- `CharacterExp = 2`
- `CostumeExp = 180`
- `Gold = 100`
- `IsRunInTheBackground = false`
- `IsCountedAsQuest = true`
- `QuestBonusId = 2`
- `IsBigWinTarget = true`

### Main Quest Sequence Placement

From `EntityMMainQuestSequenceTable.json`:

- sequence group `2` starts with:
  - `QuestId = 2`
  - then `5`
  - then `8`
  - then `11`

This is a direct quest-order fact from master data.

### Scene Rows For Quest 2

From `EntityMQuestSceneTable.json`:

- `QuestSceneId = 11`
  - `QuestId = 2`
  - `QuestSceneType = 3`
  - `IsMainFlowQuestTarget = true`
  - `IsBattleOnlyTarget = true`
  - `QuestResultType = 1`
- `QuestSceneId = 12`
  - `QuestId = 2`
  - `QuestSceneType = 2`
  - `IsMainFlowQuestTarget = true`
  - `IsBattleOnlyTarget = false`
  - `QuestResultType = 1`
- `QuestSceneId = 13`
  - `QuestId = 2`
  - `QuestSceneType = 1`
  - `IsMainFlowQuestTarget = true`
  - `QuestResultType = 2`
- `QuestSceneId = 14`
  - `QuestId = 2`
  - `QuestSceneType = 1`
  - `IsMainFlowQuestTarget = false`
  - `QuestResultType = 1`

### Sub/Replay Mapping

From `EntityMQuestRelationMainFlowTable.json`:

- for `MainFlowQuestId = 2`, difficulty rows include:
  - difficulty `1`: `SubFlowQuestId = 2`, `ReplayFlowQuestId = 30001`
  - difficulty `2`: `SubFlowQuestId = 10001`, `ReplayFlowQuestId = 40001`
  - difficulty `3`: `SubFlowQuestId = 20001`, `ReplayFlowQuestId = 50001`

### Mission Group

From `EntityMQuestMissionGroupTable.json`:

- `QuestMissionGroupId = 2` contains, in order:
  - `QuestMissionId = 21001`
  - `QuestMissionId = 21002`
  - `QuestMissionId = 2`
  - `QuestMissionId = 1`

From `EntityMQuestMissionTable.json`:

- `QuestMissionId = 21001`
  - `QuestMissionConditionType = 7`
  - `ConditionValue = 1`
  - `QuestMissionRewardId = 1`
- `QuestMissionId = 21002`
  - `QuestMissionConditionType = 7`
  - `ConditionValue = 2`
  - `QuestMissionRewardId = 1`
- `QuestMissionId = 2`
  - `QuestMissionConditionType = 1`
  - `ConditionValue = 0`
  - `QuestMissionRewardId = 1`
- `QuestMissionId = 1`
  - `QuestMissionConditionType = 9999`
  - `ConditionValue = 0`
  - `QuestMissionRewardId = 1`

From `EntityMQuestMissionRewardTable.json`:

- `QuestMissionRewardId = 1` grants:
  - `PossessionType = 12`
  - `PossessionId = 0`
  - `Count = 20`

### First-Clear Reward Group

From `EntityMQuestFirstClearRewardGroupTable.json`, `QuestFirstClearRewardGroupId = 10001` includes:

- `PossessionType = 12`, `PossessionId = 0`, `Count = 100`
- `PossessionType = 1`, `PossessionId = 10100`, `Count = 1`
- `PossessionType = 1`, `PossessionId = 10102`, `Count = 1`
- `PossessionType = 2`, `PossessionId = 101001`, `Count = 1`

### Next-Quest Unlock Chain

From `EntityMQuestTable.json`:

- `QuestId = 5` has `QuestReleaseConditionListId = 2`

From release-condition tables:

- `EntityMQuestReleaseConditionListTable.json`
  - list `2` -> group `2`
- `EntityMQuestReleaseConditionGroupTable.json`
  - group `2` -> type `4`, condition id `2`
- `EntityMQuestReleaseConditionQuestClearTable.json`
  - condition id `2` -> `QuestId = 2`

So the master-data unlock chain for quest `5` is tied to clearing quest `2`.

## What The Client Clearly Checks Locally

From dump symbols and existing probes, the client does local checks against:

- `IUserQuest`
  - via `TryFindByUserIdAndQuestId`
  - used by `Story.IsClearedQuestWithQuestId`
- `IUserQuestMission`
  - via `TryFindByUserIdAndQuestIdAndQuestMissionId`
- main-quest progress tables
  - via `ActivePlayerToEntityPlayingMainQuestStatus`

This means quest flow is not only about RPC ordering. The client also gates behavior on local table state it rebuilds from user-data diffs.

## Practical Reading Order

If you want to reconstruct flow from facts only, the shortest useful order is:

1. `EntityMQuestTable.json`
2. `EntityMMainQuestSequenceTable.json`
3. `EntityMQuestSceneTable.json`
4. `EntityMQuestRelationMainFlowTable.json`
5. `EntityMQuestMissionGroupTable.json`
6. `EntityMQuestMissionTable.json`
7. `EntityMQuestMissionRewardTable.json`
8. `EntityMQuestFirstClearRewardGroupTable.json`
9. `EntityMQuestReleaseConditionListTable.json`
10. `EntityMQuestReleaseConditionGroupTable.json`
11. `EntityMQuestReleaseConditionQuestClearTable.json`
12. `frida/hooks_story_progress_probe.js`
13. `client/dump_output/dump.cs`

## Things Master Data Does Not Fully Answer By Itself

Master data gives authoritative facts for:

- quest ordering
- scene ordering
- scene types and result types
- release conditions
- mission membership
- mission condition enums
- reward group contents

Master data does not, by itself, fully specify:

- the exact runtime order of client RPC dispatches
- which local `Story` branch consumes which table first
- the exact handoff timing between story completion, local table refresh, and any UI acquisition/result screens

For those, use the client dump plus live `Story` probes.
