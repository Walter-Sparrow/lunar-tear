package questflow

import (
	"fmt"
	"log"
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

type QuestMissionConditionType int

const (
	QuestMissionConditionTypeUnknown                                   QuestMissionConditionType = 0
	QuestMissionConditionTypeLessThanOrEqualXPeopleNotAlive            QuestMissionConditionType = 1
	QuestMissionConditionTypeMaxDamage                                 QuestMissionConditionType = 2
	QuestMissionConditionTypeSpecifiedCostumeIsInDeck                  QuestMissionConditionType = 3
	QuestMissionConditionTypeSpecifiedCharacterIsInDeck                QuestMissionConditionType = 4
	QuestMissionConditionTypeSpecifiedAttributeMainWeaponIsInDeck      QuestMissionConditionType = 5
	QuestMissionConditionTypeGreaterThanOrEqualXCostumeSkillUseCount   QuestMissionConditionType = 6
	QuestMissionConditionTypeGreaterThanOrEqualXWeaponSkillUseCount    QuestMissionConditionType = 7
	QuestMissionConditionTypeGreaterThanOrEqualXCompanionSkillUseCount QuestMissionConditionType = 8
	QuestMissionConditionTypeCostumeSkillfulWeaponAllCharacter         QuestMissionConditionType = 9
	QuestMissionConditionTypeCostumeSkillfulWeaponAnyCharacter         QuestMissionConditionType = 10
	QuestMissionConditionTypeCostumeRarityEqAllCharacter               QuestMissionConditionType = 11
	QuestMissionConditionTypeCostumeRarityGeAllCharacter               QuestMissionConditionType = 12
	QuestMissionConditionTypeCostumeRarityLeAllCharacter               QuestMissionConditionType = 13
	QuestMissionConditionTypeCostumeRarityEqAnyCharacter               QuestMissionConditionType = 14
	QuestMissionConditionTypeCostumeRarityGeAnyCharacter               QuestMissionConditionType = 15
	QuestMissionConditionTypeCostumeRarityLeAnyCharacter               QuestMissionConditionType = 16
	QuestMissionConditionTypeWeaponEvolutionGroupId                    QuestMissionConditionType = 17
	QuestMissionConditionTypeSpecifiedAttributeWeaponIsInDeck          QuestMissionConditionType = 18
	QuestMissionConditionTypeSpecifiedAttributeMainWeaponAllCharacter  QuestMissionConditionType = 19
	QuestMissionConditionTypeSpecifiedAttributeWeaponAllCharacter      QuestMissionConditionType = 20
	QuestMissionConditionTypeWeaponManSkillfulWeaponAllCharacter       QuestMissionConditionType = 21
	QuestMissionConditionTypeWeaponSkillfulWeaponAllCharacter          QuestMissionConditionType = 22
	QuestMissionConditionTypeWeaponManSkillfulWeaponAnyCharacter       QuestMissionConditionType = 23
	QuestMissionConditionTypeWeaponSkillfulWeaponAnyCharacter          QuestMissionConditionType = 24
	QuestMissionConditionTypeWeaponRarityEqAllCharacter                QuestMissionConditionType = 25
	QuestMissionConditionTypeWeaponRarityGeAllCharacter                QuestMissionConditionType = 26
	QuestMissionConditionTypeWeaponRarityLeAllCharacter                QuestMissionConditionType = 27
	QuestMissionConditionTypeWeaponMainRarityEqAllCharacter            QuestMissionConditionType = 28
	QuestMissionConditionTypeWeaponMainRarityGeAllCharacter            QuestMissionConditionType = 29
	QuestMissionConditionTypeWeaponMainRarityLeAllCharacter            QuestMissionConditionType = 30
	QuestMissionConditionTypeWeaponRarityEqAnyCharacter                QuestMissionConditionType = 31
	QuestMissionConditionTypeWeaponRarityGeAnyCharacter                QuestMissionConditionType = 32
	QuestMissionConditionTypeWeaponRarityLeAnyCharacter                QuestMissionConditionType = 33
	QuestMissionConditionTypeWeaponMainRarityEqAnyCharacter            QuestMissionConditionType = 34
	QuestMissionConditionTypeWeaponMainRarityGeAnyCharacter            QuestMissionConditionType = 35
	QuestMissionConditionTypeWeaponMainRarityLeAnyCharacter            QuestMissionConditionType = 36
	QuestMissionConditionTypeCompanionId                               QuestMissionConditionType = 37
	QuestMissionConditionTypeCompanionAttribute                        QuestMissionConditionType = 38
	QuestMissionConditionTypeCompanionCategory                         QuestMissionConditionType = 39
	QuestMissionConditionTypePartsId                                   QuestMissionConditionType = 40
	QuestMissionConditionTypePartsGroupId                              QuestMissionConditionType = 41
	QuestMissionConditionTypePartsRarityEq                             QuestMissionConditionType = 42
	QuestMissionConditionTypePartsRarityGe                             QuestMissionConditionType = 43
	QuestMissionConditionTypePartsRarityLe                             QuestMissionConditionType = 44
	QuestMissionConditionTypeDeckPowerGe                               QuestMissionConditionType = 45
	QuestMissionConditionTypeDeckPowerLe                               QuestMissionConditionType = 46
	QuestMissionConditionTypeDeckCostumeNumEq                          QuestMissionConditionType = 47
	QuestMissionConditionTypeDeckCostumeNumGe                          QuestMissionConditionType = 48
	QuestMissionConditionTypeDeckCostumeNumLe                          QuestMissionConditionType = 49
	QuestMissionConditionTypeCriticalCountGe                           QuestMissionConditionType = 50
	QuestMissionConditionTypeMinHpPercentageGe                         QuestMissionConditionType = 51
	QuestMissionConditionTypeComboCountGe                              QuestMissionConditionType = 52
	QuestMissionConditionTypeComboMaxDamageGe                          QuestMissionConditionType = 53
	QuestMissionConditionTypeLessThanOrEqualXCostumeSkillUseCount      QuestMissionConditionType = 54
	QuestMissionConditionTypeLessThanOrEqualXWeaponSkillUseCount       QuestMissionConditionType = 55
	QuestMissionConditionTypeLessThanOrEqualXCompanionSkillUseCount    QuestMissionConditionType = 56
	QuestMissionConditionTypeWithoutRecoverySkill                      QuestMissionConditionType = 57
	QuestMissionConditionTypeWithoutCostumeSkill                       QuestMissionConditionType = 58
	QuestMissionConditionTypeWithoutWeaponSkill                        QuestMissionConditionType = 59
	QuestMissionConditionTypeWithoutCompanionSkill                     QuestMissionConditionType = 60
	QuestMissionConditionTypeCharacterContainAll                       QuestMissionConditionType = 61
	QuestMissionConditionTypeCharacterContainAny                       QuestMissionConditionType = 62
	QuestMissionConditionTypeCostumeContainAll                         QuestMissionConditionType = 63
	QuestMissionConditionTypeCostumeContainAny                         QuestMissionConditionType = 64
	QuestMissionConditionTypeCostumeSkillfulWeaponContainAll           QuestMissionConditionType = 65
	QuestMissionConditionTypeCostumeSkillfulWeaponContainAny           QuestMissionConditionType = 66
	QuestMissionConditionTypeAttributeMainWeaponContainAll             QuestMissionConditionType = 67
	QuestMissionConditionTypeAttributeMainWeaponContainAny             QuestMissionConditionType = 68
	QuestMissionConditionTypeAttributeWeaponContainAll                 QuestMissionConditionType = 69
	QuestMissionConditionTypeAttributeWeaponContainAny                 QuestMissionConditionType = 70
	QuestMissionConditionTypeWeaponManSkillfulWeaponContainAll         QuestMissionConditionType = 71
	QuestMissionConditionTypeWeaponManSkillfulWeaponContainAny         QuestMissionConditionType = 72
	QuestMissionConditionTypeWeaponSkillfulWeaponContainAll            QuestMissionConditionType = 73
	QuestMissionConditionTypeWeaponSkillfulWeaponContainAny            QuestMissionConditionType = 74
	QuestMissionConditionTypeComplete                                  QuestMissionConditionType = 9999
)

type mission struct {
	QuestMissionId                    int32                     `json:"QuestMissionId"`
	QuestMissionConditionType         QuestMissionConditionType `json:"QuestMissionConditionType"`
	QuestMissionRewardId              int32                     `json:"QuestMissionRewardId"`
	QuestMissionConditionValueGroupId int32                     `json:"QuestMissionConditionValueGroupId"`
}

type PossessionType int

const (
	PossessionTypeUnknown           PossessionType = 0
	PossessionTypeCostume           PossessionType = 1
	PossessionTypeWeapon            PossessionType = 2
	PossessionTypeCompanion         PossessionType = 3
	PossessionTypeParts             PossessionType = 4
	PossessionTypeMaterial          PossessionType = 5
	PossessionTypeConsumableItem    PossessionType = 6
	PossessionTypeCostumeEnhanced   PossessionType = 7
	PossessionTypeWeaponEnhanced    PossessionType = 8
	PossessionTypeCompanionEnhanced PossessionType = 9
	PossessionTypePartsEnhanced     PossessionType = 10
	PossessionTypePaidGem           PossessionType = 11
	PossessionTypeFreeGem           PossessionType = 12
	PossessionTypeImportantItem     PossessionType = 13
	PossessionTypeThought           PossessionType = 14
	PossessionTypeMissionPassPoint  PossessionType = 15
	PossessionTypePremiumItem       PossessionType = 16
)

type missionReward struct {
	QuestMissionRewardId int32          `json:"QuestMissionRewardId"`
	PossessionType       PossessionType `json:"PossessionType"`
	PossessionId         int32          `json:"PossessionId"`
	Count                int32          `json:"Count"`
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

type questFirstClearRewardSwitch struct {
	QuestId                      int32 `json:"QuestId"`
	QuestFirstClearRewardGroupId int32 `json:"QuestFirstClearRewardGroupId"`
	SwitchConditionClearQuestId  int32 `json:"SwitchConditionClearQuestId"`
}

type questFirstClearRewardGroup struct {
	QuestFirstClearRewardGroupId int32          `json:"QuestFirstClearRewardGroupId"`
	QuestFirstClearRewardType    int32          `json:"QuestFirstClearRewardType"`
	SortOrder                    int32          `json:"SortOrder"`
	PossessionType               PossessionType `json:"PossessionType"`
	PossessionId                 int32          `json:"PossessionId"`
	Count                        int32          `json:"Count"`
	IsPickup                     bool           `json:"IsPickup"`
}

type WeaponStoryReleaseConditionType int32

const (
	WeaponStoryReleaseConditionTypeUnknown                      = 0
	WeaponStoryReleaseConditionTypeAcquisition                  = 1
	WeaponStoryReleaseConditionTypeReachSpecifiedLevel          = 2
	WeaponStoryReleaseConditionTypeReachInitialMaxLevel         = 3
	WeaponStoryReleaseConditionTypeReachOnceEvolvedMaxLevel     = 4
	WeaponStoryReleaseConditionTypeReachSpecifiedEvolutionCount = 5
	WeaponStoryReleaseConditionTypeQuestClear                   = 6
	WeaponStoryReleaseConditionTypeMainFlowSceneProgress        = 7
)

type weaponStoryReleaseCondition struct {
	WeaponStoryReleaseConditionGroupId          int32                           `json:"WeaponStoryReleaseConditionGroupId"`
	StoryIndex                                  int32                           `json:"StoryIndex"`
	WeaponStoryReleaseConditionType             WeaponStoryReleaseConditionType `json:"WeaponStoryReleaseConditionType"`
	ConditionValue                              int32                           `json:"ConditionValue"`
	WeaponStoryReleaseConditionOperationGroupId int32                           `json:"WeaponStoryReleaseConditionOperationGroupId"`
}

type weaponMaster struct {
	WeaponId                           int32 `json:"WeaponId"`
	WeaponStoryReleaseConditionGroupId int32 `json:"WeaponStoryReleaseConditionGroupId"`
}

type RewardGrant struct {
	PossessionType PossessionType
	PossessionId   int32
	Count          int32
}

type FinishOutcome struct {
	FirstClearRewards            []RewardGrant
	MissionClearRewards          []RewardGrant
	MissionClearCompleteRewards  []RewardGrant
	BigWinClearedQuestMissionIds []int32
	IsBigWin                     bool
}

func (e *NewEngine) isQuestCleared(user *store.UserState, questID int32) bool {
	quest, ok := user.Quests[questID]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for isQuestCleared", questID))
	}

	return quest.QuestStateType == store.UserQuestStateTypeCleared
}

func appendMissionRewards(dst []RewardGrant, src []missionReward) []RewardGrant {
	for _, r := range src {
		dst = append(dst, RewardGrant{
			PossessionType: r.PossessionType,
			PossessionId:   r.PossessionId,
			Count:          r.Count,
		})
	}
	return dst
}

func toRewardGrants(rows []missionReward) []RewardGrant {
	out := make([]RewardGrant, len(rows))
	for i, r := range rows {
		out[i] = RewardGrant{
			PossessionType: r.PossessionType,
			PossessionId:   r.PossessionId,
			Count:          r.Count,
		}
	}
	return out
}

func (e *NewEngine) firstClearRewardGroupID(user *store.UserState, questMeta quest) int32 {
	rewardGroupID := questMeta.QuestFirstClearRewardGroupId
	for _, switchRow := range e.firstClearRewardSwitchesByQuestId[questMeta.QuestId] {
		if e.isQuestCleared(user, switchRow.SwitchConditionClearQuestId) {
			rewardGroupID = switchRow.QuestFirstClearRewardGroupId
			break
		}
	}
	return rewardGroupID
}

func (e *NewEngine) evaluateFinishOutcome(user *store.UserState, questID int32) FinishOutcome {
	outcome := FinishOutcome{}
	quest, ok := user.Quests[questID]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for evaluateFinishOutcome", questID))
	}
	questMeta, ok := e.questById[questID]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for evaluateFinishOutcome", questID))
	}

	if !quest.IsRewardGranted {
		rewardGroupID := e.firstClearRewardGroupID(user, questMeta)
		for _, reward := range e.firstClearRewardsByGroupId[rewardGroupID] {
			outcome.FirstClearRewards = append(outcome.FirstClearRewards, RewardGrant{
				PossessionType: reward.PossessionType,
				PossessionId:   reward.PossessionId,
				Count:          reward.Count,
			})
		}
	}

	newlyClearedCount := 0
	totalNon9999 := 0
	for _, questMissionID := range e.missionIdsByQuestId[questID] {
		missionMaster, ok := e.missionById[questMissionID]
		if !ok || missionMaster.QuestMissionConditionType == QuestMissionConditionTypeComplete {
			continue
		}
		totalNon9999++

		key := store.QuestMissionKey{QuestID: questID, QuestMissionID: questMissionID}
		mission := user.QuestMissions[key]

		if !mission.IsClear {
			newlyClearedCount++
			outcome.MissionClearRewards = appendMissionRewards(
				outcome.MissionClearRewards,
				e.missionRewardsByMissionId[missionMaster.QuestMissionRewardId],
			)
		}
	}

	alreadyClearedCount := totalNon9999 - newlyClearedCount
	allWillBeClear := totalNon9999 > 0 && (alreadyClearedCount+newlyClearedCount) == totalNon9999
	if allWillBeClear {
		for _, questMissionID := range e.missionIdsByQuestId[questID] {
			missionMaster, ok := e.missionById[questMissionID]
			if !ok || missionMaster.QuestMissionConditionType != QuestMissionConditionTypeComplete {
				continue
			}
			key := store.QuestMissionKey{QuestID: questID, QuestMissionID: questMissionID}
			if !user.QuestMissions[key].IsClear {
				outcome.MissionClearCompleteRewards = appendMissionRewards(
					outcome.MissionClearCompleteRewards,
					e.missionRewardsByMissionId[missionMaster.QuestMissionRewardId],
				)
				outcome.BigWinClearedQuestMissionIds = append(outcome.BigWinClearedQuestMissionIds, questMissionID)
			}
		}
		outcome.IsBigWin = len(outcome.BigWinClearedQuestMissionIds) > 0
	}

	return outcome
}

