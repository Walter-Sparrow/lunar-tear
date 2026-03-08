// Package userdata builds user data payloads for the NieR Reincarnation client.
//
// Today the active path is plain JSON arrays of objects, which the client-side
// DarkUserDataDatabaseBuilderAppendHelper turns into typed entities via
// JObject.ToObject<EntityIUser*>. The older base64+MessagePack helpers are kept
// around for experiments, but are not the default path.
package userdata

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// EntityIUser mirrors the game's EntityIUser [MessagePackObject] with [Key(0..7)].
// Serialized as a MessagePack array of 8 elements.
type EntityIUser struct {
	_msgpack            struct{} `msgpack:",asArray"`
	UserId              int64    // Key(0)
	PlayerId            int64    // Key(1)
	OsType              int32    // Key(2) — 2 = Android
	PlatformType        int32    // Key(3) — 2 = GooglePlay
	UserRestrictionType int32    // Key(4) — 0 = None
	RegisterDatetime    int64    // Key(5) — unix millis
	GameStartDatetime   int64    // Key(6) — unix millis
	LatestVersion       int64    // Key(7)
}

// EntityIUserSetting mirrors EntityIUserSetting [Key(0..2)].
type EntityIUserSetting struct {
	_msgpack              struct{} `msgpack:",asArray"`
	UserId                int64    // Key(0)
	IsNotifyPurchaseAlert bool     // Key(1)
	LatestVersion         int64    // Key(2)
}

// EntityIUserTutorialProgress mirrors EntityIUserTutorialProgress [Key(0..4)].
type EntityIUserTutorialProgress struct {
	_msgpack      struct{} `msgpack:",asArray"`
	UserId        int64    // Key(0)
	TutorialType  int32    // Key(1)
	ProgressPhase int32    // Key(2)
	ChoiceId      int32    // Key(3)
	LatestVersion int64    // Key(4)
}

// EntityIUserQuest mirrors EntityIUserQuest [Key(0..9)].
type EntityIUserQuest struct {
	_msgpack            struct{} `msgpack:",asArray"`
	UserId              int64    // Key(0)
	QuestId             int32    // Key(1)
	QuestStateType      int32    // Key(2) — 3 = Cleared
	IsBattleOnly        bool     // Key(3)
	LatestStartDatetime int64    // Key(4) — unix millis
	ClearCount          int32    // Key(5)
	DailyClearCount     int32    // Key(6)
	LastClearDatetime   int64    // Key(7) — unix millis
	ShortestClearFrames int32    // Key(8)
	LatestVersion       int64    // Key(9)
}

// EntityIUserMainQuestFlowStatus mirrors EntityIUserMainQuestFlowStatus [Key(0..2)].
type EntityIUserMainQuestFlowStatus struct {
	_msgpack             struct{} `msgpack:",asArray"`
	UserId               int64    // Key(0)
	CurrentQuestFlowType int32    // Key(1)
	LatestVersion        int64    // Key(2)
}

// EntityIUserMainQuestMainFlowStatus mirrors EntityIUserMainQuestMainFlowStatus [Key(0..5)].
type EntityIUserMainQuestMainFlowStatus struct {
	_msgpack                struct{} `msgpack:",asArray"`
	UserId                  int64    // Key(0)
	CurrentMainQuestRouteId int32    // Key(1)
	CurrentQuestSceneId     int32    // Key(2)
	HeadQuestSceneId        int32    // Key(3)
	IsReachedLastQuestScene bool     // Key(4)
	LatestVersion           int64    // Key(5)
}

// EntityIUserMainQuestProgressStatus mirrors EntityIUserMainQuestProgressStatus [Key(0..4)].
// This table is used by ActivePlayerToEntityPlayingMainQuestStatus (0x2AB4A48).
type EntityIUserMainQuestProgressStatus struct {
	_msgpack             struct{} `msgpack:",asArray"`
	UserId               int64    // Key(0)
	CurrentQuestSceneId  int32    // Key(1)
	HeadQuestSceneId     int32    // Key(2)
	CurrentQuestFlowType int32    // Key(3) // 1 = MAIN_FLOW
	LatestVersion        int64    // Key(4)
}

// EntityIUserMainQuestSeasonRoute mirrors EntityIUserMainQuestSeasonRoute [Key(0..3)].
type EntityIUserMainQuestSeasonRoute struct {
	_msgpack          struct{} `msgpack:",asArray"`
	UserId            int64    // Key(0)
	MainQuestSeasonId int32    // Key(1)
	MainQuestRouteId  int32    // Key(2)
	LatestVersion     int64    // Key(3)
}

