package userdata

import (
	"sort"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/store"
)

func FullClientTableMap(user store.UserState) map[string]string {
	userJSON, _ := encodeJSONMaps(map[string]any{
		"userId":              user.UserID,
		"playerId":            user.PlayerID,
		"osType":              user.OsType,
		"platformType":        user.PlatformType,
		"userRestrictionType": user.UserRestrictionType,
		"registerDatetime":    user.RegisterDatetime,
		"gameStartDatetime":   user.GameStartDatetime,
		"latestVersion":       user.LatestVersion,
	})
	userSettingJSON, _ := encodeJSONRecords(&EntityIUserSetting{
		UserId:                user.UserID,
		IsNotifyPurchaseAlert: user.Setting.IsNotifyPurchaseAlert,
		LatestVersion:         user.Setting.LatestVersion,
	})
	userStatusJSON, _ := encodeJSONMaps(map[string]any{
		"userId":                user.UserID,
		"level":                 user.Status.Level,
		"exp":                   user.Status.Exp,
		"staminaMilliValue":     user.Status.StaminaMilliValue,
		"staminaUpdateDatetime": user.Status.StaminaUpdateDatetime,
		"latestVersion":         user.Status.LatestVersion,
	})
	userGemJSON, _ := encodeJSONRecords(&EntityIUserGem{
		UserId:  user.UserID,
		PaidGem: user.Gem.PaidGem,
		FreeGem: user.Gem.FreeGem,
	})
	userProfileJSON, _ := encodeJSONMaps(map[string]any{
		"userId":                          user.UserID,
		"name":                            user.Profile.Name,
		"nameUpdateDatetime":              user.Profile.NameUpdateDatetime,
		"message":                         user.Profile.Message,
		"messageUpdateDatetime":           user.Profile.MessageUpdateDatetime,
		"favoriteCostumeId":               user.Profile.FavoriteCostumeID,
		"favoriteCostumeIdUpdateDatetime": user.Profile.FavoriteCostumeIDUpdateDatetime,
		"latestVersion":                   user.Profile.LatestVersion,
	})
	userLoginJSON, _ := encodeJSONRecords(&EntityIUserLogin{
		UserId:                    user.UserID,
		TotalLoginCount:           user.Login.TotalLoginCount,
		ContinualLoginCount:       user.Login.ContinualLoginCount,
		MaxContinualLoginCount:    user.Login.MaxContinualLoginCount,
		LastLoginDatetime:         user.Login.LastLoginDatetime,
		LastComebackLoginDatetime: user.Login.LastComebackLoginDatetime,
		LatestVersion:             user.Login.LatestVersion,
	})
	userLoginBonusJSON, _ := encodeJSONRecords(&EntityIUserLoginBonus{
		UserId:                      user.UserID,
		LoginBonusId:                user.LoginBonus.LoginBonusID,
		CurrentPageNumber:           user.LoginBonus.CurrentPageNumber,
		CurrentStampNumber:          user.LoginBonus.CurrentStampNumber,
		LatestRewardReceiveDatetime: user.LoginBonus.LatestRewardReceiveDatetime,
		LatestVersion:               user.LoginBonus.LatestVersion,
	})
	userTutorialProgressJSON, _ := encodeJSONMaps(map[string]any{
		"userId":        user.UserID,
		"tutorialType":  user.Tutorial.TutorialType,
		"progressPhase": user.Tutorial.ProgressPhase,
		"choiceId":      user.Tutorial.ChoiceID,
		"latestVersion": user.Tutorial.LatestVersion,
	})
	mainQuestFlowJSON, _ := encodeJSONMaps(map[string]any{
		"userId":               user.UserID,
		"currentQuestFlowType": user.MainQuest.CurrentQuestFlowType,
		"latestVersion":        user.MainQuest.LatestVersion,
	})
	mainQuestMainFlowJSON, _ := encodeJSONMaps(map[string]any{
		"userId":                  user.UserID,
		"currentMainQuestRouteId": user.MainQuest.CurrentMainQuestRouteID,
		"currentQuestSceneId":     user.MainQuest.CurrentQuestSceneID,
		"headQuestSceneId":        user.MainQuest.HeadQuestSceneID,
		"isReachedLastQuestScene": user.MainQuest.IsReachedLastQuestScene,
		"latestVersion":           user.MainQuest.LatestVersion,
	})
	mainQuestProgressJSON, _ := encodeJSONMaps(map[string]any{
		"userId":               user.UserID,
		"currentQuestSceneId":  user.MainQuest.ProgressQuestSceneID,
		"headQuestSceneId":     user.MainQuest.ProgressHeadQuestSceneID,
		"currentQuestFlowType": user.MainQuest.ProgressQuestFlowType,
		"latestVersion":        user.MainQuest.LatestVersion,
	})
	mainQuestSeasonRouteJSON, _ := encodeJSONMaps(map[string]any{
		"userId":            user.UserID,
		"mainQuestSeasonId": user.MainQuest.MainQuestSeasonID,
		"mainQuestRouteId":  user.MainQuest.CurrentMainQuestRouteID,
		"latestVersion":     user.MainQuest.LatestVersion,
	})

	userCharacterJSON, _ := encodeJSONMaps(sortedCharacterRecords(user)...)
	userCostumeJSON, _ := encodeJSONMaps(sortedCostumeRecords(user)...)
	userWeaponJSON, _ := encodeJSONMaps(sortedWeaponRecords(user)...)
	userWeaponStoryJSON, _ := encodeJSONMaps(sortedWeaponStoryRecords(user)...)
	userCompanionJSON, _ := encodeJSONMaps(sortedCompanionRecords(user)...)
	userDeckCharacterJSON, _ := encodeJSONMaps(sortedDeckCharacterRecords(user)...)
	userDeckJSON, _ := encodeJSONMaps(sortedDeckRecords(user)...)
	userQuestJSON, _ := encodeJSONMaps(sortedQuestRecords(user)...)
	userQuestMissionJSON, _ := encodeJSONMaps(sortedQuestMissionRecords(user)...)
	userMissionJSON, _ := encodeJSONMaps(sortedMissionRecords(user)...)
	userGimmickJSON, _ := encodeJSONMaps(sortedGimmickRecords(user)...)
	userGimmickOrnamentProgressJSON, _ := encodeJSONMaps(sortedGimmickOrnamentProgressRecords(user)...)
	userGimmickSequenceJSON, _ := encodeJSONMaps(sortedGimmickSequenceRecords(user)...)
	userGimmickUnlockJSON, _ := encodeJSONMaps(sortedGimmickUnlockRecords(user)...)

	return map[string]string{
		"IUser":                        userJSON,
		"IUserSetting":                 userSettingJSON,
		"IUserStatus":                  userStatusJSON,
		"IUserGem":                     userGemJSON,
		"IUserProfile":                 userProfileJSON,
		"IUserCharacter":               userCharacterJSON,
		"IUserCostume":                 userCostumeJSON,
		"IUserWeapon":                  userWeaponJSON,
		"IUserWeaponStory":             userWeaponStoryJSON,
		"IUserCompanion":               userCompanionJSON,
		"IUserDeckCharacter":           userDeckCharacterJSON,
		"IUserDeck":                    userDeckJSON,
		"IUserLogin":                   userLoginJSON,
		"IUserLoginBonus":              userLoginBonusJSON,
		"IUserMission":                 userMissionJSON,
		"IUserMainQuestFlowStatus":     mainQuestFlowJSON,
		"IUserMainQuestMainFlowStatus": mainQuestMainFlowJSON,
		"IUserMainQuestProgressStatus": mainQuestProgressJSON,
		"IUserMainQuestSeasonRoute":    mainQuestSeasonRouteJSON,
		"IUserQuest":                   userQuestJSON,
		"IUserQuestMission":            userQuestMissionJSON,
		"IUserTutorialProgress":        userTutorialProgressJSON,
		"IUserGimmick":                 userGimmickJSON,
		"IUserGimmickOrnamentProgress": userGimmickOrnamentProgressJSON,
		"IUserGimmickSequence":         userGimmickSequenceJSON,
		"IUserGimmickUnlock":           userGimmickUnlockJSON,
	}
}

