package questflow

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"lunar-tear/server/internal/store"
)

type QuestFlowType int32

const (
	QuestFlowTypeUnknown                QuestFlowType = 0
	QuestFlowTypeMainFlow               QuestFlowType = 1
	QuestFlowTypeSubFlow                QuestFlowType = 2
	QuestFlowTypeReplayFlow             QuestFlowType = 3
	QuestFlowTypeAnotherRouteReplayFlow QuestFlowType = 4
)

func (t QuestFlowType) String() string {
	switch t {
	case QuestFlowTypeUnknown:
		return "unknown"
	case QuestFlowTypeMainFlow:
		return "main-flow"
	case QuestFlowTypeSubFlow:
		return "sub-flow"
	case QuestFlowTypeReplayFlow:
		return "replay-flow"
	case QuestFlowTypeAnotherRouteReplayFlow:
		return "another-route-replay-flow"
	default:
		return fmt.Sprintf("unknown-quest-flow(%d)", int32(t))
	}
}

type sceneMasterRow struct {
	QuestSceneID          int32 `json:"QuestSceneId"`
	QuestID               int32 `json:"QuestId"`
	SortOrder             int32 `json:"SortOrder"`
	QuestSceneType        int32 `json:"QuestSceneType"`
	IsMainFlowQuestTarget bool  `json:"IsMainFlowQuestTarget"`
	IsBattleOnlyTarget    bool  `json:"IsBattleOnlyTarget"`
	QuestResultType       int32 `json:"QuestResultType"`
}

type questMasterRow struct {
	QuestID                      int32 `json:"QuestId"`
	QuestFirstClearRewardGroupID int32 `json:"QuestFirstClearRewardGroupId"`
	QuestMissionGroupID          int32 `json:"QuestMissionGroupId"`
	QuestReleaseConditionListID  int32 `json:"QuestReleaseConditionListId"`
	UserExp                      int32 `json:"UserExp"`
	CharacterExp                 int32 `json:"CharacterExp"`
	CostumeExp                   int32 `json:"CostumeExp"`
	Gold                         int32 `json:"Gold"`
	IsRunInTheBackground         bool  `json:"IsRunInTheBackground"`
	IsCountedAsQuest             bool  `json:"IsCountedAsQuest"`
}

type mainQuestSequenceRow struct {
	MainQuestSequenceID int32 `json:"MainQuestSequenceId"`
	SortOrder           int32 `json:"SortOrder"`
	QuestID             int32 `json:"QuestId"`
}

type questMissionGroupRow struct {
	QuestMissionGroupID int32 `json:"QuestMissionGroupId"`
	SortOrder           int32 `json:"SortOrder"`
	QuestMissionID      int32 `json:"QuestMissionId"`
}

type questMissionRow struct {
	QuestMissionID                    int32 `json:"QuestMissionId"`
	QuestMissionConditionType         int32 `json:"QuestMissionConditionType"`
	QuestMissionRewardID              int32 `json:"QuestMissionRewardId"`
	QuestMissionConditionValueGroupID int32 `json:"QuestMissionConditionValueGroupId"`
}

type questMissionRewardRow struct {
	QuestMissionRewardID int32 `json:"QuestMissionRewardId"`
	PossessionType       int32 `json:"PossessionType"`
	PossessionID         int32 `json:"PossessionId"`
	Count                int32 `json:"Count"`
}

type questFirstClearRewardGroupRow struct {
	QuestFirstClearRewardGroupID int32 `json:"QuestFirstClearRewardGroupId"`
	QuestFirstClearRewardType    int32 `json:"QuestFirstClearRewardType"`
	SortOrder                    int32 `json:"SortOrder"`
	PossessionType               int32 `json:"PossessionType"`
	PossessionID                 int32 `json:"PossessionId"`
	Count                        int32 `json:"Count"`
	IsPickup                     bool  `json:"IsPickup"`
}

type questRelationMainFlowRow struct {
	MainFlowQuestID   int32 `json:"MainFlowQuestId"`
	DifficultyType    int32 `json:"DifficultyType"`
	ReplayFlowQuestID int32 `json:"ReplayFlowQuestId"`
	SubFlowQuestID    int32 `json:"SubFlowQuestId"`
}

