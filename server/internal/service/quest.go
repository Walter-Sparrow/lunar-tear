package service

import (
	"context"
	"log"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/questflow"
	"lunar-tear/server/internal/store"
	"lunar-tear/server/internal/userdata"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type QuestServiceServer struct {
	pb.UnimplementedQuestServiceServer
	store     *store.Store
	engine    *questflow.Engine
	newEngine *questflow.NewEngine
}

func NewQuestServiceServer(userStore *store.Store, engine *questflow.Engine, newEngine *questflow.NewEngine) *QuestServiceServer {
	if engine == nil {
		panic("quest flow engine is required")
	}
	if newEngine == nil {
		panic("new quest flow engine is required")
	}
	return &QuestServiceServer{store: userStore, engine: engine, newEngine: newEngine}
}

func buildSelectedQuestDiff(user store.UserState, tableNames []string) map[string]*pb.DiffData {
	return userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), tableNames))
}

func logQuestState(prefix string, user store.UserState) {
	log.Printf("[QuestService] %s %#v %#v %#v", prefix, user.MainQuest, user.Status, user.Quests)
}

func (s *QuestServiceServer) UpdateMainFlowSceneProgress(ctx context.Context, req *pb.UpdateMainFlowSceneProgressRequest) (*pb.UpdateMainFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainFlowSceneProgress: questSceneId=%d", req.QuestSceneId)

	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		s.newEngine.HandleMainFlowSceneProgress(user, req.QuestSceneId)
	})
	logQuestState("UpdateMainFlowSceneProgress state", user)

	return &pb.UpdateMainFlowSceneProgressResponse{
		DiffUserData: buildSelectedQuestDiff(user, []string{
			"IUserMainQuestFlowStatus",
			"IUserMainQuestMainFlowStatus",
			"IUserMainQuestProgressStatus",
			"IUserMainQuestSeasonRoute",
		}),
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

	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		s.newEngine.HandleMainQuestSceneProgress(user, req.QuestSceneId)
	})
	logQuestState("UpdateMainQuestSceneProgress state", user)

	return &pb.UpdateMainQuestSceneProgressResponse{
		DiffUserData: buildSelectedQuestDiff(user, []string{
			"IUserQuest",
			"IUserQuestMission",
			"IUserMainQuestFlowStatus",
			"IUserMainQuestMainFlowStatus",
			"IUserMainQuestProgressStatus",
		}),
	}, nil
}

func (s *QuestServiceServer) StartMainQuest(ctx context.Context, req *pb.StartMainQuestRequest) (*pb.StartMainQuestResponse, error) {
	log.Printf("[QuestService] StartMainQuest: %+v", req)

	userID := currentUserID(ctx, s.store)
	nowMillis := time.Now().UnixMilli()
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		s.newEngine.HandleQuestStart(user, req.QuestId, req.IsBattleOnly, nowMillis)
	})
	logQuestState("StartMainQuest state", user)

	return &pb.StartMainQuestResponse{
		BattleDropReward: []*pb.BattleDropReward{},
		DiffUserData: buildSelectedQuestDiff(user, []string{
			"IUserQuest",
			"IUserQuestMission",
			"IUserMainQuestFlowStatus",
			"IUserMainQuestMainFlowStatus",
			"IUserMainQuestProgressStatus",
			"IUserMainQuestSeasonRoute",
		}),
	}, nil
}

func toProtoRewards(grants []questflow.RewardGrant) []*pb.QuestReward {
	if len(grants) == 0 {
		return []*pb.QuestReward{}
	}
	out := make([]*pb.QuestReward, len(grants))
	for i, g := range grants {
		out[i] = &pb.QuestReward{
			PossessionType: g.PossessionType,
			PossessionId:   g.PossessionID,
			Count:          g.Count,
		}
	}
	return out
}

func (s *QuestServiceServer) FinishMainQuest(ctx context.Context, req *pb.FinishMainQuestRequest) (*pb.FinishMainQuestResponse, error) {
	log.Printf("[QuestService] FinishMainQuest: questId=%d isMainFlow=%v isRetired=%v storySkipType=%d",
		req.QuestId, req.IsMainFlow, req.IsRetired, req.StorySkipType)

	nowMillis := time.Now().UnixMilli()
	userID := currentUserID(ctx, s.store)
	// var outcome questflow.FinishOutcome
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		s.newEngine.HandleQuestFinish(user, req.QuestId, nowMillis)
	})
	logQuestState("FinishMainQuest state", user)

	return &pb.FinishMainQuestResponse{
		// DropReward:                      []*pb.QuestReward{},
		// FirstClearReward:                toProtoRewards(outcome.FirstClearRewards),
		// MissionClearReward:              toProtoRewards(outcome.MissionClearRewards),
		// MissionClearCompleteReward:      toProtoRewards(outcome.MissionClearCompleteRewards),
		// AutoOrbitResult:                 []*pb.QuestReward{},
		// IsBigWin:                        outcome.IsBigWin,
		// BigWinClearedQuestMissionIdList: outcome.BigWinClearedQuestMissionIDs,
		// ReplayFlowFirstClearReward:      []*pb.QuestReward{},
		// UserStatusCampaignReward:        []*pb.QuestReward{},
		DiffUserData: buildSelectedQuestDiff(user, []string{
			"IUserQuest",
			"IUserQuestMission",
			"IUserMainQuestFlowStatus",
			"IUserMainQuestMainFlowStatus",
			"IUserMainQuestProgressStatus",
			"IUserMainQuestSeasonRoute",
		}),
	}, nil
}

func (s *QuestServiceServer) RestartMainQuest(ctx context.Context, req *pb.RestartMainQuestRequest) (*pb.RestartMainQuestResponse, error) {
	log.Printf("[QuestService] RestartMainQuest: questId=%d isMainFlow=%v", req.QuestId, req.IsMainFlow)

	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		s.engine.HandleQuestRestart(user, req.QuestId, time.Now().UnixMilli())
	})
	logQuestState("RestartMainQuest state", user)

	return &pb.RestartMainQuestResponse{
		BattleDropReward: []*pb.BattleDropReward{},
		DiffUserData:     buildSelectedQuestDiff(user, []string{"IUserQuest", "IUserQuestMission"}),
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
