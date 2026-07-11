package stats

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

const itemProcessingSampleLimit = 50

type itemProcessingDebugMode int

const (
	itemProcessingDebugOff itemProcessingDebugMode = iota
	itemProcessingDebugSummary
	itemProcessingDebugDetail
	itemProcessingDebugTrace
)

type itemProcessingStage string

const (
	itemProcessingStageRawLore      itemProcessingStage = "raw_lore"
	itemProcessingStageTypeParse    itemProcessingStage = "type_parse"
	itemProcessingStageExtra        itemProcessingStage = "extra"
	itemProcessingStageWiki         itemProcessingStage = "wiki"
	itemProcessingStageTextureInput itemProcessingStage = "texture_input"
	itemProcessingStageNested       itemProcessingStage = "nested"
	itemProcessingStageValueLore    itemProcessingStage = "value_lore"
	itemProcessingStageStrip        itemProcessingStage = "strip"
)

type ItemProcessingSample struct {
	Source        string
	SkyBlockID    string
	MinecraftID   string
	ItemModel     string
	DisplayName   string
	Depth         int
	ContainsItems int
	Duration      time.Duration
}

type ItemProcessingStats struct {
	Route        string
	UUID         string
	ProfileID    string
	EnabledPacks []string
	Started      time.Time

	TotalItems     int
	TopLevelItems  int
	NestedItems    int
	ContainerItems int
	SourceCounts   map[string]int

	RawLoreDuration      time.Duration
	TypeParseDuration    time.Duration
	ExtraDuration        time.Duration
	NEUWikiDuration      time.Duration
	TextureInputDuration time.Duration
	NestedDuration       time.Duration
	ValueLoreDuration    time.Duration
	StripDuration        time.Duration

	NEUWikiCacheHits   int
	NEUWikiCacheMisses int

	SlowItems []ItemProcessingSample
}

func NewItemProcessingStats(route, uuid, profileID string, enabledPacks []string) *ItemProcessingStats {
	return &ItemProcessingStats{
		Route:        route,
		UUID:         uuid,
		ProfileID:    profileID,
		EnabledPacks: append([]string(nil), enabledPacks...),
		Started:      time.Now(),
		SourceCounts: map[string]int{},
		SlowItems:    []ItemProcessingSample{},
	}
}

func ItemProcessingDebugEnabled() bool {
	mode, _, _ := itemProcessingDebugConfig()
	return mode != itemProcessingDebugOff
}

func (s *ItemProcessingStats) ensure() {
	if s == nil {
		return
	}
	if s.Started.IsZero() {
		s.Started = time.Now()
	}
	if s.SourceCounts == nil {
		s.SourceCounts = map[string]int{}
	}
}

func (s *ItemProcessingStats) RecordStripDuration(duration time.Duration) {
	if s == nil {
		return
	}
	s.StripDuration += duration
}

func (s *ItemProcessingStats) recordItemStart(source string, item *skycrypttypes.Item, depth int) {
	if s == nil {
		return
	}
	s.ensure()
	s.TotalItems++
	if depth == 0 {
		s.TopLevelItems++
	} else {
		s.NestedItems++
	}
	if item != nil && item.ContainsItems != nil {
		s.ContainerItems++
	}
	s.SourceCounts[source]++
}

func (s *ItemProcessingStats) recordItemDuration(source string, item *skycrypttypes.Item, depth int, duration time.Duration) {
	if s == nil || item == nil {
		return
	}

	sample := ItemProcessingSample{
		Source:        source,
		SkyBlockID:    skyBlockIDFromStatsItem(item),
		MinecraftID:   minecraftIDFromStatsItem(item),
		ItemModel:     itemModelFromStatsItem(item),
		DisplayName:   displayNameFromStatsItem(item),
		Depth:         depth,
		ContainsItems: len(item.ContainsItems),
		Duration:      duration,
	}
	if len(s.SlowItems) < itemProcessingSampleLimit {
		s.SlowItems = append(s.SlowItems, sample)
		return
	}

	shortestIndex := 0
	for index := 1; index < len(s.SlowItems); index++ {
		if s.SlowItems[index].Duration < s.SlowItems[shortestIndex].Duration {
			shortestIndex = index
		}
	}
	if duration > s.SlowItems[shortestIndex].Duration {
		s.SlowItems[shortestIndex] = sample
	}
}

func (s *ItemProcessingStats) recordStageDuration(stage itemProcessingStage, duration time.Duration) {
	if s == nil {
		return
	}
	switch stage {
	case itemProcessingStageRawLore:
		s.RawLoreDuration += duration
	case itemProcessingStageTypeParse:
		s.TypeParseDuration += duration
	case itemProcessingStageExtra:
		s.ExtraDuration += duration
	case itemProcessingStageWiki:
		s.NEUWikiDuration += duration
	case itemProcessingStageTextureInput:
		s.TextureInputDuration += duration
	case itemProcessingStageNested:
		s.NestedDuration += duration
	case itemProcessingStageValueLore:
		s.ValueLoreDuration += duration
	case itemProcessingStageStrip:
		s.StripDuration += duration
	}
}

func (s *ItemProcessingStats) recordNEUWikiLookup(cacheHit bool, duration time.Duration) {
	if s == nil {
		return
	}
	if cacheHit {
		s.NEUWikiCacheHits++
	} else {
		s.NEUWikiCacheMisses++
	}
	s.recordStageDuration(itemProcessingStageWiki, duration)
}

