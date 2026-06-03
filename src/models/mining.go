package models

type MiningOutput struct {
	Level                  Skill             `json:"level"`
	MiningLevel            Skill             `json:"miningLevel"`
	PeakOfTheMountain      PeakOfTheMountain `json:"peakOfTheMountain"`
	SelectedPickaxeAbility string            `json:"selectedPickaxeAbility"`
	Tokens                 HotmTokens        `json:"tokens"`
	Commissions            Commissions       `json:"commissions"`
	CrystalHollows         CrystalHollows    `json:"crystalHollows"`
	Powder                 PowderOutput      `json:"powder"`
	GlaciteTunnels         GlaciteTunnels    `json:"glaciteTunnels"`
	Forge                  []ForgeOutput     `json:"forge"`
	Tools                  SkillToolsResult  `json:"tools"`
	Hotm                   []ProcessedItem   `json:"hotm"`
}

type PeakOfTheMountain struct {
	Level    int `json:"level"`
	MaxLevel int `json:"max_level"`
}

type HotmTokens struct {
	Total     int `json:"total"`
	Spent     int `json:"spent"`
	Available int `json:"available"`
}

type Commissions struct {
	Milestone   int `json:"milestone"`
	Completions int `json:"completions"`
}

type CrystalHollows struct {
	CrystalHollowsLastAccess int64              `json:"crystalHollowsLastAccess"`
	NucleusRuns              int                `json:"nucleusRuns"`
	Progress                 CrystalNucleusRuns `json:"progress"`
}

type CrystalNucleusRuns struct {
	Crystals map[string]string `json:"crystals"`
	Parts    map[string]string `json:"parts"`
}

type GlaciteTunnels struct {
	MineshaftsEntered int     `json:"mineshaftsEntered"`
	Corpses           Corpses `json:"corpses"`
	Fossils           Fossils `json:"fossils"`
}

type Corpses struct {
	Found   int      `json:"found"`
	Max     int      `json:"max"`
	Corpses []Corpse `json:"corpses"`
}

type Fossils struct {
	Found   int      `json:"found"`
	Max     int      `json:"max"`
	Fossils []Fossil `json:"fossils"`
}

type Corpse struct {
	Name    string `json:"name"`
	Amount  int    `json:"amount"`
	Texture string `json:"texture_path"`
}

type Fossil struct {
	Name    string `json:"name"`
	Found   bool   `json:"found"`
	Texture string `json:"texture_path"`
}

type PowderAmount struct {
	Spent     int `json:"spent"`
	Total     int `json:"total"`
	Available int `json:"available"`
}

type PowderOutput struct {
	Mithril  PowderAmount `json:"mithril"`
	Gemstone PowderAmount `json:"gemstone"`
	Glacite  PowderAmount `json:"glacite"`
}

type ForgeOutput struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slot         int     `json:"slot"`
	StartingTime int64   `json:"startingTime"`
	EndingTime   int64   `json:"endingTime"`
	Duration     float64 `json:"duration"`
}