type questReleaseConditionListRow struct {
	QuestReleaseConditionListID  int32 `json:"QuestReleaseConditionListId"`
	QuestReleaseConditionGroupID int32 `json:"QuestReleaseConditionGroupId"`
	ConditionOperationType       int32 `json:"ConditionOperationType"`
}

type questReleaseConditionGroupRow struct {
	QuestReleaseConditionGroupID int32 `json:"QuestReleaseConditionGroupId"`
	SortOrder                    int32 `json:"SortOrder"`
	QuestReleaseConditionType    int32 `json:"QuestReleaseConditionType"`
	QuestReleaseConditionID      int32 `json:"QuestReleaseConditionId"`
}

type questReleaseConditionQuestClearRow struct {
	QuestReleaseConditionID int32 `json:"QuestReleaseConditionId"`
	QuestID                 int32 `json:"QuestId"`
}

type questFirstClearRewardSwitchRow struct {
	QuestID                      int32 `json:"QuestId"`
	QuestFirstClearRewardGroupID int32 `json:"QuestFirstClearRewardGroupId"`
	SwitchConditionClearQuestID  int32 `json:"SwitchConditionClearQuestId"`
}

type mainQuestChapterRow struct {
	MainQuestChapterID       int32 `json:"MainQuestChapterId"`
	MainQuestRouteID         int32 `json:"MainQuestRouteId"`
	SortOrder                int32 `json:"SortOrder"`
	MainQuestSequenceGroupID int32 `json:"MainQuestSequenceGroupId"`
}

type Engine struct {
	sceneByID                         map[int32]sceneMasterRow
	questByID                         map[int32]questMasterRow
	questMissionByID                  map[int32]questMissionRow
	mainFlowQuestByQuestID            map[int32]int32
	releaseClearQuestIDsByQuestID     map[int32][]int32
	routeIDByQuestID                  map[int32]int32
	previousQuestByID                 map[int32]int32
	nextQuestByID                     map[int32]int32
	missionIDsByQuestID               map[int32][]int32
	firstClearRewardsByGroupID        map[int32][]questFirstClearRewardGroupRow
	firstClearRewardSwitchesByQuestID map[int32][]questFirstClearRewardSwitchRow
	questMissionRewardsByID           map[int32][]questMissionRewardRow
	terminalSceneIDs                  map[int32]struct{}
	lastMainFlowSceneByQuest          map[int32]int32
}

