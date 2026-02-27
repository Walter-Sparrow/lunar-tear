// Package userdata handles encoding of user data entities in the format
// expected by the NieR Reincarnation client: JSON arrays of base64-encoded
// MessagePack byte arrays.
//
// The client deserializes each table's value as List<byte[]> (C#), where
// each byte[] is a MessagePack-serialized entity using array layout with
// integer keys (matching MessagePack-CSharp's [MessagePackObject] + [Key(n)]).
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
	RegisterDatetime    int64    // Key(5) — unix timestamp (seconds)
	GameStartDatetime   int64    // Key(6) — 0 = not started
	LatestVersion       int64    // Key(7)
}

// EntityIUserSetting mirrors EntityIUserSetting [Key(0..2)].
type EntityIUserSetting struct {
	_msgpack             struct{} `msgpack:",asArray"`
	UserId               int64    // Key(0)
	IsNotifyPurchaseAlert bool    // Key(1)
	LatestVersion        int64    // Key(2)
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
	_msgpack             struct{} `msgpack:",asArray"`
	UserId               int64    // Key(0)
	QuestId              int32    // Key(1)
	QuestStateType       int32    // Key(2) — 3 = Cleared
	IsBattleOnly         bool     // Key(3)
	LatestStartDatetime  int64    // Key(4) — unix millis
	ClearCount           int32    // Key(5)
	DailyClearCount      int32    // Key(6)
	LastClearDatetime    int64    // Key(7) — unix millis
	ShortestClearFrames  int32    // Key(8)
	LatestVersion        int64    // Key(9)
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

// EntityIUserMainQuestProgressStatus mirrors EntityIUserMainQuestProgressStatus [Key(0..5)].
// This table is used by ActivePlayerToEntityPlayingMainQuestStatus (0x2AB4A48).
type EntityIUserMainQuestProgressStatus struct {
	_msgpack            struct{} `msgpack:",asArray"`
	UserId              int64    // Key(0)
	CurrentQuestSceneId int32    // Key(1)
	HeadQuestSceneId    int32    // Key(2)
	QuestFlowType       int32    // Key(3) // 1 = MAIN_FLOW
	LatestVersion       int64    // Key(4)
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
	now := time.Now().Unix()
	return map[string]string{
		"user": fmt.Sprintf(`[{"UserId":%d,"PlayerId":%d,"OsType":2,"PlatformType":2,"UserRestrictionType":0,"RegisterDatetime":%d,"GameStartDatetime":0,"LatestVersion":0}]`,
			userID, userID, now),
		"user_setting": fmt.Sprintf(`[{"UserId":%d,"IsNotifyPurchaseAlert":false,"LatestVersion":0}]`, userID),
		// Main quest tables - CRITICAL: sceneId=2 to match ActivateMainStoryWithSceneId(sceneId=2)
		"user_main_quest_flow_status":      fmt.Sprintf(`[{"UserId":%d,"CurrentQuestFlowType":1,"LatestVersion":0}]`, userID),
		"user_main_quest_main_flow_status": fmt.Sprintf(`[{"UserId":%d,"CurrentMainQuestRouteId":1,"CurrentQuestSceneId":2,"HeadQuestSceneId":2,"IsReachedLastQuestScene":false,"LatestVersion":0}]`, userID),
		"user_main_quest_progress_status":  fmt.Sprintf(`[{"UserId":%d,"CurrentQuestSceneId":2,"HeadQuestSceneId":2,"QuestFlowType":1,"LatestVersion":0}]`, userID),
		"user_main_quest_season_route":   fmt.Sprintf(`[{"UserId":%d,"MainQuestSeasonId":1,"MainQuestRouteId":1,"LatestVersion":0}]`, userID),
	}
}
