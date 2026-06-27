package texturepacks

import (
	"encoding/json"
	"strings"
)

// catharsisPackConfigType is the type for the CatharsisPackConfig namespace.
type catharsisPackConfigType struct{}

// CatharsisPackConfig is a namespace for catharsis pack configuration functions.
var CatharsisPackConfig catharsisPackConfigType

// ResolveEnabledOverlays parses the pack.mcmeta JSON from a catharsis pack and returns
// the list of overlay directory names that should be enabled based on their default config values.
// If configJson is provided, it takes precedence over the catharsis:pack/v1.config section.
// If enableAll is true, returns all overlay directories regardless of conditions.
func (catharsisPackConfigType) ResolveEnabledOverlays(packMcmetaJson string, configJson *string, overrides map[string]string, enableAll bool) []string {
	if strings.TrimSpace(packMcmetaJson) == "" {
		return []string{}
	}

	var root interface{}
	if err := json.Unmarshal([]byte(packMcmetaJson), &root); err != nil {
		return []string{}
	}

	defaults := parseConfigDefaults(root, configJson)
	applyOverrides(defaults, overrides)
	return evaluateOverlayEntries(root, defaults, enableAll)
}

// ResolveEnabledOverlaysSimple is a simpler version that only takes packMcmetaJson and enableAll.
func (c catharsisPackConfigType) ResolveEnabledOverlaysSimple(packMcmetaJson string, enableAll bool) []string {
	return c.ResolveEnabledOverlays(packMcmetaJson, nil, nil, enableAll)
}

func applyOverrides(defaults map[string]string, overrides map[string]string) {
	if overrides == nil || len(overrides) == 0 {
		return
	}

	for id, value := range overrides {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		defaults[id] = value
	}
}

// parseConfigDefaults parses the catharsis:pack/v1.config section to build a map of option id → default value.
// Boolean options map to "true"/"false", dropdown options map to the default option's value string.
func parseConfigDefaults(root interface{}, configJson *string) map[string]string {
	defaults := make(map[string]string)

	if configJson != nil && strings.TrimSpace(*configJson) != "" {
		if tryParseConfigOptionsJson(*configJson, defaults) {
			return defaults
		}
	}

	rootMap, ok := root.(map[string]interface{})
	if !ok {
		return defaults
	}

	catharsisSection, ok := rootMap["catharsis:pack/v1"].(map[string]interface{})
	if !ok {
		return defaults
	}

	configArray, ok := catharsisSection["config"].([]interface{})
	if !ok {
		return defaults
	}

	for _, entryRaw := range configArray {
		entry, ok := entryRaw.(map[string]interface{})
		if !ok {
			continue
		}

		typeVal, ok := entry["type"].(string)
		if !ok {
			continue
		}

		if strings.EqualFold(typeVal, "tab") {
			if optionsRaw, ok := entry["options"].([]interface{}); ok {
				parseOptionsArray(optionsRaw, defaults)
			}
		} else {
			parseSingleOption(entry, defaults)
		}
	}

	return defaults
}

func tryParseConfigOptionsJson(configJson string, defaults map[string]string) bool {
	var configArray []interface{}
	if err := json.Unmarshal([]byte(configJson), &configArray); err != nil {
		return false
	}

	parseOptionsArray(configArray, defaults)
	return true
}

func parseOptionsArray(optionsArray []interface{}, defaults map[string]string) {
	for _, optionRaw := range optionsArray {
		option, ok := optionRaw.(map[string]interface{})
		if !ok {
			continue
		}

		typeVal, ok := option["type"].(string)
		if !ok {
			continue
		}

		if strings.EqualFold(typeVal, "tab") {
			if nestedRaw, ok := option["options"].([]interface{}); ok {
				parseOptionsArray(nestedRaw, defaults)
			}
			continue
		}

		parseSingleOption(option, defaults)
	}
}

