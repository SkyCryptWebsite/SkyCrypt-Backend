package stats

import (
	"context"
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"sort"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func GetProfile(profiles *models.HypixelProfilesResponse, profileId ...string) (*skycrypttypes.Profile, error) {
	if len(profiles.Profiles) == 0 {
		return nil, fmt.Errorf("no profiles found")
	}

	// If profileId is provided, search for it
	if len(profileId) > 0 && profileId[0] != "" {
		targetProfileId := profileId[0]
		for _, profile := range profiles.Profiles {
			if profile.ProfileID == targetProfileId || profile.CuteName == targetProfileId {
				return &profile, nil
			}
		}
		return nil, fmt.Errorf("profile with ID %s not found", targetProfileId)
	}

	// If no profileId provided, return the selected profile or the first profile
	for _, profile := range profiles.Profiles {
		if profile.Selected {
			return &profile, nil
		}
	}

	return &profiles.Profiles[0], nil
}

func FormatProfiles(profiles *models.HypixelProfilesResponse) []*models.ProfilesStats {
	profileStats := make([]*models.ProfilesStats, 0, len(profiles.Profiles))

	for _, profile := range profiles.Profiles {
		gameMode := profile.GameMode
		if gameMode == "" {
			gameMode = "normal"
		}

		profileStats = append(profileStats, &models.ProfilesStats{
			ProfileId: profile.ProfileID,
			CuteName:  profile.CuteName,
			GameMode:  gameMode,
			Selected:  profile.Selected,
		})
	}

	return profileStats
}

func FormatMembers(profile *skycrypttypes.Profile) ([]*models.MemberStats, error) {
	return FormatMembersContext(context.Background(), profile)
}

func FormatMembersContext(ctx context.Context, profile *skycrypttypes.Profile) ([]*models.MemberStats, error) {
	memberUUIDs := make([]string, 0, len(profile.Members))
	for memberUUID := range profile.Members {
		memberUUIDs = append(memberUUIDs, memberUUID)
	}
	resolvedMembers := api.ResolvePlayersContext(ctx, memberUUIDs)
	members := make([]*models.MemberStats, 0, len(profile.Members))
	for memberUUID, memberData := range profile.Members {
		mowojang := resolvedMembers[memberUUID]
		if mowojang == nil {
			continue
		}
		members = append(members, &models.MemberStats{
			UUID:      mowojang.UUID,
			CuteName:  profile.CuteName,
			ProfileId: profile.ProfileID,
			Name:      mowojang.Name,
			Removed:   isMemberRemoved(&memberData),
		})
	}

	sort.SliceStable(members, func(i, j int) bool {
		if members[i].Removed && !members[j].Removed {
			return false
		}

		if !members[i].Removed && members[j].Removed {
			return true
		}

		return members[i].Name > members[j].Name
	})

	return members, nil
}

func isMemberRemoved(memberData *skycrypttypes.Member) bool {
	if memberData.CoopInvitation != nil && !memberData.CoopInvitation.Confirmed {
		return true
	}

	if memberData.Profile.DeletionNotice != nil && memberData.Profile.DeletionNotice.Timestamp != 0 {
		return true
	}
	return false
}

func GetFairySouls(userProfile *skycrypttypes.Member, gamemode string) *models.FairySouls {
	if gamemode == "" {
		gamemode = "normal"
	}

	total := constants.FAIRY_SOULS[gamemode]
	if total == 0 {
		total = constants.FAIRY_SOULS["normal"]
	}

	if userProfile.FairySouls == nil {
		return &models.FairySouls{
			Found: 0,
			Total: total,
		}
	}

	return &models.FairySouls{
		Found: userProfile.FairySouls.TotalCollected,
		Total: total,
	}

}
