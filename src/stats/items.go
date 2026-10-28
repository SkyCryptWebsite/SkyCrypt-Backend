package stats

import (
	"fmt"
	"skycrypt/src/utility"
	"strings"
	"sync"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
)

// recoverInventoryPanic turns a panic in an inventory goroutine into an error. The recover middleware only covers
// the handler goroutine, and an unrecovered panic here would take down every prefork process.
func recoverInventoryPanic(inventoryId string, report func(error)) {
	if r := recover(); r != nil {
		report(fmt.Errorf("panic decoding inventory %s: %v", inventoryId, r))
	}
}

func GetRawInventory(userProfile *skycrypttypes.Member, inventoryId string) string {
	switch inventoryId {
	case "inventory":
		return userProfile.Inventory.Inventory.Data
	case "enderchest":
		return userProfile.Inventory.Enderchest.Data
	case "armor":
		return userProfile.Inventory.Armor.Data
	case "equipment":
		return userProfile.Inventory.Equipment.Data
	case "personal_vault":
		return userProfile.Inventory.PersonalVault.Data
	case "wardrobe":
		// Wardrobe items live in the loadout sets, not in one encoded inventory.
		return ""
	case "sacks":
		return userProfile.Inventory.BagContents.SacksBag.Data

	case "rift_inventory":
		return userProfile.Rift.Inventory.Inventory.Data
	case "rift_enderchest":
		return userProfile.Rift.Inventory.Enderchest.Data
	case "rift_armor":
		return userProfile.Rift.Inventory.Armor.Data
	case "rift_equipment":
		return userProfile.Rift.Inventory.Equipment.Data
	case "potion_bag":
		return userProfile.Inventory.BagContents.PotionBag.Data
	case "talisman_bag":
		return userProfile.Inventory.BagContents.TalismanBag.Data
	case "fishing_bag":
		return userProfile.Inventory.BagContents.FishingBag.Data
	case "quiver":
		return userProfile.Inventory.BagContents.Quiver.Data
	}

	return ""
}

func GetInventory(useProfile *skycrypttypes.Member, inventoryId string) []*skycrypttypes.Item {
	if useProfile.Inventory == nil {
		return []*skycrypttypes.Item{}
	}

	if inventoryId == "backpack" {
		encodedInventories := map[string]*string{}
		for backpackId, backpackData := range useProfile.Inventory.Backpack {
			encodedInventories[fmt.Sprintf("backpack_%s", backpackId)] = &backpackData.Data
		}

		for backpackIconId, backpackIconData := range useProfile.Inventory.BackpackIcons {
			encodedInventories[fmt.Sprintf("backpack_icon_%s", backpackIconId)] = &backpackIconData.Data
		}

		type result struct {
			inventoryId string
			items       []*skycrypttypes.Item
			err         error
		}

		resultChan := make(chan result, len(encodedInventories))
		var wg sync.WaitGroup

		for inventoryId, inventoryData := range encodedInventories {
			wg.Add(1)
			go func(id string, data *string) {
				defer wg.Done()
				defer recoverInventoryPanic(id, func(err error) { resultChan <- result{id, nil, err} })

				decodedInventory, err := skyhelpernetworthgo.CalculateFromSpecifiedInventories(
					skyhelpernetworthgo.SpecifiedInventory{
						"inventory": skycrypttypes.EncodedItems{Data: *data},
					},
					skyhelpernetworthgo.NetworthOptions{
						IncludeItemData:  true,
						KeepInvalidItems: true,
					}.ToInternal(),
				)
				items := []*skycrypttypes.Item{}
				if decodedInventory.Types["inventory"] != nil {
					for _, item := range decodedInventory.Types["inventory"].Items {
						itemData := item.ItemData
						if itemData != nil {
							itemData.Price = item.Price
						}

						items = append(items, itemData)
					}
				}

				if err != nil {
					resultChan <- result{id, nil, err}
					return
				}

				resultChan <- result{id, items, nil}
			}(inventoryId, inventoryData)
		}

		go func() {
			wg.Wait()
			close(resultChan)
		}()

		decodedInventory := make(map[string][]*skycrypttypes.Item, len(encodedInventories))
		for res := range resultChan {
			if res.err != nil {
				fmt.Printf("Error decoding inventory %s: %v\n", res.inventoryId, res.err)
				continue
			}

			decodedInventory[res.inventoryId] = res.items
		}

		return assembleBackpacks(decodedInventory)
	}

	rawInventory := GetRawInventory(useProfile, inventoryId)
	if rawInventory == "" {
		return []*skycrypttypes.Item{}
	}

	decodedInventory, err := skyhelpernetworthgo.CalculateFromSpecifiedInventories(
		skyhelpernetworthgo.SpecifiedInventory{
			"inventory": skycrypttypes.EncodedItems{Data: rawInventory},
		},
		skyhelpernetworthgo.NetworthOptions{
			IncludeItemData:  true,
			KeepInvalidItems: true,
		}.ToInternal(),
	)
	items := []*skycrypttypes.Item{}
	if decodedInventory.Types["inventory"] != nil {
		for _, item := range decodedInventory.Types["inventory"].Items {
			itemData := item.ItemData
			if itemData != nil {
				itemData.Price = item.Price
			}

			items = append(items, itemData)
		}
	}

	if err != nil {
		fmt.Printf("Error decoding inventory %s: %v\n", inventoryId, err)
		return nil
	}

	return items
}

