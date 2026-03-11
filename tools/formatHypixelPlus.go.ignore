package main

import (
	"encoding/json"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kettek/apng"
)

type McMeta struct {
	Animation McMetaAnimation `json:"animation"`
}

type McMetaAnimation struct {
	Frametime float64 `json:"frametime"`
}

func main() {
	assetsRoot := "temp/Hypixel+ 0.23.4 for 1.21.8/assets/hplus/textures/skyblock/"
	packDirs, err := os.ReadDir(assetsRoot)
	if err != nil {
		fmt.Printf("Failed to read assets directory: %v\n", err)
		return
	}

	for _, packDir := range packDirs {
		if !packDir.IsDir() {
			continue
		}

		packAssetsPath := filepath.Join(assetsRoot, packDir.Name())
		if _, err := os.Stat(packAssetsPath); os.IsNotExist(err) {
			continue
		}

		_ = filepath.WalkDir(packAssetsPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				if filepath.Ext(d.Name()) == ".png" {
					// Get the relative path from packAssetsPath to preserve directory structure
					relPath, err := filepath.Rel(packAssetsPath, path)
					if err != nil {
						fmt.Printf("Failed to get relative path for %s: %v\n", path, err)
						return nil
					}
					outputPath := filepath.Join("output/assets/textures", packDir.Name(), relPath)
					// fmt.Printf("Path: %s\n", outputPath)
					/*
						if fmt.Sprintf("%s.png", packDir.Name()) == d.Name() {
							outputPath = filepath.Join("output/assets/textures", d.Name())
						}
					*/

					outputDir := filepath.Dir(outputPath)
					if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
						fmt.Printf("Failed to create directory %s: %v\n", outputDir, err)
						return nil
					}

					inputFile, err := os.Open(path)
					if err != nil {
						fmt.Printf("Failed to open %s: %v\n", path, err)
						return nil
					}
					defer func() {
						if err := inputFile.Close(); err != nil {
							fmt.Printf("Failed to close %s: %v\n", path, err)
						}
					}()

					outputFile, err := os.Create(outputPath)
					if err != nil {
						fmt.Printf("Failed to create %s: %v\n", outputPath, err)
						return nil
					}
					defer func() {
						if err := outputFile.Close(); err != nil {
							fmt.Printf("Failed to close %s: %v\n", outputPath, err)
						}
					}()

					if _, err := inputFile.Seek(0, 0); err != nil {
						fmt.Printf("Failed to seek in %s: %v\n", path, err)
						return nil
					}

					if _, err := outputFile.ReadFrom(inputFile); err != nil {
						fmt.Printf("Failed to copy from %s to %s: %v\n", path, outputPath, err)
						return nil
					}

					// fmt.Printf("Copied: %s to %s\n", path, outputPath)

					mcmetaPath := path + ".mcmeta"
					if _, err := os.Stat(mcmetaPath); err == nil {
						mcmetaOutputPath := outputPath + ".mcmeta"
						mcmetaInputFile, err := os.Open(mcmetaPath)
						if err != nil {
							fmt.Printf("Failed to open %s: %v\n", mcmetaPath, err)
							return nil
						}
						defer func() {
							if err := mcmetaInputFile.Close(); err != nil {
								fmt.Printf("Failed to close %s: %v\n", mcmetaPath, err)
							}
						}()

						mcmetaOutputFile, err := os.Create(mcmetaOutputPath)
						if err != nil {
							fmt.Printf("Failed to create %s: %v\n", mcmetaOutputPath, err)
							return nil
						}

						defer func() {
							if err := mcmetaOutputFile.Close(); err != nil {
								fmt.Printf("Failed to close %s: %v\n", mcmetaOutputPath, err)
							}
						}()

						if _, err := mcmetaInputFile.Seek(0, 0); err != nil {
							fmt.Printf("Failed to seek in %s: %v\n", mcmetaPath, err)
							return nil
						}

						if _, err := mcmetaOutputFile.ReadFrom(mcmetaInputFile); err != nil {
							fmt.Printf("Failed to copy from %s to %s: %v\n", mcmetaPath, mcmetaOutputPath, err)
							return nil
						}

						// fmt.Printf("Copied: %s to %s\n", mcmetaPath, mcmetaOutputPath)
					}

					/*
						modelsPath := "temp/Hypixel+ 0.23.4 for 1.21.8/assets/hplus/models/skyblock"
						for _, modelDir := range packDirs {
							if !modelDir.IsDir() {
								continue
							}

							modelsAssetsPath := filepath.Join(modelsPath, modelDir.Name())
							if _, err := os.Stat(modelsAssetsPath); os.IsNotExist(err) {
								fmt.Printf("Couldn't find %v\n", modelsAssetsPath)
								continue
							}

							filepath.WalkDir(modelsAssetsPath, func(modelPath string, md fs.DirEntry, err error) error {
								if err != nil {
									return err
								}

								if !md.IsDir() && filepath.Ext(md.Name()) == ".json" {
									if md.Name() == d.Name()[:len(d.Name())-len(filepath.Ext(d.Name()))]+".json" {
										modelOutputPath := filepath.Join("output/assets/models", md.Name())
										modelOutputDir := filepath.Dir(modelOutputPath)
										if err := os.MkdirAll(modelOutputDir, os.ModePerm); err != nil {
											fmt.Printf("Failed to create directory %s: %v\n", modelOutputDir, err)
											return nil
										}

										modelInputFile, err := os.Open(modelPath)
										if err != nil {
											fmt.Printf("Failed to open %s: %v\n", modelPath, err)
											return nil
										}
										defer modelInputFile.Close()

										modelOutputFile, err := os.Create(modelOutputPath)
										if err != nil {
											fmt.Printf("Failed to create %s: %v\n", modelOutputPath, err)
											return nil
										}
										defer modelOutputFile.Close()

										if _, err := modelInputFile.Seek(0, 0); err != nil {
											fmt.Printf("Failed to seek in %s: %v\n", modelPath, err)
											return nil
										}

										if _, err := modelOutputFile.ReadFrom(modelInputFile); err != nil {
											fmt.Printf("Failed to copy from %s to %s: %v\n", modelPath, modelOutputPath, err)
											return nil
										}

										// fmt.Printf("Copied model: %s to %s\n", modelPath, modelOutputPath)
									}
								}

								return nil
							})
						}
					*/
				} else {
					// fmt.Printf("Skipped non-PNG file: %s\n", path)
					_ = 0
				}

				return nil
			}

			return nil
		})
	}

	outputAssetsPath := "output/assets/textures"

	_ = filepath.WalkDir(outputAssetsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(d.Name()) != ".png" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			fmt.Printf("Failed to open %s: %v\n", path, err)
			return nil
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Printf("Failed to close %s: %v\n", path, err)
			}
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
				fmt.Printf("Couldn't find %+v\n", mcmetaPath)
				return nil
			}

			mcmetaData, err := os.ReadFile(mcmetaPath)
			if err != nil {
				fmt.Printf("Failed to read %s: %v\n", mcmetaPath, err)
				return nil
			}

			var mcMeta McMeta
			if err := json.Unmarshal(mcmetaData, &mcMeta); err != nil {
				fmt.Printf("Failed to parse %s: %v\n", mcmetaPath, err)
				return nil
			}

			frameCount := height / width
			delay := uint16(mcMeta.Animation.Frametime * 50 / 10)

			apngImg := apng.APNG{}
			for i := 0; i < frameCount; i++ {
				frameRect := image.Rect(0, i*width, width, (i+1)*width)
				subImg := img.(interface {
					SubImage(r image.Rectangle) image.Image
				}).SubImage(frameRect)
				apngImg.Frames = append(apngImg.Frames, apng.Frame{
					Image:            subImg,
					DelayNumerator:   delay,
					DelayDenominator: 100,
				})
			}

			outFile, err := os.Create(path)
			if err != nil {
				fmt.Printf("Failed to create APNG %s: %v\n", path, err)
				return nil
			}
			defer func() {
				if err := outFile.Close(); err != nil {
					fmt.Printf("Failed to close %s: %v\n", path, err)
				}
			}()
			if err := apng.Encode(outFile, apngImg); err != nil {
				fmt.Printf("Failed to encode APNG %s: %v\n", path, err)
				return nil
			}

			fmt.Printf("Created APNG: %s\n", path)
		}

		return nil
	})
}