func FirstEntranceClientTableMap(user store.UserState) map[string]string {
	tables := FullClientTableMap(user)
	for _, table := range []string{
		"IUserCharacter",
		"IUserCostume",
		"IUserWeapon",
		"IUserCompanion",
		"IUserDeckCharacter",
		"IUserDeck",
		"IUserGem",
		"IUserTutorialProgress",
		"IUserQuest",
		"IUserQuestMission",
		"IUserMission",
		"IUserMainQuestFlowStatus",
		"IUserMainQuestMainFlowStatus",
		"IUserMainQuestProgressStatus",
		"IUserMainQuestSeasonRoute",
		"IUserGimmick",
		"IUserGimmickOrnamentProgress",
		"IUserGimmickSequence",
		"IUserGimmickUnlock",
	} {
		tables[table] = "[]"
	}
	return tables
}

func SelectTables(all map[string]string, requested []string) map[string]string {
	selected := make(map[string]string, len(requested))
	for _, table := range requested {
		if payload, ok := all[table]; ok && payload != "" {
			selected[table] = payload
			continue
		}
		selected[table] = "[]"
	}
	return selected
}

func BuildDiffFromTables(tables map[string]string) map[string]*pb.DiffData {
	diff := make(map[string]*pb.DiffData, len(tables))
	for table, payload := range tables {
		if payload == "" {
			payload = "[]"
		}
		diff[table] = &pb.DiffData{
			UpdateRecordsJson: payload,
			DeleteKeysJson:    "[]",
		}
	}
	return diff
}

