package questflow

import (
	"fmt"
	"lunar-tear/server/internal/store"
	"sort"
	"time"
)

type QuestResultType int32

const (
	QuestResultTypeUnknown    QuestResultType = 0
	QuestResultTypeNone       QuestResultType = 1
	QuestResultTypeHalfResult QuestResultType = 2
	QuestResultTypeFullResult QuestResultType = 3
)

type QuestSceneType int32

const (
	QuestSceneTypeUnknown      QuestSceneType = 0
	QuestSceneTypeTower        QuestSceneType = 1
	QuestSceneTypePictureBook  QuestSceneType = 2
	QuestSceneTypeField        QuestSceneType = 3
	QuestSceneTypeNovel        QuestSceneType = 4
	QuestSceneTypeLimitContent QuestSceneType = 5
)

type questScene struct {
	QuestSceneId          int32           `json:"QuestSceneId"`
	QuestId               int32           `json:"QuestId"`
	SortOrder             int32           `json:"SortOrder"`
	QuestSceneType        QuestSceneType  `json:"QuestSceneType"`
	AssetBackgroundId     int32           `json:"AssetBackgroundId"`
	EventMapNumberUpper   int32           `json:"EventMapNumberUpper"`
	EventMapNumberLower   int32           `json:"EventMapNumberLower"`
	IsMainFlowQuestTarget bool            `json:"IsMainFlowQuestTarget"`
	IsBattleOnlyTarget    bool            `json:"IsBattleOnlyTarget"`
	QuestResultType       QuestResultType `json:"QuestResultType"`
	IsStorySkipTarget     bool            `json:"IsStorySkipTarget"`
}

type missionGroup struct {
	QuestMissionGroupId int32 `json:"QuestMissionGroupId"`
	SortOrder           int32 `json:"SortOrder"`
	QuestMissionId      int32 `json:"QuestMissionId"`
}

type quest struct {
	QuestId                      int32 `json:"QuestId"`
	NameQuestTextId              int32 `json:"NameQuestTextId"`
	PictureBookNameQuestTextId   int32 `json:"PictureBookNameQuestTextId"`
	QuestReleaseConditionListId  int32 `json:"QuestReleaseConditionListId"`
	StoryQuestTextId             int32 `json:"StoryQuestTextId"`
	QuestDisplayAttributeGroupId int32 `json:"QuestDisplayAttributeGroupId"`
	RecommendedDeckPower         int32 `json:"RecommendedDeckPower"`
	QuestFirstClearRewardGroupId int32 `json:"QuestFirstClearRewardGroupId"`
	QuestPickupRewardGroupId     int32 `json:"QuestPickupRewardGroupId"`
	QuestDeckRestrictionGroupId  int32 `json:"QuestDeckRestrictionGroupId"`
	QuestMissionGroupId          int32 `json:"QuestMissionGroupId"`
	Stamina                      int32 `json:"Stamina"`
	UserExp                      int32 `json:"UserExp"`
	CharacterExp                 int32 `json:"CharacterExp"`
	CostumeExp                   int32 `json:"CostumeExp"`
	Gold                         int32 `json:"Gold"`
	DailyClearableCount          int32 `json:"DailyClearableCount"`
	IsRunInTheBackground         bool  `json:"IsRunInTheBackground"`
	IsCountedAsQuest             bool  `json:"IsCountedAsQuest"`
	QuestBonusId                 int32 `json:"QuestBonusId"`
	IsNotShowAfterClear          bool  `json:"IsNotShowAfterClear"`
	IsBigWinTarget               bool  `json:"IsBigWinTarget"`
	IsUsableSkipTicket           bool  `json:"IsUsableSkipTicket"`
	QuestReplayFlowRewardGroupId int32 `json:"QuestReplayFlowRewardGroupId"`
	InvisibleQuestMissionGroupId int32 `json:"InvisibleQuestMissionGroupId"`
	FieldEffectGroupId           int32 `json:"FieldEffectGroupId"`
}

type mainQuestSequence struct {
	MainQuestSequenceId int32 `json:"MainQuestSequenceId"`
	SortOrder           int32 `json:"SortOrder"`
	QuestID             int32 `json:"QuestId"`
}

type mainQuestChapter struct {
	MainQuestChapterId         int32 `json:"MainQuestChapterId"`
	MainQuestRouteId           int32 `json:"MainQuestRouteId"`
	SortOrder                  int32 `json:"SortOrder"`
	MainQuestSequenceGroupId   int32 `json:"MainQuestSequenceGroupId"`
	PortalCageCharacterGroupId int32 `json:"PortalCageCharacterGroupId"`
	StartDatetime              int64 `json:"StartDatetime"`
	IsInvisibleInLibrary       bool  `json:"IsInvisibleInLibrary"`
	JoinLibraryChapterId       int32 `json:"JoinLibraryChapterId"`
}

type NewEngine struct {
	sceneById           map[int32]questScene
	questById           map[int32]quest
	missionIdsByQuestId map[int32][]int32
	routeIdByQuestId    map[int32]int32
}

