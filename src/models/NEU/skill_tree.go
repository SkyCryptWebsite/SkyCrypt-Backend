package neu

import "encoding/json"

type HOTMConstants struct {
	Prelude []string      `json:"prelude"`
	Hotm    SkillTreeTree `json:"hotm"`
}

type HOTFConstants struct {
	Prelude []string      `json:"prelude"`
	Hotf    SkillTreeTree `json:"hotf"`
}

type SkillTreeTree struct {
	Powders map[string]SkillTreePowder `json:"powders"`
	Perks   map[string]SkillTreePerk   `json:"perks"`
}

type SkillTreePowder struct {
	CostLine string `json:"costLine"`
}

type SkillTreePerk struct {
	Name     string               `json:"name"`
	X        int                  `json:"x"`
	Y        int                  `json:"y"`
	MaxLevel int                  `json:"maxLevel"`
	Powder   string               `json:"powder"`
	Item     string               `json:"item"`
	Cost     string               `json:"cost"`
	Lore     []SkillTreeLoreEntry `json:"lore"`
	Keys     map[string]string    `json:"-"`
}

var skillTreePerkKnownKeys = map[string]bool{
	"name": true, "x": true, "y": true, "maxLevel": true,
	"powder": true, "item": true, "cost": true, "lore": true,
}

func (p *SkillTreePerk) UnmarshalJSON(data []byte) error {
	type Alias SkillTreePerk
	aux := &struct{ *Alias }{Alias: (*Alias)(p)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Keys = make(map[string]string)
	for k, v := range raw {
		if skillTreePerkKnownKeys[k] {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			p.Keys[k] = s
		}
	}
	return nil
}

type SkillTreeLoreEntry struct {
	Text   string `json:"text"`
	OnlyIf string `json:"onlyIf,omitempty"`
}

func (e *SkillTreeLoreEntry) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.Text = s
		return nil
	}

	type Alias SkillTreeLoreEntry
	var obj Alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*e = SkillTreeLoreEntry(obj)
	return nil
}

func (e SkillTreeLoreEntry) MarshalJSON() ([]byte, error) {
	if e.OnlyIf == "" {
		return json.Marshal(e.Text)
	}
	type Alias SkillTreeLoreEntry
	return json.Marshal(Alias(e))
}