type NewEngine struct {
	sceneById                          map[int32]questScene
	missionById                        map[int32]mission
	questById                          map[int32]quest
	missionIdsByQuestId                map[int32][]int32
	routeIdByQuestId                   map[int32]int32
	sceneIdsByQuestId                  map[int32][]int32
	firstClearRewardsByGroupId         map[int32][]questFirstClearRewardGroup
	firstClearRewardSwitchesByQuestId  map[int32][]questFirstClearRewardSwitch
	missionRewardsByMissionId          map[int32][]missionReward
	weaponById                         map[int32]weaponMaster
	weaponIdsByReleaseConditionGroupId map[int32][]int32
	releaseConditionsByGroupId         map[int32][]weaponStoryReleaseCondition
}

func MakeNewEngine() *NewEngine {
	scenes, err := readJSON[questScene]("EntityMQuestSceneTable.json")
	if err != nil {
		panic(err)
	}
	sort.Slice(scenes, func(i, j int) bool {
		if scenes[i].QuestId != scenes[j].QuestId {
			return scenes[i].QuestId < scenes[j].QuestId
		}
		if scenes[i].SortOrder != scenes[j].SortOrder {
			return scenes[i].SortOrder < scenes[j].SortOrder
		}
		return scenes[i].QuestSceneId < scenes[j].QuestSceneId
	})

	missions, err := readJSON[mission]("EntityMQuestMissionTable.json")
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

	firstClearRewardSwitches, err := readJSON[questFirstClearRewardSwitch]("EntityMQuestFirstClearRewardSwitchTable.json")
	if err != nil {
		panic(err)
	}

	firstClearRewards, err := readJSON[questFirstClearRewardGroup]("EntityMQuestFirstClearRewardGroupTable.json")
	if err != nil {
		panic(err)
	}
	sort.Slice(firstClearRewards, func(i, j int) bool {
		if firstClearRewards[i].QuestFirstClearRewardGroupId != firstClearRewards[j].QuestFirstClearRewardGroupId {
			return firstClearRewards[i].QuestFirstClearRewardGroupId < firstClearRewards[j].QuestFirstClearRewardGroupId
		}
		if firstClearRewards[i].SortOrder != firstClearRewards[j].SortOrder {
			return firstClearRewards[i].SortOrder < firstClearRewards[j].SortOrder
		}
		return firstClearRewards[i].QuestFirstClearRewardType < firstClearRewards[j].QuestFirstClearRewardType
	})

	missionRewards, err := readJSON[missionReward]("EntityMQuestMissionRewardTable.json")
	if err != nil {
		panic(err)
	}

	weapons, err := readJSON[weaponMaster]("EntityMWeaponTable.json")
	if err != nil {
		panic(err)
	}

	releaseConditions, err := readJSON[weaponStoryReleaseCondition]("EntityMWeaponStoryReleaseConditionGroupTable.json")
	if err != nil {
		panic(err)
	}

	engine := &NewEngine{
		sceneById:                          make(map[int32]questScene, len(scenes)),
		missionById:                        make(map[int32]mission, len(missions)),
		questById:                          make(map[int32]quest, len(quests)),
		missionIdsByQuestId:                make(map[int32][]int32),
		routeIdByQuestId:                   make(map[int32]int32),
		sceneIdsByQuestId:                  make(map[int32][]int32),
		firstClearRewardSwitchesByQuestId:  make(map[int32][]questFirstClearRewardSwitch, len(firstClearRewardSwitches)),
		missionRewardsByMissionId:          make(map[int32][]missionReward, len(missionRewards)),
		firstClearRewardsByGroupId:         make(map[int32][]questFirstClearRewardGroup, len(firstClearRewards)),
		weaponById:                         make(map[int32]weaponMaster, len(weapons)),
		weaponIdsByReleaseConditionGroupId: make(map[int32][]int32),
		releaseConditionsByGroupId:         make(map[int32][]weaponStoryReleaseCondition),
	}

	for _, scene := range scenes {
		engine.sceneById[scene.QuestSceneId] = scene
		engine.sceneIdsByQuestId[scene.QuestId] = append(
			engine.sceneIdsByQuestId[scene.QuestId],
			scene.QuestSceneId,
		)
	}

	for _, mission := range missions {
		engine.missionById[mission.QuestMissionId] = mission
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

	for _, reward := range firstClearRewards {
		engine.firstClearRewardsByGroupId[reward.QuestFirstClearRewardGroupId] = append(
			engine.firstClearRewardsByGroupId[reward.QuestFirstClearRewardGroupId],
			reward,
		)
	}

	for _, switchRow := range firstClearRewardSwitches {
		engine.firstClearRewardSwitchesByQuestId[switchRow.QuestId] = append(
			engine.firstClearRewardSwitchesByQuestId[switchRow.QuestId],
			switchRow,
		)
	}

	for _, reward := range missionRewards {
		engine.missionRewardsByMissionId[reward.QuestMissionRewardId] = append(
			engine.missionRewardsByMissionId[reward.QuestMissionRewardId],
			reward,
		)
	}

	for _, w := range weapons {
		engine.weaponById[w.WeaponId] = w
		if w.WeaponStoryReleaseConditionGroupId != 0 {
			engine.weaponIdsByReleaseConditionGroupId[w.WeaponStoryReleaseConditionGroupId] = append(
				engine.weaponIdsByReleaseConditionGroupId[w.WeaponStoryReleaseConditionGroupId],
				w.WeaponId,
			)
		}
	}

	for _, c := range releaseConditions {
		engine.releaseConditionsByGroupId[c.WeaponStoryReleaseConditionGroupId] = append(
			engine.releaseConditionsByGroupId[c.WeaponStoryReleaseConditionGroupId],
			c,
		)
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

func isMainQuestPlayable(quest quest) bool {
	return !quest.IsRunInTheBackground && quest.IsCountedAsQuest
}

func (e *NewEngine) clearQuestMissions(user *store.UserState, questID int32, nowMillis int64) {
	for _, missionID := range e.missionIdsByQuestId[questID] {
		key := store.QuestMissionKey{QuestID: questID, QuestMissionID: missionID}
		mission := user.QuestMissions[key]
		mission.IsClear = true
		mission.ProgressValue = 1
		mission.LatestClearDatetime = nowMillis
		user.QuestMissions[key] = mission
	}
}

func (e *NewEngine) HandleQuestStart(user *store.UserState, questID int32, isBattleOnly bool, nowMillis int64) {
	quest, ok := e.questById[questID]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleQuestStart", questID))
	}

	e.initQuestState(user, questID)

	questState := user.Quests[questID]
	if questState.QuestStateType == store.UserQuestStateTypeCleared {
		return
	}

	questState.IsBattleOnly = isBattleOnly
	if isMainQuestPlayable(quest) {
		user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeMainFlow)
		questState.QuestStateType = store.UserQuestStateTypeActive
		questState.LatestStartDatetime = nowMillis
	} else {
		questState.QuestStateType = store.UserQuestStateTypeCleared
		questState.ClearCount = 1
		questState.DailyClearCount = 1
		questState.LastClearDatetime = nowMillis
	}
	user.Quests[questID] = questState
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
	user.MainQuest.HeadQuestSceneID = max(user.MainQuest.HeadQuestSceneID, questSceneId)
	user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeMainFlow)

	routeId, ok := e.routeIdByQuestId[quest.QuestId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleMainFlowSceneProgress setting currentMainQuestRouteId", quest.QuestId))
	}
	user.MainQuest.CurrentMainQuestRouteID = routeId
}