func sortedCharacterRecords(user store.UserState) []map[string]any {
	ids := make([]int, 0, len(user.Characters))
	for id := range user.Characters {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)

	records := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		row := user.Characters[int32(id)]
		records = append(records, map[string]any{
			"userId":        user.UserID,
			"characterId":   row.CharacterID,
			"level":         row.Level,
			"exp":           row.Exp,
			"latestVersion": row.LatestVersion,
		})
	}
	return records
}

func sortedCostumeRecords(user store.UserState) []map[string]any {
	keys := sortedStringKeys(user.Costumes)
	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Costumes[key]
		records = append(records, map[string]any{
			"userId":              user.UserID,
			"userCostumeUuid":     row.UserCostumeUUID,
			"costumeId":           row.CostumeID,
			"limitBreakCount":     row.LimitBreakCount,
			"level":               row.Level,
			"exp":                 row.Exp,
			"headupDisplayViewId": row.HeadupDisplayViewID,
			"acquisitionDatetime": row.AcquisitionDatetime,
			"awakenCount":         row.AwakenCount,
			"latestVersion":       row.LatestVersion,
		})
	}
	return records
}

func sortedWeaponRecords(user store.UserState) []map[string]any {
	keys := sortedStringKeys(user.Weapons)
	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Weapons[key]
		records = append(records, map[string]any{
			"userId":              user.UserID,
			"userWeaponUuid":      row.UserWeaponUUID,
			"weaponId":            row.WeaponID,
			"level":               row.Level,
			"exp":                 row.Exp,
			"limitBreakCount":     row.LimitBreakCount,
			"isProtected":         row.IsProtected,
			"acquisitionDatetime": row.AcquisitionDatetime,
			"latestVersion":       row.LatestVersion,
		})
	}
	return records
}

func sortedWeaponStoryRecords(user store.UserState) []map[string]any {
	if user.WeaponStories == nil {
		return []map[string]any{}
	}
	weaponIds := make([]int32, 0, len(user.WeaponStories))
	for weaponId := range user.WeaponStories {
		weaponIds = append(weaponIds, weaponId)
	}
	sort.Slice(weaponIds, func(i, j int) bool { return weaponIds[i] < weaponIds[j] })
	records := make([]map[string]any, 0, len(weaponIds))
	for _, weaponId := range weaponIds {
		row := user.WeaponStories[weaponId]
		records = append(records, map[string]any{
			"userId":                 user.UserID,
			"weaponId":               row.WeaponId,
			"releasedMaxStoryIndex":  row.ReleasedMaxStoryIndex,
			"latestVersion":          row.LatestVersion,
		})
	}
	return records
}

func sortedCompanionRecords(user store.UserState) []map[string]any {
	keys := sortedStringKeys(user.Companions)
	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Companions[key]
		records = append(records, map[string]any{
			"userId":              user.UserID,
			"userCompanionUuid":   row.UserCompanionUUID,
			"companionId":         row.CompanionID,
			"headupDisplayViewId": row.HeadupDisplayViewID,
			"level":               row.Level,
			"acquisitionDatetime": row.AcquisitionDatetime,
			"latestVersion":       row.LatestVersion,
		})
	}
	return records
}

