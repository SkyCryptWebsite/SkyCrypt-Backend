package stats

import (
	"fmt"

	"skycrypt/src/models"
	statsItems "skycrypt/src/stats/items"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
)

func GetCombined(
	mowojang *models.MowojangResponse,
	profiles *models.HypixelProfilesResponse,
	profile *skycrypttypes.Profile,
	player *skycrypttypes.Player,
	userProfile *skycrypttypes.Member,
	museum *skycrypttypes.Museum,
	members []*models.MemberStats,
	disabledPacks []string,
) (*models.CombinedOutput, error) {
	if userProfile.Profile == nil {
		return nil, fmt.Errorf("user profile is nil")
	}

	member := profile.Members[mowojang.UUID]
	if member.Inventory == nil {
		member.Inventory = &skycrypttypes.Inventory{}
	}

	specifiedInventories := make(skyhelpernetworthgo.SpecifiedInventory, 8+len(member.Inventory.Backpack))
	specifiedInventories["armor"] = member.Inventory.Armor
	specifiedInventories["equipment"] = member.Inventory.Equipment
	specifiedInventories["wardrobe"] = member.Inventory.Wardrobe
	specifiedInventories["inventory"] = member.Inventory.Inventory
	specifiedInventories["enderchest"] = member.Inventory.Enderchest
	specifiedInventories["talisman_bag"] = member.Inventory.BagContents.TalismanBag
	specifiedInventories["rift_armor"] = member.Rift.Inventory.Armor
	specifiedInventories["rift_equipment"] = member.Rift.Inventory.Equipment
	for backpackId, backpackData := range member.Inventory.Backpack {
		specifiedInventories["backpack_"+backpackId] = backpackData
	}

	decodedItems, err := skyhelpernetworthgo.CalculateFromSpecifiedInventories(specifiedInventories,
		skyhelpernetworthgo.NetworthOptions{
			IncludeItemData:  true,
			KeepInvalidItems: true,
		}.ToInternal())
	if err != nil {
		return nil, fmt.Errorf("failed to calculate networth: %v", err)
	}

	maxInvSize, totalCap := 0, 0
	for inventoryId := range specifiedInventories {
		invType := decodedItems.Types[inventoryId]
		if invType == nil {
			continue
		}
		n := len(invType.Items)
		if n > maxInvSize {
			maxInvSize = n
		}
		totalCap += n
	}

	combinedBuf := make([]*skycrypttypes.Item, maxInvSize)
	processedItems := make(map[string][]models.ProcessedItem, len(specifiedInventories))
	allItems := make([]models.ProcessedItem, 0, totalCap)

	for inventoryId := range specifiedInventories {
		invType := decodedItems.Types[inventoryId]
		if invType == nil || len(invType.Items) == 0 {
			continue
		}
		inventoryData := invType.Items

		buf := combinedBuf[:len(inventoryData)]
		for i, item := range inventoryData {
			buf[i] = item.ItemData
			if buf[i] != nil {
				buf[i].Price = item.Price
			}
		}

		processed := statsItems.ProcessItems(buf, inventoryId, disabledPacks)
		processedItems[inventoryId] = processed

		allItems = append(allItems, processed...)
	}

	return &models.CombinedOutput{
		Gear:         GetGear(processedItems, allItems),
		Accesssories: GetAccessories(userProfile, processedItems, disabledPacks),
		Pets:         GetPets(userProfile, profile),
		Skills: &models.SkillsOutput{
			Mining:     GetMining(userProfile, player, allItems),
			Foraging:   GetForaging(userProfile, player, allItems),
			Farming:    GetFarming(userProfile, allItems),
			Fishing:    GetFishing(userProfile, allItems),
			Enchanting: GetEnchanting(userProfile),
			Hunting:    GetAttributeShards(userProfile),
		},
		Dungeons:    GetDungeons(userProfile),
		Slayer:      GetSlayers(userProfile),
		Minions:     GetMinions(profile),
		Bestiary:    GetBestiary(userProfile),
		Collections: GetCollections(userProfile, profile),
		CrimsonIsle: GetCrimsonIsle(userProfile),
		Rift:        GetRift(userProfile, processedItems),
		Misc:        GetMisc(userProfile, profile, player),
	}, nil
}
