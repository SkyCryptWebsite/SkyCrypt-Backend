package lib

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
)

/*
func GetTexturePath(texturePath string, textureString string) string {
	textureId := textureString[strings.Index(textureString, "/")+1:]
	formattedPath := ""
	if texturePath == "Vanilla" {
		formattedPath = fmt.Sprintf("resourcepacks/%s/assets/%s", texturePath, textureId)
	} else {
		if after, ok := strings.CutPrefix(textureId, "firmskyblock:item"); ok {
			textureId = after
		}

		formattedPath = fmt.Sprintf("resourcepacks/%s/assets/cittofirmgenerated/textures/item/%s.png", texturePath, textureId)
	}

	return fmt.Sprintf("%s/assets/%s", utility.GetDomain(), formattedPath)
}

var regexCache sync.Map

func matchString(pattern, s string) (bool, error) {
	if cached, ok := regexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp).MatchString(s), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	regexCache.Store(pattern, re)
	return re.MatchString(s), nil
}
*/

func GetTexture(item models.TextureItem, disabledPacksParam ...[]string) *AppliedItemTexture {
	textures := ITEM_MAP[strings.ToLower(item.Tag.ExtraAttributes["id"].(string))]
	if len(textures) == 0 {
		return nil
	}

	disabledPacks := map[string]struct{}{}
	if len(disabledPacksParam) > 0 {
		for _, packID := range disabledPacksParam[0] {
			disabledPacks[packID] = struct{}{}
		}
	}

	for _, texture := range textures {
		if _, disabled := disabledPacks[texture.PackId]; disabled {
			continue
		}

		// TODO: Item matching?
		if texture.Path != "" {
			return &AppliedItemTexture{
				Texture:     texture.Path,
				TexturePack: texture.PackId,
			}
		}
	}

	return nil
}

var VANILLA_ITEM_MAP = map[string]string{}
var ITEM_MAP = map[string][]models.FormattedTexture{}

