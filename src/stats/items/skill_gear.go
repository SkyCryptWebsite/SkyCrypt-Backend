package stats

import (
	"sort"
	"strings"

	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
)

var skillToolCategories = map[string]map[string]bool{
	"MINING":   {"DRILL": true, "PICKAXE": true, "GAUNTLET": true},
	"FARMING":  {"FARMING_TOOL": true, "AXE": true, "SHEARS": true, "VACUUM": true, "WATERING_CAN": true},
	"FORAGING": {"AXE": true},
	"FISHING":  {"FISHING_ROD": true},
}

var excludedSkillGearCategories = map[string]map[string]bool{
	"FISHING": {"FISHING_ROD_PART": true},
}

var skillMiscCategories = map[string]map[string]bool{
	"MINING": {
		"CARNIVAL_MASK": true,
		"CHISEL":        true,
		"DEPLOYABLE":    true,
		"SPADE":         true,
	},
}

var gameStageRanks = map[string]int{
	"MASTER": 7, "PROFESSIONAL": 6, "EXPERT": 5, "SKILLED": 4,
	"INTERMEDIATE": 3, "AMATEUR": 2, "STARTER": 1,
}

type skillOwnedItem struct {
	item       models.ProcessedItem
	itemID     string
	category   string
	baseRarity int
	gameStage  string
	stageRank  int
}

type skillArmorCandidate struct {
	setID     string
	declared  bool
	pieces    map[string]skillOwnedItem
	stageRank int
	gameStage string
}

func GetSkillGear(skill string, allItems []models.ProcessedItem) models.SkillGear {
	gear := models.SkillGear{
		Tools: []models.StrippedItem{},
		Misc:  []models.StrippedItem{},
	}

	category := strings.ToUpper(strings.TrimSpace(skill))
	metadata := constants.ItemsSnapshot()
	armorSets := map[string]*skillArmorCandidate{}
	equipment := map[string]skillOwnedItem{}
	tools := []models.ProcessedItem{}
	misc := []models.ProcessedItem{}
	seenMiscIDs := map[string]bool{}

	var inspect func([]models.ProcessedItem)
	inspect = func(items []models.ProcessedItem) {
		for _, item := range items {
			itemID := GetId(item)
			itemData, ok := metadata[itemID]
			if ok && itemData.MuseumData != nil && strings.EqualFold(itemData.MuseumData.Category, category) {
				owned := skillOwnedItem{
					item:       item,
					itemID:     itemID,
					category:   strings.ToUpper(itemData.Category),
					baseRarity: rarityRank(itemData.Rarity),
					gameStage:  strings.ToUpper(itemData.MuseumData.GameStage),
					stageRank:  gameStageRank(itemData.MuseumData.GameStage),
				}

				switch owned.category {
				case "HELMET", "CHESTPLATE", "LEGGINGS", "BOOTS":
					setIDs := make([]string, 0, len(itemData.MuseumData.ArmorSetExperience))
					for setID := range itemData.MuseumData.ArmorSetExperience {
						setIDs = append(setIDs, setID)
					}

					if len(setIDs) == 0 {
						setIDs = append(setIDs, itemID)
					}

					for _, setID := range setIDs {
						candidate := armorSets[setID]
						if candidate == nil {
							candidate = &skillArmorCandidate{
								setID: setID, declared: len(itemData.MuseumData.ArmorSetExperience) > 0,
								pieces: map[string]skillOwnedItem{}, stageRank: owned.stageRank, gameStage: owned.gameStage,
							}

							armorSets[setID] = candidate
						}

						if owned.stageRank > candidate.stageRank {
							candidate.stageRank = owned.stageRank
							candidate.gameStage = owned.gameStage
						}

						if current, exists := candidate.pieces[owned.category]; !exists || betterArmorPiece(owned, current) {
							candidate.pieces[owned.category] = owned
						}
					}
				case "NECKLACE", "CLOAK", "BELT", "GLOVES", "BRACELET":
					slot := owned.category
					if slot == "BRACELET" {
						slot = "GLOVES"
					}

					if current, exists := equipment[slot]; !exists || betterEquipment(owned, current) {
						equipment[slot] = owned
					}

				default:
					if skillToolCategories[category][owned.category] {
						tools = append(tools, item)
					} else if isSkillMiscItem(category, owned.category) && !seenMiscIDs[itemID] {
						misc = append(misc, item)
						seenMiscIDs[itemID] = true
					}
				}
			}

			if len(item.ContainsItems) > 0 {
				inspect(item.ContainsItems)
			}
		}
	}

	inspect(allItems)

	if best := bestArmorSet(armorSets); best != nil {
		gear.Armor = &models.SkillArmorSet{
			SetID: best.setID, GameStage: best.gameStage,
			Pieces: models.SkillArmorPieces{
				Helmet: strippedArmorPiece(best, "HELMET"), Chestplate: strippedArmorPiece(best, "CHESTPLATE"),
				Leggings: strippedArmorPiece(best, "LEGGINGS"), Boots: strippedArmorPiece(best, "BOOTS"),
			},
		}
	}

	gear.Equipment = models.SkillEquipment{
		Necklace: strippedEquipment(equipment, "NECKLACE"), Cloak: strippedEquipment(equipment, "CLOAK"),
		Belt: strippedEquipment(equipment, "BELT"), Gloves: strippedEquipment(equipment, "GLOVES"),
	}

	sort.Sort(itemSorter(tools))
	sort.Sort(itemSorter(misc))

	gear.Tools = StripItems(&tools, models.StripOptions{Search: true})
	gear.Misc = StripItems(&misc, models.StripOptions{Search: true})

	return gear
}

