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
	case "format":
		ParseImagesAndJson(inputFolder, config)
	case "convert":
		ConvertFromVanillaToCatharsis(inputFolder, config)
	default:
		fmt.Printf("Unknown function: %s\n", function)
	}
}

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

			texturePath := itemTexture.Textures["0"]
			if texturePath == "" {
				fmt.Printf("No texture found for %s\n", path)
				return nil
			}

			texturePath = ensureWebPExt(texturePath)

			formattedTexture := models.FormattedTexture{
				ResourcePackId: config.Id,
				Path:           texturePath,
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

			// fmt.Printf("Created formatted JSON: %s\n", formattedJsonPath)
			convertedTextures++

		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through folder while converting textures: %v\n", err)
	}

	fmt.Printf("Finished converting textures. Total converted: %d\n", convertedTextures)
}

func ParseImagesAndJson(inputFolder string, config models.ResourcePackConfig) {
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

	err = filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
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
					// fmt.Printf("Deleted blacklisted model due to parent: %s\n", path)
				}

				for _, tex := range itemTexture.Textures {
					texPath := resolveTexturePath(inputFolder, path, tex)
					if _, err := os.Stat(texPath); err == nil {
						if err := os.Remove(texPath); err != nil {
							fmt.Printf("Failed to delete associated texture %s: %v\n", texPath, err)
						} else {
							// fmt.Printf("Deleted associated texture due to blacklisted parent: %s\n", texPath)
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
					// fmt.Printf("Deleted blacklisted model because of the path: %s\n", path)
				}

				for _, tex := range itemTexture.Textures {
					texPath := resolveTexturePath(inputFolder, path, tex)
					if _, err := os.Stat(texPath); err == nil {
						if err := os.Remove(texPath); err != nil {
							fmt.Printf("Failed to delete associated texture %s: %v\n", texPath, err)
						} else {
							// fmt.Printf("Deleted associated texture due to blacklisted path: %s\n", texPath)
							_ = 0
						}
					}
				}

				return nil
			}

			if len(itemTexture.Elements) > 0 || len(itemTexture.Display) > 0 {
				if err := os.Remove(path); err != nil {
					fmt.Printf("Failed to delete blockstate model %s: %v\n", path, err)
				} else {
					_ = 0
					// fmt.Printf("Deleted unsupported blockstate model: %s\n", path)
				}
				return nil
			}

			if len(itemTexture.Textures) > 1 {
				uniqueTextures := make(map[string]struct{})
				for _, tex := range itemTexture.Textures {
					uniqueTextures[tex] = struct{}{}
				}

				if len(uniqueTextures) > 1 {
					// fmt.Printf("Found %s with %d textures %s\n", info.Name(), len(itemTexture.Textures), path)

					var baseImg image.Image
					baseFromVanilla := false
					for _, tex := range itemTexture.Textures {
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

						// fmt.Printf("Created combined WebP: %s\n", outPath)

						itemTexture.Textures = map[string]string{
							"layer0": textureRef,
						}

						newJsonData, err := json.MarshalIndent(itemTexture, "", "  ")
						if err != nil {
							fmt.Printf("Failed to marshal updated JSON for %s: %v\n", path, err)
							return nil
						}

						if err := os.WriteFile(path, newJsonData, 0644); err != nil {
							fmt.Printf("Failed to write updated JSON to %s: %v\n", path, err)
							return nil
						}

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

func resolveTexturePath(inputFolder, modelPath, textureRef string) string {
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
			*candidates = append(*candidates, ensureWebPExt(normalized))
			return
		}

		*candidates = append(*candidates, ensureWebPExt(filepath.Join(inputFolder, normalized)))
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

	return ensureWebPExt(filepath.Join(filepath.Dir(modelPath), textureRef))
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
