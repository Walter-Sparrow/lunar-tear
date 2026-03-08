package mock

import (
	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/userdata"
)

// EmptyDiff returns an empty DiffUserData map (no table updates).
// Use when the RPC has nothing to sync.
func EmptyDiff() map[string]*pb.DiffData {
	return map[string]*pb.DiffData{}
}

// BaselineDiff returns a full, client-consistent DiffUserData map for the given user.
// Tables and JSON shapes match a first-entrance user state (e.g. after RegisterUser or Auth,
// before the client has completed GameStart/tutorial progression).
func BaselineDiff(userID int64) map[string]*pb.DiffData {
	tables := userdata.FirstEntranceUserDataJSONClientTables(userID)
	out := make(map[string]*pb.DiffData, len(tables))
	for table, jsonStr := range tables {
		out[table] = &pb.DiffData{UpdateRecordsJson: jsonStr}
	}
	return out
}

// StartedDiff returns the fuller started-account baseline used after GameStart.
func StartedDiff(userID int64) map[string]*pb.DiffData {
	tables := userdata.DefaultUserDataJSONClientTables(userID)
	out := make(map[string]*pb.DiffData, len(tables))
	for table, jsonStr := range tables {
		out[table] = &pb.DiffData{UpdateRecordsJson: jsonStr}
	}
	return out
}

// StartedGameStartDiff returns the currently trusted post-GameStart starter rows.
// Keep risky account/core rows such as IUser out of the GameStart diff and let
// GetUserData provide those during the earlier sync phase.
func StartedGameStartDiff(userID int64) map[string]*pb.DiffData {
	tables := userdata.DefaultUserDataJSONClientTables(userID)
	selected := []string{
		"IUserProfile",
		"IUserCharacter",
		"IUserCostume",
		"IUserWeapon",
		"IUserCompanion",
		"IUserDeckCharacter",
		"IUserDeck",
		"IUserMission",
		"IUserMainQuestFlowStatus",
		"IUserMainQuestMainFlowStatus",
		"IUserMainQuestProgressStatus",
		"IUserMainQuestSeasonRoute",
		"IUserQuest",
		"IUserTutorialProgress",
	}

	out := make(map[string]*pb.DiffData, len(selected))
	for _, table := range selected {
		if jsonStr, ok := tables[table]; ok {
			out[table] = &pb.DiffData{
				UpdateRecordsJson: jsonStr,
				DeleteKeysJson:    "[]",
			}
		}
	}
	return out
}
