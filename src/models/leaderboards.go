package models

import "time"

type LeaderboardInfo struct {
	ID        string `json:"id" example:"skill:farming"`
	Name      string `json:"name" example:"Farming"`
	Group     string `json:"group,omitempty" example:"Farming" doc:"Splits big categories, such as collection categories or dungeon modes."`
	Unit      string `json:"unit" enum:"level,xp,amount,coins,time" example:"level" doc:"level: value is a level and raw the XP. time: milliseconds. Otherwise value is the raw amount."`
	Ascending bool   `json:"ascending,omitempty" doc:"Lower ranks higher, as for dungeon run times."`
	Profile   bool   `json:"profile,omitempty" doc:"Ranks whole profiles (coop collections, garden) instead of a player on a profile."`
}

type LeaderboardCategory struct {
	ID           string            `json:"id" example:"skills"`
	Name         string            `json:"name" example:"Skills"`
	Leaderboards []LeaderboardInfo `json:"leaderboards"`
}

type LeaderboardList struct {
	Size       int64                 `json:"size" example:"1000" doc:"How many entries each leaderboard keeps."`
	Modes      []string              `json:"modes" example:"all,normal,ironman,stranded,bingo"`
	Categories []LeaderboardCategory `json:"categories"`
}

type LeaderboardPlayer struct {
	UUID     string `json:"uuid" example:"debb9e1f6dcb4f5b83c291fd5388bb56"`
	Username string `json:"username" example:"tiltedhoney"`
}

type LeaderboardEntry struct {
	Rank int64 `json:"rank" example:"1" doc:"Position on the board that was asked for, starting at 1."`
	// For a player on a profile; for profile boards these are empty and Members lists the profile's players.
	UUID      string              `json:"uuid,omitempty" example:"debb9e1f6dcb4f5b83c291fd5388bb56"`
	Username  string              `json:"username,omitempty" example:"tiltedhoney"`
	Members   []LeaderboardPlayer `json:"members,omitempty" doc:"Players on the profile, for profile boards."`
	ProfileID string              `json:"profile_id" example:"1ed76b10-45f0-44b9-8f88-d5f748bbe15a"`
	CuteName  string              `json:"cute_name" example:"Kiwi"`
	GameMode  string              `json:"game_mode" example:"normal" doc:"normal, ironman, stranded, or bingo."`
	Value     float64             `json:"value" example:"60" doc:"The shown value: a level for level units, otherwise the raw amount."`
	Raw       int64               `json:"raw" example:"111672425"`
	UpdatedAt time.Time           `json:"updated_at" doc:"When the value was last visible in the Hypixel API."`
	LastSeen  time.Time           `json:"last_seen" doc:"When the profile was last fetched."`
	Outdated  bool                `json:"outdated" doc:"The value was hidden from the API on the last fetch, so it is the last one known."`
}

type LeaderboardPage struct {
	Leaderboard LeaderboardInfo    `json:"leaderboard"`
	Mode        string             `json:"mode" example:"all"`
	Page        int64              `json:"page" example:"1"`
	Limit       int64              `json:"limit" example:"50"`
	Total       int64              `json:"total" example:"1000" doc:"Entries on this board, at most size."`
	Ready       bool               `json:"ready" doc:"False while the boards are being rebuilt; they may be incomplete."`
	Entries     []LeaderboardEntry `json:"entries"`
}

type LeaderboardRank struct {
	Leaderboard LeaderboardInfo `json:"leaderboard"`
	Category    string          `json:"category" example:"skills"`
	Rank        int64           `json:"rank" example:"3" doc:"Position among all game modes."`
	ModeRank    int64           `json:"mode_rank" example:"3" doc:"Position among profiles of the same game mode."`
	Value       float64         `json:"value" example:"60"`
	Raw         int64           `json:"raw" example:"111672425"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Outdated    bool            `json:"outdated"`
}

type LeaderboardProfileRanks struct {
	ProfileID string            `json:"profile_id" example:"1ed76b10-45f0-44b9-8f88-d5f748bbe15a"`
	CuteName  string            `json:"cute_name" example:"Kiwi"`
	GameMode  string            `json:"game_mode" example:"normal"`
	LastSeen  time.Time         `json:"last_seen" doc:"When the profile was last fetched."`
	Ranks     []LeaderboardRank `json:"ranks" doc:"Every board the profile is on, best rank first."`
}

type LeaderboardPlayerRanks struct {
	UUID     string                    `json:"uuid" example:"debb9e1f6dcb4f5b83c291fd5388bb56"`
	Size     int64                     `json:"size" example:"1000" doc:"How many entries each leaderboard keeps."`
	Profiles []LeaderboardProfileRanks `json:"profiles"`
}

type LeaderboardHiddenPlayer struct {
	UUID     string    `json:"uuid"`
	Username string    `json:"username"`
	Reason   string    `json:"reason"`
	HiddenAt time.Time `json:"hidden_at"`
}

type LeaderboardHiddenList struct {
	Hidden []LeaderboardHiddenPlayer `json:"hidden"`
}
