package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type QuestServiceServer struct {
	pb.UnimplementedQuestServiceServer
}

func NewQuestServiceServer() *QuestServiceServer {
	return &QuestServiceServer{}
}

func buildMainQuestSceneProgressDiff(sceneID int32) (map[string]*pb.DiffData, string, string, string) {
	mainFlowJSON, _ := json.Marshal([]map[string]any{
		{
			"userId":                  mock.DefaultUserID,
			"currentMainQuestRouteId": 1,
			"currentQuestSceneId":     sceneID,
			"headQuestSceneId":        sceneID,
			"isReachedLastQuestScene": false,
			"latestVersion":           0,
		},
	})
	progressJSON, _ := json.Marshal([]map[string]any{
		{
			"userId":               mock.DefaultUserID,
			"currentQuestSceneId":  sceneID,
			"headQuestSceneId":     sceneID,
			"currentQuestFlowType": 1,
			"latestVersion":        0,
		},
	})
	flowJSON, _ := json.Marshal([]map[string]any{
		{
			"userId":               mock.DefaultUserID,
			"currentQuestFlowType": 1,
			"latestVersion":        0,
		},
	})

	diff := map[string]*pb.DiffData{
		"IUserMainQuestFlowStatus": {
			UpdateRecordsJson: string(flowJSON),
			DeleteKeysJson:    "[]",
		},
		"IUserMainQuestMainFlowStatus": {
			UpdateRecordsJson: string(mainFlowJSON),
			DeleteKeysJson:    "[]",
		},
		"IUserMainQuestProgressStatus": {
			UpdateRecordsJson: string(progressJSON),
			DeleteKeysJson:    "[]",
		},
	}

	return diff, string(flowJSON), string(mainFlowJSON), string(progressJSON)
}

func (s *QuestServiceServer) UpdateMainFlowSceneProgress(ctx context.Context, req *pb.UpdateMainFlowSceneProgressRequest) (*pb.UpdateMainFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainFlowSceneProgress: questSceneId=%d", req.QuestSceneId)

	diff, flowJSON, mainFlowJSON, progressJSON := buildMainQuestSceneProgressDiff(req.QuestSceneId)

	log.Printf(
		"[QuestService] UpdateMainFlowSceneProgress diff: IUserMainQuestFlowStatus=%s IUserMainQuestMainFlowStatus=%s IUserMainQuestProgressStatus=%s",
		flowJSON, mainFlowJSON, progressJSON,
	)

	return &pb.UpdateMainFlowSceneProgressResponse{
		DiffUserData: diff,
	}, nil
}

func (s *QuestServiceServer) UpdateReplayFlowSceneProgress(ctx context.Context, req *pb.UpdateReplayFlowSceneProgressRequest) (*pb.UpdateReplayFlowSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateReplayFlowSceneProgress: questSceneId=%d", req.QuestSceneId)
	return &pb.UpdateReplayFlowSceneProgressResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *QuestServiceServer) UpdateMainQuestSceneProgress(ctx context.Context, req *pb.UpdateMainQuestSceneProgressRequest) (*pb.UpdateMainQuestSceneProgressResponse, error) {
	log.Printf("[QuestService] UpdateMainQuestSceneProgress: questSceneId=%d", req.QuestSceneId)

	diff, flowJSON, mainFlowJSON, progressJSON := buildMainQuestSceneProgressDiff(req.QuestSceneId)
	log.Printf(
		"[QuestService] UpdateMainQuestSceneProgress diff: IUserMainQuestFlowStatus=%s IUserMainQuestMainFlowStatus=%s IUserMainQuestProgressStatus=%s",
		flowJSON, mainFlowJSON, progressJSON,
	)

	return &pb.UpdateMainQuestSceneProgressResponse{
		DiffUserData: diff,
	}, nil
}

