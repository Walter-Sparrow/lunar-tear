package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
)

type GameplayServiceServer struct {
	pb.UnimplementedGameplayServiceServer
}

func NewGameplayServiceServer() *GameplayServiceServer {
	return &GameplayServiceServer{}
}

func (s *GameplayServiceServer) CheckBeforeGamePlay(ctx context.Context, req *pb.CheckBeforeGamePlayRequest) (*pb.CheckBeforeGamePlayResponse, error) {
	log.Printf("[GamePlayService] CheckBeforeGamePlay: tr=%s voiceLang=%d textLang=%d",
		req.Tr, req.VoiceClientSystemLanguageTypeId, req.TextClientSystemLanguageTypeId)

	return &pb.CheckBeforeGamePlayResponse{
		IsExistUnreadPop:   false,
		MenuGachaBadgeInfo: []*pb.MenuGachaBadgeInfo{},
		DiffUserData:       mock.EmptyDiff(),
	}, nil
}
