package notenoughupdates

import (
	"fmt"
	"os"
	neu "skycrypt/src/models/NEU"
	neustats "skycrypt/src/stats/neu"
	"time"

	jsoniter "github.com/json-iterator/go"
)

func ParseNEURepository() error {
	// timeNow := time.Now()

	constantsPath := "NotEnoughUpdates-REPO/constants"
	if _, err := os.Stat(constantsPath); os.IsNotExist(err) {
		return fmt.Errorf("constants directory does not exist: %w", err)
	}

	//fmt.Println("[NOT-ENOUGH-UPDATES] Parsing NEU repository...")

	constants, err := os.ReadDir(constantsPath)
	if err != nil {
		return fmt.Errorf("failed to read constants directory: %w", err)
	}

	for _, constant := range constants {
		if constant.IsDir() {
			// fmt.Printf("[NOT-ENOUGH-UPDATES] Skipping directory: %s\n", constant.Name())
			continue
		}

		if constant.Name() == "petnums.json" {
			filePath := fmt.Sprintf("%s/%s", constantsPath, constant.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			var petNums neu.PetNums
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(data, &petNums)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON from %s: %w", filePath, err)
			}

			NEUConstants.PetNums = petNums
		} else if constant.Name() == "pets.json" {
			filePath := fmt.Sprintf("%s/%s", constantsPath, constant.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			var pets neu.Pets
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(data, &pets)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON from %s: %w", filePath, err)
			}

			NEUConstants.Pets = pets
		} else if constant.Name() == "bestiary.json" {
			filePath := fmt.Sprintf("%s/%s", constantsPath, constant.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			var bestiaryConstants neu.NEUBestiaryRaw
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(data, &bestiaryConstants)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON from %s: %w", filePath, err)
			}

			NEUConstants.Bestiary = neustats.FormatBestiaryConstants(bestiaryConstants)
		} else if constant.Name() == "garden.json" {
			filePath := fmt.Sprintf("%s/%s", constantsPath, constant.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			var gardenConstants neu.NEUGardenRaw
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(data, &gardenConstants)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON from %s: %w", filePath, err)
			}

			NEUConstants.Garden = neustats.FormatGardenConstants(gardenConstants)
		} else if constant.Name() == "hotmlayout.json" {
			filePath := fmt.Sprintf("%s/%s", constantsPath, constant.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			var hotmConstants neu.HOTMConstants
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(data, &hotmConstants)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON from %s: %w", filePath, err)
			}

			NEUConstants.HeartOfTheMountain = hotmConstants
		} else if constant.Name() == "hotflayout.json" {
			filePath := fmt.Sprintf("%s/%s", constantsPath, constant.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			var hotfConstants neu.HOTFConstants
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(data, &hotfConstants)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON from %s: %w", filePath, err)
			}

			NEUConstants.HeartOfTheForest = hotfConstants
		} else if constant.Name() == "attribute_shards.json" {
			filePath := fmt.Sprintf("%s/%s", constantsPath, constant.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			var attributeShards neu.AttributeShardsRaw
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(data, &attributeShards)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON from %s: %w", filePath, err)
			}

			NEUConstants.AttributeShards = neustats.FormatAttributeShards(attributeShards.Attributes, GetItem)
		}
	}

	// fmt.Printf("[NOT-ENOUGH-UPDATES] Parsing completed in %s on %v\n", time.Since(timeNow), os.Getpid())

	return nil
}

func StartUpdateLoop() {
	go func() {
		for {
			time.Sleep(12 * time.Hour)

			err := UpdateNEURepository()
			if err != nil {
				fmt.Printf("Error updating NEU repository: %v\n", err)
			} else {
				err = ParseNEURepository()
				if err != nil {
					fmt.Printf("Error parsing NEU repository: %v\n", err)
				}
			}
		}
	}()
}