func parseSingleOption(option map[string]interface{}, defaults map[string]string) {
	typeVal, ok := option["type"].(string)
	if !ok {
		return
	}

	id, ok := option["id"].(string)
	if !ok || strings.TrimSpace(id) == "" {
		return
	}

	if strings.EqualFold(typeVal, "boolean") {
		defaultValue := false
		if def, ok := option["default"].(bool); ok && def {
			defaultValue = true
		}
		if defaultValue {
			defaults[id] = "true"
		} else {
			defaults[id] = "false"
		}
	} else if strings.EqualFold(typeVal, "dropdown") {
		if dropdownOptionsRaw, ok := option["options"].([]interface{}); ok {
			var defaultValue *string
			var firstValue *string

			for _, dropdownOptionRaw := range dropdownOptionsRaw {
				dropdownOption, ok := dropdownOptionRaw.(map[string]interface{})
				if !ok {
					continue
				}

				value, _ := dropdownOption["value"].(string)
				if firstValue == nil && value != "" {
					firstValue = &value
				}

				if def, ok := dropdownOption["default"].(bool); ok && def {
					defaultValue = &value
					break
				}
			}

			if defaultValue != nil {
				defaults[id] = *defaultValue
			} else if firstValue != nil {
				defaults[id] = *firstValue
			} else {
				defaults[id] = "off"
			}
		}
	}
}

// evaluateOverlayEntries evaluates the fabric:overlays.entries array and returns directory names
// whose conditions evaluate to true given the config defaults.
func evaluateOverlayEntries(root interface{}, defaults map[string]string, enableAll bool) []string {
	rootMap, ok := root.(map[string]interface{})
	if !ok {
		return []string{}
	}

	overlaysSection, ok := rootMap["fabric:overlays"].(map[string]interface{})
	if !ok {
		return []string{}
	}

	entriesArray, ok := overlaysSection["entries"].([]interface{})
	if !ok {
		return []string{}
	}

	var enabled []string

	for _, entryRaw := range entriesArray {
		entry, ok := entryRaw.(map[string]interface{})
		if !ok {
			continue
		}

		directory, ok := entry["directory"].(string)
		if !ok || strings.TrimSpace(directory) == "" {
			continue
		}

		if enableAll {
			enabled = append(enabled, directory)
			continue
		}

		condition, ok := entry["condition"]
		if !ok {
			// No condition → always enabled
			enabled = append(enabled, directory)
			continue
		}

		if evaluateCondition(condition, defaults) {
			enabled = append(enabled, directory)
		}
	}

	return enabled
}

// evaluateCondition evaluates a single overlay condition against config defaults.
func evaluateCondition(condition interface{}, defaults map[string]string) bool {
	condMap, ok := condition.(map[string]interface{})
	if !ok {
		return false
	}

	conditionType, ok := condMap["condition"].(string)
	if !ok {
		return false
	}

	if strings.EqualFold(conditionType, "catharsis:config") {
		return evaluateCatharsisConfigCondition(condMap, defaults)
	}

	if strings.EqualFold(conditionType, "fabric:not") {
		if inner, ok := condMap["value"]; ok {
			return !evaluateCondition(inner, defaults)
		}
		return false
	}

	if strings.EqualFold(conditionType, "fabric:all_of") {
		if valuesRaw, ok := condMap["values"].([]interface{}); ok {
			for _, innerRaw := range valuesRaw {
				if !evaluateCondition(innerRaw, defaults) {
					return false
				}
			}
			return true
		}
		return false
	}

	if strings.EqualFold(conditionType, "fabric:any_of") {
		if valuesRaw, ok := condMap["values"].([]interface{}); ok {
			for _, innerRaw := range valuesRaw {
				if evaluateCondition(innerRaw, defaults) {
					return true
				}
			}
		}
		return false
	}

	return false
}

// evaluateCatharsisConfigCondition evaluates a catharsis:config condition.
// If value is specified, checks if the config option equals that value.
// Otherwise, checks if the boolean config option is true.
func evaluateCatharsisConfigCondition(condition map[string]interface{}, defaults map[string]string) bool {
	id, ok := condition["id"].(string)
	if !ok || strings.TrimSpace(id) == "" {
		return false
	}

	// Check if a specific value match is required
	if requiredValue, ok := condition["value"].(string); ok {
		currentValue, exists := defaults[id]
		if !exists {
			return false
		}
		return strings.EqualFold(currentValue, requiredValue)
	}

	// Boolean check: config option must be "true"
	boolValue, exists := defaults[id]
	if !exists {
		return false
	}
	return strings.EqualFold(boolValue, "true")
}