// EntityIUserStatus mirrors EntityIUserStatus [Key(0..5)].
type EntityIUserStatus struct {
	_msgpack              struct{} `msgpack:",asArray"`
	UserId                int64    // Key(0)
	Level                 int32    // Key(1)
	Exp                   int32    // Key(2)
	StaminaMilliValue     int32    // Key(3)
	StaminaUpdateDatetime int64    // Key(4)
	LatestVersion         int64    // Key(5)
}

// EntityIUserGem mirrors EntityIUserGem [Key(0..2)].
type EntityIUserGem struct {
	_msgpack struct{} `msgpack:",asArray"`
	UserId   int64    // Key(0)
	PaidGem  int32    // Key(1)
	FreeGem  int32    // Key(2)
}

// EntityIUserProfile mirrors EntityIUserProfile [Key(0..7)].
type EntityIUserProfile struct {
	_msgpack                        struct{} `msgpack:",asArray"`
	UserId                          int64    // Key(0)
	Name                            string   // Key(1)
	NameUpdateDatetime              int64    // Key(2)
	Message                         string   // Key(3)
	MessageUpdateDatetime           int64    // Key(4)
	FavoriteCostumeId               int32    // Key(5)
	FavoriteCostumeIdUpdateDatetime int64    // Key(6)
	LatestVersion                   int64    // Key(7)
}

// EntityIUserCharacter mirrors EntityIUserCharacter [Key(0..4)].
type EntityIUserCharacter struct {
	_msgpack      struct{} `msgpack:",asArray"`
	UserId        int64    // Key(0)
	CharacterId   int32    // Key(1)
	Level         int32    // Key(2)
	Exp           int32    // Key(3)
	LatestVersion int64    // Key(4)
}

// EntityIUserCostume mirrors EntityIUserCostume [Key(0..9)].
type EntityIUserCostume struct {
	_msgpack            struct{} `msgpack:",asArray"`
	UserId              int64    // Key(0)
	UserCostumeUuid     string   // Key(1)
	CostumeId           int32    // Key(2)
	LimitBreakCount     int32    // Key(3)
	Level               int32    // Key(4)
	Exp                 int32    // Key(5)
	HeadupDisplayViewId int32    // Key(6)
	AcquisitionDatetime int64    // Key(7)
	AwakenCount         int32    // Key(8)
	LatestVersion       int64    // Key(9)
}

// EntityIUserWeapon mirrors EntityIUserWeapon [Key(0..8)].
type EntityIUserWeapon struct {
	_msgpack            struct{} `msgpack:",asArray"`
	UserId              int64    // Key(0)
	UserWeaponUuid      string   // Key(1)
	WeaponId            int32    // Key(2)
	Level               int32    // Key(3)
	Exp                 int32    // Key(4)
	LimitBreakCount     int32    // Key(5)
	IsProtected         bool     // Key(6)
	AcquisitionDatetime int64    // Key(7)
	LatestVersion       int64    // Key(8)
}

// EntityIUserCompanion mirrors EntityIUserCompanion [Key(0..6)].
type EntityIUserCompanion struct {
	_msgpack            struct{} `msgpack:",asArray"`
	UserId              int64    // Key(0)
	UserCompanionUuid   string   // Key(1)
	CompanionId         int32    // Key(2)
	HeadupDisplayViewId int32    // Key(3)
	Level               int32    // Key(4)
	AcquisitionDatetime int64    // Key(5)
	LatestVersion       int64    // Key(6)
}

// EntityIUserDeckCharacter mirrors EntityIUserDeckCharacter [Key(0..7)].
type EntityIUserDeckCharacter struct {
	_msgpack              struct{} `msgpack:",asArray"`
	UserId                int64    // Key(0)
	UserDeckCharacterUuid string   // Key(1)
	UserCostumeUuid       string   // Key(2)
	MainUserWeaponUuid    string   // Key(3)
	UserCompanionUuid     string   // Key(4)
	Power                 int32    // Key(5)
	UserThoughtUuid       string   // Key(6)
	LatestVersion         int64    // Key(7)
}