func (e *NewEngine) getLastMainFlowSceneId(questID int32) int32 {
	sceneIds := e.sceneIdsByQuestId[questID]
	if len(sceneIds) == 0 {
		panic(fmt.Sprintf("no scenes found for questId=%d", questID))
	}

	for i := len(sceneIds) - 1; i >= 0; i-- {
		sceneId := sceneIds[i]
		scene := e.sceneById[sceneId]
		if scene.IsMainFlowQuestTarget {
			return sceneId
		}
	}

	panic(fmt.Sprintf("no main flow target scene found for questId=%d", questID))
}

func (e *NewEngine) HandleMainQuestSceneProgress(user *store.UserState, questSceneId int32) {
	scene, ok := e.sceneById[questSceneId]
	if !ok {
		panic(fmt.Sprintf("unknown sceneId=%d for HandleMainQuestSceneProgress", questSceneId))
	}

	quest, ok := e.questById[scene.QuestId]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleMainQuestSceneProgress", questSceneId))
	}

	user.MainQuest.CurrentQuestSceneID = questSceneId
	user.MainQuest.HeadQuestSceneID = max(user.MainQuest.HeadQuestSceneID, questSceneId)
	lastSceneId := e.getLastMainFlowSceneId(quest.QuestId)
	user.MainQuest.IsReachedLastQuestScene = questSceneId == lastSceneId

	if isMainQuestPlayable(quest) {
		questState := user.Quests[quest.QuestId]
		if scene.QuestResultType == QuestResultTypeHalfResult {
			nowMillis := time.Now().UnixMilli()
			e.clearQuestMissions(user, quest.QuestId, nowMillis)
			e.applyExpRewards(user, quest.QuestId, nowMillis)
			e.grantWeaponStoryUnlocksForQuestScene(user, quest.QuestId, QuestResultTypeHalfResult, nowMillis)
		}
		if scene.QuestResultType == QuestResultTypeFullResult {
			questState.QuestStateType = store.UserQuestStateTypeCleared
			questState.ClearCount = 1
			questState.DailyClearCount = 1
		}
		user.Quests[quest.QuestId] = questState

		user.MainQuest.ProgressQuestSceneID = questSceneId
		user.MainQuest.ProgressHeadQuestSceneID = max(user.MainQuest.ProgressHeadQuestSceneID, questSceneId)
		user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeSubFlow)
		user.MainQuest.ProgressQuestFlowType = int32(QuestFlowTypeSubFlow)
	}
}

