package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/store"
	"lunar-tear/server/internal/userdata"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserServiceServer struct {
	pb.UnimplementedUserServiceServer
	store *store.Store
}

func NewUserServiceServer(userStore *store.Store) *UserServiceServer {
	return &UserServiceServer{store: userStore}
}

func setCommonResponseTrailers(ctx context.Context, diff map[string]*pb.DiffData, includeUpdateNames bool) {
	keys := make([]string, 0, len(diff))
	for key := range diff {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := []string{
		"x-apb-response-datetime", fmt.Sprintf("%d", time.Now().UnixMilli()),
	}
	if includeUpdateNames && len(keys) > 0 {
		pairs = append(pairs, "x-apb-update-user-data-names", keys[0])
		for _, key := range keys[1:] {
			pairs[len(pairs)-1] += "," + key
		}
	}

	if err := grpc.SetTrailer(ctx, metadata.Pairs(pairs...)); err != nil {
		log.Printf("[UserService] failed to set trailers: %v", err)
	}
}

func (s *UserServiceServer) RegisterUser(ctx context.Context, req *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	user := s.store.EnsureUser(req.Uuid)
	log.Printf("[UserService] RegisterUser: uuid=%s terminalId=%s -> userId=%d", req.Uuid, req.TerminalId, user.UserID)

	return &pb.RegisterUserResponse{
		UserId:       user.UserID,
		Signature:    fmt.Sprintf("sig_%d_%d", user.UserID, time.Now().Unix()),
		DiffUserData: userdata.BuildDiffFromTables(userdata.FirstEntranceClientTableMap(user)),
	}, nil
}

func (s *UserServiceServer) Auth(ctx context.Context, req *pb.AuthUserRequest) (*pb.AuthUserResponse, error) {
	log.Printf("[UserService] Auth: uuid=%s", req.Uuid)

	user, session := s.store.CreateSession(req.Uuid, 24*time.Hour)

	return &pb.AuthUserResponse{
		SessionKey:     session.SessionKey,
		ExpireDatetime: timestamppb.New(session.ExpireAt),
		Signature:      req.Signature,
		UserId:         user.UserID,
		DiffUserData:   userdata.BuildDiffFromTables(userdata.FirstEntranceClientTableMap(user)),
	}, nil
}

func (s *UserServiceServer) GameStart(ctx context.Context, _ *emptypb.Empty) (*pb.GameStartResponse, error) {
	log.Printf("[UserService] GameStart")

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-session-key"); len(vals) > 0 {
			log.Printf("[UserService] GameStart session: %s", vals[0])
		}
	}

	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		nowMillis := time.Now().UnixMilli()
		user.GameStartDatetime = nowMillis
		if user.Profile.Name == "Un-regist User Name" {
			user.Profile.Name = "Lunar Tear"
			user.Profile.NameUpdateDatetime = nowMillis
		}
	})
	fullTables := userdata.FullClientTableMap(user)
	diff := userdata.BuildDiffFromTables(userdata.SelectTables(fullTables, startedGameStartTables))
	setCommonResponseTrailers(ctx, diff, true)

	return &pb.GameStartResponse{
		// Apply only the starter outgame rows we need after title completion.
		// Keep IUser and other risky core-account rows out of GameStart diff.
		DiffUserData: diff,
	}, nil
}

func (s *UserServiceServer) TransferUser(ctx context.Context, req *pb.TransferUserRequest) (*pb.TransferUserResponse, error) {
	log.Printf("[UserService] TransferUser")
	user := s.store.EnsureUser(req.Uuid)
	return &pb.TransferUserResponse{
		UserId:       user.UserID,
		Signature:    "transferred-sig",
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *UserServiceServer) SetUserName(ctx context.Context, req *pb.SetUserNameRequest) (*pb.SetUserNameResponse, error) {
	log.Printf("[UserService] SetUserName: %s", req.Name)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		nowMillis := time.Now().UnixMilli()
		user.Profile.Name = req.Name
		user.Profile.NameUpdateDatetime = nowMillis
	})
	return &pb.SetUserNameResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{"IUserProfile"})),
	}, nil
}

func (s *UserServiceServer) SetUserMessage(ctx context.Context, req *pb.SetUserMessageRequest) (*pb.SetUserMessageResponse, error) {
	log.Printf("[UserService] SetUserMessage: %s", req.Message)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		nowMillis := time.Now().UnixMilli()
		user.Profile.Message = req.Message
		user.Profile.MessageUpdateDatetime = nowMillis
	})
	return &pb.SetUserMessageResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{"IUserProfile"})),
	}, nil
}

func (s *UserServiceServer) SetUserFavoriteCostumeId(ctx context.Context, req *pb.SetUserFavoriteCostumeIdRequest) (*pb.SetUserFavoriteCostumeIdResponse, error) {
	log.Printf("[UserService] SetUserFavoriteCostumeId: %d", req.FavoriteCostumeId)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		nowMillis := time.Now().UnixMilli()
		user.Profile.FavoriteCostumeID = req.FavoriteCostumeId
		user.Profile.FavoriteCostumeIDUpdateDatetime = nowMillis
	})
	return &pb.SetUserFavoriteCostumeIdResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{"IUserProfile"})),
	}, nil
}

