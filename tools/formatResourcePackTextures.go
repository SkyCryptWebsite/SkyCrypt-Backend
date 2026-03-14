package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"path/filepath"
	"skycrypt/src/models"
	"sort"
	"strconv"
	"strings"

	"github.com/HugoSmits86/nativewebp"
)

type BlacklistedProperty struct {
	isBlacklisted func(path string) bool
}

var BLACKLISTED_PATHS = map[string]BlacklistedProperty{
	"HYPIXEL_PLUS": {
		isBlacklisted: func(path string) bool {
			// We do not support dyed armor textures at the moment
			// TODO: Implement this into /api/leather endpoint and remove this blacklist
			if strings.Contains(path, "assets/hplus/models/skyblock/armor") && strings.Contains(path, "dyed") {
				return true
			}

			if strings.Contains(path, "assets/hplus/textures/skyblock/tools/abiphones/") {
				return true
			}

			return false
		},
	},
	"FURFSKY_REBORN": {
		isBlacklisted: func(parent string) bool {
			return false
		},
	},
}

var BLACKLISTED_PARENTS = map[string]BlacklistedProperty{
	"HYPIXEL_PLUS": {
		isBlacklisted: func(parent string) bool {
			// We do not support blockstate models at the moment
			if parent == "hplus:item/block" {
				return true
			}

			return false
		},
	},
	"FURFSKY_REBORN": {
		isBlacklisted: func(parent string) bool {
			return false
		},
	},
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run tools/formatResourcePackTextures.go <input_folder> <\"format|convert\">")
		return
	}

	inputFolder := os.Args[1]
	fmt.Printf("Formatting textures in %s\n", inputFolder)

	configPath := filepath.Join(inputFolder, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Failed to read config.json: %v\n", err)
		return
	}

	var config models.ResourcePackConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		fmt.Printf("Failed to parse config.json: %v\n", err)
		return
	}

	function := os.Args[2]
	switch function {
	case "formatImages":
		FormatVanillaResourcePackImages(inputFolder, config)
	case "formatJSON":
		FormatVanillaResourcePackJSON(inputFolder, config)
	default:
		fmt.Printf("Unknown function: %s\n", function)
	}
}

