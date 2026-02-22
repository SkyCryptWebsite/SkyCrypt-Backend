package models

import (
	neu "skycrypt/src/models/NEU"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

type NEUItem struct {
	MinecraftId string            `json:"itemid,omitempty"`
	Name        string            `json:"displayname,omitempty"`
	Damage      int               `json:"damage,omitempty"`
	Lore        []string          `json:"lore,omitempty"`
	NEUId       string            `json:"internalname,omitempty"`
	NBT         skycrypttypes.Tag `json:"nbttag"`
	Wiki        []string          `json:"info,omitempty"`
}

type RawNEUItem struct {
	MinecraftId string   `json:"itemid,omitempty"`
	Name        string   `json:"displayname,omitempty"`
	Damage      int      `json:"damage,omitempty"`
	Lore        []string `json:"lore,omitempty"`
	NEUId       string   `json:"internalname,omitempty"`
	NBT         string   `json:"nbttag"`
	Wiki        []string `json:"info,omitempty"`
}

type NEUConstant struct {
	PetNums            neu.PetNums           `json:"petnums,omitempty"`
	Pets               neu.Pets              `json:"pets,omitempty"`
	Bestiary           neu.BestiaryConstants `json:"bestiary,omitempty"`
	Garden             neu.NEUGarden         `json:"garden,omitempty"`
	HeartOfTheMountain neu.HOTMConstants     `json:"hotm,omitempty"`
	HeartOfTheForest   neu.HOTFConstants     `json:"hotf,omitempty"`
}
