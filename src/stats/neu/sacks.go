package neustats

import (
	neu "skycrypt/src/models/NEU"
	"strings"
)

func formatSackContents(contents []string) []string {
	formatted := make([]string, len(contents))
	for i, item := range contents {
		formatted[i] = strings.Replace(item, "-", ":", 1) // SAND-1 -> SAND:1 (NEU internal id)
	}

	return formatted
}

func FormatSacks(rawData neu.SacksRaw) neu.FormattedSack {
	formatted := make(neu.FormattedSack)
	for _, sack := range rawData.Sacks {
		formatted[sack.SackItemId] = formatSackContents(sack.Contents)
	}

	return formatted
}