func sortedDeckCharacterRecords(user store.UserState) []map[string]any {
	keys := sortedStringKeys(user.DeckCharacters)
	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.DeckCharacters[key]
		records = append(records, map[string]any{
			"userId":                user.UserID,
			"userDeckCharacterUuid": row.UserDeckCharacterUUID,
			"userCostumeUuid":       row.UserCostumeUUID,
			"mainUserWeaponUuid":    row.MainUserWeaponUUID,
			"userCompanionUuid":     row.UserCompanionUUID,
			"power":                 row.Power,
			"userThoughtUuid":       row.UserThoughtUUID,
			"latestVersion":         row.LatestVersion,
		})
	}
	return records
}

func sortedDeckRecords(user store.UserState) []map[string]any {
	keys := make([]store.DeckKey, 0, len(user.Decks))
	for key := range user.Decks {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].DeckType != keys[j].DeckType {
			return keys[i].DeckType < keys[j].DeckType
		}
		return keys[i].UserDeckNumber < keys[j].UserDeckNumber
	})

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Decks[key]
		records = append(records, map[string]any{
			"userId":                  user.UserID,
			"deckType":                row.DeckType,
			"userDeckNumber":          row.UserDeckNumber,
			"userDeckCharacterUuid01": row.UserDeckCharacterUUID01,
			"userDeckCharacterUuid02": row.UserDeckCharacterUUID02,
			"userDeckCharacterUuid03": row.UserDeckCharacterUUID03,
			"name":                    row.Name,
			"power":                   row.Power,
			"latestVersion":           row.LatestVersion,
		})
	}
	return records
}

func sortedQuestRecords(user store.UserState) []map[string]any {
	ids := make([]int, 0, len(user.Quests))
	for id := range user.Quests {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)

	records := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		row := user.Quests[int32(id)]
		records = append(records, map[string]any{
			"userId":              user.UserID,
			"questId":             row.QuestID,
			"questStateType":      row.QuestStateType,
			"isBattleOnly":        row.IsBattleOnly,
			"latestStartDatetime": row.LatestStartDatetime,
			"clearCount":          row.ClearCount,
			"dailyClearCount":     row.DailyClearCount,
			"lastClearDatetime":   row.LastClearDatetime,
			"shortestClearFrames": row.ShortestClearFrames,
			"latestVersion":       row.LatestVersion,
		})
	}
	return records
}

func sortedQuestMissionRecords(user store.UserState) []map[string]any {
	keys := make([]store.QuestMissionKey, 0, len(user.QuestMissions))
	for key := range user.QuestMissions {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].QuestID != keys[j].QuestID {
			return keys[i].QuestID < keys[j].QuestID
		}
		return keys[i].QuestMissionID < keys[j].QuestMissionID
	})

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.QuestMissions[key]
		records = append(records, map[string]any{
			"userId":              user.UserID,
			"questId":             row.QuestID,
			"questMissionId":      row.QuestMissionID,
			"progressValue":       row.ProgressValue,
			"isClear":             row.IsClear,
			"latestClearDatetime": row.LatestClearDatetime,
			"latestVersion":       row.LatestVersion,
		})
	}
	return records
}

func sortedMissionRecords(user store.UserState) []map[string]any {
	ids := make([]int, 0, len(user.Missions))
	for id := range user.Missions {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)

	records := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		row := user.Missions[int32(id)]
		records = append(records, map[string]any{
			"userId":                    user.UserID,
			"missionId":                 row.MissionID,
			"startDatetime":             row.StartDatetime,
			"progressValue":             row.ProgressValue,
			"missionProgressStatusType": row.MissionProgressStatusType,
			"clearDatetime":             row.ClearDatetime,
			"latestVersion":             row.LatestVersion,
		})
	}
	return records
}

func sortedGimmickRecords(user store.UserState) []map[string]any {
	keys := make([]store.GimmickKey, 0, len(user.Gimmick.Progress))
	for key := range user.Gimmick.Progress {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return compareGimmickKey(keys[i], keys[j]) < 0
	})

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Gimmick.Progress[key]
		records = append(records, map[string]any{
			"userId":                    user.UserID,
			"gimmickSequenceScheduleId": row.Key.GimmickSequenceScheduleID,
			"gimmickSequenceId":         row.Key.GimmickSequenceID,
			"gimmickId":                 row.Key.GimmickID,
			"isGimmickCleared":          row.IsGimmickCleared,
			"startDatetime":             row.StartDatetime,
			"latestVersion":             row.LatestVersion,
		})
	}
	return records
}