func FormatVanillaResourcePackJSON(inputFolder string, config models.ResourcePackConfig) {
	formattedTextures, formattedAnimatedTextures := 0, 0
	err := filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// fmt.Printf("Warning: skipped missing path %s: %v\n", path, err)
				return nil
			}
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			jsonData, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("Failed to read %s: %v\n", path, err)
				return nil
			}

			var itemTexture models.ItemTexture
			if err := json.Unmarshal(jsonData, &itemTexture); err != nil {
				fmt.Printf("Failed to parse %s: %v\n", path, err)
				return nil
			}

			if BLACKLISTED_PARENTS[config.Id].isBlacklisted(itemTexture.Parent) {
				if err := os.Remove(path); err != nil {
					fmt.Printf("Failed to delete blacklisted model %s: %v\n", path, err)
				} else {
					_ = 0
					fmt.Printf("Deleted blacklisted model due to parent: %s\n", path)
				}

				for _, tex := range itemTexture.Textures {
					texPath := resolveTexturePath(inputFolder, path, tex)
					if _, err := os.Stat(texPath); err == nil {
						if err := os.Remove(texPath); err != nil {
							fmt.Printf("Failed to delete associated texture %s: %v\n", texPath, err)
						} else {
							fmt.Printf("Deleted associated texture due to blacklisted parent: %s\n", texPath)
							_ = 0
						}
					}
				}

				return nil
			}

			if BLACKLISTED_PATHS[config.Id].isBlacklisted(path) {
				if err := os.Remove(path); err != nil {
					fmt.Printf("Failed to delete blacklisted model %s: %v\n", path, err)
				} else {
					_ = 0
					fmt.Printf("Deleted blacklisted model because of the path: %s\n", path)
				}

				for _, tex := range itemTexture.Textures {
					texPath := resolveTexturePath(inputFolder, path, tex)
					if _, err := os.Stat(texPath); err == nil {
						if err := os.Remove(texPath); err != nil {
							fmt.Printf("Failed to delete associated texture %s: %v\n", texPath, err)
						} else {
							fmt.Printf("Deleted associated texture due to blacklisted path: %s\n", texPath)
							_ = 0
						}
					}
				}

				return nil
			}

			if len(itemTexture.Elements) > 0 || len(itemTexture.Display) > 0 {
				if strings.HasPrefix(itemTexture.Parent, "builtin/") {
					if err := os.Remove(path); err != nil {
						fmt.Printf("Failed to delete blockstate model %s: %v\n", path, err)
					} else {
						_ = 0
						fmt.Printf("Deleted unsupported blockstate model: %s\n", path)
					}
					return nil
				}
			}

			if itemTexture.Parent == "item/template_skull" {
				if err := os.Remove(path); err != nil {
					fmt.Printf("Failed to delete custom skull model %s: %v\n", path, err)
				} else {
					_ = 0
					fmt.Printf("Deleted custom skull model: %s\n", path)
				}
				return nil
			}

			// loop thru itemTexture.Textures and if it has #0 remove it
			textures := map[string]string{}
			for key, tex := range textures {
				if tex == "" || tex == "#0" {
					continue
				}

				textures[key] = tex
			}

			if len(textures) > 1 {
				dedupedTextures, hadDuplicates := dedupeTextureLayers(textures)
				if hadDuplicates {
					// newJsonData, err := json.MarshalIndent(itemTexture, "", "  ")
					// if err != nil {
					// 	fmt.Printf("Failed to marshal deduplicated JSON for %s: %v\n", path, err)
					// 	return nil
					// }

					// if err := os.WriteFile(path, newJsonData, 0644); err != nil {
					// 	fmt.Printf("Failed to write deduplicated JSON to %s: %v\n", path, err)
					// 	return nil
					// }

					fmt.Printf("Deduplicated textures for %s, reduced from %d to %d layers\n", path, len(itemTexture.Textures), len(dedupedTextures))
				}

				if len(dedupedTextures) > 1 {
					// fmt.Printf("Found %s with %d textures %s\n", info.Name(), len(itemTexture.Textures), path)

					// orderedTextureKeys := sortedTextureLayerKeys(itemTexture.Textures)

					var baseImg image.Image
					baseFromVanilla := false
					for _, tex := range dedupedTextures {
						if strings.HasSuffix(tex, "_model") {
							continue
						}

						texPath := resolveTexturePath(inputFolder, path, tex)

						textureFromVanilla := false
						if _, err := os.Stat(texPath); os.IsNotExist(err) {
							resolvedFallback := false
							for _, prefix := range []string{"item/", "block/"} {
								after, ok := strings.CutPrefix(tex, prefix)
								if !ok {
									continue
								}

								vanillaTexPath := filepath.Join(filepath.Dir(filepath.Clean(inputFolder)), "Vanilla", "assets", ensureWebPExt(after))
								if _, err := os.Stat(vanillaTexPath); os.IsNotExist(err) {
									continue
								}

								texPath = vanillaTexPath
								textureFromVanilla = true
								resolvedFallback = true
								break
							}

							if !resolvedFallback {
								fmt.Printf("Texture %s not found in both %s and Vanilla assets fallback\n", tex, texPath)
								continue
							}
						}

						texFile, err := os.Open(texPath)
						if err != nil {
							fmt.Printf("Failed to open texture %s: %v\n", texPath, err)
							continue
						}
						defer func() {
							_ = texFile.Close()
						}()

						texImg, _, err := image.Decode(texFile)
						if err != nil {
							fmt.Printf("Failed to decode texture %s: %v\n", texPath, err)
							continue
						}

						if baseImg == nil {
							baseImg = texImg
							baseFromVanilla = textureFromVanilla
						} else {
							if baseFromVanilla && !textureFromVanilla && (baseImg.Bounds().Dx() != texImg.Bounds().Dx() || baseImg.Bounds().Dy() != texImg.Bounds().Dy()) {
								baseImg = resizeImageNearest(baseImg, texImg.Bounds().Dx(), texImg.Bounds().Dy())
								baseFromVanilla = false
							}

							if textureFromVanilla && (texImg.Bounds().Dx() != baseImg.Bounds().Dx() || texImg.Bounds().Dy() != baseImg.Bounds().Dy()) {
								texImg = resizeImageNearest(texImg, baseImg.Bounds().Dx(), baseImg.Bounds().Dy())
							}

							combinedImg := image.NewNRGBA(baseImg.Bounds())
							draw.Draw(combinedImg, combinedImg.Bounds(), baseImg, baseImg.Bounds().Min, draw.Src)
							draw.Draw(combinedImg, combinedImg.Bounds(), texImg, texImg.Bounds().Min, draw.Over)
							baseImg = combinedImg
						}
					}

					if baseImg != nil {
						outPath, textureRef := buildTextureOutputForModel(inputFolder, path)
						if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
							fmt.Printf("Failed to create output directory for %s: %v\n", outPath, err)
							return nil
						}
						outFile, err := os.Create(outPath)
						if err != nil {
							fmt.Printf("Failed to create WebP %s: %v\n", outPath, err)
							return nil
						}
						defer func() {
							_ = outFile.Close()
						}()
						if err := nativewebp.Encode(outFile, baseImg, nil); err != nil {
							fmt.Printf("Failed to encode WebP %s: %v\n", outPath, err)
							return nil
						}

						fmt.Printf("Created combined WebP for %s: %s\n", outPath, path)

						itemTexture.Textures = map[string]string{
							"layer0": textureRef,
						}

						// newJsonData, err := json.MarshalIndent(itemTexture, "", "  ")
						// if err != nil {
						// 	fmt.Printf("Failed to marshal updated JSON for %s: %v\n", path, err)
						// 	return nil
						// }

						// if err := os.WriteFile(path, newJsonData, 0644); err != nil {
						// 	fmt.Printf("Failed to write updated JSON to %s: %v\n", path, err)
						// 	return nil
						// }

						formattedTextures++
					}
				}

			}

		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through folder while parsing JSONs: %v\n", err)
	}

	fmt.Printf("Finished formatting textures. Total formatted: %d (Animated: %d)\n", formattedTextures, formattedAnimatedTextures)
}

