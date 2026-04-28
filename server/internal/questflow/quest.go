package questflow

import (
	"fmt"
	"log"

	"lunar-tear/server/internal/masterdata"
	"lunar-tear/server/internal/model"
	"lunar-tear/server/internal/store"
)

func (h *QuestHandler) initQuestState(user *store.UserState, questId int32) {
	quest := user.Quests[questId]
	quest.QuestId = questId
	user.Quests[questId] = quest

	for _, missionId := range h.MissionIdsByQuestId[questId] {
		key := store.QuestMissionKey{QuestId: questId, QuestMissionId: missionId}
		mission := user.QuestMissions[key]
		mission.QuestId = questId
		mission.QuestMissionId = missionId
		user.QuestMissions[key] = mission
	}
}

func isMainQuestPlayable(quest masterdata.EntityMQuest) bool {
	return !quest.IsRunInTheBackground && quest.IsCountedAsQuest
}

func (h *QuestHandler) clearQuestMissions(user *store.UserState, questId int32, nowMillis int64) {
	for _, missionId := range h.MissionIdsByQuestId[questId] {
		key := store.QuestMissionKey{QuestId: questId, QuestMissionId: missionId}
		mission := user.QuestMissions[key]
		mission.IsClear = true
		mission.ProgressValue = 1
		mission.LatestClearDatetime = nowMillis
		user.QuestMissions[key] = mission
	}
}

func (h *QuestHandler) HandleQuestStart(user *store.UserState, questId int32, isBattleOnly bool, userDeckNumber int32, nowMillis int64) {
	h.handleQuestStartInternal(user, questId, isBattleOnly, userDeckNumber, false, nowMillis)
}

func (h *QuestHandler) HandleQuestStartReplay(user *store.UserState, questId int32, isBattleOnly bool, userDeckNumber int32, nowMillis int64) {
	h.handleQuestStartInternal(user, questId, isBattleOnly, userDeckNumber, true, nowMillis)
}

func (h *QuestHandler) handleQuestStartInternal(user *store.UserState, questId int32, isBattleOnly bool, userDeckNumber int32, isReplayFlow bool, nowMillis int64) {
	quest, ok := h.QuestById[questId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleQuestStart", questId))
	}

	h.initQuestState(user, questId)

	if quest.Stamina > 0 {
		maxMillis := h.MaxStaminaByLevel[user.Status.Level] * 1000
		store.ConsumeStamina(user, quest.Stamina, maxMillis, nowMillis)
	}

	questState := user.Quests[questId]
	if questState.QuestStateType == model.UserQuestStateTypeCleared {
		if isReplayFlow {
			user.MainQuest.SavedCurrentQuestSceneId = user.MainQuest.CurrentQuestSceneId
			user.MainQuest.SavedHeadQuestSceneId = user.MainQuest.HeadQuestSceneId
			user.MainQuest.CurrentQuestFlowType = int32(model.QuestFlowTypeReplayFlow)
			user.MainQuest.ReplayFlowCurrentQuestSceneId = 0
			user.MainQuest.ReplayFlowHeadQuestSceneId = 0
			user.MainQuest.LatestVersion = nowMillis
			questState.QuestStateType = model.UserQuestStateTypeActive
			questState.LatestStartDatetime = nowMillis
			questState.IsBattleOnly = isBattleOnly
			questState.UserDeckNumber = userDeckNumber
			user.Quests[questId] = questState
			log.Printf("[HandleQuestStart] replay flow started for quest %d, saved scene=%d head=%d",
				questId, user.MainQuest.SavedCurrentQuestSceneId, user.MainQuest.SavedHeadQuestSceneId)
		}
		return
	}

	questState.IsBattleOnly = isBattleOnly
	questState.UserDeckNumber = userDeckNumber
	if isMainQuestPlayable(quest) {
		user.MainQuest.CurrentQuestFlowType = int32(model.QuestFlowTypeMainFlow)
		questState.QuestStateType = model.UserQuestStateTypeActive
		questState.LatestStartDatetime = nowMillis
	} else {
		questState.QuestStateType = model.UserQuestStateTypeCleared
		questState.ClearCount = 1
		questState.DailyClearCount = 1
		questState.LastClearDatetime = nowMillis

		if sceneIds := h.SceneIdsByQuestId[questId]; len(sceneIds) > 0 {
			firstSceneId := sceneIds[0]
			prevSceneId := user.MainQuest.CurrentQuestSceneId
			h.advanceMainFlowScene(user, questId, firstSceneId)
			user.MainQuest.CurrentQuestFlowType = int32(model.QuestFlowTypeMainFlow)
			log.Printf("[HandleQuestStart] background quest %d auto-cleared, scene %d -> %d", questId, prevSceneId, firstSceneId)
		}
	}
	user.Quests[questId] = questState
}

func (h *QuestHandler) applyQuestVictory(user *store.UserState, questId int32, outcome *FinishOutcome, nowMillis int64) {
	questState := user.Quests[questId]
	if !questState.IsRewardGranted {
		h.applyQuestRewards(user, questId, nowMillis)
		outcome.ChangedWeaponStoryIds = append(outcome.ChangedWeaponStoryIds,
			h.grantWeaponStoryUnlocksForQuestScene(user, questId, model.QuestResultTypeHalfResult, nowMillis)...)
		outcome.ChangedWeaponStoryIds = append(outcome.ChangedWeaponStoryIds,
			h.grantWeaponStoryUnlocksForQuestScene(user, questId, model.QuestResultTypeFullResult, nowMillis)...)
		questState.IsRewardGranted = true
	}
	for _, drop := range outcome.DropRewards {
		h.applyRewardPossession(user, drop.PossessionType, drop.PossessionId, drop.Count, nowMillis)
	}
	for _, reward := range outcome.ReplayFlowFirstClearRewards {
		h.applyRewardPossession(user, reward.PossessionType, reward.PossessionId, reward.Count, nowMillis)
	}
	questState.QuestStateType = model.UserQuestStateTypeCleared
	questState.ClearCount++
	questState.DailyClearCount++
	questState.LastClearDatetime = nowMillis
	user.Quests[questId] = questState
}

