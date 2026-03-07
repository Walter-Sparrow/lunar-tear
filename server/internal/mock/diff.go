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
// Tables and JSON shapes match userdata.DefaultUserDataJSON so the client sees
// a valid user, user_setting, and main-quest state (e.g. after Auth or GameStart).
func BaselineDiff(userID int64) map[string]*pb.DiffData {
	tables := userdata.DefaultUserDataJSON(userID)
	out := make(map[string]*pb.DiffData, len(tables))
	for table, jsonStr := range tables {
		out[table] = &pb.DiffData{UpdateRecordsJson: jsonStr}
	}
	return out
}
