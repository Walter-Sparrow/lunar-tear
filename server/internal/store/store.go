package store

import (
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"
)

const (
	defaultUUID   = "default-user"
	defaultUserID = int64(1001)

	starterCharacterID        = int32(1)
	starterCostumeID          = int32(1)
	starterWeaponID           = int32(1)
	starterCompanionID        = int32(1)
	starterQuestID            = int32(1)
	starterMissionID          = int32(1)
	starterMainQuestRouteID   = int32(1)
	starterMainQuestSeasonID  = int32(1)
	starterQuestDeckType      = int32(1)
	missionInProgress         = int32(1)
	defaultGiftPossessionType = int32(12) // FREE_GEM
	defaultGiftCount          = int32(300)

	starterCostumeUUID       = "starter-costume-0001"
	starterWeaponUUID        = "starter-weapon-0001"
	starterCompanionUUID     = "starter-companion-0001"
	starterDeckCharacterUUID = "starter-deck-character-0001"
	defaultGiftUUIDPrefix    = "default-gift"

	defaultRegisteredName = "Un-regist User Name"
	startedProfileName    = "Lunar Tear"

	defaultBirthYear            = int32(2000)
	defaultBirthMonth           = int32(1)
	defaultBackupToken          = "mock-backup-token"
	defaultChargeMoneyThisMonth = int64(0)
)

type Clock func() time.Time

type Store struct {
	mu              sync.RWMutex
	clock           Clock
	nextUserID      int64
	users           map[int64]*UserState
	userIDsByUUID   map[string]int64
	sessionToUserID map[string]int64
	sessions        map[string]SessionState
	gachaCatalog    map[int32]GachaCatalogEntry
}

type SessionState struct {
	SessionKey string
	UserID     int64
	UUID       string
	ExpireAt   time.Time
}

type UserState struct {
	UserID              int64
	UUID                string
	PlayerID            int64
	OsType              int32
	PlatformType        int32
	UserRestrictionType int32
	RegisterDatetime    int64
	GameStartDatetime   int64
	LatestVersion       int64

	BirthYear            int32
	BirthMonth           int32
	BackupToken          string
	ChargeMoneyThisMonth int64

	Setting       UserSettingState
	Status        UserStatusState
	Gem           UserGemState
	Profile       UserProfileState
	Login         UserLoginState
	LoginBonus    UserLoginBonusState
	Tutorial      TutorialProgressState
	MainQuest     MainQuestState
	Battle        BattleState
	Gifts         GiftState
	Gacha         GachaState
	Notifications NotificationState

	Characters     map[int32]CharacterState
	Costumes       map[string]CostumeState
	Weapons        map[string]WeaponState
	Companions     map[string]CompanionState
	DeckCharacters map[string]DeckCharacterState
	Decks          map[DeckKey]DeckState
	Quests         map[int32]UserQuestState
	Missions       map[int32]UserMissionState
	Gimmick        GimmickState
}

type UserSettingState struct {
	IsNotifyPurchaseAlert bool
	LatestVersion         int64
}

type UserStatusState struct {
	Level                 int32
	Exp                   int32
	StaminaMilliValue     int32
	StaminaUpdateDatetime int64
	LatestVersion         int64
}

type UserGemState struct {
	PaidGem int32
	FreeGem int32
}

type UserProfileState struct {
	Name                            string
	NameUpdateDatetime              int64
	Message                         string
	MessageUpdateDatetime           int64
	FavoriteCostumeID               int32
	FavoriteCostumeIDUpdateDatetime int64
	LatestVersion                   int64
}

type UserLoginState struct {
	TotalLoginCount           int32
	ContinualLoginCount       int32
	MaxContinualLoginCount    int32
	LastLoginDatetime         int64
	LastComebackLoginDatetime int64
	LatestVersion             int64
}

type UserLoginBonusState struct {
	LoginBonusID                int32
	CurrentPageNumber           int32
	CurrentStampNumber          int32
	LatestRewardReceiveDatetime int64
	LatestVersion               int64
}

type CharacterState struct {
	CharacterID   int32
	Level         int32
	Exp           int32
	LatestVersion int64
}

