package service

import (
	"context"
	"log"
	"slices"
	"sort"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/store"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GiftServiceServer struct {
	pb.UnimplementedGiftServiceServer
	store *store.Store
}

func NewGiftServiceServer(userStore *store.Store) *GiftServiceServer {
	return &GiftServiceServer{store: userStore}
}

func (s *GiftServiceServer) ReceiveGift(ctx context.Context, req *pb.ReceiveGiftRequest) (*pb.ReceiveGiftResponse, error) {
	log.Printf("[GiftService] ReceiveGift: giftUuids=%d", len(req.UserGiftUuid))

	userID := currentUserID(ctx, s.store)
	received := make([]string, 0, len(req.UserGiftUuid))
	_, ok := s.store.UpdateUser(userID, func(user *store.UserState) {
		nowMillis := time.Now().UnixMilli()
		remaining := make([]store.NotReceivedGiftState, 0, len(user.Gifts.NotReceived))
		for _, gift := range user.Gifts.NotReceived {
			if slices.Contains(req.UserGiftUuid, gift.UserGiftUUID) {
				received = append(received, gift.UserGiftUUID)
				user.Gifts.Received = append(user.Gifts.Received, store.ReceivedGiftState{
					GiftCommon:       gift.GiftCommon,
					ReceivedDatetime: nowMillis,
				})
				continue
			}
			remaining = append(remaining, gift)
		}
		user.Gifts.NotReceived = remaining
		user.Notifications.GiftNotReceiveCount = int32(len(user.Gifts.NotReceived))
	})
	if !ok {
		return &pb.ReceiveGiftResponse{
			ReceivedGiftUuid: []string{},
			ExpiredGiftUuid:  []string{},
			OverflowGiftUuid: []string{},
			DiffUserData:     mock.EmptyDiff(),
		}, nil
	}

	return &pb.ReceiveGiftResponse{
		ReceivedGiftUuid: received,
		ExpiredGiftUuid:  []string{},
		OverflowGiftUuid: []string{},
		DiffUserData:     mock.EmptyDiff(),
	}, nil
}

func (s *GiftServiceServer) GetGiftList(ctx context.Context, req *pb.GetGiftListRequest) (*pb.GetGiftListResponse, error) {
	log.Printf("[GiftService] GetGiftList: rewardKinds=%v expirationType=%d ascending=%v nextCursor=%d previousCursor=%d getCount=%d",
		req.RewardKindType, req.ExpirationType, req.IsAscendingSort, req.NextCursor, req.PreviousCursor, req.GetCount)

	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		user = s.store.EnsureUser("")
	}

	gifts := append([]store.NotReceivedGiftState(nil), user.Gifts.NotReceived...)
	sort.Slice(gifts, func(i, j int) bool {
		if req.IsAscendingSort {
			return gifts[i].ExpirationDatetime < gifts[j].ExpirationDatetime
		}
		return gifts[i].ExpirationDatetime > gifts[j].ExpirationDatetime
	})
	if req.GetCount > 0 && len(gifts) > int(req.GetCount) {
		gifts = gifts[:req.GetCount]
	}

	items := make([]*pb.NotReceivedGift, 0, len(gifts))
	for _, gift := range gifts {
		items = append(items, &pb.NotReceivedGift{
			GiftCommon:         toProtoGiftCommon(gift.GiftCommon),
			ExpirationDatetime: timestampOrNilGift(gift.ExpirationDatetime),
			UserGiftUuid:       gift.UserGiftUUID,
		})
	}

	return &pb.GetGiftListResponse{
		Gift:           items,
		TotalPageCount: pageCount(len(user.Gifts.NotReceived), int(req.GetCount)),
		NextCursor:     0,
		PreviousCursor: 0,
		DiffUserData:   mock.EmptyDiff(),
	}, nil
}

func (s *GiftServiceServer) GetGiftReceiveHistoryList(ctx context.Context, req *emptypb.Empty) (*pb.GetGiftReceiveHistoryListResponse, error) {
	log.Printf("[GiftService] GetGiftReceiveHistoryList")
	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		user = s.store.EnsureUser("")
	}

	items := make([]*pb.ReceivedGift, 0, len(user.Gifts.Received))
	for _, gift := range user.Gifts.Received {
		items = append(items, &pb.ReceivedGift{
			GiftCommon:       toProtoGiftCommon(gift.GiftCommon),
			ReceivedDatetime: timestampOrNilGift(gift.ReceivedDatetime),
		})
	}
	return &pb.GetGiftReceiveHistoryListResponse{
		Gift:         items,
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func toProtoGiftCommon(gift store.GiftCommonState) *pb.GiftCommon {
	return &pb.GiftCommon{
		PossessionType:        gift.PossessionType,
		PossessionId:          gift.PossessionID,
		Count:                 gift.Count,
		GrantDatetime:         timestampOrNilGift(gift.GrantDatetime),
		DescriptionGiftTextId: gift.DescriptionGiftTextID,
		EquipmentData:         gift.EquipmentData,
	}
}

func timestampOrNilGift(unixMillis int64) *timestamppb.Timestamp {
	if unixMillis == 0 {
		return nil
	}
	return timestamppb.New(time.UnixMilli(unixMillis))
}

func pageCount(total, pageSize int) int32 {
	if total == 0 {
		return 0
	}
	if pageSize <= 0 {
		return 1
	}
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	return int32(pages)
}
