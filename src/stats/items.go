package stats

import (
	"fmt"
	redis "skycrypt/src/db"
	"skycrypt/src/utility"
	"strings"
	"sync"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	jsoniter "github.com/json-iterator/go"
)

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
		return userProfile.Inventory.Wardrobe.Data
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

		maxIndex := -1
		for inventoryId := range decodedInventory {
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
		for inventoryId, items := range decodedInventory {
			if strings.HasPrefix(inventoryId, "backpack_") && !strings.Contains(inventoryId, "icon") {
				backpackIndex := strings.Split(inventoryId, "_")[1]
				index := 0
				if _, err := fmt.Sscanf(backpackIndex, "%d", &index); err != nil {
					continue
				}

				backpackIcon, iconExists := decodedInventory[fmt.Sprintf("backpack_icon_%s", backpackIndex)]
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

	rawInventory := GetRawInventory(useProfile, inventoryId)
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

func GetItems(useProfile *skycrypttypes.Member, profileId string) (map[string][]*skycrypttypes.Item, error) {
	if useProfile.Inventory == nil {
		useProfile.Inventory = &skycrypttypes.Inventory{}
	}

	encodedInventories := map[string]*string{
		"inventory":      &useProfile.Inventory.Inventory.Data,
		"enderchest":     &useProfile.Inventory.Enderchest.Data,
		"armor":          &useProfile.Inventory.Armor.Data,
		"equipment":      &useProfile.Inventory.Equipment.Data,
		"personal_vault": &useProfile.Inventory.PersonalVault.Data,
		"wardrobe":       &useProfile.Inventory.Wardrobe.Data,

		// rift
		"rift_inventory":  &useProfile.Rift.Inventory.Inventory.Data,
		"rift_enderchest": &useProfile.Rift.Inventory.Enderchest.Data,
		"rift_armor":      &useProfile.Rift.Inventory.Armor.Data,
		"rift_equipment":  &useProfile.Rift.Inventory.Equipment.Data,

		// bags
		"potion_bag":   &useProfile.Inventory.BagContents.PotionBag.Data,
		"talisman_bag": &useProfile.Inventory.BagContents.TalismanBag.Data,
		"fishing_bag":  &useProfile.Inventory.BagContents.FishingBag.Data,
		// "sacks_bag": &useProfile.Inventory.BagContents.SacksBag.Data,
		"quiver": &useProfile.Inventory.BagContents.Quiver.Data,
	}

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

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	jsonData, err := json.Marshal(output)
	if err != nil {
		fmt.Printf("Error marshaling items for caching: %v\n", err)
	} else {
		_ = redis.Set(fmt.Sprintf("items:%s", profileId), string(jsonData), 5*60)
	}

	return output, nil
}