// assembleBackpacks puts each decoded backpack inside its icon item, indexed by backpack slot. decoded maps
// "backpack_<n>" and "backpack_icon_<n>" to their items.
func assembleBackpacks(decoded map[string][]*skycrypttypes.Item) []*skycrypttypes.Item {
	maxIndex := -1
	for inventoryId := range decoded {
		if strings.HasPrefix(inventoryId, "backpack_") && !strings.Contains(inventoryId, "icon") {
			backpackIndex := strings.Split(inventoryId, "_")[1]
			index := 0
			if _, err := fmt.Sscanf(backpackIndex, "%d", &index); err != nil {
				continue
			}
			if index > maxIndex {
				maxIndex = index
			}
		}
	}

	output := make([]*skycrypttypes.Item, maxIndex+1)
	for inventoryId, items := range decoded {
		if strings.HasPrefix(inventoryId, "backpack_") && !strings.Contains(inventoryId, "icon") {
			backpackIndex := strings.Split(inventoryId, "_")[1]
			index := 0
			if _, err := fmt.Sscanf(backpackIndex, "%d", &index); err != nil {
				continue
			}

			backpackIcon, iconExists := decoded[fmt.Sprintf("backpack_icon_%s", backpackIndex)]
			if iconExists && len(backpackIcon) > 0 {
				backpackIcon[0].ContainsItems = items
				output[index] = backpackIcon[0]
			} else {
				fmt.Printf("No icon found for backpack %s\n", backpackIndex)
			}
		}
	}

	return output
}

// GetInventories is GetInventory for several inventories at once. It decodes and prices them in one batched call,
// which runs them in parallel, instead of one call per inventory.
func GetInventories(useProfile *skycrypttypes.Member, inventoryIds []string) map[string][]*skycrypttypes.Item {
	output := make(map[string][]*skycrypttypes.Item, len(inventoryIds))
	if useProfile.Inventory == nil {
		for _, inventoryId := range inventoryIds {
			output[inventoryId] = []*skycrypttypes.Item{}
		}
		return output
	}

	specified := skyhelpernetworthgo.SpecifiedInventory{}
	for _, inventoryId := range inventoryIds {
		if inventoryId == "backpack" {
			for backpackId, backpackData := range useProfile.Inventory.Backpack {
				specified["backpack_"+backpackId] = skycrypttypes.EncodedItems{Data: backpackData.Data}
			}
			for backpackIconId, backpackIconData := range useProfile.Inventory.BackpackIcons {
				specified["backpack_icon_"+backpackIconId] = skycrypttypes.EncodedItems{Data: backpackIconData.Data}
			}
			continue
		}
		if raw := GetRawInventory(useProfile, inventoryId); raw != "" {
			specified[inventoryId] = skycrypttypes.EncodedItems{Data: raw}
		}
	}

	decoded, err := skyhelpernetworthgo.CalculateFromSpecifiedInventories(specified, skyhelpernetworthgo.NetworthOptions{
		IncludeItemData:  true,
		KeepInvalidItems: true,
	}.ToInternal())
	if err != nil {
		// One unreadable inventory fails the whole batch; decode each on its own so only that one comes back empty.
		fmt.Printf("Error decoding inventories together, falling back to one by one: %v\n", err)
		for _, inventoryId := range inventoryIds {
			output[inventoryId] = GetInventory(useProfile, inventoryId)
		}
		return output
	}

	itemsOf := func(key string) []*skycrypttypes.Item {
		items := []*skycrypttypes.Item{}
		inventory := decoded.Types[key]
		if inventory == nil {
			return items
		}
		for _, item := range inventory.Items {
			itemData := item.ItemData
			if itemData != nil {
				itemData.Price = item.Price
			}
			items = append(items, itemData)
		}
		return items
	}

	for _, inventoryId := range inventoryIds {
		if inventoryId != "backpack" {
			output[inventoryId] = itemsOf(inventoryId)
			continue
		}
		backpacks := map[string][]*skycrypttypes.Item{}
		for key := range specified {
			if strings.HasPrefix(key, "backpack_") {
				backpacks[key] = itemsOf(key)
			}
		}
		output[inventoryId] = assembleBackpacks(backpacks)
	}
	return output
}

