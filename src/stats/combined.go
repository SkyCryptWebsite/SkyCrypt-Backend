package stats

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"skycrypt/src/constants"
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
	enabledPacks []string,
) (*models.CombinedOutput, error) {
	return GetCombinedContext(context.Background(), mowojang, profiles, profile, player, userProfile, museum, members, enabledPacks)
}

func GetCombinedContext(
	ctx context.Context,
	mowojang *models.MowojangResponse,
	profiles *models.HypixelProfilesResponse,
	profile *skycrypttypes.Profile,
	player *skycrypttypes.Player,
	userProfile *skycrypttypes.Member,
	museum *skycrypttypes.Museum,
	members []*models.MemberStats,
	enabledPacks []string,
) (*models.CombinedOutput, error) {
	if userProfile.Profile == nil {
		return nil, fmt.Errorf("user profile is nil")
	}

	member := profile.Members[mowojang.UUID]
	if member.Inventory == nil {
		member.Inventory = &skycrypttypes.Inventory{}
	}

	specifiedInventories := make(skyhelpernetworthgo.SpecifiedInventory, 8+len(member.Inventory.Backpack)+len(member.Loadout.Armor.Sets)*4+len(member.Loadout.Equipment.Sets)*4)
	specifiedInventories["armor"] = member.Inventory.Armor
	specifiedInventories["equipment"] = member.Inventory.Equipment
	specifiedInventories["inventory"] = member.Inventory.Inventory
	specifiedInventories["enderchest"] = member.Inventory.Enderchest
	specifiedInventories["talisman_bag"] = member.Inventory.BagContents.TalismanBag
	specifiedInventories["rift_armor"] = member.Rift.Inventory.Armor
	specifiedInventories["rift_equipment"] = member.Rift.Inventory.Equipment
	for backpackId, backpackData := range member.Inventory.Backpack {
		specifiedInventories["backpack_"+backpackId] = backpackData
	}

	for index, layout := range member.Loadout.Armor.Sets {
		inventoryId := fmt.Sprintf("loadout_armor_%d", index)

		specifiedInventories[fmt.Sprintf("%s_helmet", inventoryId)] = layout.Helmet
		specifiedInventories[fmt.Sprintf("%s_chestplate", inventoryId)] = layout.Chestplate
		specifiedInventories[fmt.Sprintf("%s_leggings", inventoryId)] = layout.Leggings
		specifiedInventories[fmt.Sprintf("%s_boots", inventoryId)] = layout.Boots
	}
	for index, layout := range member.Loadout.Equipment.Sets {
		inventoryId := fmt.Sprintf("loadout_equipment_%d", index)

		specifiedInventories[fmt.Sprintf("%s_1", inventoryId)] = layout.EquipmentSlot1
		specifiedInventories[fmt.Sprintf("%s_2", inventoryId)] = layout.EquipmentSlot2
		specifiedInventories[fmt.Sprintf("%s_3", inventoryId)] = layout.EquipmentSlot3
		specifiedInventories[fmt.Sprintf("%s_4", inventoryId)] = layout.EquipmentSlot4
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
	skillGearItems := make([]models.ProcessedItem, 0, totalCap)

	maxWardrobeIndex := -1
	armorSets := make(map[int][]models.ProcessedItem, len(member.Loadout.Armor.Sets))
	for setID := range member.Loadout.Armor.Sets {
		armorSets[setID] = make([]models.ProcessedItem, 4)
		if setID > maxWardrobeIndex {
			maxWardrobeIndex = setID
		}
	}
	equipmentSets := make(map[int][]models.ProcessedItem, len(member.Loadout.Equipment.Sets))
	maxEquipmentWardrobeIndex := -1
	for setID := range member.Loadout.Equipment.Sets {
		equipmentSets[setID] = make([]models.ProcessedItem, 4)
		if setID > maxEquipmentWardrobeIndex {
			maxEquipmentWardrobeIndex = setID
		}
	}

	wardrobeSlice := make([]models.ProcessedItem, (maxWardrobeIndex+1)*4)
	equipmentWardrobeSlice := make([]models.ProcessedItem, (maxEquipmentWardrobeIndex+1)*4)
	neuItemCache := map[string]models.NEUItem{}

	timeNow := time.Now()
	var itemProcessingStats *statsItems.ItemProcessingStats
	if statsItems.ItemProcessingDebugEnabled() {
		itemProcessingStats = statsItems.NewItemProcessingStats("combined", mowojang.UUID, profile.ProfileID, enabledPacks)
	}
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

		processed := statsItems.ProcessItemsWithNEUCacheAndStats(buf, inventoryId, neuItemCache, itemProcessingStats, enabledPacks)

		if strings.HasPrefix(inventoryId, "loadout_armor_") {
			setID, part := 0, ""
			if _, err := fmt.Sscanf(inventoryId, "loadout_armor_%d_%s", &setID, &part); err == nil {
				if offset, ok := constants.WARDROBE_SLOT_OFFSET[part]; ok && len(processed) > 0 {
					armorSets[setID][offset] = processed[0]
					wardrobeSlice[setID*4+offset] = processed[0]
				}
			}
		} else if strings.HasPrefix(inventoryId, "loadout_equipment_") {
			setID, slot := 0, 0
			if _, err := fmt.Sscanf(inventoryId, "loadout_equipment_%d_%d", &setID, &slot); err == nil && slot >= 1 && slot <= 4 && len(processed) > 0 {
				equipmentSets[setID][slot-1] = processed[0]
				equipmentWardrobeSlice[setID*4+slot-1] = processed[0]
			}
		} else {
			processedItems[inventoryId] = processed
		}

		if !strings.HasPrefix(inventoryId, "loadout_equipment_") {
			allItems = append(allItems, processed...)
		}
		if inventoryId != "rift_armor" && inventoryId != "rift_equipment" {
			skillGearItems = append(skillGearItems, processed...)
		}
	}

	processedItems["wardrobe"] = wardrobeSlice
	processedItems["equipment_wardrobe"] = equipmentWardrobeSlice
	skillGearItems = append(skillGearItems, statsItems.GetOwnedMuseumItems(museum, enabledPacks)...)

	itemProcessingDuration := time.Since(timeNow)
	fmt.Printf("Processed %d items in %v pid=%d\n", len(allItems), itemProcessingDuration, os.Getpid())
	if itemProcessingStats != nil {
		itemProcessingStats.LogIfEnabled(itemProcessingDuration)
	}

	sectionsStart := time.Now()
	sectionStart := time.Now()
	gear := GetGear(processedItems, allItems)
	gearDuration := time.Since(sectionStart)
	sectionStart = time.Now()
	accessories := GetAccessories(userProfile, processedItems, enabledPacks)
	accessoriesDuration := time.Since(sectionStart)
	sectionStart = time.Now()
	pets := GetPets(userProfile, profile)
	petsDuration := time.Since(sectionStart)
	sectionStart = time.Now()
	loadouts := GetLoadouts(
		member.Loadout,
		armorSets,
		equipmentSets,
		processedItems["armor"],
		processedItems["equipment"],
		userProfile,
		member.AccessoryBagStorage.Tuning.Slots,
	)
	loadoutsDuration := time.Since(sectionStart)
	sectionStart = time.Now()
	mining := GetMining(userProfile, player, skillGearItems)
	foraging := GetForaging(userProfile, player, skillGearItems)
	farming := GetFarming(userProfile, skillGearItems)
	fishing := GetFishing(userProfile, skillGearItems)
	enchanting := GetEnchanting(userProfile)
	hunting := GetAttributeShards(userProfile)
	skillsDuration := time.Since(sectionStart)
	sectionStart = time.Now()
	dungeons := GetDungeons(userProfile)
	slayer := GetSlayers(userProfile)
	minions := GetMinions(profile)
	bestiary := GetBestiary(userProfile)
	combatDuration := time.Since(sectionStart)
	sectionStart = time.Now()
	collections := getCollectionsWithUsernames(userProfile, profile, memberUsernameMap(members))
	collectionsDuration := time.Since(sectionStart)
	sectionStart = time.Now()
	crimsonIsle := GetCrimsonIsle(userProfile)
	rift := GetRift(userProfile, processedItems)
	misc := GetMisc(userProfile, profile, player)
	miscDuration := time.Since(sectionStart)

	sectionsDuration := time.Since(sectionsStart)
	if sectionsDuration > 50*time.Millisecond {
		fmt.Printf(
			"Combined sections in %v pid=%d gear=%v accessories=%v pets=%v loadouts=%v skills=%v combat=%v collections=%v misc=%v\n",
			sectionsDuration,
			os.Getpid(),
			gearDuration,
			accessoriesDuration,
			petsDuration,
			loadoutsDuration,
			skillsDuration,
			combatDuration,
			collectionsDuration,
			miscDuration,
		)
	}

	return &models.CombinedOutput{
		Gear:        gear,
		Loadouts:    loadouts,
		Accessories: accessories,
		Pets:        pets,
		Skills: &models.SkillsOutput{
			Mining:     mining,
			Foraging:   foraging,
			Farming:    farming,
			Fishing:    fishing,
			Enchanting: enchanting,
			Hunting:    hunting,
		},
		Dungeons:    dungeons,
		Slayer:      slayer,
		Minions:     minions,
		Bestiary:    bestiary,
		Collections: collections,
		CrimsonIsle: crimsonIsle,
		Rift:        rift,
		Misc:        misc,
	}, nil
}

func memberUsernameMap(members []*models.MemberStats) map[string]string {
	usernames := make(map[string]string, len(members))
	for _, member := range members {
		if member == nil || member.UUID == "" || member.Name == "" {
			continue
		}
		usernames[member.UUID] = member.Name
	}
	return usernames
}