func FormatVanillaResourcePackImages(inputFolder string, config models.ResourcePackConfig) {
	formattedTextures, formattedAnimatedTextures := 0, 0
	err := filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// fmt.Printf("Warning: skipped missing path %s: %v\n", path, err)
				return nil
			}
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".png") {
			file, err := os.Open(path)
			if err != nil {
				fmt.Printf("Failed to open %s: %v\n", path, err)
				return nil
			}
			defer func() {
				_ = file.Close()
			}()

			img, _, err := image.Decode(file)
			if err != nil {
				fmt.Printf("Failed to decode %s: %v\n", path, err)
				return nil
			}

			width := img.Bounds().Dx()
			height := img.Bounds().Dy()
			if width != height && height%width == 0 {
				mcmetaPath := path + ".mcmeta"
				if _, err := os.Stat(mcmetaPath); os.IsNotExist(err) {
					return nil
				}

				mcmetaData, err := os.ReadFile(mcmetaPath)
				if err != nil {
					fmt.Printf("Failed to read %s: %v\n", mcmetaPath, err)
					return nil
				}

				var mcMeta models.McMeta
				if err := json.Unmarshal(mcmetaData, &mcMeta); err != nil {
					fmt.Printf("Failed to parse %s: %v\n", mcmetaPath, err)
					return nil
				}

				frameCount := height / width
				subImager, ok := img.(interface {
					SubImage(r image.Rectangle) image.Image
				})
				if !ok {
					fmt.Printf("Image type for %s does not support SubImage\n", path)
					return nil
				}

				frameTimeTicks := mcMeta.Animation.Frametime
				if frameTimeTicks <= 0 {
					frameTimeTicks = 1
				}
				delayMs := uint(frameTimeTicks * 50)

				frames := make([]image.Image, frameCount)
				durations := make([]uint, frameCount)
				disposals := make([]uint, frameCount)
				for i := 0; i < frameCount; i++ {
					frameRect := image.Rect(0, i*width, width, (i+1)*width)
					subFrame := subImager.SubImage(frameRect)
					normalizedFrame := image.NewNRGBA(image.Rect(0, 0, width, width))
					draw.Draw(normalizedFrame, normalizedFrame.Bounds(), subFrame, subFrame.Bounds().Min, draw.Src)
					frames[i] = normalizedFrame
					durations[i] = delayMs
					disposals[i] = 0
				}

				webpAnimation := nativewebp.Animation{
					Images:          frames,
					Durations:       durations,
					Disposals:       disposals,
					LoopCount:       0,
					BackgroundColor: 0x00000000,
				}

				outPath := strings.TrimSuffix(path, ".png") + ".webp"
				outFile, err := os.Create(outPath)
				if err != nil {
					fmt.Printf("Failed to create WebP %s: %v\n", outPath, err)
					return nil
				}
				defer func() {
					_ = outFile.Close()
				}()
				if err := nativewebp.EncodeAll(outFile, &webpAnimation, nil); err != nil {
					fmt.Printf("Failed to encode WebP %s: %v\n", outPath, err)
					return nil
				}

				// fmt.Printf("Created Animated WebP: %s\n", outPath)
				formattedTextures++
				formattedAnimatedTextures++
				if err := os.Remove(path); err != nil {
					fmt.Printf("Failed to delete original PNG %s: %v\n", path, err)
				}
				if err := os.Remove(mcmetaPath); err != nil {
					fmt.Printf("Failed to delete mcmeta %s: %v\n", mcmetaPath, err)
				}

				return nil
			}

			if !strings.HasSuffix(info.Name(), ".webp") {
				outPath := strings.TrimSuffix(path, ".png") + ".webp"
				outFile, err := os.Create(outPath)
				if err != nil {
					fmt.Printf("Failed to create WebP %s: %v\n", outPath, err)
					return nil
				}
				defer func() {
					_ = outFile.Close()
				}()
				if err := nativewebp.Encode(outFile, img, nil); err != nil {
					fmt.Printf("Failed to encode WebP %s: %v\n", outPath, err)
					return nil
				}

				// fmt.Printf("Created WebP: %s\n", outPath)
				formattedTextures++

				if err := os.Remove(path); err != nil {
					fmt.Printf("Failed to delete original PNG %s: %v\n", path, err)
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through folder while parsing pngs: %v\n", err)
	}

	fmt.Printf("Finished formatting textures. Total formatted: %d (Animated: %d)\n", formattedTextures, formattedAnimatedTextures)
}

/*
func ConvertFromVanillaToCatharsis(inputFolder string, config models.ResourcePackConfig) {
	convertedTextures := 0
	err := filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// fmt.Printf("Warning: skipped missing path %s: %v\n", path, err)
				return nil
			}
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			jsonData, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("Failed to read %s: %v\n", path, err)
				return nil
			}

			var itemTexture models.ItemTexture
			if err := json.Unmarshal(jsonData, &itemTexture); err != nil {
				fmt.Printf("Failed to parse %s: %v\n", path, err)
				return nil
			}

			// Custom models
			if len(itemTexture.Elements) > 0 || itemTexture.HeadModel != "" {
				if err := os.Remove(path); err != nil {
					fmt.Printf("Failed to delete unsupported model %s: %v\n", path, err)
				} else {
					_ = 0
					// fmt.Printf("Deleted unsupported model with '0' texture reference: %s\n", path)
				}

				texPath := resolveTexturePath(inputFolder, path, itemTexture.Textures["0"])
				if _, err := os.Stat(texPath); err == nil {
					if err := os.Remove(texPath); err != nil {
						fmt.Printf("Failed to delete associated texture %s: %v\n", texPath, err)
					} else {
						// fmt.Printf("Deleted associated texture for unsupported model with '0' texture reference: %s\n", texPath)
						_ = 0
					}
				}
			}

			texturePath := itemTexture.Textures["layer0"]
			if texturePath == "" {
				// fmt.Printf("No texture found for %s\n", path)
				return nil
			}

			formattedTexture := models.CatharsisFormat{
				Model: models.CatharsisModel{
					Type: "model",
					Path: texturePath,
				},
			}

			formattedJsonData, err := json.MarshalIndent(formattedTexture, "", "  ")
			if err != nil {
				fmt.Printf("Failed to marshal formatted JSON for %s: %v\n", path, err)
				return nil
			}

			if err := os.WriteFile(path, formattedJsonData, 0644); err != nil {
				fmt.Printf("Failed to write formatted JSON to %s: %v\n", path, err)
				return nil
			}

			convertedTextures++
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through folder while converting textures: %v\n", err)
	}

	fmt.Printf("Finished converting textures. Total converted: %d\n", convertedTextures)
}
*/

func ensureWebPExt(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".webp") {
		return path
	}

	if strings.HasSuffix(lower, ".png") {
		return strings.TrimSuffix(path, path[len(path)-4:]) + ".webp"
	}

	return path + ".webp"
}

