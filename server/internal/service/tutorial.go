package service

import (
	"context"
	"log"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/store"
	"lunar-tear/server/internal/userdata"
)

type TutorialServiceServer struct {
	pb.UnimplementedTutorialServiceServer
	store *store.Store
}

func NewTutorialServiceServer(userStore *store.Store) *TutorialServiceServer {
	return &TutorialServiceServer{store: userStore}
}

func (s *TutorialServiceServer) SetTutorialProgress(ctx context.Context, req *pb.SetTutorialProgressRequest) (*pb.SetTutorialProgressResponse, error) {
	log.Printf("[TutorialService] SetTutorialProgress: type=%d phase=%d choice=%d", req.TutorialType, req.ProgressPhase, req.ChoiceId)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		user.Tutorial.TutorialType = req.TutorialType
		user.Tutorial.ProgressPhase = req.ProgressPhase
		user.Tutorial.ChoiceID = req.ChoiceId
	})
	return &pb.SetTutorialProgressResponse{
		TutorialChoiceReward: []*pb.TutorialChoiceReward{},
		DiffUserData:         userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{"IUserTutorialProgress"})),
	}, nil
}

func (s *TutorialServiceServer) SetTutorialProgressAndReplaceDeck(ctx context.Context, req *pb.SetTutorialProgressAndReplaceDeckRequest) (*pb.SetTutorialProgressAndReplaceDeckResponse, error) {
	log.Printf("[TutorialService] SetTutorialProgressAndReplaceDeck: type=%d phase=%d deckType=%d", req.TutorialType, req.ProgressPhase, req.DeckType)
	userID := currentUserID(ctx, s.store)
	user, _ := s.store.UpdateUser(userID, func(user *store.UserState) {
		user.Tutorial.TutorialType = req.TutorialType
		user.Tutorial.ProgressPhase = req.ProgressPhase

		deckKey := store.DeckKey{DeckType: req.DeckType, UserDeckNumber: req.UserDeckNumber}
		deck := user.Decks[deckKey]
		deck.DeckType = req.DeckType
		deck.UserDeckNumber = req.UserDeckNumber
		if deck.Name == "" {
			deck.Name = "Deck 1"
		}
		if deck.Power == 0 {
			deck.Power = 100
		}
		if deck.UserDeckCharacterUUID01 == "" {
			for deckCharacterID := range user.DeckCharacters {
				deck.UserDeckCharacterUUID01 = deckCharacterID
				break
			}
		}
		deck.LatestVersion = time.Now().UnixMilli()
		user.Decks[deckKey] = deck
	})
	return &pb.SetTutorialProgressAndReplaceDeckResponse{
		DiffUserData: userdata.BuildDiffFromTables(userdata.SelectTables(userdata.FullClientTableMap(user), []string{
			"IUserTutorialProgress",
			"IUserDeck",
		})),
	}, nil
}
