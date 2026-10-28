package stats

import skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"

func GetAPISettings(userProfile *skycrypttypes.Member, profile *skycrypttypes.Profile, museum *skycrypttypes.Museum) map[string]bool {
	if profile.Banking == nil {
		profile.Banking = &skycrypttypes.Banking{}
	}

	return map[string]bool{
		"skills":         userProfile.PlayerData.Experience != nil,
		"inventory":      userProfile.Inventory != nil,
		"personal_vault": userProfile.Inventory != nil && userProfile.Inventory.PersonalVault.Data != "",
		"collections":    userProfile.Collections != nil,
		"banking":        profile.Banking.Balance != nil,
		"museum":         museum != nil,
	}
}
