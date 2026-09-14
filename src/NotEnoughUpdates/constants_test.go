package notenoughupdates

import "testing"

func TestGetItemWikiDoesNotRetainMissingKey(t *testing.T) {
	const expectedName = "CACHE_KEY_OWNERSHIP_TEST"
	CACHED_NEU_ITEM_WIKIS.Delete(expectedName)
	t.Cleanup(func() { CACHED_NEU_ITEM_WIKIS.Delete(expectedName) })

	if _, ok := GetItemWiki(expectedName); ok {
		t.Fatal("expected missing test item")
	}

	var retained bool
	CACHED_NEU_ITEM_WIKIS.Range(func(key, value any) bool {
		if key == expectedName {
			retained = true
			return false
		}
		return true
	})
	if retained {
		t.Fatalf("missing item key was retained in the cache")
	}
}