func (e *NewEngine) applyExpRewards(user *store.UserState, questID int32, nowMillis int64) {
	questMeta, ok := e.questById[questID]
	if !ok {
		return
	}

	user.Status.Exp += questMeta.UserExp

	if questMeta.CharacterExp != 0 {
		for id, row := range user.Characters {
			row.Exp += questMeta.CharacterExp
			user.Characters[id] = row
		}
	}

	if questMeta.CostumeExp != 0 {
		for key, row := range user.Costumes {
			row.Exp += questMeta.CostumeExp
			user.Costumes[key] = row
		}
	}
}

func (e *NewEngine) applyQuestRewards(user *store.UserState, questID int32, nowMillis int64) {
	questMeta, ok := e.questById[questID]
	if !ok {
		return
	}

	e.applyExpRewards(user, questID, nowMillis)

	rewardGroupID := e.firstClearRewardGroupID(user, questMeta)
	for _, reward := range e.firstClearRewardsByGroupId[rewardGroupID] {
		e.applyRewardPossession(user, reward.PossessionType, reward.PossessionId, reward.Count, nowMillis)
	}
}

func (e *NewEngine) applyRewardPossession(user *store.UserState, possType PossessionType, possID, count int32, nowMillis int64) {
	switch possType {
	case PossessionTypeFreeGem:
		user.Gem.FreeGem += count
	case PossessionTypePaidGem:
		user.Gem.PaidGem += count
	case PossessionTypeCostume:
		e.grantCostume(user, possID, nowMillis)
	case PossessionTypeWeapon:
		e.grantWeapon(user, possID, nowMillis)
	case PossessionTypeCompanion:
		e.grantCompanion(user, possID, nowMillis)
	default:
		log.Printf("[NewEngine] unsupported reward possession: type=%d id=%d count=%d", possType, possID, count)
	}
}

