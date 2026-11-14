package routes

import (
	"bytes"
	"testing"

	"skycrypt/src/models"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

func testMuseum() *skycrypttypes.Museum {
	items := map[string]skycrypttypes.MuseumItem{}
	for _, id := range []string{"HYPERION", "ASPECT_OF_THE_END", "DIVAN_DRILL", "FARM_SET", "WAR_SET", "JUJU_SHORTBOW"} {
		items[id] = skycrypttypes.MuseumItem{DonatedTime: 1, Items: skycrypttypes.EncodedItems{Data: "data-" + id}}
	}
	special := []skycrypttypes.MuseumItem{{Items: skycrypttypes.EncodedItems{Data: "special"}}}
	return &skycrypttypes.Museum{Value: 100, Items: &items, Special: &special}
}

// The key is a hash of the museum's content: equal museums share a tab whatever order their map iterates in, and
// any change to the museum or the enabled packs gets a new one.
func TestMuseumTabCacheKey(t *testing.T) {
	want := museumTabCacheKey(testMuseum(), nil)
	for range 20 {
		if got := museumTabCacheKey(testMuseum(), nil); got != want {
			t.Fatalf("equal museums got different keys: %s and %s", got, want)
		}
	}

	changes := map[string]func(*skycrypttypes.Museum){
		"value":     func(m *skycrypttypes.Museum) { m.Value++ },
		"appraisal": func(m *skycrypttypes.Museum) { m.Appraisal = true },
		"item data": func(m *skycrypttypes.Museum) {
			item := (*m.Items)["HYPERION"]
			item.Items.Data += "x"
			(*m.Items)["HYPERION"] = item
		},
		"borrowing": func(m *skycrypttypes.Museum) {
			item := (*m.Items)["HYPERION"]
			item.Borrowing = true
			(*m.Items)["HYPERION"] = item
		},
		"new item":     func(m *skycrypttypes.Museum) { (*m.Items)["TERMINATOR"] = skycrypttypes.MuseumItem{} },
		"removed item": func(m *skycrypttypes.Museum) { delete(*m.Items, "WAR_SET") },
		"special": func(m *skycrypttypes.Museum) {
			*m.Special = append(*m.Special, skycrypttypes.MuseumItem{Items: skycrypttypes.EncodedItems{Data: "more"}})
		},
		// The same strings split differently between an item and the next must not collide.
		"moved bytes": func(m *skycrypttypes.Museum) {
			item := (*m.Items)["HYPERION"]
			item.FeaturedSlot, item.Items.Data = "data-", "HYPERION"
			(*m.Items)["HYPERION"] = item
		},
	}
	for name, change := range changes {
		museum := testMuseum()
		change(museum)
		if museumTabCacheKey(museum, nil) == want {
			t.Errorf("changing the %s kept the key", name)
		}
	}
	if museumTabCacheKey(testMuseum(), []string{"some-pack"}) == want {
		t.Error("changing the enabled packs kept the key")
	}
}

// buildInventory joins separately encoded tabs; that must give the bytes encoding the whole list gives.
func TestJoinedTabsMatchEncodedList(t *testing.T) {
	list := []models.Inventory{
		{Name: "Inventory", Texture: "a", Items: []models.StrippedItem{{DisplayName: "<Sword> & \"shield\""}}},
		{Name: "Museum", Texture: "b", Items: []models.StrippedItem{}},
		{Name: "Empty"},
	}
	want, err := marshalBody(list)
	if err != nil {
		t.Fatal(err)
	}
	tabs := make([][]byte, len(list))
	for i, tab := range list {
		if tabs[i], err = marshalBody(tab); err != nil {
			t.Fatal(err)
		}
	}
	got := joinJSONArray(tabs)
	if !bytes.Equal(got, want) {
		t.Fatalf("joined tabs:\n%s\nencoded list:\n%s", got, want)
	}
}
