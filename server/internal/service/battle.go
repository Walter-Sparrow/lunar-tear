package service

import (
	"context"
	"log"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/store"
)

type BattleServiceServer struct {
	pb.UnimplementedBattleServiceServer
	store *store.Store
}

func NewBattleServiceServer(userStore *store.Store) *BattleServiceServer {
	return &BattleServiceServer{store: userStore}
}

func (s *BattleServiceServer) StartWave(ctx context.Context, req *pb.StartWaveRequest) (*pb.StartWaveResponse, error) {
	log.Printf("[BattleService] StartWave: userParty=%d npcParty=%d", len(req.UserPartyInitialInfoList), len(req.NpcPartyInitialInfoList))
	userID := currentUserID(ctx, s.store)
	s.store.UpdateUser(userID, func(user *store.UserState) {
		user.Battle.IsActive = true
		user.Battle.StartCount++
		user.Battle.LastStartedAt = time.Now().UnixMilli()
		user.Battle.LastUserPartyCount = int32(len(req.UserPartyInitialInfoList))
		user.Battle.LastNpcPartyCount = int32(len(req.NpcPartyInitialInfoList))
	})
	return &pb.StartWaveResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *BattleServiceServer) FinishWave(ctx context.Context, req *pb.FinishWaveRequest) (*pb.FinishWaveResponse, error) {
	log.Printf("[BattleService] FinishWave: battleBinary=%d userParty=%d npcParty=%d elapsedFrames=%d",
		len(req.BattleBinary), len(req.UserPartyResultInfoList), len(req.NpcPartyResultInfoList), req.ElapsedFrameCount)
	userID := currentUserID(ctx, s.store)
	s.store.UpdateUser(userID, func(user *store.UserState) {
		user.Battle.IsActive = false
		user.Battle.FinishCount++
		user.Battle.LastFinishedAt = time.Now().UnixMilli()
		user.Battle.LastUserPartyCount = int32(len(req.UserPartyResultInfoList))
		user.Battle.LastNpcPartyCount = int32(len(req.NpcPartyResultInfoList))
		user.Battle.LastBattleBinarySize = int32(len(req.BattleBinary))
		user.Battle.LastElapsedFrameCount = req.ElapsedFrameCount
	})
	return &pb.FinishWaveResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}
