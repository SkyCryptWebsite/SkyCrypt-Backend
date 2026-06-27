package data

import (
	"image/color"
	"strings"
)

type BiomeTintConfiguration struct {
	GrassTextures      map[string]struct{}
	GrassBlocks        map[string]struct{}
	FoliageTextures    map[string]struct{}
	FoliageBlocks      map[string]struct{}
	DryFoliageTextures map[string]struct{}
	DryFoliageBlocks   map[string]struct{}
	ItemTintExclusions map[string]struct{}
	ConstantColors     map[string]color.RGBA
}

func NewBiomeTintConfiguration(
	grassTextures, grassBlocks, foliageTextures, foliageBlocks, dryFoliageTextures, dryFoliageBlocks, itemTintExclusions map[string]struct{},
	constantColors map[string]color.RGBA) *BiomeTintConfiguration {
	return &BiomeTintConfiguration{
		GrassTextures:      grassTextures,
		GrassBlocks:        grassBlocks,
		FoliageTextures:    foliageTextures,
		FoliageBlocks:      foliageBlocks,
		DryFoliageTextures: dryFoliageTextures,
		DryFoliageBlocks:   dryFoliageBlocks,
		ItemTintExclusions: itemTintExclusions,
		ConstantColors:     constantColors,
	}
}

func LoadDefault() *BiomeTintConfiguration {
	return NewBiomeTintConfiguration(
		createSet(GrassTextureKeys),
		createSet(GrassBlockKeys),
		createSet(FoliageTextureKeys),
		createSet(FoliageBlockKeys),
		createSet(DryFoliageTextureKeys),
		createSet(DryFoliageBlockKeys),
		createSet(ItemTintExclusionKeys),
		createColorMap(),
	)
}

func createSet(values []string) map[string]struct{} {
	set := make(map[string]struct{})
	if values == nil {
		return set
	}

	for _, v := range values {
		normalized := strings.TrimSpace(v)
		if normalized == "" {
			continue
		}
		set[strings.ToLower(normalized)] = struct{}{}
	}

	return set
}

func createColorMap() map[string]color.RGBA {
	result := make(map[string]color.RGBA)
	for _, entry := range ConstantColorEntries {
		key := strings.TrimSpace(entry.Key)
		if key == "" {
			continue
		}
		normalized := strings.ToLower(key)
		result[normalized] = color.RGBA{R: entry.R, G: entry.G, B: entry.B, A: 255}
	}
	return result
}

var GrassTextureKeys = []string{
	"grass",
	"tall_grass",
	"short_grass",
	"large_fern",
	"fern",
	"grass_block_top",
	"grass_block_side_overlay",
	"grass_block_snow",
	"hanging_roots",
	"pale_hanging_moss",
	"pale_hanging_moss_tip",
	"moss",
	"moss_block",
	"moss_carpet",
	"pale_moss_block",
	"pale_moss_carpet",
	"sugar_cane",
	"cattail",
	"kelp",
	"kelp_top",
	"kelp_plant",
	"seagrass",
	"seagrass_top",
	"tall_seagrass_top",
	"sea_grass",
}

var GrassBlockKeys = []string{
	"grass_block",
	"grass",
	"tall_grass",
	"short_grass",
	"large_fern",
	"fern",
	"hanging_roots",
	"pale_hanging_moss",
	"pale_hanging_moss_tip",
	"moss_block",
	"moss_carpet",
	"pale_moss_block",
	"pale_moss_carpet",
	"seagrass",
	"tall_seagrass",
	"kelp",
	"kelp_plant",
	"sugar_cane",
	"cattail",
	"potted_fern",
}

var FoliageTextureKeys = []string{
	"oak_leaves",
	"spruce_leaves",
	"birch_leaves",
	"jungle_leaves",
	"acacia_leaves",
	"dark_oak_leaves",
	"mangrove_leaves",
	"pale_oak_leaves",
	"azalea_leaves",
	"flowering_azalea_leaves",
	"vine",
	"cave_vines",
	"cave_vines_body",
	"cave_vines_body_lit",
	"cave_vines_head",
	"cave_vines_head_lit",
	"cave_vines_lit",
	"cave_vines_plant",
	"cave_vines_plant_lit",
	"oak_sapling",
	"spruce_sapling",
	"birch_sapling",
	"jungle_sapling",
	"acacia_sapling",
	"dark_oak_sapling",
	"mangrove_propagule",
	"pale_oak_sapling",
	"azalea",
	"flowering_azalea",
	"big_dripleaf_top",
	"big_dripleaf_stem",
	"big_dripleaf_stem_bottom",
	"big_dripleaf_stem_mid",
	"small_dripleaf_top",
	"small_dripleaf_stem",
	"small_dripleaf_stem_top",
}

var FoliageBlockKeys = []string{
	"oak_leaves",
	"spruce_leaves",
	"birch_leaves",
	"jungle_leaves",
	"acacia_leaves",
	"dark_oak_leaves",
	"mangrove_leaves",
	"pale_oak_leaves",
	"azalea_leaves",
	"flowering_azalea_leaves",
	"vine",
	"cave_vines",
	"cave_vines_plant",
	"cave_vines_lit",
	"cave_vines_plant_lit",
	"oak_sapling",
	"spruce_sapling",
	"birch_sapling",
	"jungle_sapling",
	"acacia_sapling",
	"dark_oak_sapling",
	"mangrove_propagule",
	"pale_oak_sapling",
	"azalea",
	"flowering_azalea",
	"big_dripleaf",
	"big_dripleaf_stem",
	"small_dripleaf",
	"small_dripleaf_stem",
	"potted_oak_sapling",
	"potted_spruce_sapling",
	"potted_birch_sapling",
	"potted_jungle_sapling",
	"potted_acacia_sapling",
	"potted_dark_oak_sapling",
	"potted_mangrove_propagule",
	"potted_pale_oak_sapling",
	"potted_azalea_bush",
	"potted_flowering_azalea_bush",
}

var DryFoliageTextureKeys = []string{
	"dead_bush",
	"leaf_litter",
	"leaf_litter_1",
	"leaf_litter_2",
	"leaf_litter_3",
	"leaf_litter_4",
	"short_dry_grass",
	"tall_dry_grass",
}

var DryFoliageBlockKeys = []string{
	"dead_bush",
	"leaf_litter",
	"leaf_litter_1",
	"leaf_litter_2",
	"leaf_litter_3",
	"leaf_litter_4",
	"short_dry_grass",
	"tall_dry_grass",
	"potted_dead_bush",
}

var ItemTintExclusionKeys = []string{
	"oak_sapling",
	"spruce_sapling",
	"birch_sapling",
	"jungle_sapling",
	"acacia_sapling",
	"dark_oak_sapling",
	"mangrove_propagule",
	"pale_oak_sapling",
	"azalea",
	"flowering_azalea",
	"cherry_sapling",
}

var ConstantColorEntries = []struct {
	Key     string
	R, G, B uint8
}{
	{"birch_leaves", 128, 167, 85},
	{"spruce_leaves", 97, 153, 97},
	{"lily_pad", 32, 128, 48},
}
