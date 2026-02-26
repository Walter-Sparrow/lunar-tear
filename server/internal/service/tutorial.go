package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
)

type TutorialServiceServer struct {
	pb.UnimplementedTutorialServiceServer
}

func NewTutorialServiceServer() *TutorialServiceServer {
	return &TutorialServiceServer{}
}

func (s *TutorialServiceServer) SetTutorialProgress(ctx context.Context, req *pb.SetTutorialProgressRequest) (*pb.SetTutorialProgressResponse, error) {
	log.Printf("[TutorialService] SetTutorialProgress: type=%d phase=%d choice=%d", req.TutorialType, req.ProgressPhase, req.ChoiceId)
	return &pb.SetTutorialProgressResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}

func (s *TutorialServiceServer) SetTutorialProgressAndReplaceDeck(ctx context.Context, req *pb.SetTutorialProgressAndReplaceDeckRequest) (*pb.SetTutorialProgressAndReplaceDeckResponse, error) {
	log.Printf("[TutorialService] SetTutorialProgressAndReplaceDeck: type=%d phase=%d choice=%d", req.TutorialType, req.ProgressPhase, req.ChoiceId)
	return &pb.SetTutorialProgressAndReplaceDeckResponse{
		DiffUserData: map[string]*pb.DiffData{},
	}, nil
}