func isSkillMiscItem(skill, itemCategory string) bool {
	if excludedSkillGearCategories[skill][itemCategory] {
		return false
	}

	if categories, restricted := skillMiscCategories[skill]; restricted {
		return categories[itemCategory]
	}

	return true
}

func gameStageRank(stage string) int {
	return gameStageRanks[strings.ToUpper(stage)]
}

func rarityRank(rarity string) int {
	rank := utility.IndexOf(constants.RARITIES, strings.ToLower(rarity))
	if rank < 0 {
		return -1
	}

	return rank
}

func betterArmorPiece(left, right skillOwnedItem) bool {
	if rarityRank(left.item.Rarity) != rarityRank(right.item.Rarity) {
		return rarityRank(left.item.Rarity) > rarityRank(right.item.Rarity)
	}

	if left.item.Recombobulated != right.item.Recombobulated {
		return left.item.Recombobulated
	}

	if itemSorter([]models.ProcessedItem{left.item, right.item}).Less(0, 1) {
		return true
	}

	if itemSorter([]models.ProcessedItem{right.item, left.item}).Less(0, 1) {
		return false
	}

	if left.item.Source != right.item.Source {
		return left.item.Source < right.item.Source
	}

	return left.item.ItemIndex < right.item.ItemIndex
}

func betterEquipment(left, right skillOwnedItem) bool {
	if left.stageRank != right.stageRank {
		return left.stageRank > right.stageRank
	}

	if rarityRank(left.item.Rarity) != rarityRank(right.item.Rarity) {
		return rarityRank(left.item.Rarity) > rarityRank(right.item.Rarity)
	}

	if left.baseRarity != right.baseRarity {
		return left.baseRarity > right.baseRarity
	}

	if left.item.Recombobulated != right.item.Recombobulated {
		return left.item.Recombobulated
	}

	if itemSorter([]models.ProcessedItem{left.item, right.item}).Less(0, 1) {
		return true
	}

	if itemSorter([]models.ProcessedItem{right.item, left.item}).Less(0, 1) {
		return false
	}

	if left.itemID != right.itemID {
		return left.itemID < right.itemID
	}

	if left.item.Source != right.item.Source {
		return left.item.Source < right.item.Source
	}

	return left.item.ItemIndex < right.item.ItemIndex
}

func bestArmorSet(sets map[string]*skillArmorCandidate) *skillArmorCandidate {
	var best *skillArmorCandidate
	for _, candidate := range sets {
		if best == nil || betterArmorSet(candidate, best) {
			best = candidate
		}
	}

	return best
}

func betterArmorSet(left, right *skillArmorCandidate) bool {
	if left.stageRank != right.stageRank {
		return left.stageRank > right.stageRank
	}

	leftAverage, leftCount, leftMax := armorRarityStats(left)
	rightAverage, rightCount, rightMax := armorRarityStats(right)
	if leftAverage != rightAverage {
		return leftAverage > rightAverage
	}

	if leftCount != rightCount {
		return leftCount > rightCount
	}

	if leftMax != rightMax {
		return leftMax > rightMax
	}

	if left.declared != right.declared {
		return left.declared
	}

	return left.setID < right.setID
}

func armorRarityStats(candidate *skillArmorCandidate) (float64, int, int) {
	total, maximum := 0, -1
	for _, piece := range candidate.pieces {
		total += piece.baseRarity
		if piece.baseRarity > maximum {
			maximum = piece.baseRarity
		}
	}

	return float64(total) / float64(len(candidate.pieces)), len(candidate.pieces), maximum
}

func strippedArmorPiece(candidate *skillArmorCandidate, slot string) *models.StrippedItem {
	piece, ok := candidate.pieces[slot]
	if !ok {
		return nil
	}

	return StripItem(&piece.item, models.StripOptions{Search: true})
}

func strippedEquipment(equipment map[string]skillOwnedItem, slot string) *models.StrippedItem {
	item, ok := equipment[slot]
	if !ok {
		return nil
	}

	return StripItem(&item.item, models.StripOptions{Search: true})
}
