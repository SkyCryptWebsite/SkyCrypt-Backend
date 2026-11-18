package constants

import (
	"math"
	"testing"
)

func TestGetBonusStatsCountsEveryLevel(t *testing.T) {
	cases := []struct {
		slayer string
		level  int
		stat   string
		want   float64
	}{
		// 2+2+3+3+4+4+5+5+6
		{"slayer_zombie", 9, "health", 34},
		{"slayer_zombie", 8, "health_regen", 50},
		// 1+1+1+1+2+2+2+3+3: levels past the number of steps used to be dropped.
		{"slayer_spider", 9, "critical_damage", 16},
		{"slayer_wolf", 9, "health", 12},
		{"slayer_wolf", 0, "health", 0},
	}

	for _, tc := range cases {
		got := GetBonusStats(tc.level, STATS_BONUS[tc.slayer])[tc.stat]
		if got != tc.want {
			t.Errorf("GetBonusStats(%d, %s)[%s] = %v, want %v", tc.level, tc.slayer, tc.stat, got, tc.want)
		}
	}
}

func TestCombatMagicFind(t *testing.T) {
	for level, want := range map[int]float64{0: 0, 10: 1, 20: 3, 30: 6, 40: 10, 50: 15, 60: 20} {
		got := GetBonusStat(level, "skill_combat", 60)["magic_find"]
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("combat level %d magic find = %v, want %v", level, got, want)
		}
	}
}