func sortedGimmickOrnamentProgressRecords(user store.UserState) []map[string]any {
	keys := make([]store.GimmickOrnamentKey, 0, len(user.Gimmick.OrnamentProgress))
	for key := range user.Gimmick.OrnamentProgress {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return compareGimmickOrnamentKey(keys[i], keys[j]) < 0
	})

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Gimmick.OrnamentProgress[key]
		records = append(records, map[string]any{
			"userId":                    user.UserID,
			"gimmickSequenceScheduleId": row.Key.GimmickSequenceScheduleID,
			"gimmickSequenceId":         row.Key.GimmickSequenceID,
			"gimmickId":                 row.Key.GimmickID,
			"gimmickOrnamentIndex":      row.Key.GimmickOrnamentIndex,
			"progressValueBit":          row.ProgressValueBit,
			"baseDatetime":              row.BaseDatetime,
			"latestVersion":             row.LatestVersion,
		})
	}
	return records
}

func sortedGimmickSequenceRecords(user store.UserState) []map[string]any {
	keys := make([]store.GimmickSequenceKey, 0, len(user.Gimmick.Sequences))
	for key := range user.Gimmick.Sequences {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].GimmickSequenceScheduleID != keys[j].GimmickSequenceScheduleID {
			return keys[i].GimmickSequenceScheduleID < keys[j].GimmickSequenceScheduleID
		}
		return keys[i].GimmickSequenceID < keys[j].GimmickSequenceID
	})

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Gimmick.Sequences[key]
		records = append(records, map[string]any{
			"userId":                    user.UserID,
			"gimmickSequenceScheduleId": row.Key.GimmickSequenceScheduleID,
			"gimmickSequenceId":         row.Key.GimmickSequenceID,
			"isGimmickSequenceCleared":  row.IsGimmickSequenceCleared,
			"clearDatetime":             row.ClearDatetime,
			"latestVersion":             row.LatestVersion,
		})
	}
	return records
}

func sortedGimmickUnlockRecords(user store.UserState) []map[string]any {
	keys := make([]store.GimmickKey, 0, len(user.Gimmick.Unlocks))
	for key := range user.Gimmick.Unlocks {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return compareGimmickKey(keys[i], keys[j]) < 0
	})

	records := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		row := user.Gimmick.Unlocks[key]
		records = append(records, map[string]any{
			"userId":                    user.UserID,
			"gimmickSequenceScheduleId": row.Key.GimmickSequenceScheduleID,
			"gimmickSequenceId":         row.Key.GimmickSequenceID,
			"gimmickId":                 row.Key.GimmickID,
			"isUnlocked":                row.IsUnlocked,
			"latestVersion":             row.LatestVersion,
		})
	}
	return records
}

func sortedStringKeys[T any](rows map[string]T) []string {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compareGimmickKey(a, b store.GimmickKey) int {
	if a.GimmickSequenceScheduleID != b.GimmickSequenceScheduleID {
		if a.GimmickSequenceScheduleID < b.GimmickSequenceScheduleID {
			return -1
		}
		return 1
	}
	if a.GimmickSequenceID != b.GimmickSequenceID {
		if a.GimmickSequenceID < b.GimmickSequenceID {
			return -1
		}
		return 1
	}
	if a.GimmickID < b.GimmickID {
		return -1
	}
	if a.GimmickID > b.GimmickID {
		return 1
	}
	return 0
}

func compareGimmickOrnamentKey(a, b store.GimmickOrnamentKey) int {
	if cmp := compareGimmickKey(
		store.GimmickKey{
			GimmickSequenceScheduleID: a.GimmickSequenceScheduleID,
			GimmickSequenceID:         a.GimmickSequenceID,
			GimmickID:                 a.GimmickID,
		},
		store.GimmickKey{
			GimmickSequenceScheduleID: b.GimmickSequenceScheduleID,
			GimmickSequenceID:         b.GimmickSequenceID,
			GimmickID:                 b.GimmickID,
		},
	); cmp != 0 {
		return cmp
	}
	if a.GimmickOrnamentIndex < b.GimmickOrnamentIndex {
		return -1
	}
	if a.GimmickOrnamentIndex > b.GimmickOrnamentIndex {
		return 1
	}
	return 0
}