type CostumeState struct {
	UserCostumeUUID     string
	CostumeID           int32
	LimitBreakCount     int32
	Level               int32
	Exp                 int32
	HeadupDisplayViewID int32
	AcquisitionDatetime int64
	AwakenCount         int32
	LatestVersion       int64
}

type WeaponState struct {
	UserWeaponUUID      string
	WeaponID            int32
	Level               int32
	Exp                 int32
	LimitBreakCount     int32
	IsProtected         bool
	AcquisitionDatetime int64
	LatestVersion       int64
}

type CompanionState struct {
	UserCompanionUUID   string
	CompanionID         int32
	HeadupDisplayViewID int32
	Level               int32
	AcquisitionDatetime int64
	LatestVersion       int64
}

type DeckCharacterState struct {
	UserDeckCharacterUUID string
	UserCostumeUUID       string
	MainUserWeaponUUID    string
	UserCompanionUUID     string
	Power                 int32
	UserThoughtUUID       string
	LatestVersion         int64
}

type DeckKey struct {
	DeckType       int32
	UserDeckNumber int32
}

type DeckState struct {
	DeckType                int32
	UserDeckNumber          int32
	UserDeckCharacterUUID01 string
	UserDeckCharacterUUID02 string
	UserDeckCharacterUUID03 string
	Name                    string
	Power                   int32
	LatestVersion           int64
}

type TutorialProgressState struct {
	TutorialType  int32
	ProgressPhase int32
	ChoiceID      int32
	LatestVersion int64
}

type MainQuestState struct {
	CurrentQuestFlowType     int32
	CurrentMainQuestRouteID  int32
	CurrentQuestSceneID      int32
	HeadQuestSceneID         int32
	IsReachedLastQuestScene  bool
	ProgressQuestSceneID     int32
	ProgressHeadQuestSceneID int32
	ProgressQuestFlowType    int32
	MainQuestSeasonID        int32
	LatestVersion            int64
}

type BattleState struct {
	IsActive              bool
	StartCount            int32
	FinishCount           int32
	LastStartedAt         int64
	LastFinishedAt        int64
	LastUserPartyCount    int32
	LastNpcPartyCount     int32
	LastBattleBinarySize  int32
	LastElapsedFrameCount int64
}

type UserQuestState struct {
	QuestID             int32
	QuestStateType      int32
	IsBattleOnly        bool
	LatestStartDatetime int64
	ClearCount          int32
	DailyClearCount     int32
	LastClearDatetime   int64
	ShortestClearFrames int32
	LatestVersion       int64
}

type UserMissionState struct {
	MissionID                 int32
	StartDatetime             int64
	ProgressValue             int32
	MissionProgressStatusType int32
	ClearDatetime             int64
	LatestVersion             int64
}

type GimmickSequenceKey struct {
	GimmickSequenceScheduleID int32
	GimmickSequenceID         int32
}

type GimmickKey struct {
	GimmickSequenceScheduleID int32
	GimmickSequenceID         int32
	GimmickID                 int32
}

type GimmickOrnamentKey struct {
	GimmickSequenceScheduleID int32
	GimmickSequenceID         int32
	GimmickID                 int32
	GimmickOrnamentIndex      int32
}

type GimmickState struct {
	Progress         map[GimmickKey]GimmickProgressState
	OrnamentProgress map[GimmickOrnamentKey]GimmickOrnamentProgressState
	Sequences        map[GimmickSequenceKey]GimmickSequenceState
	Unlocks          map[GimmickKey]GimmickUnlockState
}

type GimmickProgressState struct {
	Key              GimmickKey
	IsGimmickCleared bool
	StartDatetime    int64
	LatestVersion    int64
}

type GimmickOrnamentProgressState struct {
	Key              GimmickOrnamentKey
	ProgressValueBit int32
	BaseDatetime     int64
	LatestVersion    int64
}

type GimmickSequenceState struct {
	Key                      GimmickSequenceKey
	IsGimmickSequenceCleared bool
	ClearDatetime            int64
	LatestVersion            int64
}

type GimmickUnlockState struct {
	Key           GimmickKey
	IsUnlocked    bool
	LatestVersion int64
}

