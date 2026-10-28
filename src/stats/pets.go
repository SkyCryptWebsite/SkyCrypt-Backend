package stats

import (
	"fmt"
	"reflect"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	stats "skycrypt/src/stats/items"
	"sync/atomic"

	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"slices"
	"sort"
	"strings"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
)

type petProcessingContext struct {
	prices            map[string]float64
	networthService   *skyhelpernetworthgo.CalculatorService
	networthOptions   skyhelpernetworthgo.NetworthOptions
	maxPetIds         map[string]int
	defaultPetTexture string
	domain            string
}

func newPetProcessingContext() *petProcessingContext {
	prices, _ := skyhelpernetworthgo.GetPrices(true, 0, 0)
	domain := utility.GetDomain()
	return &petProcessingContext{
		prices:            prices,
		networthService:   skyhelpernetworthgo.NewCalculatorService(),
		networthOptions:   skyhelpernetworthgo.NetworthOptions{},
		maxPetIds:         getMaxPetIds(),
		defaultPetTexture: fmt.Sprintf("%s/api/head/bc8ea1f51f253ff5142ca11ae45193a4ad8c3ab5e9c6eec8ba7a4fcb7bac40", domain),
		domain:            domain,
	}
}

func getMaxPetIds() map[string]int {
	maxPetIds := make(map[string]int)
	for petType, petData := range notenoughupdates.Constants().PetNums {
		if len(petData) == 0 || petType == "BINGO" {
			continue
		}

		if constants.PET_PARENTS[petType] != "" {
			petType = constants.PET_PARENTS[petType]
		}

		maxRarity := 0
		for rarity := range petData {
			rarityIndex := slices.Index(constants.RARITIES, strings.ToLower(rarity))
			if rarityIndex > maxRarity {
				maxRarity = rarityIndex
			}
		}

		maxPetIds[petType] = maxRarity
	}

	return maxPetIds
}

func getPetLevel(pet skycrypttypes.Pet) models.PetLevel {
	petData := notenoughupdates.Constants().Pets

	rarityOffset := petData.CustomPetLeveling[pet.Type].RarityOffset[pet.Rarity]
	if rarityOffset == nil {
		rarityOffset = petData.PetRarityOffset[pet.Rarity]
	}

	for i := len(constants.RARITIES) - 1; i >= 1; i-- {
		if rarityOffset != nil {
			break
		}

		rarityOffset = petData.CustomPetLeveling[pet.Type].RarityOffset[strings.ToUpper(constants.RARITIES[i])]
		if rarityOffset == nil {
			rarityOffset = petData.PetRarityOffset[strings.ToUpper(constants.RARITIES[i])]
		}
	}

	if rarityOffset == nil {
		return models.PetLevel{}
	}

	maxLevel := petData.CustomPetLeveling[pet.Type].MaxLevel
	if maxLevel == 0 {
		maxLevel = 100
	}

	endIndex := min(*rarityOffset+maxLevel-1, len(petData.PetLevels))
	if *rarityOffset >= len(petData.PetLevels) {
		return models.PetLevel{}
	}

	baseLevels := petData.PetLevels[*rarityOffset:endIndex]
	var levels []int
	levels = append(levels, baseLevels...)

	if customPetData, exists := petData.CustomPetLeveling[pet.Type]; exists && customPetData.PetLevels != nil {
		levels = append(levels, customPetData.PetLevels...)
	}

	level := 1
	xpMaxLevel := 0
	currentXp := 0
	for i := 0; i < maxLevel-1; i++ {
		xpMaxLevel += levels[i]
		if xpMaxLevel <= int(pet.Experience) {
			level++
			currentXp = int(pet.Experience) - xpMaxLevel
		}
	}

	xpForNext := 0
	if level-1 < len(levels) {
		xpForNext = levels[level-1]
	}

	progress := 0.0
	if xpForNext != 0 {
		progress = float64(currentXp) / float64(xpForNext)
	}

	return models.PetLevel{
		Experience:            int(pet.Experience),
		Level:                 level,
		MaxLevel:              maxLevel,
		CurrentExperience:     currentXp,
		ExperienceForNext:     xpForNext,
		Progress:              progress,
		ExperienceForMaxLevel: xpMaxLevel,
	}

}

