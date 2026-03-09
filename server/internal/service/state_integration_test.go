package service

import (
	"context"
	"testing"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/store"

	"google.golang.org/grpc/metadata"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func TestGameStartUsesStoreBackedStartedSnapshot(t *testing.T) {
	userStore := store.New(func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	userService := NewUserServiceServer(userStore)

	authResp, err := userService.Auth(context.Background(), &pb.AuthUserRequest{
		Uuid:      "user-1",
		Signature: "sig",
	})
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-session-key", authResp.SessionKey))
	gameStartResp, err := userService.GameStart(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GameStart returned error: %v", err)
	}

	if _, ok := gameStartResp.DiffUserData["IUser"]; ok {
		t.Fatal("GameStart should not include IUser in started diff")
	}
	if len(gameStartResp.DiffUserData) != len(startedGameStartTables) {
		t.Fatalf("GameStart diff len = %d, want %d", len(gameStartResp.DiffUserData), len(startedGameStartTables))
	}
	if profile := gameStartResp.DiffUserData["IUserProfile"].UpdateRecordsJson; profile == "" || !contains(profile, `"name":"Lunar Tear"`) {
		t.Fatalf("IUserProfile payload = %s, want started profile name", profile)
	}
}

func TestQuestLifecycleMutatesStoreState(t *testing.T) {
	userStore := store.New(func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	userService := NewUserServiceServer(userStore)
	questService := NewQuestServiceServer(userStore)

	authResp, err := userService.Auth(context.Background(), &pb.AuthUserRequest{
		Uuid:      "user-2",
		Signature: "sig",
	})
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-session-key", authResp.SessionKey))

	if _, err := questService.UpdateMainFlowSceneProgress(ctx, &pb.UpdateMainFlowSceneProgressRequest{QuestSceneId: 1}); err != nil {
		t.Fatalf("UpdateMainFlowSceneProgress returned error: %v", err)
	}
	if _, err := questService.StartMainQuest(ctx, &pb.StartMainQuestRequest{QuestId: 1, IsMainFlow: true, UserDeckNumber: 1}); err != nil {
		t.Fatalf("StartMainQuest returned error: %v", err)
	}
	if _, err := questService.UpdateMainQuestSceneProgress(ctx, &pb.UpdateMainQuestSceneProgressRequest{QuestSceneId: 3}); err != nil {
		t.Fatalf("UpdateMainQuestSceneProgress returned error: %v", err)
	}
	finishResp, err := questService.FinishMainQuest(ctx, &pb.FinishMainQuestRequest{QuestId: 1, IsMainFlow: true})
	if err != nil {
		t.Fatalf("FinishMainQuest returned error: %v", err)
	}

	user, ok := userStore.SnapshotUser(authResp.UserId)
	if !ok {
		t.Fatal("user snapshot missing after quest lifecycle")
	}
	quest := user.Quests[1]
	if quest.QuestStateType != 2 {
		t.Fatalf("quest state = %d, want 2", quest.QuestStateType)
	}
	if quest.ClearCount != 1 {
		t.Fatalf("quest clear count = %d, want 1", quest.ClearCount)
	}
	if user.MainQuest.ProgressQuestFlowType != 0 || user.MainQuest.ProgressQuestSceneID != 0 {
		t.Fatalf("main quest progress should be cleared, got flow=%d scene=%d", user.MainQuest.ProgressQuestFlowType, user.MainQuest.ProgressQuestSceneID)
	}
	if user.MainQuest.CurrentQuestSceneID != 3 || user.MainQuest.HeadQuestSceneID != 3 {
		t.Fatalf("main quest scene pointer should be preserved from scene progress, got current=%d head=%d", user.MainQuest.CurrentQuestSceneID, user.MainQuest.HeadQuestSceneID)
	}
	if finishResp.DiffUserData["IUserQuest"].DeleteKeysJson != "[]" {
		t.Fatalf("FinishMainQuest DeleteKeysJson = %q, want []", finishResp.DiffUserData["IUserQuest"].DeleteKeysJson)
	}
}

func TestInitSequenceScheduleReturnsStoreBackedGimmickTables(t *testing.T) {
	userStore := store.New(func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	userService := NewUserServiceServer(userStore)
	gimmickService := NewGimmickServiceServer(userStore)

	authResp, err := userService.Auth(context.Background(), &pb.AuthUserRequest{
		Uuid:      "user-3",
		Signature: "sig",
	})
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-session-key", authResp.SessionKey))

	resp, err := gimmickService.InitSequenceSchedule(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("InitSequenceSchedule returned error: %v", err)
	}

	for _, table := range gimmickDiffTables {
		row, ok := resp.DiffUserData[table]
		if !ok {
			t.Fatalf("missing gimmick diff table %s", table)
		}
		if row.DeleteKeysJson != "[]" {
			t.Fatalf("%s DeleteKeysJson = %q, want []", table, row.DeleteKeysJson)
		}
		if row.UpdateRecordsJson != "[]" {
			t.Fatalf("%s UpdateRecordsJson = %q, want [] for empty bootstrap gimmick state", table, row.UpdateRecordsJson)
		}
	}
}

func contains(s, want string) bool {
	return len(s) >= len(want) && (s == want || contextContains(s, want))
}

func contextContains(s, want string) bool {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