type NotificationState struct {
	GiftNotReceiveCount       int32
	FriendRequestReceiveCount int32
	IsExistUnreadInformation  bool
}

type GiftState struct {
	NotReceived []NotReceivedGiftState
	Received    []ReceivedGiftState
}

type GiftCommonState struct {
	PossessionType        int32
	PossessionID          int32
	Count                 int32
	GrantDatetime         int64
	DescriptionGiftTextID int32
	EquipmentData         []byte
}

type NotReceivedGiftState struct {
	GiftCommon         GiftCommonState
	ExpirationDatetime int64
	UserGiftUUID       string
}

type ReceivedGiftState struct {
	GiftCommon       GiftCommonState
	ReceivedDatetime int64
}

type GachaState struct {
	RewardAvailable        bool
	TodaysCurrentDrawCount int32
	DailyMaxCount          int32
	ConvertedGachaMedal    ConvertedGachaMedalState
}

type ConvertedGachaMedalState struct {
	ConvertedMedalPossession []ConsumableItemState
	ObtainPossession         *ConsumableItemState
}

type ConsumableItemState struct {
	ConsumableItemID int32
	Count            int32
}

type GachaCatalogEntry struct {
	GachaID                    int32
	GachaLabelType             int32
	GachaModeType              int32
	GachaAutoResetType         int32
	GachaAutoResetPeriod       int32
	NextAutoResetDatetime      int64
	IsUserGachaUnlock          bool
	StartDatetime              int64
	EndDatetime                int64
	RelatedMainQuestChapterID  int32
	RelatedEventQuestChapterID int32
	PromotionMovieAssetID      int32
	GachaMedalID               int32
	GachaDecorationType        int32
	SortOrder                  int32
	IsInactive                 bool
	InformationID              int32
	GachaMode                  []byte
}

func New(clock Clock) *Store {
	if clock == nil {
		clock = time.Now
	}

	return &Store{
		clock:           clock,
		nextUserID:      defaultUserID,
		users:           make(map[int64]*UserState),
		userIDsByUUID:   make(map[string]int64),
		sessionToUserID: make(map[string]int64),
		sessions:        make(map[string]SessionState),
		gachaCatalog:    make(map[int32]GachaCatalogEntry),
	}
}

func (s *Store) EnsureUser(uuid string) UserState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getOrCreateLocked(normalizeUUID(uuid)).clone()
}

func (s *Store) CreateSession(uuid string, ttl time.Duration) (UserState, SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.getOrCreateLocked(normalizeUUID(uuid))
	now := s.clock()
	session := SessionState{
		SessionKey: fmt.Sprintf("session_%d_%d", user.UserID, now.UnixNano()),
		UserID:     user.UserID,
		UUID:       user.UUID,
		ExpireAt:   now.Add(ttl),
	}

	s.sessionToUserID[session.SessionKey] = user.UserID
	s.sessions[session.SessionKey] = session

	return user.clone(), session
}

func (s *Store) ResolveUserID(sessionKey string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.sessionToUserID[sessionKey]
	return userID, ok
}

func (s *Store) SnapshotUser(userID int64) (UserState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userID]
	if !ok {
		return UserState{}, false
	}
	return user.clone(), true
}

func (s *Store) UpdateUser(userID int64, mutate func(*UserState)) (UserState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return UserState{}, false
	}
	mutate(user)
	return user.clone(), true
}

func (s *Store) DefaultUserID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.users[defaultUserID]; ok {
		return defaultUserID
	}
	if len(s.users) == 0 {
		return defaultUserID
	}

	var minUserID int64
	for userID := range s.users {
		if minUserID == 0 || userID < minUserID {
			minUserID = userID
		}
	}
	return minUserID
}

func (s *Store) SnapshotGachaCatalog() []GachaCatalogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]GachaCatalogEntry, 0, len(s.gachaCatalog))
	for _, entry := range s.gachaCatalog {
		out = append(out, cloneGachaCatalogEntry(entry))
	}
	return out
}

func (s *Store) ReplaceGachaCatalog(entries []GachaCatalogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gachaCatalog = make(map[int32]GachaCatalogEntry, len(entries))
	for _, entry := range entries {
		s.gachaCatalog[entry.GachaID] = cloneGachaCatalogEntry(entry)
	}
}

