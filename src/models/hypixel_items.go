package models

type HypixelItemsResponse struct {
	Success     bool          `json:"success"`
	Cause       string        `json:"cause,omitempty"`
	LastUpdated int64         `json:"lastUpdated"`
	Items       []HypixelItem `json:"items"`
}

type HypixelItem struct {
	Material          string                 `json:"material"`
	Skin              skin                   `json:"skin,omitempty"`
	Name              string                 `json:"name"`
	Category          string                 `json:"category"`
	Rarity            string                 `json:"tier"`
	SkyBlockID        string                 `json:"id,omitempty"`
	Damage            int                    `json:"damage,omitempty"`
	Origin            string                 `json:"origin,omitempty"`
	RiftTransferrable bool                   `json:"rift_transferrable,omitempty"`
	MuseumData        *hypixelItemMuseumData `json:"museum_data,omitempty"`
	Color             string                 `json:"color,omitempty"`
	GemstoneSlots     []struct {
		SlotType string `json:"slot_type"`
	} `json:"gemstone_slots"`
}

type ProcessedHypixelItem struct {
	SkyblockID        string                 `json:"skyblock_id"`
	Material          string                 `json:"material"`
	Name              string                 `json:"name"`
	ItemId            int                    `json:"item_id"`
	Rarity            string                 `json:"rarity"`
	Damage            int                    `json:"damage"`
	Texture           string                 `json:"texture"`
	Category          string                 `json:"category"`
	Origin            string                 `json:"origin,omitempty"`
	RiftTransferrable bool                   `json:"rift_transferrable,omitempty"`
	MuseumData        *hypixelItemMuseumData `json:"museum_data,omitempty"`
	Color             string                 `json:"hex_color,omitempty"`
	TextureId         string                 `json:"texture_id,omitempty"`
	GemstoneSlots     []struct {
		SlotType string `json:"slot_type"`
	} `json:"gemstone_slots"`
}

type skin struct {
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

type hypixelItemMuseumData struct {
	Experience         int               `json:"donation_xp"`
	Parent             map[string]string `json:"parent"`
	ArmorSetExperience map[string]int    `json:"armor_set_donation_xp"`
	Category           string            `json:"category"`
}
