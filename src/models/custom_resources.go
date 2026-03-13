package models

import skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"

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

type VanillaTexture struct {
	VanillaId string `json:"vanillaId"`
	Damage    int    `json:"damage"`
}

type McMeta struct {
	Animation McMetaAnimation `json:"animation"`
}

type McMetaAnimation struct {
	Frametime int `json:"frametime"`
}

type FormattedTexture struct {
	Path           string `json:"path"`
	ResourcePackId string `json:"resourcePackId"`
}