func (s *ItemProcessingStats) LogIfEnabled(duration time.Duration) bool {
	return s.WriteDebugSummary(os.Stdout, duration)
}

func (s *ItemProcessingStats) WriteDebugSummary(writer io.Writer, duration time.Duration) bool {
	if s == nil || writer == nil {
		return false
	}
	s.ensure()
	mode, threshold, topN := itemProcessingDebugConfig()
	if mode == itemProcessingDebugOff {
		return false
	}
	if duration <= 0 {
		duration = time.Since(s.Started)
	}
	if threshold > 0 && duration < threshold {
		return false
	}

	perItem := time.Duration(0)
	if s.TotalItems > 0 {
		perItem = time.Duration(int64(duration) / int64(s.TotalItems))
	}
	stageTotal := s.RawLoreDuration + s.TypeParseDuration + s.ExtraDuration + s.NEUWikiDuration + s.TextureInputDuration + s.NestedDuration + s.ValueLoreDuration + s.StripDuration
	unattributed := duration - stageTotal
	if unattributed < 0 {
		unattributed = 0
	}

	if _, err := fmt.Fprintf(
		writer,
		"[ITEM_PROCESSING_DEBUG] route=%s uuid=%s profile=%s pid=%d duration=%s per_item=%s items=%d top_level=%d nested=%d containers=%d sources=%s enabled_packs=%s stages raw_lore=%s type_parse=%s extra=%s wiki=%s texture_input=%s nested=%s value_lore=%s strip=%s unattributed=%s neu_wiki_cache_hits=%d neu_wiki_cache_misses=%d\n",
		emptyDebugValue(s.Route),
		emptyDebugValue(s.UUID),
		emptyDebugValue(s.ProfileID),
		os.Getpid(),
		duration,
		perItem,
		s.TotalItems,
		s.TopLevelItems,
		s.NestedItems,
		s.ContainerItems,
		sourceCountsDebugString(s.SourceCounts),
		debugStringList(s.EnabledPacks),
		s.RawLoreDuration,
		s.TypeParseDuration,
		s.ExtraDuration,
		s.NEUWikiDuration,
		s.TextureInputDuration,
		s.NestedDuration,
		s.ValueLoreDuration,
		s.StripDuration,
		unattributed,
		s.NEUWikiCacheHits,
		s.NEUWikiCacheMisses,
	); err != nil {
		return false
	}

	if mode >= itemProcessingDebugDetail {
		if err := writeSlowItemSamples(writer, s.SlowItems, topN); err != nil {
			return false
		}
	}

	return true
}

func itemProcessingDebugConfig() (itemProcessingDebugMode, time.Duration, int) {
	mode := itemProcessingDebugOff
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ITEM_PROCESSING_DEBUG"))) {
	case "summary":
		mode = itemProcessingDebugSummary
	case "detail":
		mode = itemProcessingDebugDetail
	case "trace":
		mode = itemProcessingDebugTrace
	case "1", "true", "yes", "on":
		mode = itemProcessingDebugSummary
	}

	threshold := 250 * time.Millisecond
	if raw := strings.TrimSpace(os.Getenv("ITEM_PROCESSING_DEBUG_SLOW_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			threshold = time.Duration(parsed) * time.Millisecond
		}
	}

	topN := 10
	if raw := strings.TrimSpace(os.Getenv("ITEM_PROCESSING_DEBUG_TOP_N")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			topN = parsed
		}
	}

	return mode, threshold, topN
}

func writeSlowItemSamples(writer io.Writer, samples []ItemProcessingSample, topN int) error {
	sorted := append([]ItemProcessingSample(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Duration > sorted[j].Duration
	})
	if len(sorted) > topN {
		sorted = sorted[:topN]
	}
	for index, sample := range sorted {
		if _, err := fmt.Fprintf(
			writer,
			"[ITEM_PROCESSING_DEBUG] slow_item rank=%d source=%s skyblock_id=%q minecraft_id=%q item_model=%q name=%q depth=%d contains_items=%d duration=%s\n",
			index+1,
			emptyDebugValue(sample.Source),
			sample.SkyBlockID,
			sample.MinecraftID,
			sample.ItemModel,
			sample.DisplayName,
			sample.Depth,
			sample.ContainsItems,
			sample.Duration,
		); err != nil {
			return err
		}
	}
	return nil
}

func sourceCountsDebugString(sourceCounts map[string]int) string {
	if len(sourceCounts) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(sourceCounts))
	for key := range sourceCounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, sourceCounts[key]))
	}
	return strings.Join(parts, ",")
}

func debugStringList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	cloned := append([]string(nil), values...)
	sort.Strings(cloned)
	return strings.Join(cloned, ",")
}

func emptyDebugValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func skyBlockIDFromStatsItem(item *skycrypttypes.Item) string {
	if item == nil || item.Tag == nil || item.Tag.ExtraAttributes == nil {
		return ""
	}
	return strings.TrimSpace(item.Tag.ExtraAttributes.Id)
}

func minecraftIDFromStatsItem(item *skycrypttypes.Item) string {
	if item == nil || item.ID == nil {
		return ""
	}
	return strconv.Itoa(*item.ID)
}

func itemModelFromStatsItem(item *skycrypttypes.Item) string {
	if item == nil || item.Tag == nil {
		return ""
	}
	return strings.TrimSpace(item.Tag.ItemModel)
}

func displayNameFromStatsItem(item *skycrypttypes.Item) string {
	if item == nil || item.Tag == nil {
		return ""
	}
	return strings.TrimSpace(item.Tag.Display.Name)
}
