package models

import (
	"encoding/json"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

type ItemTexture struct {
	Parent           string              `json:"parent"`
	Textures         map[string]string   `json:"textures"`
	Overrides        []Override          `json:"overrides"`
	Elements         []TextureElement    `json:"elements,omitempty"`
	HeadModel        string              `json:"firmament:head_model,omitempty"`
	ResourcePackId   string              `json:"resourcePackId,omitempty"`
	FormattedTexture string              `json:"formattedTexture,omitempty"`
	Display          map[string]Position `json:"display,omitempty"`
}

type Position struct {
	Translation [3]float64 `json:"translation"`
	Rotation    [3]float64 `json:"rotation"`
	Scale       [3]float64 `json:"scale"`
}

type TextureElement struct {
	From     [3]float64             `json:"from"`
	To       [3]float64             `json:"to"`
	Rotation *TextureRotation       `json:"rotation,omitempty"`
	Faces    map[string]TextureFace `json:"faces"`
}

type TextureRotation struct {
	Angle  float64    `json:"angle"`
	Axis   string     `json:"axis"`
	Origin [3]float64 `json:"origin"`
}

type TextureFace struct {
	UV      [4]float64 `json:"uv"`
	Texture string     `json:"texture"`
}

type Override struct {
	Predicate map[string]interface{} `json:"predicate"`
	Texture   string                 `json:"model"`
}

type TextureItem struct {
	Count   *int                                     `nbt:"Count" json:"Count,omitempty"`
	Damage  *int                                     `nbt:"Damage" json:"Damage,omitempty"`
	ID      *int                                     `nbt:"id" json:"id,omitempty"`
	Tag     skycrypttypes.TextureItemExtraAttributes `nbt:"tag" json:"tag,omitempty"`
	RawId   string                                   `nbt:"raw_id" json:"raw_id,omitempty"`
	Texture string                                   `nbt:"texture" json:"texture,omitempty"`
}

type McMeta struct {
	Animation McMetaAnimation `json:"animation"`
}

type McMetaAnimation struct {
	Frametime float64 `json:"frametime"`
}

type FormattedTexture struct {
	Path   string `json:"path"`
	PackId string `json:"packId"`
}

// NEW STUFF
type VanillaTexture struct {
	Path string `json:"path"`
}

type PackConfigDescriptionExtra struct {
	Text  string `json:"text"`
	Color string `json:"color,omitempty"`
}

type packConfigTextValue struct {
	Text  string                       `json:"text"`
	Color string                       `json:"color,omitempty"`
	Extra []PackConfigDescriptionExtra `json:"extra,omitempty"`
}

type PackConfigOption struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ID          string `json:"id,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

var packConfigColorCodes = map[string]string{
	"black":        "0",
	"dark_blue":    "1",
	"dark_green":   "2",
	"dark_aqua":    "3",
	"dark_red":     "4",
	"dark_purple":  "5",
	"gold":         "6",
	"gray":         "7",
	"dark_gray":    "8",
	"blue":         "9",
	"green":        "a",
	"aqua":         "b",
	"red":          "c",
	"light_purple": "d",
	"yellow":       "e",
	"white":        "f",
}

func formatPackConfigText(text, color string) string {
	if text == "" {
		return ""
	}

	if code, ok := packConfigColorCodes[strings.ToLower(color)]; ok {
		return "\u00a7" + code + text
	}

	return text
}

func flattenPackConfigTextValue(value packConfigTextValue) string {
	parts := make([]string, 0, len(value.Extra)+1)
	if value.Text != "" {
		parts = append(parts, formatPackConfigText(value.Text, value.Color))
	}

	for _, extra := range value.Extra {
		parts = append(parts, formatPackConfigText(extra.Text, extra.Color))
	}

	return strings.Join(parts, "")
}

func (option *PackConfigOption) UnmarshalJSON(data []byte) error {
	type rawPackConfigOption struct {
		Type        string          `json:"type"`
		Title       json.RawMessage `json:"title"`
		Description json.RawMessage `json:"description"`
		ID          string          `json:"id,omitempty"`
		Default     bool            `json:"default,omitempty"`
	}

	var raw rawPackConfigOption
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	option.Type = raw.Type
	option.ID = raw.ID
	option.Default = raw.Default
	option.Title = ""
	option.Description = ""

	if len(raw.Title) != 0 && string(raw.Title) != "null" {
		if err := json.Unmarshal(raw.Title, &option.Title); err != nil {
			var richTitle packConfigTextValue
			if err := json.Unmarshal(raw.Title, &richTitle); err != nil {
				return err
			}

			option.Title = flattenPackConfigTextValue(richTitle)
		}
	}

	if len(raw.Description) == 0 || string(raw.Description) == "null" {
		return nil
	}

	if err := json.Unmarshal(raw.Description, &option.Description); err == nil {
		return nil
	}

	var richDescription packConfigTextValue

	if err := json.Unmarshal(raw.Description, &richDescription); err != nil {
		return err
	}

	option.Description = flattenPackConfigTextValue(richDescription)

	return nil
}

type PackConfigSection struct {
	Type    string             `json:"type"`
	Title   string             `json:"title"`
	Options []PackConfigOption `json:"options"`
}

func (section *PackConfigSection) UnmarshalJSON(data []byte) error {
	type rawPackConfigSection struct {
		Type    string             `json:"type"`
		Title   json.RawMessage    `json:"title"`
		Options []PackConfigOption `json:"options"`
	}

	var raw rawPackConfigSection
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	section.Type = raw.Type
	section.Options = raw.Options
	section.Title = ""

	if len(raw.Title) == 0 || string(raw.Title) == "null" {
		return nil
	}

	if err := json.Unmarshal(raw.Title, &section.Title); err == nil {
		return nil
	}

	var richTitle packConfigTextValue
	if err := json.Unmarshal(raw.Title, &richTitle); err != nil {
		return err
	}

	section.Title = flattenPackConfigTextValue(richTitle)

	return nil
}

type PackConfig struct {
	Pack struct {
		Description      string `json:"description"`
		PackFormat       int    `json:"pack_format"`
		SupportedFormats []int  `json:"supported_formats"`
		MinFormat        int    `json:"min_format"`
		MaxFormat        int    `json:"max_format"`

		Name     string `json:"name"`
		Author   string `json:"author"`
		Url      string `json:"url"`
		Disabled bool   `json:"disabled,omitempty"`
	} `json:"pack"`
	CatharsisPackV1 struct {
		ID           string `json:"id"`
		Version      string `json:"version"`
		Dependencies struct {
			Catharsis string `json:"catharsis"`
		} `json:"dependencies"`
		Config []PackConfigSection `json:"config"`
	} `json:"catharsis:pack/v1"`
	FabricOverlays struct {
		Entries []struct {
			Directory string `json:"directory"`
			Condition struct {
				Condition string `json:"condition"`
				Pack      string `json:"pack"`
				ID        string `json:"id"`
			} `json:"condition"`
		} `json:"entries"`
	} `json:"fabric:overlays"`
}

type FormattedResourcePack struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Author string `json:"author"`
	Url    string `json:"url"`
	Icon   string `json:"icon"`

	Category map[string]FormattedCatharsisTexture `json:"category,omitempty"`
}

type FormattedCatharsisTexture struct {
}

type CatharsisModelData struct {
	Model CatharsisModel `json:"model"`
}

type CatharsisModel struct {
	Type     string             `json:"type"`
	Property string             `json:"property"`
	OnTrue   CatharsisModelBool `json:"on_true"`
	OnFalse  CatharsisModelBool `json:"on_false"`
	Model    string             `json:"model,omitempty"`
	DataType string             `json:"data_type,omitempty"`
}

type CatharsisModelBool struct {
	Type     string                 `json:"type"`
	Model    string                 `json:"model"`
	Cases    []CatharsisModelCases  `json:"cases"`
	Property string                 `json:"property,omitempty"`
	FallBack CatharsisModelFallback `json:"fallback,omitempty"`
}

type CatharsisModelCases struct {
	When  string `json:"when"`
	Model struct {
		Type  string `json:"type"`
		Model string `json:"model"`
	} `json:"model"`
}

type CatharsisModelFallback struct {
	Type  string `json:"type"`
	Model string `json:"model"`
}
