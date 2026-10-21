package routes

import (
	"context"
	"skycrypt/src/api"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"golang.org/x/sync/errgroup"
)

// profileData is what the profile endpoints load from Hypixel for one player and profile.
type profileData struct {
	Mowojang *models.MowojangResponse
	Profiles *models.HypixelProfilesResponse
	Profile  *skycrypttypes.Profile
	// Player is only set when loaded with player: true.
	Player *skycrypttypes.Player
	// Museums maps member UUIDs to their museum, and is only set when loaded with museum: true.
	Museums map[string]*skycrypttypes.Museum
}

type profileParts struct {
	player bool
	museum bool
	// name looks up the display name of a player given as a bare UUID (Name unset) alongside the fetch.
	name bool
}

// Member returns a copy of the requested player's member data, so callers may fill in defaults on it.
func (d *profileData) Member() *skycrypttypes.Member {
	member := d.Profile.Members[d.Mowojang.UUID]
	return &member
}

// Museum returns the requested player's museum, or nil.
func (d *profileData) Museum() *skycrypttypes.Museum {
	return d.Museums[d.Mowojang.UUID]
}

// loadProfileData fetches the profile for an already resolved player, along with the parts asked for, in parallel.
// An empty profileId means the selected profile. When the museum is wanted it starts before the profile is known
// where it can: straight away for a profile UUID, or from the last selected profile otherwise.
func loadProfileData(ctx context.Context, mowojang *models.MowojangResponse, profileId string, parts profileParts) (*profileData, error) {
	data := &profileData{Mowojang: mowojang}
	uuid := mowojang.UUID
	isProfileUUID := utility.IsUUID(profileId)
	g, groupCtx := errgroup.WithContext(ctx)

	var resolveErr error
	resolved := make(chan struct{})
	if parts.name && mowojang.Name == "" {
		go func() {
			defer close(resolved)
			player, err := api.ResolvePlayerContext(ctx, uuid)
			if err != nil {
				resolveErr = failedTo("Failed to resolve player", err)
				return
			}
			data.Mowojang = player
		}()
	} else {
		close(resolved)
	}

	type museumResult struct {
		profileID string
		museum    map[string]*skycrypttypes.Museum
		err       error
	}
	var selectedMuseumCh chan museumResult

	if parts.museum && isProfileUUID {
		g.Go(func() error {
			var err error
			data.Museums, err = api.GetMuseumContext(groupCtx, profileId)
			return err
		})
	}

	if parts.museum && profileId == "" {
		if cachedProfileID := getCachedSelectedProfileID(ctx, uuid); cachedProfileID != "" {
			selectedMuseumCh = make(chan museumResult, 1)
			g.Go(func() error {
				museum, err := api.GetMuseumContext(groupCtx, cachedProfileID)
				selectedMuseumCh <- museumResult{profileID: cachedProfileID, museum: museum, err: err}
				return nil
			})
		}
	}

	if parts.player {
		g.Go(func() error {
			var err error
			data.Player, err = api.GetPlayerContext(groupCtx, uuid)
			return err
		})
	}

	g.Go(func() error {
		var err error
		data.Profiles, err = api.GetProfilesContext(groupCtx, uuid)
		if err != nil {
			return err
		}

		data.Profile, err = stats.GetProfile(data.Profiles, profileId)
		if err != nil {
			return err
		}
		if profileId == "" {
			cacheSelectedProfileID(groupCtx, uuid, data.Profile.ProfileID)
		}

		if !parts.museum || isProfileUUID {
			return nil
		}
		if selectedMuseumCh != nil {
			result := <-selectedMuseumCh
			if result.err == nil && result.profileID == data.Profile.ProfileID {
				data.Museums = result.museum
				return nil
			}
		}
		data.Museums, err = api.GetMuseumContext(groupCtx, data.Profile.ProfileID)
		return err
	})

	err := g.Wait()
	<-resolved
	// A failed lookup wins, as it did when the lookup ran before the fetch.
	if resolveErr != nil {
		return nil, resolveErr
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}
