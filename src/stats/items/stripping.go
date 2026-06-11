package stats

import (
	"skycrypt/src/models"
)

func StripItems(items *[]models.ProcessedItem, stripOpts ...models.StripOptions) []models.StrippedItem {
	var opts models.StripOptions
	if len(stripOpts) > 0 {
		opts = stripOpts[0]
	}

	output := make([]models.StrippedItem, len(*items))
	for i, item := range *items {
		output[i] = *StripItem(&item, opts)

		if len(item.ContainsItems) > 0 {
			output[i].ContainsItems = StripItems(&item.ContainsItems, models.StripOptions{
				Search: opts.Search,
				Nested: true, // All nested items should be displayed inline
			})
		}
	}

	return output
}

func StripItem(item *models.ProcessedItem, stripOpts ...models.StripOptions) *models.StrippedItem {
	if item == nil {
		return nil
	}

	var opts models.StripOptions
	if len(stripOpts) > 0 {
		opts = stripOpts[0]
	}

	output := &models.StrippedItem{
		DisplayName:    item.DisplayName,
		Lore:           item.Lore,
		Rarity:         item.Rarity,
		Recombobulated: item.Recombobulated,
		Texture:        item.Texture,
		Shiny:          item.Shiny,
	}

	if opts.Search {
		output.Source = item.Source
	}

	if item.IsInactive != nil {
		output.IsInactive = item.IsInactive
	}

	if item.Count != nil && *item.Count > 1 {
		output.Count = item.Count
	}

	if item.Wiki != nil {
		output.Wiki = item.Wiki
	}

	if item.TexturePack != "" {
		output.TexturePack = item.TexturePack
	}

	if opts.Nested && item.DisplayName != "" && item.ContainsItems != nil {
		output.DisplayInline = true
	}

	return output
}

func StripPets(pets []models.ProcessedPet) []models.StrippedPet {
	output := make([]models.StrippedPet, len(pets))
	for i, pet := range pets {
		output[i] = StripPet(pet)
	}

	return output
}

func StripPet(pet models.ProcessedPet) models.StrippedPet {
	return models.StrippedPet{
		Active:      pet.Active,
		Type:        pet.Type,
		Rarity:      pet.Rarity,
		Level:       pet.Level.Level,
		MaxLevel:    pet.Level.MaxLevel,
		DisplayName: pet.Name,
		Texture:     pet.Texture,
		Lore:        pet.Lore,
		Stats:       pet.Stats,
	}
}