func (s *UserServiceServer) GetUserProfile(ctx context.Context, req *pb.GetUserProfileRequest) (*pb.GetUserProfileResponse, error) {
	log.Printf("[UserService] GetUserProfile: playerId=%d", req.PlayerId)
	userID := req.PlayerId
	if userID == 0 {
		userID = currentUserID(ctx, s.store)
	}
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		return &pb.GetUserProfileResponse{DiffUserData: mock.EmptyDiff()}, nil
	}

	deckCharacters := []*pb.ProfileDeckCharacter{}
	if deck, ok := user.Decks[store.DeckKey{DeckType: 1, UserDeckNumber: 1}]; ok && deck.UserDeckCharacterUUID01 != "" {
		if deckCharacter, ok := user.DeckCharacters[deck.UserDeckCharacterUUID01]; ok {
			costumeID := int32(0)
			if costume, ok := user.Costumes[deckCharacter.UserCostumeUUID]; ok {
				costumeID = costume.CostumeID
			}
			mainWeaponID := int32(0)
			mainWeaponLevel := int32(0)
			if weapon, ok := user.Weapons[deckCharacter.MainUserWeaponUUID]; ok {
				mainWeaponID = weapon.WeaponID
				mainWeaponLevel = weapon.Level
			}
			deckCharacters = append(deckCharacters, &pb.ProfileDeckCharacter{
				CostumeId:       costumeID,
				MainWeaponId:    mainWeaponID,
				MainWeaponLevel: mainWeaponLevel,
			})
		}
	}

	return &pb.GetUserProfileResponse{
		Level:             user.Status.Level,
		Name:              user.Profile.Name,
		FavoriteCostumeId: user.Profile.FavoriteCostumeID,
		Message:           user.Profile.Message,
		IsFriend:          false,
		LatestUsedDeck: &pb.ProfileDeck{
			Power:         100,
			DeckCharacter: deckCharacters,
		},
		PvpInfo: &pb.ProfilePvpInfo{},
		GamePlayHistory: &pb.GamePlayHistory{
			HistoryItem:              []*pb.PlayHistoryItem{},
			HistoryCategoryGraphItem: []*pb.PlayHistoryCategoryGraphItem{},
		},
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *UserServiceServer) SetBirthYearMonth(ctx context.Context, req *pb.SetBirthYearMonthRequest) (*pb.SetBirthYearMonthResponse, error) {
	log.Printf("[UserService] SetBirthYearMonth: %d/%d", req.BirthYear, req.BirthMonth)
	userID := currentUserID(ctx, s.store)
	s.store.UpdateUser(userID, func(user *store.UserState) {
		user.BirthYear = req.BirthYear
		user.BirthMonth = req.BirthMonth
	})
	return &pb.SetBirthYearMonthResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetBirthYearMonth(ctx context.Context, _ *emptypb.Empty) (*pb.GetBirthYearMonthResponse, error) {
	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		return &pb.GetBirthYearMonthResponse{BirthYear: mock.DefaultBirthYear, BirthMonth: mock.DefaultBirthMonth, DiffUserData: mock.EmptyDiff()}, nil
	}
	return &pb.GetBirthYearMonthResponse{BirthYear: user.BirthYear, BirthMonth: user.BirthMonth, DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetChargeMoney(ctx context.Context, _ *emptypb.Empty) (*pb.GetChargeMoneyResponse, error) {
	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		return &pb.GetChargeMoneyResponse{ChargeMoneyThisMonth: mock.DefaultChargeMoneyThisMonth, DiffUserData: mock.EmptyDiff()}, nil
	}
	return &pb.GetChargeMoneyResponse{ChargeMoneyThisMonth: user.ChargeMoneyThisMonth, DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) SetUserSetting(ctx context.Context, req *pb.SetUserSettingRequest) (*pb.SetUserSettingResponse, error) {
	log.Printf("[UserService] SetUserSetting: isNotifyPurchaseAlert=%v", req.IsNotifyPurchaseAlert)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		user.Setting.IsNotifyPurchaseAlert = req.IsNotifyPurchaseAlert
	})
	return &pb.SetUserSettingResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{"IUserSetting"})),
	}, nil
}

func (s *UserServiceServer) GetAndroidArgs(ctx context.Context, req *pb.GetAndroidArgsRequest) (*pb.GetAndroidArgsResponse, error) {
	return &pb.GetAndroidArgsResponse{Nonce: "Mama", ApiKey: "1234567890", DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetBackupToken(ctx context.Context, req *pb.GetBackupTokenRequest) (*pb.GetBackupTokenResponse, error) {
	userID := currentUserID(ctx, s.store)
	user, ok := s.store.SnapshotUser(userID)
	if !ok {
		return &pb.GetBackupTokenResponse{BackupToken: mock.DefaultBackupToken, DiffUserData: mock.EmptyDiff()}, nil
	}
	return &pb.GetBackupTokenResponse{BackupToken: user.BackupToken, DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) CheckTransferSetting(ctx context.Context, _ *emptypb.Empty) (*pb.CheckTransferSettingResponse, error) {
	return &pb.CheckTransferSettingResponse{DiffUserData: mock.EmptyDiff()}, nil
}

func (s *UserServiceServer) GetUserGamePlayNote(ctx context.Context, req *pb.GetUserGamePlayNoteRequest) (*pb.GetUserGamePlayNoteResponse, error) {
	return &pb.GetUserGamePlayNoteResponse{DiffUserData: mock.EmptyDiff()}, nil
}
