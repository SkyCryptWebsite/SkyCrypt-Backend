package notenoughupdates

import (
	"fmt"
	"os"
	"skycrypt/src/models"
	"strings"
	"sync"
	"sync/atomic"

	jsoniter "github.com/json-iterator/go"
)

// neuConstants is parsed into a fresh value and swapped in whole, so a reload never races with readers.
var neuConstants atomic.Pointer[models.NEUConstant]

// Constants returns the parsed NEU constants. Treat the result as read-only.
func Constants() *models.NEUConstant {
	if current := neuConstants.Load(); current != nil {
		return current
	}
	return &models.NEUConstant{}
}

// knownItemIDs lists the items the repository has, so lookups for anything else (vanilla IDs, pet rarity fallbacks)
// return without a failing disk read. Nil until the first parse, when every lookup still goes to disk.
var knownItemIDs atomic.Pointer[map[string]struct{}]

func isKnownItem(name string) bool {
	known := knownItemIDs.Load()
	if known == nil {
		return true
	}
	_, ok := (*known)[name]
	return ok
}

// indexItems records which item files the repository has.
func indexItems() error {
	entries, err := os.ReadDir("NotEnoughUpdates-REPO/items")
	if err != nil {
		return fmt.Errorf("failed to list NEU items: %w", err)
	}
	known := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if name, ok := strings.CutSuffix(entry.Name(), ".json"); ok && !entry.IsDir() {
			known[name] = struct{}{}
		}
	}
	knownItemIDs.Store(&known)
	return nil
}

var CACHED_NEU_ITEMS sync.Map
var CACHED_NEU_ITEM_WIKIS sync.Map

type rawNEUItemWiki struct {
	Wiki []string `json:"info,omitempty"`
}

func GetItem(name string) (models.NEUItem, error) {
	if item, ok := CACHED_NEU_ITEMS.Load(name); ok {
		return item.(models.NEUItem), nil
	}
	if !isKnownItem(name) {
		return models.NEUItem{}, fmt.Errorf("NEU has no item %s", name)
	}

	itemsPath := "NotEnoughUpdates-REPO/items"
	/*
		if _, err := os.Stat(itemsPath); os.IsNotExist(err) {
			return models.NEUItem{}, fmt.Errorf("items directory does not exist: %w", err)
		}
	*/

	itemPath := fmt.Sprintf("%s/%s.json", itemsPath, name)
	data, err := os.ReadFile(itemPath)
	if err != nil {
		return models.NEUItem{}, fmt.Errorf("failed to read item file %s: %w", itemPath, err)
	}

	var item models.RawNEUItem
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(data, &item)
	if err != nil {
		return models.NEUItem{}, fmt.Errorf("failed to unmarshal JSON from %s: %w", itemPath, err)
	}

	parsedNBT, _ := ParseNBTToItem(item.NBT)
	NEUItem := models.NEUItem{
		MinecraftId: item.MinecraftId,
		Name:        item.Name,
		Damage:      item.Damage,
		Lore:        item.Lore,
		NEUId:       item.NEUId,
		NBT:         parsedNBT,
		Wiki:        item.Wiki,
	}

	// Clone only what is kept: fasthttp may reuse the caller's buffer.
	CACHED_NEU_ITEMS.Store(strings.Clone(name), NEUItem)

	return NEUItem, nil
}

func GetItemWiki(name string) ([]string, bool) {
	if item, ok := CACHED_NEU_ITEMS.Load(name); ok {
		return cloneWiki(item.(models.NEUItem).Wiki), true
	}
	if wiki, ok := CACHED_NEU_ITEM_WIKIS.Load(name); ok {
		return cloneWiki(wiki.([]string)), true
	}
	if !isKnownItem(name) {
		return nil, false
	}

	itemPath := fmt.Sprintf("NotEnoughUpdates-REPO/items/%s.json", name)
	data, err := os.ReadFile(itemPath)
	if err != nil {
		return nil, false
	}

	var item rawNEUItemWiki
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, false
	}

	wiki := cloneWiki(item.Wiki)
	CACHED_NEU_ITEM_WIKIS.Store(strings.Clone(name), wiki)
	return cloneWiki(wiki), true
}

func cloneWiki(wiki []string) []string {
	if len(wiki) == 0 {
		return nil
	}
	return append([]string(nil), wiki...)
}