func (s *QuestServiceServer) StartMainQuest(ctx context.Context, req *pb.StartMainQuestRequest) (*pb.StartMainQuestResponse, error) {
	log.Printf("[QuestService] StartMainQuest: questId=%d isMainFlow=%v deckNum=%d battleOnly=%v replayFlow=%v",
		req.QuestId, req.IsMainFlow, req.UserDeckNumber, req.IsBattleOnly, req.IsReplayFlow)

	nowMillis := time.Now().UnixMilli()
	questJSON, _ := json.Marshal([]map[string]any{
		{
			"userId":              mock.DefaultUserID,
			"questId":             req.QuestId,
			"questStateType":      1,
			"isBattleOnly":        req.IsBattleOnly,
			"latestStartDatetime": nowMillis,
			"clearCount":          0,
			"dailyClearCount":     0,
			"lastClearDatetime":   int64(0),
			"shortestClearFrames": 0,
			"latestVersion":       0,
		},
	})

	diff := map[string]*pb.DiffData{
		"IUserQuest": {
			UpdateRecordsJson: string(questJSON),
			DeleteKeysJson:    "[]",
		},
	}

	log.Printf("[QuestService] StartMainQuest diff: IUserQuest=%s", string(questJSON))

	return &pb.StartMainQuestResponse{
		BattleDropReward: []*pb.BattleDropReward{},
		DiffUserData:     diff,
	}, nil
}

func (s *QuestServiceServer) FinishMainQuest(ctx context.Context, req *pb.FinishMainQuestRequest) (*pb.FinishMainQuestResponse, error) {
	log.Printf("[QuestService] FinishMainQuest: questId=%d isMainFlow=%v isRetired=%v storySkipType=%d",
		req.QuestId, req.IsMainFlow, req.IsRetired, req.StorySkipType)

	nowMillis := time.Now().UnixMilli()
	questJSON, _ := json.Marshal([]map[string]any{
		{
			"userId":              mock.DefaultUserID,
			"questId":             req.QuestId,
			// Client clear checks use QuestStateType == 2.
			"questStateType":      2,
			"isBattleOnly":        false,
			"latestStartDatetime": nowMillis,
			"clearCount":          1,
			"dailyClearCount":     1,
			"lastClearDatetime":   nowMillis,
			"shortestClearFrames": 600,
			"latestVersion":       0,
		},
	})
	sceneDiff, flowJSON, mainFlowJSON, progressJSON := buildMainQuestSceneProgressDiff(3)

	diff := map[string]*pb.DiffData{
		"IUserQuest": {
			UpdateRecordsJson: string(questJSON),
			DeleteKeysJson:    "[]",
		},
		"IUserMainQuestFlowStatus":     sceneDiff["IUserMainQuestFlowStatus"],
		"IUserMainQuestMainFlowStatus": sceneDiff["IUserMainQuestMainFlowStatus"],
		"IUserMainQuestProgressStatus": sceneDiff["IUserMainQuestProgressStatus"],
	}

	log.Printf(
		"[QuestService] FinishMainQuest diff: IUserQuest=%s IUserMainQuestFlowStatus=%s IUserMainQuestMainFlowStatus=%s IUserMainQuestProgressStatus=%s",
		string(questJSON), flowJSON, mainFlowJSON, progressJSON,
	)

	return &pb.FinishMainQuestResponse{
		DiffUserData: diff,
	}, nil
}

func (s *QuestServiceServer) FinishAutoOrbit(ctx context.Context, req *emptypb.Empty) (*pb.FinishAutoOrbitResponse, error) {
	log.Printf("[QuestService] FinishAutoOrbit")
	return &pb.FinishAutoOrbitResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}

func (s *QuestServiceServer) SetQuestSceneChoice(ctx context.Context, req *pb.SetQuestSceneChoiceRequest) (*pb.SetQuestSceneChoiceResponse, error) {
	log.Printf("[QuestService] SetQuestSceneChoice: questSceneId=%d choiceNumber=%d",
		req.QuestSceneId, req.ChoiceNumber)
	return &pb.SetQuestSceneChoiceResponse{
		DiffUserData: mock.EmptyDiff(),
	}, nil
}
