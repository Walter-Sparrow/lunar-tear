package service

import (
	"context"
	"log"
	"strconv"
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
	store  *store.Store
	engine *questflow.Engine
}

func NewQuestServiceServer(userStore *store.Store, engine *questflow.Engine) *QuestServiceServer {
	if engine == nil {
		panic("quest flow engine is required")
	}
	return &QuestServiceServer{store: userStore, engine: engine}
}

func buildSelectedQuestDiff(user store.UserState, tableNames []string) map[string]*pb.DiffData {
	return userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), tableNames))
}

func int32String(value int32) string {
	return strconv.FormatInt(int64(value), 10)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func logQuestState(prefix string, user store.UserState, descriptor *questflow.SceneDescriptor) {
	mainQuest := user.MainQuest
	line := "[QuestService] " + prefix
	currentQuestID := int32(0)
	nextQuestID := int32(0)
	if descriptor != nil {
		currentQuestID = descriptor.QuestID
		nextQuestID = descriptor.NextQuestID
		line += " sceneId=" + int32String(descriptor.SceneID) +
			" questId=" + int32String(descriptor.QuestID) +
			" phase=" + descriptor.Phase.String() +
			" background=" + boolString(descriptor.IsBackgroundQuest) +
			" nextQuestId=" + int32String(descriptor.NextQuestID)
	}
	line += " activeQuestId=" + int32String(mainQuest.ActiveQuestID) +
		" clearReadyQuestId=" + int32String(mainQuest.ClearReadyQuestID) +
		" currentSceneId=" + int32String(mainQuest.CurrentQuestSceneID) +
		" headSceneId=" + int32String(mainQuest.HeadQuestSceneID) +
		" currentFlowType=" + int32String(mainQuest.CurrentQuestFlowType) +
		" progressSceneId=" + int32String(mainQuest.ProgressQuestSceneID) +
		" progressHeadSceneId=" + int32String(mainQuest.ProgressHeadQuestSceneID) +
		" progressFlowType=" + int32String(mainQuest.ProgressQuestFlowType) +
		" isReachedLast=" + boolString(mainQuest.IsReachedLastQuestScene)
	log.Print(line)

	if currentQuestID == 0 {
		currentQuestID = mainQuest.ActiveQuestID
	}
	if currentQuestID == 0 {
		currentQuestID = mainQuest.ClearReadyQuestID
	}
	if currentQuestID != 0 {
		if quest, ok := user.Quests[currentQuestID]; ok {
			log.Printf("[QuestService] %s currentQuest questId=%d stateType=%d isBattleOnly=%v latestStart=%d clearCount=%d dailyClearCount=%d lastClear=%d",
				prefix, quest.QuestID, quest.QuestStateType, quest.IsBattleOnly, quest.LatestStartDatetime, quest.ClearCount, quest.DailyClearCount, quest.LastClearDatetime)
		} else {
			log.Printf("[QuestService] %s currentQuest questId=%d missing", prefix, currentQuestID)
		}
	}
	if nextQuestID != 0 {
		if quest, ok := user.Quests[nextQuestID]; ok {
			log.Printf("[QuestService] %s nextQuest questId=%d stateType=%d isBattleOnly=%v latestStart=%d clearCount=%d dailyClearCount=%d lastClear=%d",
				prefix, quest.QuestID, quest.QuestStateType, quest.IsBattleOnly, quest.LatestStartDatetime, quest.ClearCount, quest.DailyClearCount, quest.LastClearDatetime)
		} else {
			log.Printf("[QuestService] %s nextQuest questId=%d missing", prefix, nextQuestID)
		}
	}
}

func sceneTransitionDiffTables(descriptor questflow.SceneDescriptor) []string {
	if descriptor.IsBackgroundQuest {
		return []string{
			"IUserMainQuestFlowStatus",
			"IUserMainQuestMainFlowStatus",
			"IUserMainQuestProgressStatus",
			"IUserMainQuestSeasonRoute",
		}
	}
	return []string{
		"IUserQuest",
		"IUserQuestMission",
		"IUserMainQuestFlowStatus",
		"IUserMainQuestMainFlowStatus",
		"IUserMainQuestProgressStatus",
		"IUserMainQuestSeasonRoute",
	}
}

func (s *QuestServiceServer) UpdateMainFlowSceneProgress(ctx context.Context, req *pb.UpdateMainFlowSceneProgressRequest) (*pb.UpdateMainFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainFlowSceneProgress: questSceneId=%d", req.QuestSceneId)

	userID := currentUserID(ctx, s.store)
	var descriptor questflow.SceneDescriptor
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		descriptor, _ = s.engine.ApplySceneTransition(user, req.QuestSceneId, questflow.SceneUpdateModeMainFlow, time.Now().UnixMilli())
	})
	logQuestState("UpdateMainFlowSceneProgress state", user, &descriptor)

	return &pb.UpdateMainFlowSceneProgressResponse{
		DiffUserData: buildSelectedQuestDiff(user, sceneTransitionDiffTables(descriptor)),
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
	var descriptor questflow.SceneDescriptor
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		descriptor, _ = s.engine.ApplySceneTransition(user, req.QuestSceneId, questflow.SceneUpdateModeQuestProgress, time.Now().UnixMilli())
	})
	logQuestState("UpdateMainQuestSceneProgress state", user, &descriptor)

	return &pb.UpdateMainQuestSceneProgressResponse{
		DiffUserData: buildSelectedQuestDiff(user, sceneTransitionDiffTables(descriptor)),
	}, nil
}

func (s *QuestServiceServer) StartMainQuest(ctx context.Context, req *pb.StartMainQuestRequest) (*pb.StartMainQuestResponse, error) {
	log.Printf("[QuestService] StartMainQuest: questId=%d isMainFlow=%v deckNum=%d battleOnly=%v replayFlow=%v",
		req.QuestId, req.IsMainFlow, req.UserDeckNumber, req.IsBattleOnly, req.IsReplayFlow)

	nowMillis := time.Now().UnixMilli()
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		s.engine.ApplyQuestStart(user, req.QuestId, req.IsBattleOnly, nowMillis)
	})
	logQuestState("StartMainQuest state", user, nil)

	return &pb.StartMainQuestResponse{
		BattleDropReward: []*pb.BattleDropReward{},
		DiffUserData:     buildSelectedQuestDiff(user, []string{"IUserQuest", "IUserQuestMission"}),
	}, nil
}

func (s *QuestServiceServer) FinishMainQuest(ctx context.Context, req *pb.FinishMainQuestRequest) (*pb.FinishMainQuestResponse, error) {
	log.Printf("[QuestService] FinishMainQuest: questId=%d isMainFlow=%v isRetired=%v storySkipType=%d",
		req.QuestId, req.IsMainFlow, req.IsRetired, req.StorySkipType)

	nowMillis := time.Now().UnixMilli()
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		s.engine.ApplyQuestFinish(user, req.QuestId, req.IsMainFlow, nowMillis)
	})
	logQuestState("FinishMainQuest state", user, nil)

	return &pb.FinishMainQuestResponse{
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
