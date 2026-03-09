package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type GimmickServiceServer struct {
	pb.UnimplementedGimmickServiceServer
}

func NewGimmickServiceServer() *GimmickServiceServer {
	return &GimmickServiceServer{}
}

func (s *GimmickServiceServer) UpdateSequence(ctx context.Context, req *pb.UpdateSequenceRequest) (*pb.UpdateSequenceResponse, error) {
	log.Printf("[GimmickService] UpdateSequence: scheduleId=%d sequenceId=%d",
		req.GimmickSequenceScheduleId, req.GimmickSequenceId)
	return &pb.UpdateSequenceResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *GimmickServiceServer) UpdateGimmickProgress(ctx context.Context, req *pb.UpdateGimmickProgressRequest) (*pb.UpdateGimmickProgressResponse, error) {
	log.Printf("[GimmickService] UpdateGimmickProgress: scheduleId=%d sequenceId=%d gimmickId=%d ornamentIndex=%d progressValueBit=%d flowType=%d",
		req.GimmickSequenceScheduleId, req.GimmickSequenceId, req.GimmickId, req.GimmickOrnamentIndex, req.ProgressValueBit, req.FlowType)
	return &pb.UpdateGimmickProgressResponse{
		GimmickOrnamentReward:      []*pb.GimmickReward{},
		IsSequenceCleared:          false,
		GimmickSequenceClearReward: []*pb.GimmickReward{},
		DiffUserData:               mock.EmptyDiff(),
	}, nil
}

func (s *GimmickServiceServer) InitSequenceSchedule(ctx context.Context, _ *emptypb.Empty) (*pb.InitSequenceScheduleResponse, error) {
	log.Printf("[GimmickService] InitSequenceSchedule")
	return &pb.InitSequenceScheduleResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *GimmickServiceServer) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	log.Printf("[GimmickService] Unlock: gimmickKeys=%d", len(req.GimmickKey))
	return &pb.UnlockResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}
