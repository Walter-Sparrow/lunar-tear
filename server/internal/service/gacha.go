package service

import (
	"context"
	"log"
	"sort"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/store"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GachaServiceServer struct {
	pb.UnimplementedGachaServiceServer
	store *store.Store
}

func NewGachaServiceServer(userStore *store.Store) *GachaServiceServer {
	return &GachaServiceServer{store: userStore}
}

func (s *GachaServiceServer) GetGachaList(ctx context.Context, req *pb.GetGachaListRequest) (*pb.GetGachaListResponse, error) {
	log.Printf("[GachaService] GetGachaList: labels=%v", req.GachaLabelType)

	catalog := s.store.SnapshotGachaCatalog()
	sort.Slice(catalog, func(i, j int) bool {
		if catalog[i].SortOrder != catalog[j].SortOrder {
			return catalog[i].SortOrder < catalog[j].SortOrder
		}
		return catalog[i].GachaID < catalog[j].GachaID
	})

	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		user = s.store.EnsureUser("")
	}

	gachaList := make([]*pb.Gacha, 0, len(catalog))
	for _, entry := range catalog {
		if !matchesGachaLabel(req.GachaLabelType, entry.GachaLabelType) {
			continue
		}
		gachaList = append(gachaList, toProtoGacha(entry))
	}

	return &pb.GetGachaListResponse{
		Gacha:               gachaList,
		ConvertedGachaMedal: toProtoConvertedGachaMedal(user.Gacha.ConvertedGachaMedal),
		DiffUserData:        mock.EmptyDiff(),
	}, nil
}

func (s *GachaServiceServer) GetGacha(ctx context.Context, req *pb.GetGachaRequest) (*pb.GetGachaResponse, error) {
	log.Printf("[GachaService] GetGacha: ids=%v", req.GachaId)

	catalog := s.store.SnapshotGachaCatalog()
	byID := make(map[int32]*pb.Gacha, len(req.GachaId))
	for _, wantedID := range req.GachaId {
		for _, entry := range catalog {
			if entry.GachaID == wantedID {
				byID[wantedID] = toProtoGacha(entry)
				break
			}
		}
	}

	return &pb.GetGachaResponse{
		Gacha:        byID,
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *GachaServiceServer) Draw(ctx context.Context, req *pb.DrawRequest) (*pb.DrawResponse, error) {
	log.Printf("[GachaService] Draw: gachaId=%d phaseId=%d execCount=%d", req.GachaId, req.GachaPricePhaseId, req.ExecCount)
	return &pb.DrawResponse{
		NextGacha:          nil,
		GachaResult:        []*pb.DrawGachaOddsItem{},
		GachaBonus:         []*pb.GachaBonus{},
		MenuGachaBadgeInfo: []*pb.MenuGachaBadgeInfo{},
		DiffUserData:       mock.EmptyDiff(),
	}, nil
}

func (s *GachaServiceServer) ResetBoxGacha(ctx context.Context, req *pb.ResetBoxGachaRequest) (*pb.ResetBoxGachaResponse, error) {
	log.Printf("[GachaService] ResetBoxGacha: gachaId=%d", req.GachaId)
	return &pb.ResetBoxGachaResponse{
		Gacha:        nil,
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *GachaServiceServer) GetRewardGacha(ctx context.Context, req *emptypb.Empty) (*pb.GetRewardGachaResponse, error) {
	log.Printf("[GachaService] GetRewardGacha")
	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		user = s.store.EnsureUser("")
	}

	return &pb.GetRewardGachaResponse{
		Available:              user.Gacha.RewardAvailable,
		TodaysCurrentDrawCount: user.Gacha.TodaysCurrentDrawCount,
		DailyMaxCount:          user.Gacha.DailyMaxCount,
		DiffUserData:           mock.EmptyDiff(),
	}, nil
}

func (s *GachaServiceServer) RewardDraw(ctx context.Context, req *pb.RewardDrawRequest) (*pb.RewardDrawResponse, error) {
	log.Printf("[GachaService] RewardDraw: placement=%q reward=%q amount=%q", req.PlacementName, req.RewardName, req.RewardAmount)
	return &pb.RewardDrawResponse{
		RewardGachaResult: []*pb.RewardGachaItem{},
		DiffUserData:      mock.EmptyDiff(),
	}, nil
}

func matchesGachaLabel(labels []int32, label int32) bool {
	if len(labels) == 0 {
		return true
	}
	for _, candidate := range labels {
		if candidate == label {
			return true
		}
	}
	return false
}

func toProtoGacha(entry store.GachaCatalogEntry) *pb.Gacha {
	return &pb.Gacha{
		GachaId:                    entry.GachaID,
		GachaLabelType:             entry.GachaLabelType,
		GachaModeType:              entry.GachaModeType,
		GachaAutoResetType:         entry.GachaAutoResetType,
		GachaAutoResetPeriod:       entry.GachaAutoResetPeriod,
		NextAutoResetDatetime:      timestampOrNil(entry.NextAutoResetDatetime),
		GachaUnlockCondition:       []*pb.GachaUnlockCondition{},
		IsUserGachaUnlock:          entry.IsUserGachaUnlock,
		StartDatetime:              timestampOrNil(entry.StartDatetime),
		EndDatetime:                timestampOrNil(entry.EndDatetime),
		GachaPricePhase:            []*pb.GachaPricePhase{},
		RelatedMainQuestChapterId:  entry.RelatedMainQuestChapterID,
		RelatedEventQuestChapterId: entry.RelatedEventQuestChapterID,
		PromotionMovieAssetId:      entry.PromotionMovieAssetID,
		GachaMedalId:               entry.GachaMedalID,
		GachaDecorationType:        entry.GachaDecorationType,
		SortOrder:                  entry.SortOrder,
		IsInactive:                 entry.IsInactive,
		InformationId:              entry.InformationID,
		GachaMode:                  entry.GachaMode,
	}
}

func toProtoConvertedGachaMedal(state store.ConvertedGachaMedalState) *pb.ConvertedGachaMedal {
	items := make([]*pb.ConsumableItemPossession, 0, len(state.ConvertedMedalPossession))
	for _, item := range state.ConvertedMedalPossession {
		items = append(items, &pb.ConsumableItemPossession{
			ConsumableItemId: item.ConsumableItemID,
			Count:            item.Count,
		})
	}

	var obtain *pb.ConsumableItemPossession
	if state.ObtainPossession != nil {
		obtain = &pb.ConsumableItemPossession{
			ConsumableItemId: state.ObtainPossession.ConsumableItemID,
			Count:            state.ObtainPossession.Count,
		}
	}

	return &pb.ConvertedGachaMedal{
		ConvertedMedalPossession: items,
		ObtainPossession:         obtain,
	}
}

func timestampOrNil(unixMillis int64) *timestamppb.Timestamp {
	if unixMillis == 0 {
		return nil
	}
	return timestamppb.New(time.UnixMilli(unixMillis))
}
