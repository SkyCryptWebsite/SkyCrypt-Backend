package neu

type SacksRaw struct {
	Sacks map[string]SackRaw `json:"sacks"`
}

type SackRaw struct {
	SackItemId string   `json:"item"`
	Contents   []string `json:"contents"`
}

type FormattedSack map[string][]string
