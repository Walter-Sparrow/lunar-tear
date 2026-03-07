package service

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserServiceServer struct {
	pb.UnimplementedUserServiceServer
	nextUserID atomic.Int64
}

func NewUserServiceServer() *UserServiceServer {
	s := &UserServiceServer{}
	s.nextUserID.Store(1000)
	return s
}

func (s *UserServiceServer) RegisterUser(ctx context.Context, req *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	userID := s.nextUserID.Add(1)
	log.Printf("[UserService] RegisterUser: uuid=%s terminalId=%s -> userId=%d", req.Uuid, req.TerminalId, userID)

	return &pb.RegisterUserResponse{
		UserId:       userID,
		Signature:    fmt.Sprintf("sig_%d_%d", userID, time.Now().Unix()),
		DiffUserData: mock.BaselineDiff(userID),
	}, nil
}

func (s *UserServiceServer) Auth(ctx context.Context, req *pb.AuthUserRequest) (*pb.AuthUserResponse, error) {
	log.Printf("[UserService] Auth: uuid=%s", req.Uuid)

	sessionKey := fmt.Sprintf("session_%s_%d", req.Uuid, time.Now().Unix())
	expire := time.Now().Add(24 * time.Hour)

	return &pb.AuthUserResponse{
		SessionKey:     sessionKey,
		ExpireDatetime: timestamppb.New(expire),
		Signature:      req.Signature,
		UserId:         mock.DefaultUserID,
		DiffUserData:   mock.BaselineDiff(mock.DefaultUserID),
	}, nil
}

func (s *UserServiceServer) GameStart(ctx context.Context, _ *emptypb.Empty) (*pb.GameStartResponse, error) {
	log.Printf("[UserService] GameStart")

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-session-key"); len(vals) > 0 {
			log.Printf("[UserService] GameStart session: %s", vals[0])
		}
	}

	return &pb.GameStartResponse{
		DiffUserData: mock.BaselineDiff(mock.DefaultUserID),
	}, nil
}

func (s *UserServiceServer) TransferUser(ctx context.Context, req *pb.TransferUserRequest) (*pb.TransferUserResponse, error) {
	log.Printf("[UserService] TransferUser")
	return &pb.TransferUserResponse{
		UserId:       mock.DefaultUserID,
		Signature:    "transferred-sig",
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *UserServiceServer) SetUserName(ctx context.Context, req *pb.SetUserNameRequest) (*pb.SetUserNameResponse, error) {
	log.Printf("[UserService] SetUserName: %s", req.Name)
	return &pb.SetUserNameResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) SetUserMessage(ctx context.Context, req *pb.SetUserMessageRequest) (*pb.SetUserMessageResponse, error) {
	log.Printf("[UserService] SetUserMessage: %s", req.Message)
	return &pb.SetUserMessageResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) SetUserFavoriteCostumeId(ctx context.Context, req *pb.SetUserFavoriteCostumeIdRequest) (*pb.SetUserFavoriteCostumeIdResponse, error) {
	log.Printf("[UserService] SetUserFavoriteCostumeId: %d", req.FavoriteCostumeId)
	return &pb.SetUserFavoriteCostumeIdResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetUserProfile(ctx context.Context, req *pb.GetUserProfileRequest) (*pb.GetUserProfileResponse, error) {
	log.Printf("[UserService] GetUserProfile: playerId=%d", req.PlayerId)
	return &pb.GetUserProfileResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) SetBirthYearMonth(ctx context.Context, req *pb.SetBirthYearMonthRequest) (*pb.SetBirthYearMonthResponse, error) {
	log.Printf("[UserService] SetBirthYearMonth: %d/%d", req.BirthYear, req.BirthMonth)
	return &pb.SetBirthYearMonthResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetBirthYearMonth(ctx context.Context, _ *emptypb.Empty) (*pb.GetBirthYearMonthResponse, error) {
	return &pb.GetBirthYearMonthResponse{BirthYear: mock.DefaultBirthYear, BirthMonth: mock.DefaultBirthMonth, DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetChargeMoney(ctx context.Context, _ *emptypb.Empty) (*pb.GetChargeMoneyResponse, error) {
	return &pb.GetChargeMoneyResponse{ChargeMoneyThisMonth: mock.DefaultChargeMoneyThisMonth, DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) SetUserSetting(ctx context.Context, req *pb.SetUserSettingRequest) (*pb.SetUserSettingResponse, error) {
	log.Printf("[UserService] SetUserSetting: isNotifyPurchaseAlert=%v", req.IsNotifyPurchaseAlert)
	return &pb.SetUserSettingResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetAndroidArgs(ctx context.Context, req *pb.GetAndroidArgsRequest) (*pb.GetAndroidArgsResponse, error) {
	return &pb.GetAndroidArgsResponse{Nonce: "Mama", ApiKey: "1234567890", DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetBackupToken(ctx context.Context, req *pb.GetBackupTokenRequest) (*pb.GetBackupTokenResponse, error) {
	return &pb.GetBackupTokenResponse{BackupToken: mock.DefaultBackupToken, DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) CheckTransferSetting(ctx context.Context, _ *emptypb.Empty) (*pb.CheckTransferSettingResponse, error) {
	return &pb.CheckTransferSettingResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetUserGamePlayNote(ctx context.Context, req *pb.GetUserGamePlayNoteRequest) (*pb.GetUserGamePlayNoteResponse, error) {
	return &pb.GetUserGamePlayNoteResponse{DiffUserData: mock.EmptyDiff()}, nil
}