// EntityIUserDeck mirrors EntityIUserDeck [Key(0..8)].
type EntityIUserDeck struct {
	_msgpack                struct{} `msgpack:",asArray"`
	UserId                  int64    // Key(0)
	DeckType                int32    // Key(1)
	UserDeckNumber          int32    // Key(2)
	UserDeckCharacterUuid01 string   // Key(3)
	UserDeckCharacterUuid02 string   // Key(4)
	UserDeckCharacterUuid03 string   // Key(5)
	Name                    string   // Key(6)
	Power                   int32    // Key(7)
	LatestVersion           int64    // Key(8)
}

// EntityIUserLogin mirrors EntityIUserLogin [Key(0..6)].
type EntityIUserLogin struct {
	_msgpack                  struct{} `msgpack:",asArray"`
	UserId                    int64    // Key(0)
	TotalLoginCount           int32    // Key(1)
	ContinualLoginCount       int32    // Key(2)
	MaxContinualLoginCount    int32    // Key(3)
	LastLoginDatetime         int64    // Key(4)
	LastComebackLoginDatetime int64    // Key(5)
	LatestVersion             int64    // Key(6)
}

// EntityIUserLoginBonus mirrors EntityIUserLoginBonus [Key(0..5)].
type EntityIUserLoginBonus struct {
	_msgpack                    struct{} `msgpack:",asArray"`
	UserId                      int64    // Key(0)
	LoginBonusId                int32    // Key(1)
	CurrentPageNumber           int32    // Key(2)
	CurrentStampNumber          int32    // Key(3)
	LatestRewardReceiveDatetime int64    // Key(4)
	LatestVersion               int64    // Key(5)
}

// EntityIUserMission mirrors EntityIUserMission [Key(0..6)].
type EntityIUserMission struct {
	_msgpack                  struct{} `msgpack:",asArray"`
	UserId                    int64    // Key(0)
	MissionId                 int32    // Key(1)
	StartDatetime             int64    // Key(2)
	ProgressValue             int32    // Key(3)
	MissionProgressStatusType int32    // Key(4)
	ClearDatetime             int64    // Key(5)
	LatestVersion             int64    // Key(6)
}

// EncodeRecords serializes a slice of entities to the client-expected format:
// a JSON array of base64-encoded MessagePack byte strings.
func EncodeRecords(entities ...any) (string, error) {
	b64List := make([]string, 0, len(entities))
	for _, e := range entities {
		data, err := msgpack.Marshal(e)
		if err != nil {
			return "", fmt.Errorf("msgpack marshal: %w", err)
		}
		b64List = append(b64List, base64.StdEncoding.EncodeToString(data))
	}
	jsonBytes, err := json.Marshal(b64List)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	return string(jsonBytes), nil
}

func encodeJSONRecords(entities ...any) (string, error) {
	jsonBytes, err := json.Marshal(entities)
	if err != nil {
		return "", fmt.Errorf("json marshal records: %w", err)
	}
	return string(jsonBytes), nil
}

func encodeJSONMaps(records ...map[string]any) (string, error) {
	jsonBytes, err := json.Marshal(records)
	if err != nil {
		return "", fmt.Errorf("json marshal maps: %w", err)
	}
	return string(jsonBytes), nil
}

// DefaultUserData returns pre-built user data tables for a fresh user.
// We provide BOTH msgpack-encoded (base64) and plain JSON variants.
// The server tries msgpack first; if the client doesn't accept it, switch to JSON.
func DefaultUserData(userID int64) map[string]string {
	now := time.Now().Unix()

	userRecord, _ := EncodeRecords(&EntityIUser{
		UserId:           userID,
		PlayerId:         userID,
		OsType:           2,
		PlatformType:     2,
		RegisterDatetime: now,
	})

	settingRecord, _ := EncodeRecords(&EntityIUserSetting{
		UserId: userID,
	})

	data := map[string]string{
		"user":         userRecord,
		"user_setting": settingRecord,
	}
	return data
}