func init() {
	if os.Getenv("FIBER_PREFORK_CHILD") != "" {
		return
	}

	assetsRoot := "assets/resourcepacks"
	packDirs, err := os.ReadDir(assetsRoot)
	if err != nil {
		fmt.Printf("Failed to read assets directory: %v\n", err)
		return
	}

	for _, packDir := range packDirs {
		if packDir.Name() == "Vanilla" {
			err := filepath.WalkDir(filepath.Join(assetsRoot, packDir.Name(), "assets"), func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}

				fileName := filepath.Base(path)
				textureId := strings.Replace(fileName, ".webp", "", 1)

				formattedPath := fmt.Sprintf("resourcepacks/%s/assets/%s", packDir.Name(), textureId)
				formattedUrl := fmt.Sprintf("%s/assets/%s", utility.GetDomain(), formattedPath)
				VANILLA_ITEM_MAP[textureId] = formattedUrl
				return nil
			})

			if err != nil {
				fmt.Printf("Failed to process Vanilla pack: %v\n", err)
			}
			continue
		}

		config, err := GetPackConfig(assetsRoot, packDir)
		if err != nil {
			fmt.Printf("Failed to get config for pack %s: %v\n", packDir.Name(), err)
			continue
		}

		relativePath := filepath.Join(assetsRoot, packDir.Name())
		for _, category := range config.FabricOverlays.Entries {
			joinedPath := filepath.Join(relativePath, category.Directory)
			if _, err := os.Stat(joinedPath); os.IsNotExist(err) {
				// fmt.Printf("Directory not found for %s category, skipping\n", category.Directory)
				continue
			}

			fmt.Printf("Path: %s %s\n", joinedPath, category.Directory)
			err := filepath.WalkDir(joinedPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}

				fmt.Printf("Found %s\n", path)
				return nil
			})

			if err != nil {
				fmt.Printf("Failed to process category %s in pack %s: %v\n", category.Directory, packDir.Name(), err)
			}

		}

		/*
			packAssetsPath := filepath.Join(assetsRoot, packDir.Name(), "assets")
			if _, err := os.Stat(packAssetsPath); os.IsNotExist(err) {
				fmt.Printf("No assets directory found for pack %s, skipping\n", packDir.Name())
				continue
			}*/

		/*
			_ = filepath.WalkDir(packAssetsPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}

				// Special handling for Vanilla pack - we want to add all textures as fallback options, even if they don't have a corresponding JSON model
				if packDir.Name() == "Vanilla" {
					fileName := filepath.Base(path)
					textureId := strings.Replace(fileName, ".webp", "", 1)

					formattedPath := fmt.Sprintf("resourcepacks/%s/assets/%s", packDir.Name(), textureId)
					VANILLA_ITEM_MAP[textureId] = fmt.Sprintf("%s/assets/%s", utility.GetDomain(), formattedPath)
					return nil
				} else {
					return nil
				}
					if !strings.HasSuffix(path, ".json") {
						return nil
					}

					data, err := os.ReadFile(path)
					if err != nil {
						fmt.Printf("Failed to read %s: %v\n", path, err)
						return nil
					}

					model := models.CatharsisFormat{}
					if err := json.Unmarshal(data, &model); err != nil {
						fmt.Printf("Failed to parse %s: %v\n", path, err)
						return nil
					}

					if model.Model.Path == "" {
						// fmt.Printf("No model path found in %s, skipping\n", path)
						return nil
					}

					if strings.HasPrefix(model.Model.Path, "item/") {
						return nil
					}

					formattedTexture := models.FormattedTexture{
						Path:   ResolveTexturePath(filepath.Join(assetsRoot, packDir.Name(), "assets"), path, strings.Split(model.Model.Path, ":")[1]),
						PackId: config.CatharsisPackV1.ID,
					}

					fileName := filepath.Base(path)
					itemName := fileName[:len(fileName)-len(filepath.Ext(fileName))]
					if _, exists := ITEM_MAP[itemName]; !exists {
						ITEM_MAP[itemName] = []models.FormattedTexture{}
					}

					ITEM_MAP[itemName] = append(ITEM_MAP[itemName], formattedTexture)
					return nil

			})
		*/
	}
}

type AppliedItemTexture struct {
	Texture     string
	TexturePack string
}

func ApplyTexture(item models.TextureItem, disabledPacksParam ...[]string) AppliedItemTexture {
	// ? NOTE: we're ignoring enchanted books because they're quite expensive to render and not really worth the performance hit
	if item.Tag.ExtraAttributes == nil || item.Tag.ExtraAttributes["id"] == "ENCHANTED_BOOK" {
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/assets/resourcepacks/Vanilla/assets/enchanted_book.webp", utility.GetDomain())}
	}

	disabledPacks := []string{}
	if len(disabledPacksParam) > 0 {
		disabledPacks = disabledPacksParam[0]
	}

	customTexture := GetTexture(item, disabledPacks)
	if customTexture != nil {
		if !strings.Contains(customTexture.Texture, "Vanilla") && !strings.Contains(customTexture.Texture, "skull") {
			return *customTexture
		}
	}

	if item.Tag.SkullOwner != nil && len(item.Tag.SkullOwner.Properties.Textures) > 0 && item.Tag.SkullOwner.Properties.Textures[0].Value != "" {
		skinHash := utility.GetSkinHash(item.Tag.SkullOwner.Properties.Textures[0].Value)
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/head/%s", utility.GetDomain(), skinHash)}
	}

	// Preparsed texture from /api/item endpoint
	if item.Texture != "" {
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/head/%s", utility.GetDomain(), item.Texture)}
	}

	if *item.ID >= 298 && *item.ID <= 301 {
		armorType := constants.ARMOR_TYPES[*item.ID-298]

		armorColor := fmt.Sprintf("%06X", item.Tag.Display.Color)
		if item.Tag.ExtraAttributes["dye_item"] != "" {
			if !utility.IsArmorHexColorsEnabled() {
				idStr, ok := item.Tag.ExtraAttributes["id"].(string)
				if ok {
					defaultHexColor := constants.ITEMS[idStr].Color
					if defaultHexColor != "" && defaultHexColor != "FFFFFF" && defaultHexColor != "000000" {
						armorColor = defaultHexColor
					}
				}
			}
		}

		return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/leather/%s/%s", utility.GetDomain(), armorType, armorColor)}
	}

	textureId := constants.GetVanillaItemId(constants.ItemModel{
		NumericId:  *item.ID,
		ItemDamage: *item.Damage,
	})

	if texture, ok := VANILLA_ITEM_MAP[textureId]; ok {
		return AppliedItemTexture{Texture: texture}
	}

	vanillaPath := fmt.Sprintf("assets/resourcepacks/Vanilla/assets/%s.webp", strings.ToLower(item.RawId))
	if _, err := os.Stat(vanillaPath); err == nil {
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/%s", utility.GetDomain(), vanillaPath)}
	}

	fmt.Printf("[CUSTOM_RESOURCES] No custom texture found for item %s, returning default barrier texture\n", item.Tag.ExtraAttributes["id"])
	return AppliedItemTexture{Texture: fmt.Sprintf("%s/assets/resourcepacks/Vanilla/assets/barrier.webp", utility.GetDomain())}
}

