package notenoughupdates

import (
	"fmt"
	"os"
	"skycrypt/src/models"

	jsoniter "github.com/json-iterator/go"
)

var NEUConstants = models.NEUConstant{}
var CACHED_NEU_ITEMS = make(map[string]models.NEUItem)

func GetItem(name string) (models.NEUItem, error) {
	if item, exists := CACHED_NEU_ITEMS[name]; exists {
		return item, nil
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

	CACHED_NEU_ITEMS[name] = NEUItem

	return NEUItem, nil
}