// DefaultUserDataJSON returns user data as plain JSON.
// Verified: client accepts JSON format and parses it correctly.
func DefaultUserDataJSON(userID int64) map[string]string {
	nowMillis := time.Now().UnixMilli()
	const (
		starterCharacterID = int32(1)
		starterCostumeID   = int32(1)
		starterWeaponID    = int32(1)
		starterCompanionID = int32(1)
		starterQuestID     = int32(1)
		starterMissionID   = int32(1)
		questDeckType      = int32(1)
		missionInProgress  = int32(1)
	)
	const (
		starterCostumeUUID       = "starter-costume-0001"
		starterWeaponUUID        = "starter-weapon-0001"
		starterCompanionUUID     = "starter-companion-0001"
		starterDeckCharacterUUID = "starter-deck-character-0001"
	)
	userJSON, _ := encodeJSONMaps(map[string]any{
		"userId":              userID,
		"playerId":            userID,
		"osType":              2,
		"platformType":        2,
		"userRestrictionType": 0,
		"registerDatetime":    nowMillis,
		"gameStartDatetime":   nowMillis,
		"latestVersion": 0,
	})
	userSettingJSON, _ := encodeJSONRecords(&EntityIUserSetting{
		UserId:                userID,
		IsNotifyPurchaseAlert: false,
		LatestVersion:         0,
	})
	mainQuestFlowJSON, _ := encodeJSONRecords(&EntityIUserMainQuestFlowStatus{
		UserId:               userID,
		CurrentQuestFlowType: 1,
		LatestVersion:        0,
	})
	mainQuestMainFlowJSON, _ := encodeJSONRecords(&EntityIUserMainQuestMainFlowStatus{
		UserId:                  userID,
		CurrentMainQuestRouteId: 1,
		CurrentQuestSceneId:     1,
		HeadQuestSceneId:        1,
		IsReachedLastQuestScene: false,
		LatestVersion:           0,
	})
	mainQuestProgressJSON, _ := encodeJSONRecords(&EntityIUserMainQuestProgressStatus{
		UserId:               userID,
		CurrentQuestSceneId:  1,
		HeadQuestSceneId:     1,
		CurrentQuestFlowType: 1,
		LatestVersion:        0,
	})
	mainQuestSeasonRouteJSON, _ := encodeJSONRecords(&EntityIUserMainQuestSeasonRoute{
		UserId:            userID,
		MainQuestSeasonId: 1,
		MainQuestRouteId:  1,
		LatestVersion:     0,
	})
	userStatusJSON, _ := encodeJSONRecords(&EntityIUserStatus{
		UserId:                userID,
		Level:                 1,
		Exp:                   0,
		StaminaMilliValue:     60000,
		StaminaUpdateDatetime: nowMillis,
		LatestVersion:         0,
	})
	userGemJSON, _ := encodeJSONRecords(&EntityIUserGem{
		UserId:  userID,
		PaidGem: 0,
		FreeGem: 0,
	})
	userProfileJSON, _ := encodeJSONRecords(&EntityIUserProfile{
		UserId:                          userID,
		Name:                            "Lunar Tear",
		NameUpdateDatetime:              nowMillis,
		Message:                         "",
		MessageUpdateDatetime:           nowMillis,
		FavoriteCostumeId:               starterCostumeID,
		FavoriteCostumeIdUpdateDatetime: nowMillis,
		LatestVersion:                   0,
	})
	userCharacterJSON, _ := encodeJSONRecords(&EntityIUserCharacter{
		UserId:        userID,
		CharacterId:   starterCharacterID,
		Level:         1,
		Exp:           0,
		LatestVersion: 0,
	})
	userCostumeJSON, _ := encodeJSONRecords(&EntityIUserCostume{
		UserId:              userID,
		UserCostumeUuid:     starterCostumeUUID,
		CostumeId:           starterCostumeID,
		LimitBreakCount:     0,
		Level:               1,
		Exp:                 0,
		HeadupDisplayViewId: 0,
		AcquisitionDatetime: nowMillis,
		AwakenCount:         0,
		LatestVersion:       0,
	})
	userWeaponJSON, _ := encodeJSONRecords(&EntityIUserWeapon{
		UserId:              userID,
		UserWeaponUuid:      starterWeaponUUID,
		WeaponId:            starterWeaponID,
		Level:               1,
		Exp:                 0,
		LimitBreakCount:     0,
		IsProtected:         false,
		AcquisitionDatetime: nowMillis,
		LatestVersion:       0,
	})
	userCompanionJSON, _ := encodeJSONRecords(&EntityIUserCompanion{
		UserId:              userID,
		UserCompanionUuid:   starterCompanionUUID,
		CompanionId:         starterCompanionID,
		HeadupDisplayViewId: 0,
		Level:               1,
		AcquisitionDatetime: nowMillis,
		LatestVersion:       0,
	})
	userDeckCharacterJSON, _ := encodeJSONRecords(&EntityIUserDeckCharacter{
		UserId:                userID,
		UserDeckCharacterUuid: starterDeckCharacterUUID,
		UserCostumeUuid:       starterCostumeUUID,
		MainUserWeaponUuid:    starterWeaponUUID,
		UserCompanionUuid:     starterCompanionUUID,
		Power:                 100,
		UserThoughtUuid:       "",
		LatestVersion:         0,
	})
	userDeckJSON, _ := encodeJSONRecords(&EntityIUserDeck{
		UserId:                  userID,
		DeckType:                questDeckType,
		UserDeckNumber:          1,
		UserDeckCharacterUuid01: starterDeckCharacterUUID,
		UserDeckCharacterUuid02: "",
		UserDeckCharacterUuid03: "",
		Name:                    "Deck 1",
		Power:                   100,
		LatestVersion:           0,
	})
	userLoginJSON, _ := encodeJSONRecords(&EntityIUserLogin{
		UserId:                    userID,
		TotalLoginCount:           1,
		ContinualLoginCount:       1,
		MaxContinualLoginCount:    1,
		LastLoginDatetime:         nowMillis,
		LastComebackLoginDatetime: 0,
		LatestVersion:             0,
	})
	userLoginBonusJSON, _ := encodeJSONRecords(&EntityIUserLoginBonus{
		UserId:                      userID,
		LoginBonusId:                1,
		CurrentPageNumber:           1,
		CurrentStampNumber:          0,
		LatestRewardReceiveDatetime: 0,
		LatestVersion:               0,
	})
	userTutorialProgressJSON, _ := encodeJSONRecords(&EntityIUserTutorialProgress{
		UserId:        userID,
		TutorialType:  1,
		ProgressPhase: 0,
		ChoiceId:      0,
		LatestVersion: 0,
	})
	userQuestJSON, _ := encodeJSONRecords(&EntityIUserQuest{
		UserId:              userID,
		QuestId:             starterQuestID,
		QuestStateType:      0,
		IsBattleOnly:        false,
		LatestStartDatetime: nowMillis,
		ClearCount:          0,
		DailyClearCount:     0,
		LastClearDatetime:   0,
		ShortestClearFrames: 0,
		LatestVersion:       0,
	})
	userMissionJSON, _ := encodeJSONRecords(&EntityIUserMission{
		UserId:                    userID,
		MissionId:                 starterMissionID,
		StartDatetime:             nowMillis,
		ProgressValue:             0,
		MissionProgressStatusType: missionInProgress,
		ClearDatetime:             0,
		LatestVersion:             0,
	})

	return map[string]string{
		"user":                             userJSON,
		"user_setting":                     userSettingJSON,
		"user_status":                      userStatusJSON,
		"user_gem":                         userGemJSON,
		"user_profile":                     userProfileJSON,
		"user_character":                   userCharacterJSON,
		"user_costume":                     userCostumeJSON,
		"user_weapon":                      userWeaponJSON,
		"user_companion":                   userCompanionJSON,
		"user_deck_character":              userDeckCharacterJSON,
		"user_deck":                        userDeckJSON,
		"user_login":                       userLoginJSON,
		"user_login_bonus":                 userLoginBonusJSON,
		"user_tutorial_progress":           userTutorialProgressJSON,
		"user_quest":                       userQuestJSON,
		"user_mission":                     userMissionJSON,
		"user_main_quest_flow_status":      mainQuestFlowJSON,
		"user_main_quest_main_flow_status": mainQuestMainFlowJSON,
		"user_main_quest_progress_status":  mainQuestProgressJSON,
		"user_main_quest_season_route":     mainQuestSeasonRouteJSON,
	}
}