func MustLoad() *Engine {
	scenes, err := readJSON[sceneMasterRow]("EntityMQuestSceneTable.json")
	if err != nil {
		panic(err)
	}
	quests, err := readJSON[questMasterRow]("EntityMQuestTable.json")
	if err != nil {
		panic(err)
	}
	sequences, err := readJSON[mainQuestSequenceRow]("EntityMMainQuestSequenceTable.json")
	if err != nil {
		panic(err)
	}
	questMissionGroups, err := readJSON[questMissionGroupRow]("EntityMQuestMissionGroupTable.json")
	if err != nil {
		panic(err)
	}
	questMissions, err := readJSON[questMissionRow]("EntityMQuestMissionTable.json")
	if err != nil {
		panic(err)
	}
	questMissionRewards, err := readJSON[questMissionRewardRow]("EntityMQuestMissionRewardTable.json")
	if err != nil {
		panic(err)
	}
	firstClearRewardGroups, err := readJSON[questFirstClearRewardGroupRow]("EntityMQuestFirstClearRewardGroupTable.json")
	if err != nil {
		panic(err)
	}
	questRelations, err := readJSON[questRelationMainFlowRow]("EntityMQuestRelationMainFlowTable.json")
	if err != nil {
		panic(err)
	}
	releaseConditionLists, err := readJSON[questReleaseConditionListRow]("EntityMQuestReleaseConditionListTable.json")
	if err != nil {
		panic(err)
	}
	releaseConditionGroups, err := readJSON[questReleaseConditionGroupRow]("EntityMQuestReleaseConditionGroupTable.json")
	if err != nil {
		panic(err)
	}
	releaseConditionQuestClears, err := readJSON[questReleaseConditionQuestClearRow]("EntityMQuestReleaseConditionQuestClearTable.json")
	if err != nil {
		panic(err)
	}
	firstClearRewardSwitches, err := readJSON[questFirstClearRewardSwitchRow]("EntityMQuestFirstClearRewardSwitchTable.json")
	if err != nil {
		panic(err)
	}
	mainQuestChapters, err := readJSON[mainQuestChapterRow]("EntityMMainQuestChapterTable.json")
	if err != nil {
		panic(err)
	}

	engine := &Engine{
		sceneByID:                         make(map[int32]sceneMasterRow, len(scenes)),
		questByID:                         make(map[int32]questMasterRow, len(quests)),
		questMissionByID:                  make(map[int32]questMissionRow, len(questMissions)),
		mainFlowQuestByQuestID:            make(map[int32]int32),
		releaseClearQuestIDsByQuestID:     make(map[int32][]int32),
		routeIDByQuestID:                  make(map[int32]int32),
		previousQuestByID:                 make(map[int32]int32),
		nextQuestByID:                     make(map[int32]int32),
		missionIDsByQuestID:               make(map[int32][]int32),
		firstClearRewardsByGroupID:        make(map[int32][]questFirstClearRewardGroupRow),
		firstClearRewardSwitchesByQuestID: make(map[int32][]questFirstClearRewardSwitchRow),
		questMissionRewardsByID:           make(map[int32][]questMissionRewardRow),
		terminalSceneIDs:                  make(map[int32]struct{}),
		lastMainFlowSceneByQuest:          make(map[int32]int32),
	}

	for _, scene := range scenes {
		engine.sceneByID[scene.QuestSceneID] = scene
		if scene.QuestResultType == 2 || scene.QuestResultType == 3 {
			engine.terminalSceneIDs[scene.QuestSceneID] = struct{}{}
		}
		if scene.IsMainFlowQuestTarget {
			if existingSceneID, ok := engine.lastMainFlowSceneByQuest[scene.QuestID]; !ok || scene.SortOrder > engine.sceneByID[existingSceneID].SortOrder {
				engine.lastMainFlowSceneByQuest[scene.QuestID] = scene.QuestSceneID
			}
		}
	}

	for _, quest := range quests {
		engine.questByID[quest.QuestID] = quest
	}
	for _, relation := range questRelations {
		if relation.DifficultyType != 1 {
			continue
		}
		engine.mainFlowQuestByQuestID[relation.MainFlowQuestID] = relation.MainFlowQuestID
		if relation.SubFlowQuestID != 0 {
			engine.mainFlowQuestByQuestID[relation.SubFlowQuestID] = relation.MainFlowQuestID
		}
		if relation.ReplayFlowQuestID != 0 {
			engine.mainFlowQuestByQuestID[relation.ReplayFlowQuestID] = relation.MainFlowQuestID
		}
	}
	for _, mission := range questMissions {
		engine.questMissionByID[mission.QuestMissionID] = mission
	}

	sort.Slice(sequences, func(i, j int) bool {
		if sequences[i].MainQuestSequenceID != sequences[j].MainQuestSequenceID {
			return sequences[i].MainQuestSequenceID < sequences[j].MainQuestSequenceID
		}
		if sequences[i].SortOrder != sequences[j].SortOrder {
			return sequences[i].SortOrder < sequences[j].SortOrder
		}
		return sequences[i].QuestID < sequences[j].QuestID
	})
	for i := 0; i+1 < len(sequences); i++ {
		engine.nextQuestByID[sequences[i].QuestID] = sequences[i+1].QuestID
		engine.previousQuestByID[sequences[i+1].QuestID] = sequences[i].QuestID
	}
	chapterBySequenceID := make(map[int32]mainQuestChapterRow, len(mainQuestChapters))
	for _, row := range mainQuestChapters {
		chapterBySequenceID[row.MainQuestSequenceGroupID] = row
	}
	for _, row := range sequences {
		if chapter, ok := chapterBySequenceID[row.MainQuestSequenceID]; ok {
			engine.routeIDByQuestID[row.QuestID] = chapter.MainQuestRouteID
		}
	}

	sort.Slice(questMissionGroups, func(i, j int) bool {
		if questMissionGroups[i].QuestMissionGroupID != questMissionGroups[j].QuestMissionGroupID {
			return questMissionGroups[i].QuestMissionGroupID < questMissionGroups[j].QuestMissionGroupID
		}
		if questMissionGroups[i].SortOrder != questMissionGroups[j].SortOrder {
			return questMissionGroups[i].SortOrder < questMissionGroups[j].SortOrder
		}
		return questMissionGroups[i].QuestMissionID < questMissionGroups[j].QuestMissionID
	})
	missionIDsByGroupID := make(map[int32][]int32)
	for _, row := range questMissionGroups {
		missionIDsByGroupID[row.QuestMissionGroupID] = append(missionIDsByGroupID[row.QuestMissionGroupID], row.QuestMissionID)
	}
	for questID, quest := range engine.questByID {
		missionIDs := missionIDsByGroupID[quest.QuestMissionGroupID]
		if len(missionIDs) == 0 {
			continue
		}
		engine.missionIDsByQuestID[questID] = append([]int32(nil), missionIDs...)
	}
	sort.Slice(firstClearRewardGroups, func(i, j int) bool {
		if firstClearRewardGroups[i].QuestFirstClearRewardGroupID != firstClearRewardGroups[j].QuestFirstClearRewardGroupID {
			return firstClearRewardGroups[i].QuestFirstClearRewardGroupID < firstClearRewardGroups[j].QuestFirstClearRewardGroupID
		}
		if firstClearRewardGroups[i].QuestFirstClearRewardType != firstClearRewardGroups[j].QuestFirstClearRewardType {
			return firstClearRewardGroups[i].QuestFirstClearRewardType < firstClearRewardGroups[j].QuestFirstClearRewardType
		}
		return firstClearRewardGroups[i].SortOrder < firstClearRewardGroups[j].SortOrder
	})
	for _, row := range firstClearRewardGroups {
		engine.firstClearRewardsByGroupID[row.QuestFirstClearRewardGroupID] = append(engine.firstClearRewardsByGroupID[row.QuestFirstClearRewardGroupID], row)
	}
	for _, row := range questMissionRewards {
		engine.questMissionRewardsByID[row.QuestMissionRewardID] = append(engine.questMissionRewardsByID[row.QuestMissionRewardID], row)
	}
	for _, row := range firstClearRewardSwitches {
		engine.firstClearRewardSwitchesByQuestID[row.QuestID] = append(engine.firstClearRewardSwitchesByQuestID[row.QuestID], row)
	}
	clearQuestIDByReleaseConditionID := make(map[int32]int32, len(releaseConditionQuestClears))
	for _, row := range releaseConditionQuestClears {
		clearQuestIDByReleaseConditionID[row.QuestReleaseConditionID] = row.QuestID
	}
	clearQuestIDsByGroupID := make(map[int32][]int32)
	for _, row := range releaseConditionGroups {
		if row.QuestReleaseConditionType != 4 {
			continue
		}
		clearQuestID, ok := clearQuestIDByReleaseConditionID[row.QuestReleaseConditionID]
		if !ok {
			continue
		}
		clearQuestIDsByGroupID[row.QuestReleaseConditionGroupID] = append(clearQuestIDsByGroupID[row.QuestReleaseConditionGroupID], clearQuestID)
	}
	clearQuestIDsByListID := make(map[int32][]int32)
	for _, row := range releaseConditionLists {
		clearQuestIDsByListID[row.QuestReleaseConditionListID] = append(clearQuestIDsByListID[row.QuestReleaseConditionListID], clearQuestIDsByGroupID[row.QuestReleaseConditionGroupID]...)
	}
	for questID, quest := range engine.questByID {
		if quest.QuestReleaseConditionListID == 0 {
			continue
		}
		clearQuestIDs := clearQuestIDsByListID[quest.QuestReleaseConditionListID]
		if len(clearQuestIDs) == 0 {
			continue
		}
		engine.releaseClearQuestIDsByQuestID[questID] = append([]int32(nil), clearQuestIDs...)
	}

	return engine
}

