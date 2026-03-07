package service

import (
	"context"
	"log"

	pb "lunar-tear/server/gen/proto"
	"lunar-tear/server/internal/mock"
	"lunar-tear/server/internal/userdata"

	"google.golang.org/protobuf/types/known/emptypb"
)

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

	// The game expects table keys like "IUser", not snake_case filenames.
	// We keep our internal defaults in snake_case and map them to IUser* keys.
	defaultsSnake := userdata.DefaultUserDataJSON(mock.DefaultUserID)
	defaults := map[string]string{
		"IUser":                     defaultsSnake["user"],
		"IUserSetting":              defaultsSnake["user_setting"],
		"IUserMainQuestFlowStatus":  defaultsSnake["user_main_quest_flow_status"],
		"IUserMainQuestMainFlowStatus": defaultsSnake["user_main_quest_main_flow_status"],
		"IUserMainQuestProgressStatus": defaultsSnake["user_main_quest_progress_status"],
		"IUserMainQuestSeasonRoute": defaultsSnake["user_main_quest_season_route"],
		// Optional / usually empty for fresh user
		"IUserQuest": "[]",
	}
	result := make(map[string]string)

	for _, table := range req.TableName {
		if data, ok := defaults[table]; ok && data != "" {
			log.Printf("[DataService]   %s -> (len=%d)", table, len(data))
			result[table] = data
		} else {
			// Important: keep the key present with an empty JSON array.
			// Client deserializes each value as List<object>.
			log.Printf("[DataService]   %s -> []", table)
			result[table] = "[]"
		}
	}

	return &pb.UserDataGetResponse{
		UserDataJson: result,
	}, nil
}

func defaultTableNames() []string {
	return []string{
		// Extracted from nier-rein-apps DarkUserDataDatabaseBuilderAppendHelper
		// (parsers/appenders/differs keys). The client should only request these keys.
		"IUser",
		"IUserApple",
		"IUserAutoSaleSettingDetail",
		"IUserBeginnerCampaign",
		"IUserBigHuntMaxScore",
		"IUserBigHuntProgressStatus",
		"IUserBigHuntScheduleMaxScore",
		"IUserBigHuntStatus",
		"IUserBigHuntWeeklyMaxScore",
		"IUserBigHuntWeeklyStatus",
		"IUserCageOrnamentReward",
		"IUserCharacter",
		"IUserCharacterBoard",
		"IUserCharacterBoardAbility",
		"IUserCharacterBoardCompleteReward",
		"IUserCharacterBoardStatusUp",
		"IUserCharacterCostumeLevelBonus",
		"IUserCharacterRebirth",
		"IUserCharacterViewerField",
		"IUserComebackCampaign",
		"IUserCompanion",
		"IUserConsumableItem",
		"IUserContentsStory",
		"IUserCostume",
		"IUserCostumeActiveSkill",
		"IUserCostumeAwakenStatusUp",
		"IUserCostumeLevelBonusReleaseStatus",
		"IUserCostumeLotteryEffect",
		"IUserCostumeLotteryEffectAbility",
		"IUserCostumeLotteryEffectPending",
		"IUserCostumeLotteryEffectStatusUp",
		"IUserDeck",
		"IUserDeckCharacter",
		"IUserDeckCharacterDressupCostume",
		"IUserDeckLimitContentRestricted",
		"IUserDeckPartsGroup",
		"IUserDeckSubWeaponGroup",
		"IUserDeckTypeNote",
		"IUserDokan",
		"IUserEventQuestDailyGroupCompleteReward",
		"IUserEventQuestGuerrillaFreeOpen",
		"IUserEventQuestLabyrinthSeason",
		"IUserEventQuestLabyrinthStage",
		"IUserEventQuestProgressStatus",
		"IUserEventQuestTowerAccumulationReward",
		"IUserExplore",
		"IUserExploreScore",
		"IUserExtraQuestProgressStatus",
		"IUserFacebook",
		"IUserGem",
		"IUserGimmick",
		"IUserGimmickOrnamentProgress",
		"IUserGimmickSequence",
		"IUserGimmickUnlock",
		"IUserImportantItem",
		"IUserLimitedOpen",
		"IUserLogin",
		"IUserLoginBonus",
		"IUserMainQuestFlowStatus",
		"IUserMainQuestMainFlowStatus",
		"IUserMainQuestProgressStatus",
		"IUserMainQuestReplayFlowStatus",
		"IUserMainQuestSeasonRoute",
		"IUserMaterial",
		"IUserMission",
		"IUserMissionCompletionProgress",
		"IUserMissionPassPoint",
		"IUserMovie",
		"IUserNaviCutIn",
		"IUserOmikuji",
		"IUserParts",
		"IUserPartsGroupNote",
		"IUserPartsPreset",
		"IUserPartsPresetTag",
		"IUserPartsStatusSub",
		"IUserPortalCageStatus",
		"IUserPossessionAutoConvert",
		"IUserPremiumItem",
		"IUserProfile",
		"IUserPvpDefenseDeck",
		"IUserPvpStatus",
		"IUserPvpWeeklyResult",
		"IUserQuest",
		"IUserQuestAutoOrbit",
		"IUserQuestLimitContentStatus",
		"IUserQuestMission",
		"IUserQuestReplayFlowRewardGroup",
		"IUserQuestSceneChoice",
		"IUserQuestSceneChoiceHistory",
		"IUserSetting",
		"IUserShopItem",
		"IUserShopReplaceable",
		"IUserShopReplaceableLineup",
		"IUserSideStoryQuest",
		"IUserSideStoryQuestSceneProgressStatus",
		"IUserStatus",
		"IUserThought",
		"IUserTripleDeck",
		"IUserTutorialProgress",
		"IUserWeapon",
		"IUserWeaponAbility",
		"IUserWeaponAwaken",
		"IUserWeaponNote",
		"IUserWeaponSkill",
		"IUserWeaponStory",
		"IUserWebviewPanelMission",
	}
}
