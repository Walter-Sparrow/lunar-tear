package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
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
	return &pb.UpdateMainFlowSceneProgressResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}

func (s *QuestServiceServer) UpdateReplayFlowSceneProgress(ctx context.Context, req *pb.UpdateReplayFlowSceneProgressRequest) (*pb.UpdateReplayFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateReplayFlowSceneProgress: questSceneId=%d", req.QuestSceneId)
	return &pb.UpdateReplayFlowSceneProgressResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}

func (s *QuestServiceServer) UpdateMainQuestSceneProgress(ctx context.Context, req *pb.UpdateMainQuestSceneProgressRequest) (*pb.UpdateMainQuestSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainQuestSceneProgress: questSceneId=%d", req.QuestSceneId)
	return &pb.UpdateMainQuestSceneProgressResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}

func (s *QuestServiceServer) StartMainQuest(ctx context.Context, req *pb.StartMainQuestRequest) (*pb.StartMainQuestResponse, error) {
	log.Printf("[QuestService] StartMainQuest: questId=%d isMainFlow=%v deckNum=%d battleOnly=%v replayFlow=%v",
		req.QuestId, req.IsMainFlow, req.UserDeckNumber, req.IsBattleOnly, req.IsReplayFlow)
	return &pb.StartMainQuestResponse{
		BattleDropReward: []*pb.BattleDropReward{},
		DiffUserData:     map[string]*pb.DiffData{},
	}, nil
}

func (s *QuestServiceServer) FinishMainQuest(ctx context.Context, req *pb.FinishMainQuestRequest) (*pb.FinishMainQuestResponse, error) {
	log.Printf("[QuestService] FinishMainQuest: questId=%d isMainFlow=%v isRetired=%v storySkipType=%d",
		req.QuestId, req.IsMainFlow, req.IsRetired, req.StorySkipType)
	return &pb.FinishMainQuestResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}

func (s *QuestServiceServer) FinishAutoOrbit(ctx context.Context, req *emptypb.Empty) (*pb.FinishAutoOrbitResponse, error) {
	log.Printf("[QuestService] FinishAutoOrbit")
	return &pb.FinishAutoOrbitResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}

func (s *QuestServiceServer) SetQuestSceneChoice(ctx context.Context, req *pb.SetQuestSceneChoiceRequest) (*pb.SetQuestSceneChoiceResponse, error) {
	log.Printf("[QuestService] SetQuestSceneChoice: questSceneId=%d choiceEffectId=%d",
		req.QuestSceneId, req.QuestSceneChoiceEffectId)
	return &pb.SetQuestSceneChoiceResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}