func getPetData(level int, petType string, rarity string) map[string]float64 {
	petNums := notenoughupdates.Constants().PetNums
	petData, exists := petNums[petType]
	if !exists {
		return nil
	}

	rarityData, exists := petData[rarity]
	if !exists {
		return nil
	}

	lvlMin := rarityData.Level1
	if lvlMin == nil {
		return map[string]float64{}
	}

	lvlMax := rarityData.Level100
	if lvlMax == nil {
		return map[string]float64{}
	}

	statPerLevel := float64(utility.Min(level, 100)-1) / float64(100-1)
	output := make(map[string]float64)

	allKeys := make(map[string]bool)
	for key := range lvlMin.StatNums {
		allKeys[key] = true
	}
	for key := range lvlMax.StatNums {
		allKeys[key] = true
	}

	for key := range allKeys {
		var lowStat, highStat float64

		if val, exists := lvlMin.StatNums[key]; exists {
			lowStat = val
		}

		if val, exists := lvlMax.StatNums[key]; exists {
			highStat = val
		}

		output[key] = utility.Round(lowStat+(highStat-lowStat)*statPerLevel, 2)
	}

	for i, val := range lvlMin.OtherNums {
		if i < len(lvlMax.OtherNums) {
			lowStat := val
			highStat := lvlMax.OtherNums[i]
			output[fmt.Sprintf("otherNum_%d", i)] = utility.Round(lowStat+(highStat-lowStat)*statPerLevel, 2)
		}
	}

	return output
}

