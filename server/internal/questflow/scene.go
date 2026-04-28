package questflow

import (
	"fmt"
	"log"

	"lunar-tear/server/internal/gametime"
	"lunar-tear/server/internal/model"
	"lunar-tear/server/internal/store"
)

func (h *QuestHandler) applySceneGrants(user *store.UserState, questSceneId int32, nowMillis int64) {
	grants, ok := h.SceneGrantsBySceneId[questSceneId]
	if !ok {
		return
	}
	for _, g := range grants {
		h.applyRewardPossession(user, model.PossessionType(g.PossessionType), g.PossessionId, g.Count, nowMillis)
	}
}

func (h *QuestHandler) isSceneAhead(newSceneId, currentHeadId int32) bool {
	if currentHeadId == 0 {
		return true
	}
	return h.SceneById[newSceneId].SortOrder > h.SceneById[currentHeadId].SortOrder
}

func (h *QuestHandler) advanceMainFlowScene(user *store.UserState, questId, sceneId int32) {
	user.MainQuest.CurrentQuestSceneId = sceneId
	if h.isSceneAhead(sceneId, user.MainQuest.HeadQuestSceneId) {
		user.MainQuest.HeadQuestSceneId = sceneId
	}

	lastSceneId := h.getChapterLastSceneId(questId)
	user.MainQuest.IsReachedLastQuestScene = sceneId == lastSceneId

	if routeId, ok := h.RouteIdByQuestId[questId]; ok {
		user.MainQuest.CurrentMainQuestRouteId = routeId
		if seasonId, ok := h.SeasonIdByRouteId[routeId]; ok {
			user.MainQuest.MainQuestSeasonId = seasonId
		}
	}
}

func (h *QuestHandler) HandleMainFlowSceneProgress(user *store.UserState, questSceneId int32, nowMillis int64) {
	scene, ok := h.SceneById[questSceneId]
	if !ok {
		panic(fmt.Sprintf("unknown sceneId=%d for HandleMainFlowSceneProgress", questSceneId))
	}

	quest, ok := h.QuestById[scene.QuestId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleMainFlowSceneProgress", questSceneId))
	}

	h.advanceMainFlowScene(user, quest.QuestId, questSceneId)
	user.MainQuest.CurrentQuestFlowType = int32(model.QuestFlowTypeMainFlow)

	if user.SideStoryActiveProgress.CurrentSideStoryQuestId != 0 {
		user.SideStoryActiveProgress = store.SideStoryActiveProgress{
			LatestVersion: nowMillis,
		}
	}

	user.PortalCageStatus.IsCurrentProgress = false
	user.PortalCageStatus.LatestVersion = nowMillis

	h.applySceneGrants(user, questSceneId, nowMillis)
}

func (h *QuestHandler) advanceTutorialsForScene(user *store.UserState, sceneId int32) {
	currentScene, ok := h.SceneById[sceneId]
	if !ok {
		log.Printf("[advanceTutorialsForScene] unknown sceneId=%d", sceneId)
		return
	}
	for _, cond := range h.TutorialUnlockConditions {
		condScene, ok := h.SceneById[cond.ConditionValue]
		if !ok {
			log.Printf("[advanceTutorialsForScene] unknown conditionValue=%d", cond.ConditionValue)
			continue
		}
		if currentScene.SortOrder >= condScene.SortOrder {
			if _, exists := user.Tutorials[cond.TutorialType]; !exists {
				user.Tutorials[cond.TutorialType] = store.TutorialProgressState{
					TutorialType:  cond.TutorialType,
					ProgressPhase: 99999,
				}
			}
		}
	}
}

func (h *QuestHandler) getLastMainFlowSceneId(questId int32) int32 {
	sceneIds := h.SceneIdsByQuestId[questId]
	if len(sceneIds) == 0 {
		panic(fmt.Sprintf("no scenes found for questId=%d", questId))
	}
	return sceneIds[len(sceneIds)-1]
}

func (h *QuestHandler) getChapterLastSceneId(questId int32) int32 {
	if id, ok := h.ChapterLastSceneByQuestId[questId]; ok {
		return id
	}
	return h.getLastMainFlowSceneId(questId)
}

func (h *QuestHandler) HandleReplayFlowSceneProgress(user *store.UserState, questSceneId int32, nowMillis int64) {
	user.MainQuest.ReplayFlowCurrentQuestSceneId = questSceneId
	if user.MainQuest.ReplayFlowHeadQuestSceneId == 0 || h.isSceneAhead(questSceneId, user.MainQuest.ReplayFlowHeadQuestSceneId) {
		user.MainQuest.ReplayFlowHeadQuestSceneId = questSceneId
	}
	user.MainQuest.LatestVersion = nowMillis
	log.Printf("[HandleReplayFlowSceneProgress] sceneId=%d replayHead=%d", questSceneId, user.MainQuest.ReplayFlowHeadQuestSceneId)
}

func (h *QuestHandler) HandleMainQuestSceneProgress(user *store.UserState, questSceneId int32) {
	scene, ok := h.SceneById[questSceneId]
	if !ok {
		panic(fmt.Sprintf("unknown sceneId=%d for HandleMainQuestSceneProgress", questSceneId))
	}

	quest, ok := h.QuestById[scene.QuestId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleMainQuestSceneProgress", questSceneId))
	}

	if isMainQuestPlayable(quest) {
		if model.QuestResultType(scene.QuestResultType) == model.QuestResultTypeHalfResult {
			nowMillis := gametime.NowMillis()
			h.clearQuestMissions(user, quest.QuestId, nowMillis)
		}

		user.MainQuest.ProgressQuestSceneId = questSceneId
		if h.isSceneAhead(questSceneId, user.MainQuest.ProgressHeadQuestSceneId) {
			user.MainQuest.ProgressHeadQuestSceneId = questSceneId
		}
		user.MainQuest.CurrentQuestFlowType = int32(model.QuestFlowTypeSubFlow)
		user.MainQuest.ProgressQuestFlowType = int32(model.QuestFlowTypeSubFlow)
	} else {
		user.MainQuest.CurrentQuestSceneId = questSceneId
		if h.isSceneAhead(questSceneId, user.MainQuest.HeadQuestSceneId) {
			user.MainQuest.HeadQuestSceneId = questSceneId
		}
		lastSceneId := h.getChapterLastSceneId(quest.QuestId)
		user.MainQuest.IsReachedLastQuestScene = questSceneId == lastSceneId
	}
}
