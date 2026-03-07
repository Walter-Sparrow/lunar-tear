package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type NotificationServiceServer struct {
	pb.UnimplementedNotificationServiceServer
}

func NewNotificationServiceServer() *NotificationServiceServer {
	return &NotificationServiceServer{}
}

func (s *NotificationServiceServer) GetHeaderNotification(ctx context.Context, req *emptypb.Empty) (*pb.GetHeaderNotificationResponse, error) {
	log.Printf("[NotificationService] GetHeaderNotification")
	return &pb.GetHeaderNotificationResponse{
		GiftNotReceiveCount:        0,
		FriendRequestReceiveCount:  0,
		IsExistUnreadInformation:   false,
		DiffUserData:               mock.EmptyDiff(),
	}, nil
}