func normalizeUUID(uuid string) string {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return defaultUUID
	}
	return uuid
}

func (s *Store) getOrCreateLocked(uuid string) *UserState {
	if userID, ok := s.userIDsByUUID[uuid]; ok {
		return s.users[userID]
	}

	userID := s.nextUserID
	s.nextUserID++

	user := seedUserState(userID, uuid, s.clock().UnixMilli())
	s.users[userID] = user
	s.userIDsByUUID[uuid] = userID
	return user
}

func seedUserState(userID int64, uuid string, nowMillis int64) *UserState {
	return &UserState{
		UserID:               userID,
		UUID:                 uuid,
		PlayerID:             userID,
		OsType:               2,
		PlatformType:         2,
		UserRestrictionType:  0,
		RegisterDatetime:     nowMillis,
		GameStartDatetime:    nowMillis,
		LatestVersion:        0,
		BirthYear:            defaultBirthYear,
		BirthMonth:           defaultBirthMonth,
		BackupToken:          defaultBackupToken,
		ChargeMoneyThisMonth: defaultChargeMoneyThisMonth,
		Setting: UserSettingState{
			IsNotifyPurchaseAlert: false,
			LatestVersion:         0,
		},
		Status: UserStatusState{
			Level:                 1,
			Exp:                   0,
			StaminaMilliValue:     60000,
			StaminaUpdateDatetime: nowMillis,
			LatestVersion:         0,
		},
		Gem: UserGemState{
			PaidGem: 0,
			FreeGem: 0,
		},
		Profile: UserProfileState{
			Name:                            defaultRegisteredName,
			NameUpdateDatetime:              nowMillis,
			Message:                         "",
			MessageUpdateDatetime:           nowMillis,
			FavoriteCostumeID:               0,
			FavoriteCostumeIDUpdateDatetime: nowMillis,
			LatestVersion:                   0,
		},
		Login: UserLoginState{
			TotalLoginCount:           1,
			ContinualLoginCount:       1,
			MaxContinualLoginCount:    1,
			LastLoginDatetime:         nowMillis,
			LastComebackLoginDatetime: 0,
			LatestVersion:             0,
		},
		LoginBonus: UserLoginBonusState{
			LoginBonusID:                1,
			CurrentPageNumber:           1,
			CurrentStampNumber:          0,
			LatestRewardReceiveDatetime: 0,
			LatestVersion:               0,
		},
		Tutorial: TutorialProgressState{
			TutorialType:  1,
			ProgressPhase: 0,
			ChoiceID:      0,
			LatestVersion: 0,
		},
		Battle: BattleState{
			IsActive:              false,
			StartCount:            0,
			FinishCount:           0,
			LastStartedAt:         0,
			LastFinishedAt:        0,
			LastUserPartyCount:    0,
			LastNpcPartyCount:     0,
			LastBattleBinarySize:  0,
			LastElapsedFrameCount: 0,
		},
		Gifts: GiftState{
			NotReceived: []NotReceivedGiftState{
				{
					GiftCommon: GiftCommonState{
						PossessionType:        defaultGiftPossessionType,
						PossessionID:          0,
						Count:                 defaultGiftCount,
						GrantDatetime:         nowMillis,
						DescriptionGiftTextID: 0,
						EquipmentData:         nil,
					},
					ExpirationDatetime: nowMillis + int64((7*24*time.Hour)/time.Millisecond),
					UserGiftUUID:       fmt.Sprintf("%s-%d-1", defaultGiftUUIDPrefix, userID),
				},
			},
			Received: []ReceivedGiftState{},
		},
		Gacha: GachaState{
			RewardAvailable:        false,
			TodaysCurrentDrawCount: 0,
			DailyMaxCount:          0,
			ConvertedGachaMedal: ConvertedGachaMedalState{
				ConvertedMedalPossession: []ConsumableItemState{},
				ObtainPossession:         nil,
			},
		},
		MainQuest: MainQuestState{
			CurrentQuestFlowType:     0,
			CurrentMainQuestRouteID:  starterMainQuestRouteID,
			CurrentQuestSceneID:      0,
			HeadQuestSceneID:         0,
			IsReachedLastQuestScene:  false,
			ProgressQuestSceneID:     0,
			ProgressHeadQuestSceneID: 0,
			ProgressQuestFlowType:    0,
			MainQuestSeasonID:        starterMainQuestSeasonID,
			LatestVersion:            0,
		},
		Notifications: NotificationState{
			GiftNotReceiveCount:       1,
			FriendRequestReceiveCount: 0,
			IsExistUnreadInformation:  false,
		},
		Characters: map[int32]CharacterState{
			starterCharacterID: {
				CharacterID:   starterCharacterID,
				Level:         1,
				Exp:           0,
				LatestVersion: 0,
			},
		},
		Costumes: map[string]CostumeState{
			starterCostumeUUID: {
				UserCostumeUUID:     starterCostumeUUID,
				CostumeID:           starterCostumeID,
				LimitBreakCount:     0,
				Level:               1,
				Exp:                 0,
				HeadupDisplayViewID: 0,
				AcquisitionDatetime: nowMillis,
				AwakenCount:         0,
				LatestVersion:       0,
			},
		},
		Weapons: map[string]WeaponState{
			starterWeaponUUID: {
				UserWeaponUUID:      starterWeaponUUID,
				WeaponID:            starterWeaponID,
				Level:               1,
				Exp:                 0,
				LimitBreakCount:     0,
				IsProtected:         false,
				AcquisitionDatetime: nowMillis,
				LatestVersion:       0,
			},
		},
		Companions: map[string]CompanionState{
			starterCompanionUUID: {
				UserCompanionUUID:   starterCompanionUUID,
				CompanionID:         starterCompanionID,
				HeadupDisplayViewID: 0,
				Level:               1,
				AcquisitionDatetime: nowMillis,
				LatestVersion:       0,
			},
		},
		DeckCharacters: map[string]DeckCharacterState{
			starterDeckCharacterUUID: {
				UserDeckCharacterUUID: starterDeckCharacterUUID,
				UserCostumeUUID:       starterCostumeUUID,
				MainUserWeaponUUID:    starterWeaponUUID,
				UserCompanionUUID:     starterCompanionUUID,
				Power:                 100,
				UserThoughtUUID:       "",
				LatestVersion:         0,
			},
		},
		Decks: map[DeckKey]DeckState{
			{DeckType: starterQuestDeckType, UserDeckNumber: 1}: {
				DeckType:                starterQuestDeckType,
				UserDeckNumber:          1,
				UserDeckCharacterUUID01: starterDeckCharacterUUID,
				UserDeckCharacterUUID02: "",
				UserDeckCharacterUUID03: "",
				Name:                    "Deck 1",
				Power:                   100,
				LatestVersion:           0,
			},
		},
		Quests: map[int32]UserQuestState{
			starterQuestID: {
				QuestID:             starterQuestID,
				QuestStateType:      0,
				IsBattleOnly:        false,
				LatestStartDatetime: nowMillis,
				ClearCount:          0,
				DailyClearCount:     0,
				LastClearDatetime:   0,
				ShortestClearFrames: 0,
				LatestVersion:       0,
			},
		},
		Missions: map[int32]UserMissionState{
			starterMissionID: {
				MissionID:                 starterMissionID,
				StartDatetime:             nowMillis,
				ProgressValue:             0,
				MissionProgressStatusType: missionInProgress,
				ClearDatetime:             0,
				LatestVersion:             0,
			},
		},
		Gimmick: GimmickState{
			Progress:         make(map[GimmickKey]GimmickProgressState),
			OrnamentProgress: make(map[GimmickOrnamentKey]GimmickOrnamentProgressState),
			Sequences:        make(map[GimmickSequenceKey]GimmickSequenceState),
			Unlocks:          make(map[GimmickKey]GimmickUnlockState),
		},
	}
}