func (e *NewEngine) grantCostume(user *store.UserState, costumeID int32, nowMillis int64) {
	for _, row := range user.Costumes {
		if row.CostumeID == costumeID {
			return
		}
	}
	key := fmt.Sprintf("reward-costume-%d", costumeID)
	user.Costumes[key] = store.CostumeState{
		UserCostumeUUID:     key,
		CostumeID:           costumeID,
		Level:               1,
		AcquisitionDatetime: nowMillis,
	}
}

func (e *NewEngine) grantWeapon(user *store.UserState, weaponID int32, nowMillis int64) {
	for _, row := range user.Weapons {
		if row.WeaponID == weaponID {
			return
		}
	}
	key := fmt.Sprintf("reward-weapon-%d", weaponID)
	user.Weapons[key] = store.WeaponState{
		UserWeaponUUID:      key,
		WeaponID:            weaponID,
		Level:               1,
		AcquisitionDatetime: nowMillis,
	}
}

func (e *NewEngine) grantCompanion(user *store.UserState, companionID int32, nowMillis int64) {
	for _, row := range user.Companions {
		if row.CompanionID == companionID {
			return
		}
	}
	key := fmt.Sprintf("reward-companion-%d", companionID)
	user.Companions[key] = store.CompanionState{
		UserCompanionUUID:   key,
		CompanionID:         companionID,
		Level:               1,
		AcquisitionDatetime: nowMillis,
	}
}

