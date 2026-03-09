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

func TestGachaServiceReadsStoreBackedCatalogAndRewardState(t *testing.T) {
	userStore := store.New(func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	userStore.ReplaceGachaCatalog([]store.GachaCatalogEntry{
		{
			GachaID:        100,
			GachaLabelType: 1,
			GachaModeType:  1,
			StartDatetime:  time.Unix(1_700_000_000, 0).UnixMilli(),
			EndDatetime:    time.Unix(1_700_086_400, 0).UnixMilli(),
			SortOrder:      10,
		},
	})

	userService := NewUserServiceServer(userStore)
	gachaService := NewGachaServiceServer(userStore)

	authResp, err := userService.Auth(context.Background(), &pb.AuthUserRequest{
		Uuid:      "user-4",
		Signature: "sig",
	})
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	_, ok := userStore.UpdateUser(authResp.UserId, func(user *store.UserState) {
		user.Gacha.RewardAvailable = true
		user.Gacha.TodaysCurrentDrawCount = 2
		user.Gacha.DailyMaxCount = 5
		user.Gacha.ConvertedGachaMedal.ConvertedMedalPossession = []store.ConsumableItemState{
			{ConsumableItemID: 3001, Count: 7},
		}
		user.Gacha.ConvertedGachaMedal.ObtainPossession = &store.ConsumableItemState{
			ConsumableItemID: 3002,
			Count:            1,
		}
	})
	if !ok {
		t.Fatal("failed to update user gacha state")
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-session-key", authResp.SessionKey))
	listResp, err := gachaService.GetGachaList(ctx, &pb.GetGachaListRequest{GachaLabelType: []int32{1}})
	if err != nil {
		t.Fatalf("GetGachaList returned error: %v", err)
	}
	if len(listResp.Gacha) != 1 || listResp.Gacha[0].GachaId != 100 {
		t.Fatalf("GetGachaList returned %+v, want seeded catalog entry", listResp.Gacha)
	}
	if got := listResp.ConvertedGachaMedal.GetConvertedMedalPossession(); len(got) != 1 || got[0].ConsumableItemId != 3001 {
		t.Fatalf("ConvertedGachaMedal converted possession = %+v, want seeded state", got)
	}

	rewardResp, err := gachaService.GetRewardGacha(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetRewardGacha returned error: %v", err)
	}
	if !rewardResp.Available || rewardResp.TodaysCurrentDrawCount != 2 || rewardResp.DailyMaxCount != 5 {
		t.Fatalf("GetRewardGacha = %+v, want seeded reward state", rewardResp)
	}
}

func TestGiftServiceUsesStoreBackedDefaultGift(t *testing.T) {
	userStore := store.New(func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	userService := NewUserServiceServer(userStore)
	giftService := NewGiftServiceServer(userStore)
	notificationService := NewNotificationServiceServer(userStore)

	authResp, err := userService.Auth(context.Background(), &pb.AuthUserRequest{
		Uuid:      "user-5",
		Signature: "sig",
	})
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-session-key", authResp.SessionKey))
	listResp, err := giftService.GetGiftList(ctx, &pb.GetGiftListRequest{GetCount: 10})
	if err != nil {
		t.Fatalf("GetGiftList returned error: %v", err)
	}
	if len(listResp.Gift) != 1 {
		t.Fatalf("GetGiftList gift count = %d, want 1 default gift", len(listResp.Gift))
	}
	defaultGiftUUID := listResp.Gift[0].UserGiftUuid
	if defaultGiftUUID == "" {
		t.Fatal("default gift UUID should not be empty")
	}

	headerResp, err := notificationService.GetHeaderNotification(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetHeaderNotification returned error: %v", err)
	}
	if headerResp.GiftNotReceiveCount != 1 {
		t.Fatalf("GiftNotReceiveCount = %d, want 1", headerResp.GiftNotReceiveCount)
	}

	receiveResp, err := giftService.ReceiveGift(ctx, &pb.ReceiveGiftRequest{UserGiftUuid: []string{defaultGiftUUID}})
	if err != nil {
		t.Fatalf("ReceiveGift returned error: %v", err)
	}
	if len(receiveResp.ReceivedGiftUuid) != 1 || receiveResp.ReceivedGiftUuid[0] != defaultGiftUUID {
		t.Fatalf("ReceiveGift received UUIDs = %+v, want [%s]", receiveResp.ReceivedGiftUuid, defaultGiftUUID)
	}

	listResp, err = giftService.GetGiftList(ctx, &pb.GetGiftListRequest{GetCount: 10})
	if err != nil {
		t.Fatalf("GetGiftList after receive returned error: %v", err)
	}
	if len(listResp.Gift) != 0 {
		t.Fatalf("GetGiftList after receive gift count = %d, want 0", len(listResp.Gift))
	}

	historyResp, err := giftService.GetGiftReceiveHistoryList(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetGiftReceiveHistoryList returned error: %v", err)
	}
	if len(historyResp.Gift) != 1 {
		t.Fatalf("GetGiftReceiveHistoryList gift count = %d, want 1", len(historyResp.Gift))
	}

	headerResp, err = notificationService.GetHeaderNotification(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetHeaderNotification after receive returned error: %v", err)
	}
	if headerResp.GiftNotReceiveCount != 0 {
		t.Fatalf("GiftNotReceiveCount after receive = %d, want 0", headerResp.GiftNotReceiveCount)
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