func (u UserState) clone() UserState {
	out := u
	out.Characters = maps.Clone(u.Characters)
	out.Costumes = maps.Clone(u.Costumes)
	out.Weapons = maps.Clone(u.Weapons)
	out.Companions = maps.Clone(u.Companions)
	out.DeckCharacters = maps.Clone(u.DeckCharacters)
	out.Decks = maps.Clone(u.Decks)
	out.Quests = maps.Clone(u.Quests)
	out.Missions = maps.Clone(u.Missions)
	out.Gimmick = GimmickState{
		Progress:         maps.Clone(u.Gimmick.Progress),
		OrnamentProgress: maps.Clone(u.Gimmick.OrnamentProgress),
		Sequences:        maps.Clone(u.Gimmick.Sequences),
		Unlocks:          maps.Clone(u.Gimmick.Unlocks),
	}
	out.Gacha = GachaState{
		RewardAvailable:        u.Gacha.RewardAvailable,
		TodaysCurrentDrawCount: u.Gacha.TodaysCurrentDrawCount,
		DailyMaxCount:          u.Gacha.DailyMaxCount,
		ConvertedGachaMedal: ConvertedGachaMedalState{
			ConvertedMedalPossession: append([]ConsumableItemState(nil), u.Gacha.ConvertedGachaMedal.ConvertedMedalPossession...),
			ObtainPossession:         cloneConsumableItemPtr(u.Gacha.ConvertedGachaMedal.ObtainPossession),
		},
	}
	out.Gifts = GiftState{
		NotReceived: cloneNotReceivedGifts(u.Gifts.NotReceived),
		Received:    cloneReceivedGifts(u.Gifts.Received),
	}
	out.Battle = BattleState{
		IsActive:              u.Battle.IsActive,
		StartCount:            u.Battle.StartCount,
		FinishCount:           u.Battle.FinishCount,
		LastStartedAt:         u.Battle.LastStartedAt,
		LastFinishedAt:        u.Battle.LastFinishedAt,
		LastUserPartyCount:    u.Battle.LastUserPartyCount,
		LastNpcPartyCount:     u.Battle.LastNpcPartyCount,
		LastBattleBinarySize:  u.Battle.LastBattleBinarySize,
		LastElapsedFrameCount: u.Battle.LastElapsedFrameCount,
	}
	return out
}