func GetItems(useProfile *skycrypttypes.Member, profileId string, memberUUID string) (map[string][]*skycrypttypes.Item, error) {
	if useProfile.Inventory == nil {
		useProfile.Inventory = &skycrypttypes.Inventory{}
	}

	encodedInventories := map[string]*string{
		"inventory":      &useProfile.Inventory.Inventory.Data,
		"enderchest":     &useProfile.Inventory.Enderchest.Data,
		"armor":          &useProfile.Inventory.Armor.Data,
		"equipment":      &useProfile.Inventory.Equipment.Data,
		"personal_vault": &useProfile.Inventory.PersonalVault.Data,

		// rift
		"rift_inventory":  &useProfile.Rift.Inventory.Inventory.Data,
		"rift_enderchest": &useProfile.Rift.Inventory.Enderchest.Data,
		"rift_armor":      &useProfile.Rift.Inventory.Armor.Data,
		"rift_equipment":  &useProfile.Rift.Inventory.Equipment.Data,

		// bags
		"potion_bag":   &useProfile.Inventory.BagContents.PotionBag.Data,
		"talisman_bag": &useProfile.Inventory.BagContents.TalismanBag.Data,
		"fishing_bag":  &useProfile.Inventory.BagContents.FishingBag.Data,
		"sacks_bag":    &useProfile.Inventory.BagContents.SacksBag.Data,
		"quiver":       &useProfile.Inventory.BagContents.Quiver.Data,
	}

	for backpackId, backpackData := range useProfile.Inventory.Backpack {
		encodedInventories[fmt.Sprintf("backpack_%s", backpackId)] = &backpackData.Data
	}

	for backpackIconId, backpackIconData := range useProfile.Inventory.BackpackIcons {
		encodedInventories[fmt.Sprintf("backpack_icon_%s", backpackIconId)] = &backpackIconData.Data
	}

	for index, layout := range useProfile.Loadout.Armor.Sets {
		inventoryId := fmt.Sprintf("wardrobe_%d", index)

		encodedInventories[fmt.Sprintf("%s_helmet", inventoryId)] = &layout.Helmet.Data
		encodedInventories[fmt.Sprintf("%s_chestplate", inventoryId)] = &layout.Chestplate.Data
		encodedInventories[fmt.Sprintf("%s_leggings", inventoryId)] = &layout.Leggings.Data
		encodedInventories[fmt.Sprintf("%s_boots", inventoryId)] = &layout.Boots.Data
	}

	type result struct {
		inventoryId string
		items       []*skycrypttypes.Item
		err         error
	}

	resultChan := make(chan result, len(encodedInventories))
	var wg sync.WaitGroup

	for inventoryId, inventoryData := range encodedInventories {
		wg.Add(1)
		go func(id string, data *string) {
			defer wg.Done()
			defer recoverInventoryPanic(id, func(err error) { resultChan <- result{id, nil, err} })

			decodedInventory, err := utility.DecodeInventory(data)
			if err != nil {
				resultChan <- result{id, nil, err}
				return
			}

			resultChan <- result{id, decodedInventory.Items, nil}
		}(inventoryId, inventoryData)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	decodedInventory := make(map[string][]*skycrypttypes.Item, len(encodedInventories))
	for res := range resultChan {
		if res.err != nil {
			fmt.Printf("Error decoding inventory %s: %v\n", res.inventoryId, res.err)
			continue
		}

		decodedInventory[res.inventoryId] = res.items
	}

	output := make(map[string][]*skycrypttypes.Item)
	for inventoryId, items := range decodedInventory {
		if !strings.Contains(inventoryId, "backpack") {
			output[inventoryId] = items
		}

		if strings.HasPrefix(inventoryId, "backpack_") && !strings.Contains(inventoryId, "icon") {
			if output["backpack"] == nil {
				output["backpack"] = []*skycrypttypes.Item{}
			}

			backpackIndex := strings.Split(inventoryId, "_")[1]
			backpackIcon, iconExists := decodedInventory[fmt.Sprintf("backpack_icon_%s", backpackIndex)]
			if iconExists && len(backpackIcon) > 0 {
				backpackIcon[0].ContainsItems = items

				output["backpack"] = append(output["backpack"], backpackIcon[0])
			} else {
				fmt.Printf("No icon found for backpack %s\n", backpackIndex)
			}
		}
	}

	return output, nil
}
