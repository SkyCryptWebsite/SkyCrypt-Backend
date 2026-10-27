package store

import (
	"encoding/json"
	"fmt"

	"skycrypt/src/models"
)

// SplitProfile returns a profile's id, its cute name and the profile object without "selected" and "cute_name". Both
// depend on who requested the profiles (each coop member has their own fruit name), so they are stored per member and
// added back when a response is rebuilt.
func SplitProfile(raw []byte) (id, cuteName string, object []byte, err error) {
	fields, err := objectFields(raw)
	if err != nil {
		return "", "", nil, err
	}

	out := make([]byte, 0, len(raw))
	out = append(out, '{')
	for _, f := range fields {
		switch f.key {
		case "selected":
			continue
		case "cute_name":
			if err := json.Unmarshal(f.value, &cuteName); err != nil {
				return "", "", nil, fmt.Errorf("store: bad cute_name: %w", err)
			}
			continue
		case "profile_id":
			if err := json.Unmarshal(f.value, &id); err != nil {
				return "", "", nil, fmt.Errorf("store: bad profile_id: %w", err)
			}
		}
		if len(out) > 1 {
			out = append(out, ',')
		}
		out = append(out, f.rawKey...)
		out = append(out, ':')
		out = append(out, f.value...)
	}
	out = append(out, '}')

	if id == "" {
		return "", "", nil, fmt.Errorf("store: profile without profile_id")
	}
	return id, cuteName, out, nil
}

// TrimPlayer keeps the part of a /v2/player body that SkyCrypt reads. The rest is other minigames' stats, often
// hundreds of KB, so the stored body is the typed player marshalled back into Hypixel's response shape.
func TrimPlayer(body []byte) ([]byte, error) {
	var response models.HypixelPlayerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("store: player response without success")
	}
	return json.Marshal(response)
}