func cloneGachaCatalogEntry(entry GachaCatalogEntry) GachaCatalogEntry {
	out := entry
	out.GachaMode = append([]byte(nil), entry.GachaMode...)
	return out
}

func cloneConsumableItemPtr(item *ConsumableItemState) *ConsumableItemState {
	if item == nil {
		return nil
	}
	out := *item
	return &out
}

func cloneNotReceivedGifts(gifts []NotReceivedGiftState) []NotReceivedGiftState {
	out := make([]NotReceivedGiftState, len(gifts))
	for i, gift := range gifts {
		out[i] = NotReceivedGiftState{
			GiftCommon: GiftCommonState{
				PossessionType:        gift.GiftCommon.PossessionType,
				PossessionID:          gift.GiftCommon.PossessionID,
				Count:                 gift.GiftCommon.Count,
				GrantDatetime:         gift.GiftCommon.GrantDatetime,
				DescriptionGiftTextID: gift.GiftCommon.DescriptionGiftTextID,
				EquipmentData:         append([]byte(nil), gift.GiftCommon.EquipmentData...),
			},
			ExpirationDatetime: gift.ExpirationDatetime,
			UserGiftUUID:       gift.UserGiftUUID,
		}
	}
	return out
}

func cloneReceivedGifts(gifts []ReceivedGiftState) []ReceivedGiftState {
	out := make([]ReceivedGiftState, len(gifts))
	for i, gift := range gifts {
		out[i] = ReceivedGiftState{
			GiftCommon: GiftCommonState{
				PossessionType:        gift.GiftCommon.PossessionType,
				PossessionID:          gift.GiftCommon.PossessionID,
				Count:                 gift.GiftCommon.Count,
				GrantDatetime:         gift.GiftCommon.GrantDatetime,
				DescriptionGiftTextID: gift.GiftCommon.DescriptionGiftTextID,
				EquipmentData:         append([]byte(nil), gift.GiftCommon.EquipmentData...),
			},
			ReceivedDatetime: gift.ReceivedDatetime,
		}
	}
	return out
}