// DefaultUserDataJSONClientTables returns the same data as DefaultUserDataJSON,
// but keyed by the IUser* names used by the client append/diff helpers.
func DefaultUserDataJSONClientTables(userID int64) map[string]string {
	defaultsSnake := DefaultUserDataJSON(userID)
	return map[string]string{
		"IUser":                        defaultsSnake["user"],
		"IUserSetting":                 defaultsSnake["user_setting"],
		"IUserStatus":                  defaultsSnake["user_status"],
		"IUserGem":                     defaultsSnake["user_gem"],
		"IUserProfile":                 defaultsSnake["user_profile"],
		"IUserCharacter":               defaultsSnake["user_character"],
		"IUserCostume":                 defaultsSnake["user_costume"],
		"IUserWeapon":                  defaultsSnake["user_weapon"],
		"IUserCompanion":               defaultsSnake["user_companion"],
		"IUserDeckCharacter":           defaultsSnake["user_deck_character"],
		"IUserDeck":                    defaultsSnake["user_deck"],
		"IUserLogin":                   defaultsSnake["user_login"],
		"IUserLoginBonus":              defaultsSnake["user_login_bonus"],
		"IUserMission":                 defaultsSnake["user_mission"],
		"IUserMainQuestFlowStatus":     defaultsSnake["user_main_quest_flow_status"],
		"IUserMainQuestMainFlowStatus": defaultsSnake["user_main_quest_main_flow_status"],
		"IUserMainQuestProgressStatus": defaultsSnake["user_main_quest_progress_status"],
		"IUserMainQuestSeasonRoute":    defaultsSnake["user_main_quest_season_route"],
		"IUserQuest":                   defaultsSnake["user_quest"],
		"IUserTutorialProgress":        defaultsSnake["user_tutorial_progress"],
	}
}

