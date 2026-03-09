package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/store"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type NotificationServiceServer struct {
	pb.UnimplementedNotificationServiceServer
	store *store.Store
}

func NewNotificationServiceServer(userStore *store.Store) *NotificationServiceServer {
	return &NotificationServiceServer{store: userStore}
}

func (s *NotificationServiceServer) GetHeaderNotification(ctx context.Context, req *emptypb.Empty) (*pb.GetHeaderNotificationResponse, error) {
	log.Printf("[NotificationService] GetHeaderNotification")
	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		return &pb.GetHeaderNotificationResponse{
			GiftNotReceiveCount:       0,
			FriendRequestReceiveCount: 0,
			IsExistUnreadInformation:  false,
			DiffUserData:              mock.EmptyDiff(),
		}, nil
	}
	return &pb.GetHeaderNotificationResponse{
		GiftNotReceiveCount:       user.Notifications.GiftNotReceiveCount,
		FriendRequestReceiveCount: user.Notifications.FriendRequestReceiveCount,
		IsExistUnreadInformation:  user.Notifications.IsExistUnreadInformation,
		DiffUserData:              mock.EmptyDiff(),
	}, nil
}
