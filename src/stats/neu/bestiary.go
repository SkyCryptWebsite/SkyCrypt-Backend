package neustats

import (
	"fmt"
	neu "skycrypt/src/models/NEU"
	"skycrypt/src/utility"
)

func getTexture(mob neu.NEUBestiaryRawMob) string {
	if mob.Texture == "" {
		return fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), mob.Item)
	}

	skinHash := utility.GetSkinHash(mob.Texture)
	if skinHash == "" {
		return fmt.Sprintf("%s/api/item/BARRIER", utility.GetDomain())
	}

	return fmt.Sprintf("%s/api/head/%s", utility.GetDomain(), skinHash)
}

func GetIslandTexture(island neu.NEUBestiaryRawIslandData) string {
	if island.Icon.Texture == "" {
		return fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), island.Icon.Item)
	}

	return fmt.Sprintf("%s/api/head/%s", utility.GetDomain(), utility.GetSkinHash(island.Icon.Texture))
}

func FormatBestiaryConstants(bestiaryConstants neu.NEUBestiaryRaw) neu.BestiaryConstants {
	formatted := neu.BestiaryConstants{
		Brackets: bestiaryConstants.Brackets,
		Islands:  make(map[string]neu.BestiaryCategory),
	}

	for name, islandData := range bestiaryConstants.Islands {
		if islandData.HasSubcategories {
			for subcategoryName, subcategoryMobs := range islandData.SubcategoryIslands {
				category := neu.BestiaryCategory{
					Name:    utility.GetRawLore(subcategoryName),
					Texture: GetIslandTexture(islandData),
					Mobs:    make([]neu.BestiaryMob, len(subcategoryMobs.Mobs)),
				}

				for i, mob := range subcategoryMobs.Mobs {
					category.Mobs[i] = neu.BestiaryMob{
						Name:    utility.GetRawLore(mob.Name),
						Texture: getTexture(mob),
						Cap:     mob.Cap,
						Mobs:    mob.Mobs,
						Bracket: mob.Bracket,
					}
				}

				formatted.Islands[fmt.Sprintf("%s_%s", name, subcategoryName)] = category
			}
			continue
		}

		category := neu.BestiaryCategory{
			Name:    utility.GetRawLore(islandData.Name),
			Texture: GetIslandTexture(islandData),
			Mobs:    make([]neu.BestiaryMob, len(islandData.Mobs)),
		}

		for i, mob := range islandData.Mobs {
			category.Mobs[i] = neu.BestiaryMob{
				Name:    utility.GetRawLore(mob.Name),
				Texture: getTexture(mob),
				Cap:     mob.Cap,
				Mobs:    mob.Mobs,
				Bracket: mob.Bracket,
			}
		}

		formatted.Islands[name] = category
	}

	return formatted
}
