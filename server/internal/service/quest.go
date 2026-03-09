package service

import (
	"context"
	"log"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/store"
	"lunar-tear/server/internal/userdata"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type QuestServiceServer struct {
	pb.UnimplementedQuestServiceServer
	store *store.Store
}

func NewQuestServiceServer(userStore *store.Store) *QuestServiceServer {
	return &QuestServiceServer{store: userStore}
}

func buildSelectedQuestDiff(user store.UserState, tableNames []string) map[string]*pb.DiffData {
	return userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), tableNames))
}

func (s *QuestServiceServer) UpdateMainFlowSceneProgress(ctx context.Context, req *pb.UpdateMainFlowSceneProgressRequest) (*pb.UpdateMainFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainFlowSceneProgress: questSceneId=%d", req.QuestSceneId)

	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		user.MainQuest.CurrentQuestFlowType = 1
		user.MainQuest.CurrentQuestSceneID = req.QuestSceneId
		user.MainQuest.HeadQuestSceneID = req.QuestSceneId
		user.MainQuest.IsReachedLastQuestScene = false
		user.MainQuest.ProgressQuestSceneID = req.QuestSceneId
		user.MainQuest.ProgressHeadQuestSceneID = req.QuestSceneId
		user.MainQuest.ProgressQuestFlowType = 1
	})

	return &pb.UpdateMainFlowSceneProgressResponse{
		DiffUserData: buildSelectedQuestDiff(user, mainQuestDiffTables),
	}, nil
}

func (s *QuestServiceServer) UpdateReplayFlowSceneProgress(ctx context.Context, req *pb.UpdateReplayFlowSceneProgressRequest) (*pb.UpdateReplayFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateReplayFlowSceneProgress: questSceneId=%d", req.QuestSceneId)
	return &pb.UpdateReplayFlowSceneProgressResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *QuestServiceServer) UpdateMainQuestSceneProgress(ctx context.Context, req *pb.UpdateMainQuestSceneProgressRequest) (*pb.UpdateMainQuestSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainQuestSceneProgress: questSceneId=%d", req.QuestSceneId)

	isRunning := req.QuestSceneId < 3
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		user.MainQuest.CurrentQuestSceneID = req.QuestSceneId
		user.MainQuest.HeadQuestSceneID = req.QuestSceneId
		user.MainQuest.IsReachedLastQuestScene = !isRunning
		if isRunning {
			user.MainQuest.CurrentQuestFlowType = 1
			user.MainQuest.ProgressQuestSceneID = req.QuestSceneId
			user.MainQuest.ProgressHeadQuestSceneID = req.QuestSceneId
			user.MainQuest.ProgressQuestFlowType = 1
			return
		}
		user.MainQuest.CurrentQuestFlowType = 0
		user.MainQuest.ProgressQuestSceneID = 0
		user.MainQuest.ProgressHeadQuestSceneID = 0
		user.MainQuest.ProgressQuestFlowType = 0
	})

	return &pb.UpdateMainQuestSceneProgressResponse{
		DiffUserData: buildSelectedQuestDiff(user, mainQuestDiffTables),
	}, nil
}

func (s *QuestServiceServer) StartMainQuest(ctx context.Context, req *pb.StartMainQuestRequest) (*pb.StartMainQuestResponse, error) {
	log.Printf("[QuestService] StartMainQuest: questId=%d isMainFlow=%v deckNum=%d battleOnly=%v replayFlow=%v",
		req.QuestId, req.IsMainFlow, req.UserDeckNumber, req.IsBattleOnly, req.IsReplayFlow)

	nowMillis := time.Now().UnixMilli()
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		quest := user.Quests[req.QuestId]
		quest.QuestID = req.QuestId
		quest.QuestStateType = 1
		quest.IsBattleOnly = req.IsBattleOnly
		quest.LatestStartDatetime = nowMillis
		user.Quests[req.QuestId] = quest
	})

	return &pb.StartMainQuestResponse{
		BattleDropReward: []*pb.BattleDropReward{},
		DiffUserData:     buildSelectedQuestDiff(user, []string{"IUserQuest"}),
	}, nil
}

func (s *QuestServiceServer) FinishMainQuest(ctx context.Context, req *pb.FinishMainQuestRequest) (*pb.FinishMainQuestResponse, error) {
	log.Printf("[QuestService] FinishMainQuest: questId=%d isMainFlow=%v isRetired=%v storySkipType=%d",
		req.QuestId, req.IsMainFlow, req.IsRetired, req.StorySkipType)

	nowMillis := time.Now().UnixMilli()
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		quest := user.Quests[req.QuestId]
		quest.QuestID = req.QuestId
		quest.QuestStateType = 2
		quest.IsBattleOnly = false
		if quest.LatestStartDatetime == 0 {
			quest.LatestStartDatetime = nowMillis
		}
		quest.ClearCount++
		quest.DailyClearCount++
		quest.LastClearDatetime = nowMillis
		quest.ShortestClearFrames = 600
		user.Quests[req.QuestId] = quest

		// Preserve the latest scene pointer established by scene-progress RPCs.
		// FinishMainQuest should only clear the active "running quest" markers.
		user.MainQuest.CurrentQuestFlowType = 0
		user.MainQuest.ProgressQuestSceneID = 0
		user.MainQuest.ProgressHeadQuestSceneID = 0
		user.MainQuest.ProgressQuestFlowType = 0
	})

	return &pb.FinishMainQuestResponse{
		DiffUserData: buildSelectedQuestDiff(user, []string{
			"IUserQuest",
			"IUserMainQuestFlowStatus",
			"IUserMainQuestMainFlowStatus",
			"IUserMainQuestProgressStatus",
		}),
	}, nil
}

func (s *QuestServiceServer) FinishAutoOrbit(ctx context.Context, req *emptypb.Empty) (*pb.FinishAutoOrbitResponse, error) {
	log.Printf("[QuestService] FinishAutoOrbit")
	return &pb.FinishAutoOrbitResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *QuestServiceServer) SetQuestSceneChoice(ctx context.Context, req *pb.SetQuestSceneChoiceRequest) (*pb.SetQuestSceneChoiceResponse, error) {
	log.Printf("[QuestService] SetQuestSceneChoice: questSceneId=%d choiceNumber=%d",
		req.QuestSceneId, req.ChoiceNumber)
	return &pb.SetQuestSceneChoiceResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}
