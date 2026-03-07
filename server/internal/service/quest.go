package service

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type QuestServiceServer struct {
	pb.UnimplementedQuestServiceServer
}

func NewQuestServiceServer() *QuestServiceServer {
	return &QuestServiceServer{}
}

func (s *QuestServiceServer) UpdateMainFlowSceneProgress(ctx context.Context, req *pb.UpdateMainFlowSceneProgressRequest) (*pb.UpdateMainFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainFlowSceneProgress: questSceneId=%d", req.QuestSceneId)

	flowJSON := fmt.Sprintf(`[{"UserId":1001,"CurrentMainQuestRouteId":1,"CurrentQuestSceneId":%d,"HeadQuestSceneId":%d,"IsReachedLastQuestScene":false,"LatestVersion":0}]`,
		req.QuestSceneId, req.QuestSceneId)

	diff := map[string]*pb.DiffData{
		"user_main_quest_main_flow_status": {UpdateRecordsJson: flowJSON},
	}

	return &pb.UpdateMainFlowSceneProgressResponse{
		DiffUserData: diff,
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
	return &pb.UpdateMainQuestSceneProgressResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *QuestServiceServer) StartMainQuest(ctx context.Context, req *pb.StartMainQuestRequest) (*pb.StartMainQuestResponse, error) {
	log.Printf("[QuestService] StartMainQuest: questId=%d isMainFlow=%v deckNum=%d battleOnly=%v replayFlow=%v",
		req.QuestId, req.IsMainFlow, req.UserDeckNumber, req.IsBattleOnly, req.IsReplayFlow)

	now := time.Now().Unix()
	questJSON := fmt.Sprintf(`[{"UserId":1001,"QuestId":%d,"QuestStateType":1,"IsBattleOnly":false,"LatestStartDatetime":%d,"ClearCount":0,"DailyClearCount":0,"LastClearDatetime":0,"ShortestClearFrames":0,"LatestVersion":0}]`,
		req.QuestId, now)

	diff := map[string]*pb.DiffData{
		"user_quest": {UpdateRecordsJson: questJSON},
	}

	return &pb.StartMainQuestResponse{
		BattleDropReward: []*pb.BattleDropReward{},
		DiffUserData:     diff,
	}, nil
}

func (s *QuestServiceServer) FinishMainQuest(ctx context.Context, req *pb.FinishMainQuestRequest) (*pb.FinishMainQuestResponse, error) {
	log.Printf("[QuestService] FinishMainQuest: questId=%d isMainFlow=%v isRetired=%v storySkipType=%d",
		req.QuestId, req.IsMainFlow, req.IsRetired, req.StorySkipType)

	now := time.Now().Unix()
	questJSON := fmt.Sprintf(`[{"UserId":1001,"QuestId":%d,"QuestStateType":3,"IsBattleOnly":false,"LatestStartDatetime":%d,"ClearCount":1,"DailyClearCount":1,"LastClearDatetime":%d,"ShortestClearFrames":600,"LatestVersion":0}]`,
		req.QuestId, now, now)

	flowJSON := `[{"UserId":1001,"CurrentMainQuestRouteId":1,"CurrentQuestSceneId":3,"HeadQuestSceneId":3,"IsReachedLastQuestScene":false,"LatestVersion":0}]`

	diff := map[string]*pb.DiffData{
		"user_quest":                       {UpdateRecordsJson: questJSON},
		"user_main_quest_main_flow_status": {UpdateRecordsJson: flowJSON},
	}

	log.Printf("[QuestService] FinishMainQuest diff: user_quest=%s flow_status=%s", questJSON, flowJSON)

	return &pb.FinishMainQuestResponse{
		DiffUserData: diff,
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