func readJSON[T any](filename string) ([]T, error) {
	path := filepath.Join("assets", "master_data", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var out []T
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return out, nil
}

func (e *Engine) ApplyBootstrap(user *store.UserState, profile store.BootstrapProfile, nowMillis int64) {
	switch profile {
	case "", store.BootstrapProfileFresh:
		return
	case store.BootstrapProfileMainQuestScene9:
		e.HandleMainFlowSceneProgress(user, 9, nowMillis)
	default:
		panic(fmt.Sprintf("unknown bootstrap profile %q", profile))
	}
}

func (e *Engine) mainFlowQuestID(questID int32) int32 {
	if mainFlowQuestID, ok := e.mainFlowQuestByQuestID[questID]; ok && mainFlowQuestID != 0 {
		return mainFlowQuestID
	}
	return questID
}

func (e *Engine) ensureQuestVisible(user *store.UserState, questID int32, active bool, nowMillis int64) {
	if questID == 0 {
		return
	}
	quest := user.Quests[questID]
	quest.QuestID = questID
	if active && quest.QuestStateType == 0 {
		quest.QuestStateType = store.UserQuestStateTypeActive
	}
	if active && quest.LatestStartDatetime == 0 {
		quest.LatestStartDatetime = nowMillis
	}
	user.Quests[questID] = quest

	for _, questMissionID := range e.missionIDsByQuestID[questID] {
		key := store.QuestMissionKey{QuestID: questID, QuestMissionID: questMissionID}
		mission := user.QuestMissions[key]
		mission.QuestID = questID
		mission.QuestMissionID = questMissionID
		user.QuestMissions[key] = mission
	}
}

func (e *Engine) materializeQuestClearState(user *store.UserState, questID int32, grantRewards bool, nowMillis int64) {
	quest := user.Quests[questID]
	grantFirstClearRewards := grantRewards && !quest.IsRewardGranted
	quest.QuestID = questID
	quest.QuestStateType = store.UserQuestStateTypeCleared
	quest.IsBattleOnly = false
	if quest.LatestStartDatetime == 0 {
		quest.LatestStartDatetime = nowMillis
	}
	if quest.ClearCount == 0 {
		quest.ClearCount = 1
	}
	if quest.DailyClearCount == 0 {
		quest.DailyClearCount = 1
	}
	if quest.LastClearDatetime == 0 {
		quest.LastClearDatetime = nowMillis
	}
	if quest.ShortestClearFrames == 0 {
		quest.ShortestClearFrames = 600
	}
	if grantFirstClearRewards {
		quest.IsRewardGranted = true
	}
	user.Quests[questID] = quest

	// Clear missions
	for _, questMissionID := range e.missionIDsByQuestID[questID] {
		key := store.QuestMissionKey{QuestID: questID, QuestMissionID: questMissionID}
		mission := user.QuestMissions[key]
		mission.QuestID = questID
		mission.QuestMissionID = questMissionID

		missionMaster, ok := e.questMissionByID[questMissionID]
		if !ok {
			user.QuestMissions[key] = mission
			continue
		}
		// Auto-clear non-9999 missions (server auto-completes since we skip real battle)
		if missionMaster.QuestMissionConditionType != 9999 {
			if !mission.IsClear {
				mission.IsClear = true
				mission.ProgressValue = 1
				mission.LatestClearDatetime = nowMillis
			}
		}
		user.QuestMissions[key] = mission
	}

	// Now check if all non-9999 missions are cleared -> auto-clear 9999 (COMPLETE)
	allNonCompleteCleared := true
	for _, questMissionID := range e.missionIDsByQuestID[questID] {
		missionMaster, ok := e.questMissionByID[questMissionID]
		if !ok || missionMaster.QuestMissionConditionType == 9999 {
			continue
		}
		key := store.QuestMissionKey{QuestID: questID, QuestMissionID: questMissionID}
		if !user.QuestMissions[key].IsClear {
			allNonCompleteCleared = false
			break
		}
	}
	if allNonCompleteCleared {
		for _, questMissionID := range e.missionIDsByQuestID[questID] {
			missionMaster, ok := e.questMissionByID[questMissionID]
			if !ok || missionMaster.QuestMissionConditionType != 9999 {
				continue
			}
			key := store.QuestMissionKey{QuestID: questID, QuestMissionID: questMissionID}
			m := user.QuestMissions[key]
			if !m.IsClear {
				m.IsClear = true
				m.ProgressValue = 1
				m.LatestClearDatetime = nowMillis
				user.QuestMissions[key] = m
			}
		}
	}

}

func (e *Engine) unlockReleasedQuests(user *store.UserState, clearedQuestID int32, nowMillis int64) bool {
	unlockedAny := false
	for questID, requiredClearQuestIDs := range e.releaseClearQuestIDsByQuestID {
		if len(requiredClearQuestIDs) == 0 {
			continue
		}
		allCleared := true
		for _, requiredQuestID := range requiredClearQuestIDs {
			if !e.isQuestCleared(user, requiredQuestID) {
				allCleared = false
				break
			}
		}
		if !allCleared {
			continue
		}
		e.ensureQuestVisible(user, questID, false, nowMillis)
		unlockedAny = true
	}
	return unlockedAny
}

func (e *Engine) isQuestCleared(user *store.UserState, questID int32) bool {
	if quest, ok := user.Quests[questID]; ok && quest.QuestStateType == store.UserQuestStateTypeCleared {
		return true
	}
	return false
}

func (e *Engine) effectiveFirstClearRewardGroupID(user *store.UserState, questMeta questMasterRow) int32 {
	rewardGroupID := questMeta.QuestFirstClearRewardGroupID
	for _, switchRow := range e.firstClearRewardSwitchesByQuestID[questMeta.QuestID] {
		if e.isQuestCleared(user, switchRow.SwitchConditionClearQuestID) {
			rewardGroupID = switchRow.QuestFirstClearRewardGroupID
			break
		}
	}
	return rewardGroupID
}

// HandleMainFlowSceneProgress updates MainFlowStatus and FlowStatus tables only
func (e *Engine) HandleMainFlowSceneProgress(user *store.UserState, sceneID int32, nowMillis int64) {
	scene, ok := e.sceneByID[sceneID]
	if !ok {
		log.Printf("[QuestFlow] HandleMainFlowSceneProgress: unknown sceneId=%d", sceneID)
		return
	}

	// Ensure the scene's quest is visible (creates IUserQuest + IUserQuestMission rows)
	e.ensureQuestVisible(user, scene.QuestID, false, nowMillis)

	// Auto-clear previous background/non-counted quests that should be done
	prevQuestID := e.previousQuestByID[e.mainFlowQuestID(scene.QuestID)]
	if prevQuestID != 0 {
		prevMeta, ok := e.questByID[prevQuestID]
		if ok && (prevMeta.IsRunInTheBackground || !prevMeta.IsCountedAsQuest) {
			prevRow := user.Quests[prevQuestID]
			if prevRow.QuestStateType != store.UserQuestStateTypeCleared {
				e.materializeQuestClearState(user, prevQuestID, false, nowMillis)
			}
		}
	}

	// Update IUserMainQuestMainFlowStatus fields
	user.MainQuest.CurrentQuestSceneID = sceneID
	if sceneID > user.MainQuest.HeadQuestSceneID {
		user.MainQuest.HeadQuestSceneID = sceneID
	}

	// Update route from scene's quest
	routeID := e.routeIDByQuestID[e.mainFlowQuestID(scene.QuestID)]
	if routeID != 0 {
		user.MainQuest.CurrentMainQuestRouteID = routeID
	}

	// Update IUserMainQuestFlowStatus
	user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeMainFlow)

	// DO NOT touch ProgressStatus fields — that is HandleQuestSceneProgress's job
}

