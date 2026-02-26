package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
)

type GamePlayServiceServer struct {
	pb.UnimplementedGamePlayServiceServer
}

func NewGamePlayServiceServer() *GamePlayServiceServer {
	return &GamePlayServiceServer{}
}

func (s *GamePlayServiceServer) CheckBeforeGamePlay(ctx context.Context, req *pb.CheckBeforeGamePlayRequest) (*pb.CheckBeforeGamePlayResponse, error) {
	log.Printf("[GamePlayService] CheckBeforeGamePlay: tr=%s voiceLang=%d textLang=%d",
		req.Tr, req.VoiceClientSystemLanguageTypeId, req.TextClientSystemLanguageTypeId)

	return &pb.CheckBeforeGamePlayResponse{
		IsExistUnreadPop:  false,
		MenuGachaBadgeInfo: []*pb.MenuGachaBadgeInfo{},
		DiffUserData:      map[string]*pb.DiffData{},
	}, nil
}