func getProfilePets(userProfile *skycrypttypes.Member, pets *[]skycrypttypes.Pet, petCtx *petProcessingContext) []models.ProcessedPet {
	if petCtx == nil {
		petCtx = newPetProcessingContext()
	}

	output := make([]models.ProcessedPet, 0, len(*pets))
	for _, pet := range *pets {
		if pet.Rarity == "" {
			continue
		}

		applyTierBoost(&pet)

		outputPet := models.ProcessedPet{
			Type:      pet.Type,
			Name:      utility.TitleCase(pet.Type),
			Rarity:    pet.Rarity,
			Active:    pet.Active,
			Price:     0,
			Level:     getPetLevel(pet),
			Texture:   petCtx.defaultPetTexture,
			Lore:      []string{"§cThis pet is not saved in the repository", "", "§cIf you expected it to be there please send a message in", "§c§l#neu-support §r§con §ldiscord.gg/moulberry"},
			Stats:     map[string]float64{},
			CandyUsed: pet.CandyUsed,
			Skin:      pet.Skin,
		}
		petDataRarity := strings.ToUpper(pet.Rarity)
		NEUItemId := fmt.Sprintf("%s;%d", pet.Type, slices.Index(constants.RARITIES, strings.ToLower(pet.Rarity)))
		NEUItem, err := notenoughupdates.GetItem(NEUItemId)
		if err != nil {
			for i := len(constants.RARITIES) - 1; i >= 1; i-- {
				NEUItemId = fmt.Sprintf("%s;%d", pet.Type, i)
				NEUItem, err = notenoughupdates.GetItem(NEUItemId)
				petDataRarity = constants.RARITIES[i]
				if err == nil {
					break
				}
			}

		}

		if pet.Skin != "" {
			skinId := fmt.Sprintf("PET_SKIN_%s", pet.Skin)
			skinData, err := notenoughupdates.GetItem(skinId)
			if err == nil && skinData.NBT.SkullOwner != nil && len(skinData.NBT.SkullOwner.Properties.Textures) > 0 {
				var textureId = utility.GetSkinHash(skinData.NBT.SkullOwner.Properties.Textures[0].Value)
				outputPet.Texture = fmt.Sprintf("%s/api/head/%s", petCtx.domain, textureId)

				outputPet.Name += " ✦"
			}
		} else if NEUItem.NBT.SkullOwner != nil && len(NEUItem.NBT.SkullOwner.Properties.Textures) > 0 {
			var textureId = utility.GetSkinHash(NEUItem.NBT.SkullOwner.Properties.Textures[0].Value)
			outputPet.Texture = fmt.Sprintf("%s/api/head/%s", petCtx.domain, textureId)
		}

		data := getPetData(outputPet.Level.Level, pet.Type, strings.ToUpper(petDataRarity))
		for key, value := range data {
			if strings.Contains(key, "otherNum") {
				newKey := strings.ReplaceAll(key, "otherNum_", "")
				data[newKey] = value
				delete(data, key)
				continue
			}

			if pet.Active && len(key) > 1 {
				outputPet.Stats[strings.ToLower(key)] = value
			}
		}

		outputPet.Lore = []string{}
		for _, line := range NEUItem.Lore {
			// ? NOTE: This is a work around for a Montezuma pet, needed otherwise the description will be incorrect
			if outputPet.Type == "FRACTURED_MONTEZUMA_SOUL" {
				soulPieces := len(userProfile.Rift.DeadCats.FoundCats)
				if line == "§7Found: §96/9 Soul Pieces" {
					outputPet.Lore = append(outputPet.Lore, fmt.Sprintf("§7Found: §9%d/9 Soul Pieces", soulPieces))
					continue
				}

				if line == "§7Rift Time: §a+100s" {
					outputPet.Lore = append(outputPet.Lore, fmt.Sprintf("§7Rift Time: §a+%ds", 10+soulPieces*15))
					continue
				}

				if line == "§7Mana Regen: §a+12%" {
					outputPet.Lore = append(outputPet.Lore, fmt.Sprintf("§7Mana Regen: §a+%d%%", soulPieces*2))
					continue
				}
			}

			if strings.HasPrefix(line, "§7§eRight-click to add this pet to") {
				break
			}

			outputPet.Lore = append(outputPet.Lore, utility.ReplaceVariables(line, data))
		}

		if pet.HeldItem != "" {
			heldItem, err := notenoughupdates.GetItem(pet.HeldItem)
			if err != nil {
				if len(outputPet.Lore) > 0 && strings.TrimSpace(outputPet.Lore[len(outputPet.Lore)-1]) != "" {
					outputPet.Lore = append(outputPet.Lore, "")
				}

				outputPet.Lore = append(outputPet.Lore, fmt.Sprintf("§6Held item: %s", utility.TitleCase(pet.HeldItem)))
				outputPet.Lore = append(outputPet.Lore, "§cCould not find held item in Not Enough Updates repository.")
			}

			spaces := 0
			outputPet.Lore = append(outputPet.Lore, "", fmt.Sprintf("§6Held Item: %s", heldItem.Name))
			for _, line := range heldItem.Lore {
				if strings.Trim(line, " ") == "" {
					spaces++
				} else if spaces == 2 {
					outputPet.Lore = append(outputPet.Lore, line)
				} else if spaces > 2 {
					break
				}
			}
		}

		if len(outputPet.Lore) > 0 && strings.TrimSpace(outputPet.Lore[len(outputPet.Lore)-1]) != "" {
			outputPet.Lore = append(outputPet.Lore, "")
		}

		if outputPet.Level.Experience >= outputPet.Level.ExperienceForMaxLevel {
			outputPet.Lore = append(outputPet.Lore, `§bMAX LEVEL`)
		} else {
			outputPet.Lore = append(outputPet.Lore, fmt.Sprintf("§7Progress to Level %d: §e%.1f%%", outputPet.Level.Level+1, outputPet.Level.Progress*100))
			progress := int(outputPet.Level.Progress * 20)
			numerator := utility.FormatNumber(outputPet.Level.CurrentExperience)
			denominator := utility.FormatNumber(outputPet.Level.ExperienceForNext)
			outputPet.Lore = append(outputPet.Lore, fmt.Sprintf("§2%s§f%s §e%s §6/ §e%s", strings.Repeat("─", progress), strings.Repeat("─", 20-progress), numerator, denominator))
		}

		outputPet.Lore = append(outputPet.Lore,
			"",
			"§7Total XP: §e"+utility.FormatNumber(outputPet.Level.Experience)+" §6/ §e"+utility.FormatNumber(outputPet.Level.ExperienceForMaxLevel)+" §6("+fmt.Sprintf("%.2f", (float64(outputPet.Level.Experience)/float64(outputPet.Level.ExperienceForMaxLevel))*100)+"%)",
			fmt.Sprintf("§7Candy Used: §e%d §6/ §e10", outputPet.CandyUsed),
		)

		// TODO: Gotta improve this one day, its kinda ugly
		networthResult := petCtx.networthService.NewSkyBlockPetCalculator(&pet, petCtx.prices, petCtx.networthOptions)
		petCtx.networthService.CalculatePet(networthResult)
		price := networthResult.Price + networthResult.BasePrice
		if price > 0 {
			outputPet.Lore = append(outputPet.Lore, "", fmt.Sprintf("§7Item Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(price)), utility.FormatNumber(price)))

		}

		output = append(output, outputPet)
	}

	sortProcessedPets(output)
	return output
}

func applyTierBoost(pet *skycrypttypes.Pet) {
	if pet.HeldItem == "PET_ITEM_TIER_BOOST" {
		pet.Rarity = constants.RARITIES[slices.Index(constants.RARITIES, strings.ToLower(pet.Rarity))+1]
	}
}

// getActivePetStats returns the active pet's stats and the pet score, matching what GetPets reports, without
// processing every owned pet: the score only needs each pet's type, rarity and level. ok is false with no pets.
func getActivePetStats(userProfile *skycrypttypes.Member) (activeStats map[string]float64, score models.PetScore, ok bool) {
	var allPets []skycrypttypes.Pet
	if userProfile.Pets != nil {
		allPets = append(allPets, userProfile.Pets.Pets...)
	}
	if montezuma := userProfile.Rift.DeadCats.Montezuma; montezuma.Rarity != "" {
		montezuma.Active = false
		allPets = append(allPets, montezuma)
	}

	scorePets := make([]models.ProcessedPet, 0, len(allPets))
	var activePets []skycrypttypes.Pet
	for _, pet := range allPets {
		if pet.Rarity == "" {
			continue
		}
		if pet.Active {
			activePets = append(activePets, pet)
		}
		applyTierBoost(&pet)
		scorePets = append(scorePets, models.ProcessedPet{Type: pet.Type, Rarity: pet.Rarity, Level: getPetLevel(pet)})
	}
	if len(scorePets) == 0 {
		return nil, models.PetScore{}, false
	}

	if len(activePets) > 0 {
		// getProfilePets sorts, so with several active pets this is the one GetPets lists first.
		activeStats = getProfilePets(userProfile, &activePets, nil)[0].Stats
	}
	return activeStats, GetPetScore(scorePets), true
}

func sortProcessedPets(output []models.ProcessedPet) {
	sort.Slice(output, func(i, j int) bool {
		if output[i].Rarity != output[j].Rarity {
			return slices.Index(constants.RARITIES, strings.ToLower(output[i].Rarity)) > slices.Index(constants.RARITIES, strings.ToLower(output[j].Rarity))
		} else if output[i].Level.Level != output[j].Level.Level {
			return output[i].Level.Level > output[j].Level.Level
		}

		return output[i].Name < output[j].Name
	})
}

// montezumaSoul shows the profile's found soul pieces in its lore, so it is the one missing pet processed per request.
const montezumaSoul = "FRACTURED_MONTEZUMA_SOUL"

// missingPetCatalog is every pet a profile can be missing, already processed. Nothing in it depends on the profile,
// only on NEU data, prices and whether the profile is bingo, so it is built once per combination of those.
type missingPetCatalog struct {
	neu    *models.NEUConstant
	prices map[string]float64 // held, not just its address, so the map can't be freed and the address reused
	domain string
	pets   map[string]models.ProcessedPet
}

// missingPetCatalogs holds one catalog for normal profiles and one for bingo.
var missingPetCatalogs [2]atomic.Pointer[missingPetCatalog]

func missingPetCandidate(petType string, bingo bool, petCtx *petProcessingContext) (skycrypttypes.Pet, bool) {
	if petType == "BINGO" && !bingo {
		return skycrypttypes.Pet{}, false
	}
	rarityIndex := petCtx.maxPetIds[petType]
	if rarityIndex == 0 {
		return skycrypttypes.Pet{}, false
	}

	pet := skycrypttypes.Pet{
		Type:       petType,
		Active:     false,
		Experience: 0,
		Rarity:     strings.ToUpper(constants.RARITIES[rarityIndex]),
		CandyUsed:  0,
	}
	pet.Experience = float64(getPetLevel(pet).ExperienceForMaxLevel)
	return pet, true
}

func missingPetsFor(bingo bool, petCtx *petProcessingContext) map[string]models.ProcessedPet {
	neu := notenoughupdates.Constants()
	slot := &missingPetCatalogs[0]
	if bingo {
		slot = &missingPetCatalogs[1]
	}
	if catalog := slot.Load(); catalog != nil && catalog.neu == neu && reflect.ValueOf(catalog.prices).Pointer() == reflect.ValueOf(petCtx.prices).Pointer() && catalog.domain == petCtx.domain {
		return catalog.pets
	}

	candidates := make([]skycrypttypes.Pet, 0, len(neu.PetNums))
	for petType := range neu.PetNums {
		if petType == montezumaSoul {
			continue
		}
		if pet, ok := missingPetCandidate(petType, bingo, petCtx); ok {
			candidates = append(candidates, pet)
		}
	}

	pets := make(map[string]models.ProcessedPet, len(candidates))
	for _, pet := range getProfilePets(&skycrypttypes.Member{}, &candidates, petCtx) {
		pets[pet.Type] = pet
	}
	slot.Store(&missingPetCatalog{neu: neu, prices: petCtx.prices, domain: petCtx.domain, pets: pets})
	return pets
}

func getMissingPets(userProfile *skycrypttypes.Member, pets []models.ProcessedPet, gameMode string, petCtx *petProcessingContext) []models.ProcessedPet {
	if petCtx == nil {
		petCtx = newPetProcessingContext()
	}

	ownedPetTypes := make(map[string]struct{}, len(pets))
	for _, pet := range pets {
		ownedPetTypes[pet.Type] = struct{}{}
	}

	bingo := gameMode == "bingo"
	catalog := missingPetsFor(bingo, petCtx)
	output := make([]models.ProcessedPet, 0, len(catalog)+1)
	for petType, pet := range catalog {
		if _, owned := ownedPetTypes[petType]; !owned {
			output = append(output, pet)
		}
	}

	if _, owned := ownedPetTypes[montezumaSoul]; !owned {
		if _, known := notenoughupdates.Constants().PetNums[montezumaSoul]; known {
			if pet, ok := missingPetCandidate(montezumaSoul, bingo, petCtx); ok {
				output = append(output, getProfilePets(userProfile, &[]skycrypttypes.Pet{pet}, petCtx)...)
			}
		}
	}

	sortProcessedPets(output)
	return output
}

func GetPetScore(pets []models.ProcessedPet) models.PetScore {
	highestRarity, highestLevel := map[string]int{}, map[string]int{}
	for _, pet := range pets {
		if pet.Type == "FRACTURED_MONTEZUMA_SOUL" {
			continue
		}

		if pet.Level.Experience >= pet.Level.ExperienceForMaxLevel {
			highestLevel[pet.Type] = 1
		}

		rarityIndex := slices.Index(constants.RARITIES, strings.ToLower(pet.Rarity))
		if rarityIndex > highestRarity[pet.Type] || highestRarity[pet.Type] == 0 {
			highestRarity[pet.Type] = rarityIndex
		}
	}

	rarity := 0
	for _, r := range highestRarity {
		rarity += r + 1
	}

	total := rarity + len(highestLevel)

	keys := make([]int, 0, len(constants.PET_REWARDS))
	for k := range constants.PET_REWARDS {
		keys = append(keys, k)
	}

	sort.Ints(keys)
	bonus := map[string]float64{}
	for _, key := range keys {
		value := constants.PET_REWARDS[key]
		if total > key {
			bonus = value
		}
	}

	unlocked := false
	reward := make([]models.PetScoreReward, 0, len(constants.PET_REWARD_ORDER))
	for _, score := range constants.PET_REWARD_ORDER {
		bonusValue := constants.PET_REWARDS[score]
		isUnlocked := !unlocked && total >= score
		reward = append(reward, models.PetScoreReward{
			Score:    score,
			Bonus:    int(bonusValue["magic_find"]),
			Unlocked: isUnlocked,
		})

		if isUnlocked {
			unlocked = true
		}
	}

	output := models.PetScore{
		Amount: total,
		Stats:  bonus,
		Reward: reward,
	}

	return output
}

func GetPets(userProfile *skycrypttypes.Member, profile *skycrypttypes.Profile) *models.OutputPets {
	if userProfile.Pets == nil {
		userProfile.Pets = &skycrypttypes.Pets{}
	}

	allPets := []skycrypttypes.Pet{}
	allPets = append(allPets, userProfile.Pets.Pets...)
	if userProfile.Rift.DeadCats.Montezuma.Rarity != "" {
		userProfile.Rift.DeadCats.Montezuma.Active = false
		allPets = append(allPets, userProfile.Rift.DeadCats.Montezuma)
	}

	petCtx := newPetProcessingContext()
	pets := getProfilePets(userProfile, &allPets, petCtx)
	petAmount, skinAmount, totalPetExp, totalCandyUsed := map[string]int{}, 0, 0, 0
	for _, pet := range pets {
		totalPetExp += pet.Level.Experience
		petAmount[pet.Type]++

		if pet.Skin != "" {
			skinAmount++
		}

		if pet.CandyUsed > 0 {
			totalCandyUsed += pet.CandyUsed
		}
	}

	output := &models.OutputPets{
		Pets:               stats.StripPets(pets),
		Amount:             len(petAmount),
		Total:              len(petCtx.maxPetIds),
		AmountSkins:        skinAmount,
		TotalPetExperience: totalPetExp,
		TotalCandyUsed:     totalCandyUsed,
		PetScore:           GetPetScore(pets),
	}

	output.MissingPets = stats.StripPets(getMissingPets(userProfile, pets, profile.GameMode, petCtx))

	return output
}