// HandleQuestSceneProgress updates ProgressStatus and FlowStatus tables, handles terminal scenes
func (e *Engine) HandleQuestSceneProgress(user *store.UserState, sceneID int32, nowMillis int64) {
	scene, ok := e.sceneByID[sceneID]
	if !ok {
		log.Printf("[QuestFlow] HandleQuestSceneProgress: unknown sceneId=%d", sceneID)
		return
	}

	// Ensure quest row exists and is active
	e.ensureQuestVisible(user, scene.QuestID, true, nowMillis)
	quest := user.Quests[scene.QuestID]
	if quest.QuestStateType != store.UserQuestStateTypeCleared {
		quest.QuestStateType = store.UserQuestStateTypeActive
	}
	if quest.LatestStartDatetime == 0 {
		quest.LatestStartDatetime = nowMillis
	}
	user.Quests[scene.QuestID] = quest

	// Update IUserMainQuestProgressStatus fields
	user.MainQuest.ProgressQuestSceneID = sceneID
	if sceneID > user.MainQuest.ProgressHeadQuestSceneID {
		user.MainQuest.ProgressHeadQuestSceneID = sceneID
	}
	user.MainQuest.ProgressQuestFlowType = int32(QuestFlowTypeSubFlow)

	// Update IUserMainQuestFlowStatus
	user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeSubFlow)

	// If this is a terminal scene, mark quest as cleared immediately
	// (client reads IUserQuest.questStateType after scene progress to check IsClearedQuestWithQuestId)
	if _, isTerminal := e.terminalSceneIDs[sceneID]; isTerminal {
		e.materializeQuestClearState(user, scene.QuestID, false, nowMillis)
	}

	// DO NOT touch MainFlowStatus scene pointers — that is HandleMainFlowSceneProgress's job
}