func MakeNewEngine() *NewEngine {
	scenes, err := readJSON[questScene]("EntityMQuestSceneTable.json")
	if err != nil {
		panic(err)
	}

	quests, err := readJSON[quest]("EntityMQuestTable.json")
	if err != nil {
		panic(err)
	}

	missionGroups, err := readJSON[missionGroup]("EntityMQuestMissionGroupTable.json")
	if err != nil {
		panic(err)
	}
	sort.Slice(missionGroups, func(i, j int) bool {
		if missionGroups[i].QuestMissionGroupId != missionGroups[j].QuestMissionGroupId {
			return missionGroups[i].QuestMissionGroupId < missionGroups[j].QuestMissionGroupId
		}
		if missionGroups[i].SortOrder != missionGroups[j].SortOrder {
			return missionGroups[i].SortOrder < missionGroups[j].SortOrder
		}
		return missionGroups[i].QuestMissionId < missionGroups[j].QuestMissionId
	})

	sequences, err := readJSON[mainQuestSequence]("EntityMMainQuestSequenceTable.json")
	if err != nil {
		panic(err)
	}
	sort.Slice(sequences, func(i, j int) bool {
		if sequences[i].MainQuestSequenceId != sequences[j].MainQuestSequenceId {
			return sequences[i].MainQuestSequenceId < sequences[j].MainQuestSequenceId
		}
		if sequences[i].SortOrder != sequences[j].SortOrder {
			return sequences[i].SortOrder < sequences[j].SortOrder
		}
		return sequences[i].QuestID < sequences[j].QuestID
	})

	mainQuestChapters, err := readJSON[mainQuestChapter]("EntityMMainQuestChapterTable.json")
	if err != nil {
		panic(err)
	}

	engine := &NewEngine{
		sceneById:           make(map[int32]questScene, len(scenes)),
		questById:           make(map[int32]quest, len(quests)),
		missionIdsByQuestId: make(map[int32][]int32),
		routeIdByQuestId:    make(map[int32]int32),
	}

	for _, scene := range scenes {
		engine.sceneById[scene.QuestSceneId] = scene
	}

	for _, quest := range quests {
		engine.questById[quest.QuestId] = quest
	}

	missionIdsByGroupId := make(map[int32][]int32, len(missionGroups))
	for _, missionGroup := range missionGroups {
		missionIdsByGroupId[missionGroup.QuestMissionGroupId] = append(
			missionIdsByGroupId[missionGroup.QuestMissionGroupId],
			missionGroup.QuestMissionId,
		)
	}
	for questId, quest := range engine.questById {
		missionIDs := missionIdsByGroupId[quest.QuestMissionGroupId]
		if len(missionIDs) == 0 {
			continue
		}
		engine.missionIdsByQuestId[questId] = append([]int32(nil), missionIDs...)
	}

	chapterBySequenceID := make(map[int32]mainQuestChapter, len(mainQuestChapters))
	for _, chapter := range mainQuestChapters {
		chapterBySequenceID[chapter.MainQuestSequenceGroupId] = chapter
	}
	for _, sequence := range sequences {
		if chapter, ok := chapterBySequenceID[sequence.MainQuestSequenceId]; ok {
			engine.routeIdByQuestId[sequence.QuestID] = chapter.MainQuestRouteId
		}
	}

	return engine
}

func (e *NewEngine) initQuestState(user *store.UserState, questID int32) {
	quest := user.Quests[questID]
	quest.QuestID = questID
	user.Quests[questID] = quest

	for _, missionID := range e.missionIdsByQuestId[questID] {
		key := store.QuestMissionKey{QuestID: questID, QuestMissionID: missionID}
		mission := user.QuestMissions[key]
		mission.QuestID = questID
		mission.QuestMissionID = missionID
		user.QuestMissions[key] = mission
	}
}

func (e *NewEngine) HandleQuestStart(user *store.UserState, questID int32, isMainFlow bool, isBattleOnly bool) {
	if _, ok := e.questById[questID]; !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleQuestStart", questID))
	}

	e.initQuestState(user, questID)

	quest := user.Quests[questID]
	quest.QuestStateType = store.UserQuestStateTypeActive
	quest.IsBattleOnly = isBattleOnly
	quest.LatestStartDatetime = time.Now().UnixMilli()
	user.Quests[questID] = quest
}

func (e *NewEngine) HandleMainFlowSceneProgress(user *store.UserState, questSceneId int32) {
	scene, ok := e.sceneById[questSceneId]
	if !ok {
		panic(fmt.Sprintf("unknown sceneId=%d for HandleMainFlowSceneProgress", questSceneId))
	}

	quest, ok := e.questById[scene.QuestId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleMainFlowSceneProgress", questSceneId))
	}

	user.MainQuest.CurrentQuestSceneID = questSceneId
	if questSceneId > user.MainQuest.HeadQuestSceneID {
		user.MainQuest.HeadQuestSceneID = questSceneId
	}

	user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeMainFlow)

	routeId, ok := e.routeIdByQuestId[quest.QuestId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleMainFlowSceneProgress setting currentMainQuestRouteId", quest.QuestId))
	}
	user.MainQuest.CurrentMainQuestRouteID = routeId
}
