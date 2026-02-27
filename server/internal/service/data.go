package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/userdata"

	"google.golang.org/protobuf/types/known/emptypb"
)

const defaultUserID int64 = 1001

type DataServiceServer struct {
	pb.UnimplementedDataServiceServer
}

func NewDataServiceServer() *DataServiceServer {
	return &DataServiceServer{}
}

func (s *DataServiceServer) GetLatestMasterDataVersion(ctx context.Context, _ *emptypb.Empty) (*pb.MasterDataGetLatestVersionResponse, error) {
	log.Printf("[DataService] GetLatestMasterDataVersion")
	return &pb.MasterDataGetLatestVersionResponse{
		LatestMasterDataVersion: "20240404193219",
	}, nil
}

func (s *DataServiceServer) GetUserDataName(ctx context.Context, _ *emptypb.Empty) (*pb.UserDataGetNameResponse, error) {
	log.Printf("[DataService] GetUserDataName")
	return &pb.UserDataGetNameResponse{
		TableName: defaultTableNames(),
	}, nil
}

func (s *DataServiceServer) GetUserDataNameV2(ctx context.Context, _ *emptypb.Empty) (*pb.UserDataGetNameResponseV2, error) {
	log.Printf("[DataService] GetUserDataNameV2")
	return &pb.UserDataGetNameResponseV2{
		TableNameList: []*pb.TableNameList{
			{TableName: defaultTableNames()},
		},
	}, nil
}

func (s *DataServiceServer) GetUserData(ctx context.Context, req *pb.UserDataGetRequest) (*pb.UserDataGetResponse, error) {
	log.Printf("[DataService] GetUserData: tables=%v", req.TableName)
	log.Printf("[DataService] Available tables: user_main_quest_main_flow_status, user_main_quest_flow_status, etc.")

	defaults := userdata.DefaultUserDataJSON(defaultUserID)
	result := make(map[string]string)

	for _, table := range req.TableName {
		if data, ok := defaults[table]; ok {
			log.Printf("[DataService]   %s -> %s (len=%d)", table, data, len(data))
			result[table] = data
		} else {
			log.Printf("[DataService]   %s -> [] (empty, not in defaults)", table)
			result[table] = "[]"
		}
	}

	return &pb.UserDataGetResponse{
		UserDataJson: result,
	}, nil
}

func defaultTableNames() []string {
	return []string{
		"user",
		"user_quest",
		"user_main_quest_flow_status",
		"user_main_quest_main_flow_status",
		"user_main_quest_progress_status",
		"user_main_quest_season_route",
		"user_weapon",
		"user_costume",
		"user_companion",
		"user_parts",
		"user_deck",
		"user_gacha",
		"user_material",
		"user_consumable_item",
		"user_mission",
		"user_gift",
		"user_login_bonus",
		"user_tutorial",
		"user_explore",
		"user_shop",
		"user_setting",
		"user_character",
		"user_character_board",
		"user_pvp",
		"user_big_hunt",
	}
}
