package models

import (
	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

type ProfilesStats struct {
	ProfileId string `json:"profile_id"`
	CuteName  string `json:"cute_name"`
	GameMode  string `json:"game_mode"`
	Selected  bool   `json:"selected"`
}

type MemberStats struct {
	UUID      string `json:"uuid"`
	CuteName  string `json:"cute_name"`
	ProfileId string `json:"profile_id"`
	Name      string `json:"username"`
	Removed   bool   `json:"removed"`
}

type StatsOutput struct {
	Username        string                         `json:"username"`
	UUID            string                         `json:"uuid"`
	ProfileID       string                         `json:"profile_id"`
	ProfileCuteName string                         `json:"profile_cute_name"`
	GameMode        string                         `json:"game_mode,omitempty"`
	Selected        bool                           `json:"selected"`
	Profiles        []*ProfilesStats               `json:"profiles"`
	Members         []*MemberStats                 `json:"members"`
	Social          skycrypttypes.SocialMediaLinks `json:"social"`
	Rank            *RankOutput                    `json:"rank"`
	Skills          *Skills                        `json:"skills"`
	SkyBlockLevel   Skill                          `json:"skyblock_level"`
	Joined          int64                          `json:"joined"`
	Purse           float64                        `json:"purse"`
	Bank            *float64                       `json:"bank"`
	PersonalBank    float64                        `json:"personalBank"`
	FairySouls      *FairySouls                    `json:"fairySouls"`
	APISettings     map[string]bool                `json:"apiSettings"`
}