func (e *NewEngine) grantWeaponStoryUnlock(user *store.UserState, weaponID, storyIndex int32, nowMillis int64) {
	if user.WeaponStories == nil {
		user.WeaponStories = make(map[int32]store.WeaponStoryState)
	}
	cur := user.WeaponStories[weaponID]
	if storyIndex <= cur.ReleasedMaxStoryIndex {
		return
	}
	user.WeaponStories[weaponID] = store.WeaponStoryState{
		WeaponId:              weaponID,
		ReleasedMaxStoryIndex: storyIndex,
		LatestVersion:         nowMillis,
	}
}

func (e *NewEngine) grantWeaponStoryUnlocksForQuestScene(user *store.UserState, questID int32, resultType QuestResultType, nowMillis int64) {
	if resultType == QuestResultTypeHalfResult {
		questMeta, ok := e.questById[questID]
		if !ok {
			return
		}
		rewardGroupID := e.firstClearRewardGroupID(user, questMeta)
		for _, reward := range e.firstClearRewardsByGroupId[rewardGroupID] {
			if reward.PossessionType != PossessionTypeWeapon {
				continue
			}
			weaponID := reward.PossessionId
			weapon, ok := e.weaponById[weaponID]
			if !ok || weapon.WeaponStoryReleaseConditionGroupId == 0 {
				continue
			}
			groupID := weapon.WeaponStoryReleaseConditionGroupId
			for _, cond := range e.releaseConditionsByGroupId[groupID] {
				if cond.WeaponStoryReleaseConditionType == WeaponStoryReleaseConditionTypeAcquisition && cond.ConditionValue == 0 {
					e.grantWeaponStoryUnlock(user, weaponID, cond.StoryIndex, nowMillis)
				}
			}
		}
		return
	}
	if resultType == QuestResultTypeFullResult {
		for groupID, conditions := range e.releaseConditionsByGroupId {
			for _, cond := range conditions {
				if cond.WeaponStoryReleaseConditionType == WeaponStoryReleaseConditionTypeQuestClear && cond.ConditionValue == questID {
					for _, weaponID := range e.weaponIdsByReleaseConditionGroupId[groupID] {
						e.grantWeaponStoryUnlock(user, weaponID, cond.StoryIndex, nowMillis)
					}
					break
				}
			}
		}
	}
}

func (e *NewEngine) HandleQuestFinish(user *store.UserState, questID int32, nowMillis int64) FinishOutcome {
	_, ok := e.questById[questID]
	if !ok {
		panic(fmt.Sprintf("unknown questId=%d for HandleQuestFinish", questID))
	}

	outcome := e.evaluateFinishOutcome(user, questID)

	questState := user.Quests[questID]
	if !questState.IsRewardGranted {
		e.applyQuestRewards(user, questID, nowMillis)
		e.grantWeaponStoryUnlocksForQuestScene(user, questID, QuestResultTypeFullResult, nowMillis)
		questState.IsRewardGranted = true
	}
	questState.QuestStateType = store.UserQuestStateTypeCleared
	questState.ClearCount = 1
	questState.DailyClearCount = 1
	questState.LastClearDatetime = nowMillis

	user.MainQuest.ProgressQuestSceneID = 0
	user.MainQuest.ProgressHeadQuestSceneID = 0
	user.MainQuest.ProgressQuestFlowType = 0
	user.MainQuest.CurrentQuestFlowType = int32(QuestFlowTypeUnknown)
	user.Quests[questID] = questState

	e.clearQuestMissions(user, questID, nowMillis)

	return outcome
}