func (h *QuestHandler) HandleQuestFinish(user *store.UserState, questId int32, isRetired, isAnnihilated bool, nowMillis int64) FinishOutcome {
	quest, ok := h.QuestById[questId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleQuestFinish", questId))
	}

	outcome := h.evaluateFinishOutcome(user, questId)
	wasReplay := user.MainQuest.CurrentQuestFlowType == int32(model.QuestFlowTypeReplayFlow)

	if !isRetired {
		h.applyQuestVictory(user, questId, &outcome, nowMillis)

		if isMainQuestPlayable(quest) && !wasReplay {
			lastSceneId := h.getLastMainFlowSceneId(questId)
			h.advanceMainFlowScene(user, questId, lastSceneId)
		}
	}

	if isRetired && !isAnnihilated && quest.Stamina > 1 {
		refund := quest.Stamina - 1
		maxMillis := h.MaxStaminaByLevel[user.Status.Level] * 1000
		store.RecoverStamina(user, refund*1000, maxMillis, nowMillis)
	}

	user.MainQuest.ProgressQuestSceneId = 0
	user.MainQuest.ProgressHeadQuestSceneId = 0
	user.MainQuest.ProgressQuestFlowType = 0
	user.MainQuest.CurrentQuestFlowType = int32(model.QuestFlowTypeUnknown)

	if wasReplay {
		if user.MainQuest.SavedCurrentQuestSceneId > 0 {
			user.MainQuest.CurrentQuestSceneId = user.MainQuest.SavedCurrentQuestSceneId
		}
		if user.MainQuest.SavedHeadQuestSceneId > 0 {
			user.MainQuest.HeadQuestSceneId = user.MainQuest.SavedHeadQuestSceneId
		}
		user.MainQuest.SavedCurrentQuestSceneId = 0
		user.MainQuest.SavedHeadQuestSceneId = 0
		user.MainQuest.ReplayFlowCurrentQuestSceneId = 0
		user.MainQuest.ReplayFlowHeadQuestSceneId = 0
		log.Printf("[HandleQuestFinish] replay flow ended for quest %d, restored scene=%d head=%d",
			questId, user.MainQuest.CurrentQuestSceneId, user.MainQuest.HeadQuestSceneId)
	}

	h.clearQuestMissions(user, questId, nowMillis)

	return outcome
}

func (h *QuestHandler) HandleQuestSkip(user *store.UserState, questId, skipCount int32, nowMillis int64) FinishOutcome {
	questDef, ok := h.QuestById[questId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleQuestSkip", questId))
	}

	maxMillis := h.MaxStaminaByLevel[user.Status.Level] * 1000
	store.ConsumeStamina(user, skipCount, maxMillis, nowMillis)

	skipTicketId := h.Config.ConsumableItemIdForQuestSkipTicket
	user.ConsumableItems[skipTicketId] -= skipCount
	if user.ConsumableItems[skipTicketId] < 0 {
		user.ConsumableItems[skipTicketId] = 0
	}

	var allDrops []RewardGrant
	for range skipCount {
		drops := h.computeDropRewards(questDef)
		for _, drop := range drops {
			h.applyRewardPossession(user, drop.PossessionType, drop.PossessionId, drop.Count, nowMillis)
		}
		allDrops = append(allDrops, drops...)

		if questDef.Gold != 0 {
			user.ConsumableItems[h.Config.ConsumableItemIdForGold] += questDef.Gold
		}
		h.applyExpRewards(user, questId, nowMillis)
	}

	questState := user.Quests[questId]
	questState.ClearCount += skipCount
	questState.DailyClearCount += skipCount
	questState.LastClearDatetime = nowMillis
	user.Quests[questId] = questState

	log.Printf("[HandleQuestSkip] questId=%d skipCount=%d drops=%d gold=%d", questId, skipCount, len(allDrops), questDef.Gold*skipCount)
	return FinishOutcome{DropRewards: allDrops}
}

func (h *QuestHandler) HandleQuestRestart(user *store.UserState, questId int32, nowMillis int64) {
	questDef, ok := h.QuestById[questId]
	if ok && isMainQuestPlayable(questDef) {
		user.MainQuest.CurrentQuestFlowType = int32(model.QuestFlowTypeMainFlow)
	}

	quest := user.Quests[questId]
	quest.QuestId = questId
	quest.QuestStateType = model.UserQuestStateTypeActive
	quest.IsBattleOnly = false
	quest.LatestStartDatetime = nowMillis
	user.Quests[questId] = quest

	for _, missionId := range h.MissionIdsByQuestId[questId] {
		key := store.QuestMissionKey{QuestId: questId, QuestMissionId: missionId}
		m := user.QuestMissions[key]
		m.QuestId = questId
		m.QuestMissionId = missionId
		m.IsClear = false
		m.ProgressValue = 0
		m.LatestClearDatetime = 0
		user.QuestMissions[key] = m
	}
}