// HandleQuestStart sets quest to active, creates mission rows
func (e *Engine) HandleQuestStart(user *store.UserState, questID int32, isBattleOnly bool, nowMillis int64) {
	// Ensure quest and mission rows exist
	e.ensureQuestVisible(user, questID, true, nowMillis)

	quest := user.Quests[questID]
	quest.QuestID = questID
	if quest.QuestStateType != store.UserQuestStateTypeCleared {
		quest.QuestStateType = store.UserQuestStateTypeActive
	}
	quest.IsBattleOnly = isBattleOnly
	quest.LatestStartDatetime = nowMillis
	user.Quests[questID] = quest

	// No MainFlowStatus or ProgressStatus changes — those are driven by scene progress RPCs
}

// HandleQuestFinish clears quest, grants rewards, resets progress status
func (e *Engine) HandleQuestFinish(user *store.UserState, questID int32, isMainFlow bool, nowMillis int64) {
	e.ensureQuestVisible(user, questID, true, nowMillis)

	// Compute rewards BEFORE mutating state (buildFinishOutcome reads current IsClear flags)
	// outcome := e.buildFinishOutcome(user, questID)

	// Mark quest cleared + clear missions + grant rewards
	e.materializeQuestClearState(user, questID, true, nowMillis)

	// Unlock next quests via release conditions
	mainFlowID := e.mainFlowQuestID(questID)
	if !e.unlockReleasedQuests(user, questID, nowMillis) {
		if nextQuestID, ok := e.nextQuestByID[mainFlowID]; ok && nextQuestID != 0 {
			e.ensureQuestVisible(user, nextQuestID, false, nowMillis)
		}
	}

	// Reset IUserMainQuestProgressStatus (quest is done, no in-quest progress)
	user.MainQuest.ProgressQuestSceneID = 0
	user.MainQuest.ProgressHeadQuestSceneID = 0
	user.MainQuest.ProgressQuestFlowType = 0

	// Reset IUserMainQuestFlowStatus
	user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeUnknown)

	// Update isReachedLastQuestScene — only true if the current MainFlowStatus scene
	// is the last main-flow-target scene for this quest
	lastMainFlowScene := e.lastMainFlowSceneByQuest[mainFlowID]
	user.MainQuest.IsReachedLastQuestScene = lastMainFlowScene != 0 &&
		user.MainQuest.CurrentQuestSceneID >= lastMainFlowScene

	// return outcome
}

// HandleQuestRestart resets quest and mission progress for replay
func (e *Engine) HandleQuestRestart(user *store.UserState, questID int32, nowMillis int64) {
	quest := user.Quests[questID]
	quest.QuestID = questID
	quest.QuestStateType = store.UserQuestStateTypeActive
	quest.IsBattleOnly = false
	quest.LatestStartDatetime = nowMillis
	user.Quests[questID] = quest

	// Reset mission progress for this quest
	for _, questMissionID := range e.missionIDsByQuestID[questID] {
		key := store.QuestMissionKey{QuestID: questID, QuestMissionID: questMissionID}
		m := user.QuestMissions[key]
		m.QuestID = questID
		m.QuestMissionID = questMissionID
		m.IsClear = false
		m.ProgressValue = 0
		m.LatestClearDatetime = 0
		user.QuestMissions[key] = m
	}
}
