package api

import (
	"skycrypt/src/models"
	"strings"
	"testing"
)

func TestProcessCollectionsUsesLegacyItemDamage(t *testing.T) {
	tests := []struct {
		collectionID string
		itemName     string
		wantSuffix   string
	}{
		{collectionID: "INK_SACK:4", itemName: "Lapis Lazuli", wantSuffix: "/api/item/lapis_lazuli"},
		{collectionID: "RAW_FISH:2", itemName: "Tropical Fish", wantSuffix: "/api/item/tropical_fish"},
		{collectionID: "LOG_2:1", itemName: "Dark Oak Log", wantSuffix: "/api/item/dark_oak_log"},
	}

	for _, test := range tests {
		t.Run(test.collectionID, func(t *testing.T) {
			collections := map[string]models.HypixelCollection{
				"mining": {
					Items: map[string]models.HypixelCollectionItem{
						test.collectionID: {Name: test.itemName},
					},
				},
			}
			got := processCollections(collections)["mining"].Collections[0].Texture
			if !strings.HasSuffix(got, test.wantSuffix) {
				t.Fatalf("collection texture = %q, want suffix %q", got, test.wantSuffix)
			}
		})
	}
}