// FirstEntranceUserDataJSONClientTables returns a pre-GameStart baseline.
// Keep only core account rows that title/login can plausibly need, while leaving
// gameplay progression and starter party data empty until GameStart.
func FirstEntranceUserDataJSONClientTables(userID int64) map[string]string {
	tables := DefaultUserDataJSONClientTables(userID)
	nowMillis := time.Now().UnixMilli()
	// Runtime ctor(Dictionary<string, object>) reads these keys as raw objects and
	// immediately passes them to MPDateTime.ConvertMPDateTime(object), so they must
	// stay plain unix-millis scalars here, not nested maps.
	userJSON, _ := encodeJSONMaps(map[string]any{
		"userId":              userID,
		"playerId":            userID,
		"osType":              2,
		"platformType":        2,
		"userRestrictionType": 0,
		"registerDatetime":    nowMillis,
		"gameStartDatetime":   nowMillis,
		"latestVersion": 0,
	})
	userStatusJSON, _ := encodeJSONRecords(&EntityIUserStatus{
		UserId:                userID,
		Level:                 1,
		Exp:                   0,
		StaminaMilliValue:     60000,
		StaminaUpdateDatetime: nowMillis,
		LatestVersion:         0,
	})
	userProfileJSON, _ := encodeJSONRecords(&EntityIUserProfile{
		UserId:                          userID,
		Name:                            "Un-regist User Name",
		NameUpdateDatetime:              nowMillis,
		Message:                         "",
		MessageUpdateDatetime:           nowMillis,
		FavoriteCostumeId:               0,
		FavoriteCostumeIdUpdateDatetime: nowMillis,
		LatestVersion:                   0,
	})
	userLoginJSON, _ := encodeJSONRecords(&EntityIUserLogin{
		UserId:                    userID,
		TotalLoginCount:           1,
		ContinualLoginCount:       1,
		MaxContinualLoginCount:    1,
		LastLoginDatetime:         nowMillis,
		LastComebackLoginDatetime: 0,
		LatestVersion:             0,
	})
	userSettingJSON, _ := encodeJSONRecords(&EntityIUserSetting{
		UserId:                userID,
		IsNotifyPurchaseAlert: false,
		LatestVersion:         0,
	})
	userLoginBonusJSON, _ := encodeJSONRecords(&EntityIUserLoginBonus{
		UserId:                      userID,
		LoginBonusId:                1,
		CurrentPageNumber:           1,
		CurrentStampNumber:          0,
		LatestRewardReceiveDatetime: 0,
		LatestVersion:               0,
	})
	tables["IUser"] = userJSON
	tables["IUserCharacter"] = "[]"
	tables["IUserCostume"] = "[]"
	tables["IUserWeapon"] = "[]"
	tables["IUserCompanion"] = "[]"
	tables["IUserDeckCharacter"] = "[]"
	tables["IUserDeck"] = "[]"
	tables["IUserGem"] = "[]"
	tables["IUserProfile"] = userProfileJSON
	tables["IUserLogin"] = userLoginJSON
	tables["IUserLoginBonus"] = userLoginBonusJSON
	tables["IUserSetting"] = userSettingJSON
	tables["IUserStatus"] = userStatusJSON
	tables["IUserTutorialProgress"] = "[]"
	tables["IUserQuest"] = "[]"
	tables["IUserMission"] = "[]"
	tables["IUserMainQuestFlowStatus"] = "[]"
	tables["IUserMainQuestMainFlowStatus"] = "[]"
	tables["IUserMainQuestProgressStatus"] = "[]"
	tables["IUserMainQuestSeasonRoute"] = "[]"

	return tables
}
