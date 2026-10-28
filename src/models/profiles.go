package models

import skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"

type HypixelProfilesResponse struct {
	Success  bool                    `json:"success"`
	Cause    string                  `json:"cause,omitempty"`
	Profiles []skycrypttypes.Profile `json:"profiles"`
}