func EnsureWebPExt(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".webp") {
		return path
	}

	if strings.HasSuffix(lower, ".png") {
		return strings.TrimSuffix(path, path[len(path)-4:]) + ".webp"
	}

	return path + ".webp"
}

func ResolveTexturePath(inputFolder, modelPath, textureRef string) string {
	textureRef = strings.TrimSpace(textureRef)
	if textureRef == "" {
		return ""
	}

	sep := string(filepath.Separator)
	addCandidate := func(candidates *[]string, relPath string) {
		if relPath == "" {
			return
		}

		normalized := filepath.Clean(filepath.FromSlash(relPath))
		if filepath.IsAbs(normalized) {
			*candidates = append(*candidates, EnsureWebPExt(normalized))
			return
		}

		*candidates = append(*candidates, EnsureWebPExt(filepath.Join(inputFolder, normalized)))
	}

	candidates := make([]string, 0, 6)
	addCandidate(&candidates, textureRef)
	addCandidate(&candidates, filepath.Join(filepath.Dir(modelPath), textureRef))

	if !strings.HasPrefix(filepath.FromSlash(textureRef), "assets"+sep) {
		namespace, name, hasNamespace := strings.Cut(textureRef, ":")
		if hasNamespace {
			addCandidate(&candidates, filepath.Join("assets", namespace, "textures", name))

			modelPathSlash := filepath.ToSlash(filepath.Clean(modelPath))
			if modelRoot, _, found := strings.Cut(modelPathSlash, "/models/"); found && strings.HasSuffix(modelRoot, "/"+namespace) {
				addCandidate(&candidates, filepath.Join(filepath.FromSlash(modelRoot), "textures", name))
			}

			addCandidate(&candidates, filepath.Join("assets", namespace, "assets", namespace, "textures", name))
		} else {
			addCandidate(&candidates, filepath.Join("assets", "minecraft", "textures", textureRef))
		}
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if len(candidates) > 0 {
		return candidates[0]
	}

	return EnsureWebPExt(filepath.Join(filepath.Dir(modelPath), textureRef))
}

func GetPackConfig(assetsRoot string, packDir os.DirEntry) (*models.PackConfig, error) {
	configPath := filepath.Join(assetsRoot, packDir.Name(), "pack.mcmeta")
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("pack.mcmeta not found for pack %s", packDir.Name())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pack.mcmeta for pack %s", packDir.Name())
	}

	var config models.PackConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse pack.mcmeta for pack %s, err: %v", packDir.Name(), err)
	}

	if config.Pack.Disabled {
		return nil, fmt.Errorf("resource pack %s is disabled", packDir.Name())
	}

	return &config, nil
}
