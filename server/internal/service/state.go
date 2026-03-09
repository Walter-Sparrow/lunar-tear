package service

import (
	"context"

	"lunar-tear/server/internal/store"

	"google.golang.org/grpc/metadata"
)

var startedGameStartTables = []string{
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

var mainQuestDiffTables = []string{
	"IUserMainQuestFlowStatus",
	"IUserMainQuestMainFlowStatus",
	"IUserMainQuestProgressStatus",
}

var gimmickDiffTables = []string{
	"IUserGimmick",
	"IUserGimmickOrnamentProgress",
	"IUserGimmickSequence",
	"IUserGimmickUnlock",
}

func currentUserID(ctx context.Context, userStore *store.Store) int64 {
	if userStore == nil {
		return 0
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-session-key"); len(vals) > 0 {
			if userID, ok := userStore.ResolveUserID(vals[0]); ok {
				return userID
			}
		}
	}

	defaultUserID := userStore.DefaultUserID()
	if _, ok := userStore.SnapshotUser(defaultUserID); ok {
		return defaultUserID
	}
	return userStore.EnsureUser("").UserID
}
