package stats

import (
	"skycrypt/src/db"
	"skycrypt/src/models"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func GetStats(mowojang *models.MowojangResponse, profiles *models.HypixelProfilesResponse, profile *skycrypttypes.Profile, player *skycrypttypes.Player, userProfile *skycrypttypes.Member, museum *skycrypttypes.Museum, members []*models.MemberStats) (*models.StatsOutput, error) {
	if userProfile.Profile == nil {
		userProfile.Profile = &skycrypttypes.ProfileData{}
	}

	return &models.StatsOutput{
		Username:        mowojang.Name,
		DisplayName:     db.GetDisplayName(mowojang.Name, mowojang.UUID),
		UUID:            mowojang.UUID,
		ProfileID:       profile.ProfileID,
		ProfileCuteName: profile.CuteName,
		GameMode:        profile.GameMode,
		Selected:        profile.Selected,
		Profiles:        FormatProfiles(profiles),
		Members:         members,
		Social:          player.SocialMedia.Links,
		Rank:            GetRank(player),
		Skills:          GetSkills(userProfile, profile, player),
		SkyBlockLevel:   GetSkyBlockLevel(userProfile),
		Joined:          userProfile.Profile.FirstJoin,
		Purse:           userProfile.Currencies.CoinPurse,
		Bank:            profile.Banking.Balance,
		PersonalBank:    userProfile.Profile.BankAccount,
		FairySouls:      GetFairySouls(userProfile, profile.GameMode),
		APISettings:     GetAPISettings(userProfile, profile, museum),
	}, nil
}
