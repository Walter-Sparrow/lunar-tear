package service

import (
	"context"
	"log"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/store"
	"lunar-tear/server/internal/userdata"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type GimmickServiceServer struct {
	pb.UnimplementedGimmickServiceServer
	store *store.Store
}

func NewGimmickServiceServer(userStore *store.Store) *GimmickServiceServer {
	return &GimmickServiceServer{store: userStore}
}

func (s *GimmickServiceServer) UpdateSequence(ctx context.Context, req *pb.UpdateSequenceRequest) (*pb.UpdateSequenceResponse, error) {
	log.Printf("[GimmickService] UpdateSequence: scheduleId=%d sequenceId=%d",
		req.GimmickSequenceScheduleId, req.GimmickSequenceId)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		key := store.GimmickSequenceKey{
			GimmickSequenceScheduleID: req.GimmickSequenceScheduleId,
			GimmickSequenceID:         req.GimmickSequenceId,
		}
		sequence := user.Gimmick.Sequences[key]
		sequence.Key = key
		user.Gimmick.Sequences[key] = sequence
	})
	return &pb.UpdateSequenceResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{"IUserGimmickSequence"})),
	}, nil
}

func (s *GimmickServiceServer) UpdateGimmickProgress(ctx context.Context, req *pb.UpdateGimmickProgressRequest) (*pb.UpdateGimmickProgressResponse, error) {
	log.Printf("[GimmickService] UpdateGimmickProgress: scheduleId=%d sequenceId=%d gimmickId=%d ornamentIndex=%d progressValueBit=%d flowType=%d",
		req.GimmickSequenceScheduleId, req.GimmickSequenceId, req.GimmickId, req.GimmickOrnamentIndex, req.ProgressValueBit, req.FlowType)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		nowMillis := time.Now().UnixMilli()
		progressKey := store.GimmickKey{
			GimmickSequenceScheduleID: req.GimmickSequenceScheduleId,
			GimmickSequenceID:         req.GimmickSequenceId,
			GimmickID:                 req.GimmickId,
		}
		progress := user.Gimmick.Progress[progressKey]
		progress.Key = progressKey
		progress.StartDatetime = nowMillis
		user.Gimmick.Progress[progressKey] = progress

		ornamentKey := store.GimmickOrnamentKey{
			GimmickSequenceScheduleID: req.GimmickSequenceScheduleId,
			GimmickSequenceID:         req.GimmickSequenceId,
			GimmickID:                 req.GimmickId,
			GimmickOrnamentIndex:      req.GimmickOrnamentIndex,
		}
		ornament := user.Gimmick.OrnamentProgress[ornamentKey]
		ornament.Key = ornamentKey
		ornament.ProgressValueBit = req.ProgressValueBit
		ornament.BaseDatetime = nowMillis
		user.Gimmick.OrnamentProgress[ornamentKey] = ornament
	})
	return &pb.UpdateGimmickProgressResponse{
		GimmickOrnamentReward:      []*pb.GimmickReward{},
		IsSequenceCleared:          false,
		GimmickSequenceClearReward: []*pb.GimmickReward{},
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{
			"IUserGimmick",
			"IUserGimmickOrnamentProgress",
		})),
	}, nil
}

func (s *GimmickServiceServer) InitSequenceSchedule(ctx context.Context, _ *emptypb.Empty) (*pb.InitSequenceScheduleResponse, error) {
	log.Printf("[GimmickService] InitSequenceSchedule")
	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		user = s.store.EnsureUser("")
	}
	return &pb.InitSequenceScheduleResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), gimmickDiffTables)),
	}, nil
}

func (s *GimmickServiceServer) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	log.Printf("[GimmickService] Unlock: gimmickKeys=%d", len(req.GimmickKey))
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		for _, item := range req.GimmickKey {
			key := store.GimmickKey{
				GimmickSequenceScheduleID: item.GimmickSequenceScheduleId,
				GimmickSequenceID:         item.GimmickSequenceId,
				GimmickID:                 item.GimmickId,
			}
			unlock := user.Gimmick.Unlocks[key]
			unlock.Key = key
			unlock.IsUnlocked = true
			user.Gimmick.Unlocks[key] = unlock
		}
	})
	return &pb.UnlockResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{"IUserGimmickUnlock"})),
	}, nil
}
