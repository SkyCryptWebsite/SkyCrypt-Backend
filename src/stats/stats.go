package stats

import (
	"skycrypt/src/models"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

func GetStats(mowojang *models.MowojangResponse, profiles *models.HypixelProfilesResponse, profile *skycrypttypes.Profile, player *skycrypttypes.Player, userProfile *skycrypttypes.Member, museum *skycrypttypes.Museum, members []*models.MemberStats) (*models.StatsOutput, error) {
	if userProfile.Profile == nil {
		userProfile.Profile = &skycrypttypes.ProfileData{}
	}

	// Banking is missing when the profile's bank API is off. GetAPISettings fills it in below, but Go doesn't say
	// whether that call runs before the other fields of the literal are read, so don't depend on it here.
	var bank *float64
	if profile.Banking != nil {
		bank = profile.Banking.Balance
	}

	return &models.StatsOutput{
		Username:        mowojang.Name,
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
		Bank:            bank,
		PersonalBank:    userProfile.Profile.BankAccount,
		FairySouls:      GetFairySouls(userProfile, profile.GameMode),
		APISettings:     GetAPISettings(userProfile, profile, museum),
	}, nil
}