func sortedTextureLayerKeys(textures map[string]string) []string {
	keys := make([]string, 0, len(textures))
	for key := range textures {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		iLayer, iIndex := parseLayerKey(keys[i])
		jLayer, jIndex := parseLayerKey(keys[j])

		if iLayer && jLayer {
			return iIndex < jIndex
		}

		if iLayer != jLayer {
			return iLayer
		}

		return keys[i] < keys[j]
	})

	return keys
}

func dedupeTextureLayers(textures map[string]string) (map[string]string, bool) {
	orderedKeys := sortedTextureLayerKeys(textures)
	uniqueTextures := make(map[string]struct{}, len(textures))
	deduped := make(map[string]string, len(textures))

	layerIndex := 0
	for _, key := range orderedKeys {
		tex := strings.TrimSpace(textures[key])
		if tex == "" {
			continue
		}

		// if tex == "#0" {
		// 	tex = textures["0"]
		// }

		if _, exists := uniqueTextures[tex]; exists {
			continue
		}

		uniqueTextures[tex] = struct{}{}
		deduped[fmt.Sprintf("layer%d", layerIndex)] = tex
		layerIndex++
	}

	hadDuplicates := len(deduped) < len(textures)
	if !hadDuplicates {
		return textures, false
	}

	return deduped, true
}

