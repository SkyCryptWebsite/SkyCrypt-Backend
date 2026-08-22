package notenoughupdates

import (
	"testing"
	"unsafe"
)

func TestGetItemWikiOwnsCacheKey(t *testing.T) {
	const expectedName = "CACHE_KEY_OWNERSHIP_TEST"
	CACHED_NEU_ITEM_WIKIS.Delete(expectedName)
	t.Cleanup(func() { CACHED_NEU_ITEM_WIKIS.Delete(expectedName) })

	nameBytes := []byte(expectedName)
	name := unsafe.String(&nameBytes[0], len(nameBytes))
	if _, ok := GetItemWiki(name); ok {
		t.Fatal("expected missing test item")
	}
	nameBytes[0] = 'X'

	var found bool
	CACHED_NEU_ITEM_WIKIS.Range(func(key, value any) bool {
		if key == expectedName {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatalf("cache key changed after input mutation")
	}
}
