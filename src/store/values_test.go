package store

import (
	"testing"
	"time"
)

func TestMergeKeepsNewerValuesFromALaterResponse(t *testing.T) {
	older, newer := time.Unix(1000, 0), time.Unix(2000, 0)
	stored := valueSet{
		"skill:farming":    {value: 500, updatedAt: newer},
		"kills:zombie":     {value: 9, updatedAt: newer},
		"collection:WHEAT": {value: 70, updatedAt: older},
	}

	// A response fetched before the stored one arrives late.
	merged := mergeValues(stored, map[string]int64{"skill:farming": 400, "collection:WHEAT": 80}, nil, older.Add(time.Second), nil)
	if merged["skill:farming"].value != 500 {
		t.Errorf("an older response overwrote farming: %+v", merged["skill:farming"])
	}
	if merged["collection:WHEAT"].value != 80 {
		t.Errorf("a newer value for wheat was dropped: %+v", merged["collection:WHEAT"])
	}
	if _, ok := merged["kills:zombie"]; !ok {
		t.Error("an older response that lacks a key dropped the newer value")
	}
}

func TestMergeKeepsOtherOwnersKeys(t *testing.T) {
	at := time.Unix(1000, 0)
	stored := valueSet{"garden:level": {value: 5, updatedAt: at}, "coop_collection:WHEAT": {value: 1, updatedAt: at}}
	merged := mergeValues(stored, map[string]int64{"coop_collection:WHEAT": 2}, nil, at.Add(time.Hour), isCoopKey)
	if merged["garden:level"].value != 5 || merged["coop_collection:WHEAT"].value != 2 {
		t.Fatalf("merge = %+v", merged)
	}
}
