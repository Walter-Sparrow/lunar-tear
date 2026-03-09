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

const (
	questSceneTypeStory      = int32(1)
	questSceneTypeTransition = int32(2)
	questSceneTypeBattle     = int32(3)
	questStateTypeActive     = int32(1)
	questStateTypeCleared    = int32(3)
)

type SceneUpdateMode int

const (
	SceneUpdateModeMainFlow SceneUpdateMode = iota + 1
	SceneUpdateModeQuestProgress
)

func (m SceneUpdateMode) String() string {
	switch m {
	case SceneUpdateModeMainFlow:
		return "main-flow"
	case SceneUpdateModeQuestProgress:
		return "quest-progress"
	default:
		return fmt.Sprintf("unknown-mode(%d)", int(m))
	}
}

type ScenePhase int

const (
	ScenePhaseUnknown ScenePhase = iota
	ScenePhaseBackground
	ScenePhaseRunning
	ScenePhaseTransition
	ScenePhaseBattleEntry
	ScenePhaseTerminal
	ScenePhasePostClearTail
)

func (p ScenePhase) String() string {
	switch p {
	case ScenePhaseUnknown:
		return "unknown"
	case ScenePhaseBackground:
		return "background"
	case ScenePhaseRunning:
		return "running"
	case ScenePhaseTransition:
		return "transition"
	case ScenePhaseBattleEntry:
		return "battle-entry"
	case ScenePhaseTerminal:
		return "terminal"
	case ScenePhasePostClearTail:
		return "post-clear-tail"
	default:
		return fmt.Sprintf("unknown-phase(%d)", int(p))
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
	QuestID              int32 `json:"QuestId"`
	QuestMissionGroupID  int32 `json:"QuestMissionGroupId"`
	IsRunInTheBackground bool  `json:"IsRunInTheBackground"`
	IsCountedAsQuest     bool  `json:"IsCountedAsQuest"`
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

type SceneDescriptor struct {
	SceneID           int32
	QuestID           int32
	PreviousQuestID   int32
	NextQuestID       int32
	MissionIDs        []int32
	Phase             ScenePhase
	IsCountedQuest    bool
	IsBackgroundQuest bool
}

type Engine struct {
	sceneByID             map[int32]sceneMasterRow
	questByID             map[int32]questMasterRow
	previousQuestByID     map[int32]int32
	nextQuestByID         map[int32]int32
	missionIDsByQuestID   map[int32][]int32
	firstTerminalSortByID map[int32]int32
	lastSceneSortByID     map[int32]int32
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

	engine := &Engine{
		sceneByID:             make(map[int32]sceneMasterRow, len(scenes)),
		questByID:             make(map[int32]questMasterRow, len(quests)),
		previousQuestByID:     make(map[int32]int32),
		nextQuestByID:         make(map[int32]int32),
		missionIDsByQuestID:   make(map[int32][]int32),
		firstTerminalSortByID: make(map[int32]int32),
		lastSceneSortByID:     make(map[int32]int32),
	}
	for _, scene := range scenes {
		engine.sceneByID[scene.QuestSceneID] = scene
		if scene.SortOrder > engine.lastSceneSortByID[scene.QuestID] {
			engine.lastSceneSortByID[scene.QuestID] = scene.SortOrder
		}
		if scene.QuestResultType == 2 || scene.QuestResultType == 3 {
			current, ok := engine.firstTerminalSortByID[scene.QuestID]
			if !ok || scene.SortOrder < current {
				engine.firstTerminalSortByID[scene.QuestID] = scene.SortOrder
			}
		}
	}
	for _, quest := range quests {
		engine.questByID[quest.QuestID] = quest
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
		e.ApplySceneTransition(user, 9, SceneUpdateModeMainFlow, nowMillis)
	default:
		panic(fmt.Sprintf("unknown bootstrap profile %q", profile))
	}
}

func (e *Engine) DescribeScene(sceneID int32) (SceneDescriptor, bool) {
	if e == nil {
		log.Printf("[QuestFlow] not-implemented quest transition: sceneId=%d reason=nil-engine", sceneID)
		return SceneDescriptor{}, false
	}
	scene, ok := e.sceneByID[sceneID]
	if !ok {
		log.Printf("[QuestFlow] unknown quest transition: sceneId=%d reason=missing-scene-master-row", sceneID)
		return SceneDescriptor{}, false
	}
	quest, ok := e.questByID[scene.QuestID]
	if !ok {
		log.Printf("[QuestFlow] unknown quest transition: sceneId=%d questId=%d reason=missing-quest-master-row", sceneID, scene.QuestID)
		return SceneDescriptor{}, false
	}
	descriptor := SceneDescriptor{
		SceneID:           scene.QuestSceneID,
		QuestID:           scene.QuestID,
		PreviousQuestID:   e.previousQuestByID[scene.QuestID],
		NextQuestID:       e.nextQuestByID[scene.QuestID],
		MissionIDs:        append([]int32(nil), e.missionIDsByQuestID[scene.QuestID]...),
		IsCountedQuest:    quest.IsCountedAsQuest,
		IsBackgroundQuest: quest.IsRunInTheBackground || !quest.IsCountedAsQuest,
		Phase:             ScenePhaseUnknown,
	}

	switch {
	case isPostClearTail(e.firstTerminalSortByID[scene.QuestID], scene):
		descriptor.Phase = ScenePhasePostClearTail
	case scene.QuestResultType == 2 || scene.QuestResultType == 3:
		descriptor.Phase = ScenePhaseTerminal
	case descriptor.IsBackgroundQuest && scene.SortOrder == e.lastSceneSortByID[scene.QuestID]:
		descriptor.Phase = ScenePhaseTerminal
	case scene.IsBattleOnlyTarget || scene.QuestSceneType == questSceneTypeBattle:
		descriptor.Phase = ScenePhaseBattleEntry
	case scene.QuestSceneType != questSceneTypeStory:
		descriptor.Phase = ScenePhaseTransition
	case descriptor.IsBackgroundQuest || !scene.IsMainFlowQuestTarget:
		descriptor.Phase = ScenePhaseRunning
	default:
		descriptor.Phase = ScenePhaseTransition
	}

	return descriptor, true
}

func isPostClearTail(firstTerminalSort int32, scene sceneMasterRow) bool {
	return firstTerminalSort != 0 && scene.SortOrder > firstTerminalSort
}

func (e *Engine) ApplySceneTransition(user *store.UserState, sceneID int32, mode SceneUpdateMode, nowMillis int64) (SceneDescriptor, bool) {
	descriptor, ok := e.DescribeScene(sceneID)
	if !ok {
		log.Printf("[QuestFlow] not-implemented quest transition: sceneId=%d mode=%s", sceneID, mode.String())
		return SceneDescriptor{}, false
	}
	if descriptor.Phase == ScenePhaseUnknown {
		log.Printf("[QuestFlow] unknown quest transition: sceneId=%d questId=%d mode=%s phase=%s", sceneID, descriptor.QuestID, mode.String(), descriptor.Phase.String())
	}

	e.ensureQuestVisible(user, descriptor.QuestID, true, nowMillis)
	e.reconcileQuestHandoff(user, descriptor, nowMillis)

	user.MainQuest.CurrentQuestSceneID = sceneID
	user.MainQuest.HeadQuestSceneID = sceneID
	user.MainQuest.ActiveQuestID = descriptor.QuestID

	if descriptor.Phase == ScenePhaseTerminal {
		if !descriptor.IsBackgroundQuest {
			e.markQuestCleared(user, descriptor.QuestID, nowMillis)
		}
		user.MainQuest.ClearReadyQuestID = descriptor.QuestID
		if descriptor.NextQuestID != 0 {
			e.ensureQuestVisible(user, descriptor.NextQuestID, false, nowMillis)
		}
	} else if descriptor.Phase != ScenePhasePostClearTail {
		user.MainQuest.ClearReadyQuestID = 0
	}

	activeFlow := false
	reachedLast := false
	switch mode {
	case SceneUpdateModeMainFlow:
		activeFlow = descriptor.Phase == ScenePhaseRunning || descriptor.Phase == ScenePhaseTransition
		reachedLast = !activeFlow
	case SceneUpdateModeQuestProgress:
		activeFlow = descriptor.Phase == ScenePhaseRunning || (descriptor.Phase == ScenePhaseTerminal && !descriptor.IsBackgroundQuest)
		reachedLast = descriptor.Phase == ScenePhaseTerminal || !activeFlow
	}

	user.MainQuest.IsReachedLastQuestScene = reachedLast
	if activeFlow {
		user.MainQuest.CurrentQuestFlowType = 1
		user.MainQuest.ProgressQuestSceneID = sceneID
		user.MainQuest.ProgressHeadQuestSceneID = sceneID
		user.MainQuest.ProgressQuestFlowType = 1
	} else {
		user.MainQuest.CurrentQuestFlowType = 0
		user.MainQuest.ProgressQuestSceneID = 0
		user.MainQuest.ProgressHeadQuestSceneID = 0
		user.MainQuest.ProgressQuestFlowType = 0
	}

	return descriptor, true
}

func (e *Engine) ApplyQuestStart(user *store.UserState, questID int32, isBattleOnly bool, nowMillis int64) {
	questMeta, ok := e.questByID[questID]
	if !ok {
		log.Printf("[QuestFlow] unknown quest transition: start questId=%d reason=missing-quest-master-row", questID)
	}
	descriptor := SceneDescriptor{
		QuestID:           questID,
		PreviousQuestID:   e.previousQuestByID[questID],
		IsCountedQuest:    questMeta.IsCountedAsQuest,
		IsBackgroundQuest: questMeta.IsRunInTheBackground || !questMeta.IsCountedAsQuest,
	}
	e.ensureQuestVisible(user, questID, true, nowMillis)
	e.reconcileQuestHandoff(user, descriptor, nowMillis)

	quest := user.Quests[questID]
	quest.QuestID = questID
	quest.IsBattleOnly = isBattleOnly
	quest.QuestStateType = questStateTypeActive
	quest.LatestStartDatetime = nowMillis
	user.Quests[questID] = quest
	user.MainQuest.ActiveQuestID = questID
}

func (e *Engine) ApplyQuestFinish(user *store.UserState, questID int32, isMainFlow bool, nowMillis int64) {
	switch clearReadyQuestID := user.MainQuest.ClearReadyQuestID; {
	case clearReadyQuestID == 0:
		log.Printf("[QuestFlow] not-implemented quest transition: finish questId=%d reason=missing-clear-ready currentSceneId=%d activeQuestId=%d",
			questID, user.MainQuest.CurrentQuestSceneID, user.MainQuest.ActiveQuestID)
	case clearReadyQuestID != questID:
		log.Printf("[QuestFlow] unknown quest transition: finish questId=%d clearReadyQuestId=%d currentSceneId=%d activeQuestId=%d",
			questID, clearReadyQuestID, user.MainQuest.CurrentQuestSceneID, user.MainQuest.ActiveQuestID)
	}

	e.ensureQuestVisible(user, questID, true, nowMillis)

	e.markQuestCleared(user, questID, nowMillis)

	if isMainFlow {
		if nextQuestID, ok := e.nextQuestByID[questID]; ok && nextQuestID != 0 {
			e.ensureQuestVisible(user, nextQuestID, false, nowMillis)
		} else {
			log.Printf("[QuestFlow] not-implemented quest transition: finish questId=%d reason=missing-next-main-quest", questID)
		}
	}

	if user.MainQuest.ActiveQuestID == questID {
		user.MainQuest.ActiveQuestID = 0
	}
	if user.MainQuest.ClearReadyQuestID == questID {
		user.MainQuest.ClearReadyQuestID = 0
	}
	user.MainQuest.IsReachedLastQuestScene = true
	user.MainQuest.CurrentQuestFlowType = 0
	user.MainQuest.ProgressQuestSceneID = 0
	user.MainQuest.ProgressHeadQuestSceneID = 0
	user.MainQuest.ProgressQuestFlowType = 0
}

func (e *Engine) reconcileQuestHandoff(user *store.UserState, descriptor SceneDescriptor, nowMillis int64) {
	if descriptor.QuestID == 0 || !descriptor.IsCountedQuest {
		return
	}
	previousQuestID := descriptor.PreviousQuestID
	if previousQuestID == 0 {
		return
	}
	previousQuest, ok := e.questByID[previousQuestID]
	if !ok {
		return
	}
	if !previousQuest.IsRunInTheBackground && previousQuest.IsCountedAsQuest {
		return
	}

	row := user.Quests[previousQuestID]
	if row.QuestStateType == questStateTypeCleared {
		return
	}
	e.markQuestCleared(user, previousQuestID, nowMillis)
}

func (e *Engine) ensureQuestVisible(user *store.UserState, questID int32, active bool, nowMillis int64) {
	if questID == 0 {
		return
	}
	quest := user.Quests[questID]
	quest.QuestID = questID
	if active && quest.QuestStateType == 0 {
		quest.QuestStateType = questStateTypeActive
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

func (e *Engine) markQuestCleared(user *store.UserState, questID int32, nowMillis int64) {
	quest := user.Quests[questID]
	quest.QuestID = questID
	quest.QuestStateType = questStateTypeCleared
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
	user.Quests[questID] = quest
}
