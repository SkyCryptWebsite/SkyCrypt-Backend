package notenoughupdates

import (
	"fmt"
	"os"
	"skycrypt/src/models"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var NEUConstants = models.NEUConstant{}
var CACHED_NEU_ITEMS sync.Map
var CACHED_NEU_ITEM_WIKIS sync.Map

type rawNEUItemWiki struct {
	Wiki []string `json:"info,omitempty"`
}

func GetItem(name string) (models.NEUItem, error) {
	if item, ok := CACHED_NEU_ITEMS.Load(name); ok {
		return item.(models.NEUItem), nil
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

	CACHED_NEU_ITEMS.Store(name, NEUItem)

	return NEUItem, nil
}

func GetItemWiki(name string) ([]string, bool) {
	if item, ok := CACHED_NEU_ITEMS.Load(name); ok {
		return cloneWiki(item.(models.NEUItem).Wiki), true
	}
	if wiki, ok := CACHED_NEU_ITEM_WIKIS.Load(name); ok {
		return cloneWiki(wiki.([]string)), true
	}

	itemPath := fmt.Sprintf("NotEnoughUpdates-REPO/items/%s.json", name)
	data, err := os.ReadFile(itemPath)
	if err != nil {
		CACHED_NEU_ITEM_WIKIS.Store(name, []string{})
		return nil, false
	}

	var item rawNEUItemWiki
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(data, &item); err != nil {
		CACHED_NEU_ITEM_WIKIS.Store(name, []string{})
		return nil, false
	}

	wiki := cloneWiki(item.Wiki)
	CACHED_NEU_ITEM_WIKIS.Store(name, wiki)
	return cloneWiki(wiki), true
}

func cloneWiki(wiki []string) []string {
	if len(wiki) == 0 {
		return nil
	}
	return append([]string(nil), wiki...)
}