func parseLayerKey(key string) (bool, int) {
	if !strings.HasPrefix(key, "layer") {
		return false, 0
	}

	layerNumber := strings.TrimPrefix(key, "layer")
	if layerNumber == "" {
		return true, 0
	}

	parsed, err := strconv.Atoi(layerNumber)
	if err != nil {
		return false, 0
	}

	return true, parsed
}

func resolveTexturePath(inputFolder, modelPath, textureRef string) string {
	if textureRef == "#0" {
		return ""
	}

	splitString := strings.Split(textureRef, ":")
	category := splitString[0]
	item := splitString[1]

	formattedPath := filepath.Join(inputFolder, category, "assets", category, "textures", item+".webp")

	return formattedPath

}

func buildTextureOutputForModel(inputFolder, modelPath string) (string, string) {
	cleanInput := filepath.Clean(inputFolder)
	cleanModel := filepath.Clean(modelPath)

	relPath, err := filepath.Rel(cleanInput, cleanModel)
	if err != nil {
		outPath := strings.TrimSuffix(modelPath, ".json") + ".webp"
		return outPath, filepath.Base(outPath)
	}

	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) >= 4 && parts[0] == "assets" && parts[2] == "models" {
		namespace := parts[1]
		textureRelParts := append([]string{"assets", namespace, "textures"}, parts[3:]...)
		textureRelPath := strings.Join(textureRelParts, "/")
		textureRelPath = strings.TrimSuffix(textureRelPath, ".json")
		outPath := filepath.Join(cleanInput, filepath.FromSlash(textureRelPath+".webp"))
		textureRef := namespace + ":" + strings.TrimSuffix(strings.Join(parts[3:], "/"), ".json")
		return outPath, textureRef
	}

	if len(parts) >= 6 && parts[0] == "assets" && parts[2] == "assets" && parts[4] == "models" {
		namespace := parts[3]
		textureRelParts := append([]string{"assets", parts[1], "assets", namespace, "textures"}, parts[5:]...)
		textureRelPath := strings.Join(textureRelParts, "/")
		textureRelPath = strings.TrimSuffix(textureRelPath, ".json")
		outPath := filepath.Join(cleanInput, filepath.FromSlash(textureRelPath+".webp"))
		textureRef := namespace + ":" + strings.TrimSuffix(strings.Join(parts[5:], "/"), ".json")
		return outPath, textureRef
	}

	outPath := strings.TrimSuffix(modelPath, ".json") + ".webp"
	return outPath, filepath.Base(outPath)
}

func resizeImageNearest(src image.Image, width, height int) image.Image {
	if width <= 0 || height <= 0 {
		return src
	}

	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW == width && srcH == height {
		return src
	}

	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + (y*srcH)/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + (x*srcW)/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}
