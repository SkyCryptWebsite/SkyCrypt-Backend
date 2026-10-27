package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// fetchSample downloads raw Hypixel responses into dir: the seed players, every coop member of their profiles, and
// the members of the seeds' guilds, up to limit players. Each player gets player.json and profiles.json, and each
// profile gets museum and garden bodies. Players already on disk are skipped, so a stopped run can resume.
func fetchSample(ctx context.Context, dir string, seeds []string, limit int) error {
	key := strings.TrimSpace(os.Getenv("HYPIXEL_API_KEY"))
	if key == "" {
		return errors.New("HYPIXEL_API_KEY is not set")
	}
	client := &hypixelClient{key: key, http: &http.Client{Timeout: 30 * time.Second}}

	queue := []string{}
	for _, seed := range seeds {
		uuid, err := mojangUUID(ctx, client.http, seed)
		if err != nil {
			fmt.Printf("resolve %s: %v\n", seed, err)
			continue
		}
		queue = append(queue, uuid)
	}
	seedCount := len(queue)
	if seedCount == 0 {
		return errors.New("no seed player could be resolved")
	}

	seen := map[string]bool{}
	seenProfiles := map[string]bool{}
	for i := 0; i < len(queue) && len(seen) < limit; i++ {
		uuid := queue[i]
		if seen[uuid] {
			continue
		}
		seen[uuid] = true

		playerDir := filepath.Join(dir, "players", uuid)
		profilesPath := filepath.Join(playerDir, "profiles.json")
		body, err := os.ReadFile(profilesPath)
		if err != nil {
			if err := os.MkdirAll(playerDir, 0o755); err != nil {
				return err
			}
			if err := client.save(ctx, "/v2/player", url.Values{"uuid": {uuid}}, filepath.Join(playerDir, "player.json")); err != nil {
				fmt.Printf("player %s: %v\n", uuid, err)
				continue
			}
			if err := client.save(ctx, "/v2/skyblock/profiles", url.Values{"uuid": {uuid}}, profilesPath); err != nil {
				fmt.Printf("profiles %s: %v\n", uuid, err)
				continue
			}
			body, _ = os.ReadFile(profilesPath)
		}

		var profiles struct {
			Profiles []struct {
				ProfileID string                     `json:"profile_id"`
				Members   map[string]json.RawMessage `json:"members"`
			} `json:"profiles"`
		}
		if err := json.Unmarshal(body, &profiles); err != nil {
			fmt.Printf("profiles %s: %v\n", uuid, err)
			continue
		}

		for _, profile := range profiles.Profiles {
			for member := range profile.Members {
				queue = append(queue, member)
			}
			if seenProfiles[profile.ProfileID] {
				continue
			}
			seenProfiles[profile.ProfileID] = true

			profileDir := filepath.Join(dir, "profiles", profile.ProfileID)
			if _, err := os.Stat(filepath.Join(profileDir, "museum.json")); err == nil {
				continue
			}
			if err := os.MkdirAll(profileDir, 0o755); err != nil {
				return err
			}
			query := url.Values{"profile": {profile.ProfileID}}
			if err := client.save(ctx, "/v2/skyblock/museum", query, filepath.Join(profileDir, "museum.json")); err != nil {
				fmt.Printf("museum %s: %v\n", profile.ProfileID, err)
			}
			if err := client.save(ctx, "/v2/skyblock/garden", query, filepath.Join(profileDir, "garden.json")); err != nil {
				fmt.Printf("garden %s: %v\n", profile.ProfileID, err)
			}
		}

		// Guild members widen the sample from the top players to ordinary ones.
		if i < seedCount {
			guild, err := client.get(ctx, "/v2/guild", url.Values{"player": {uuid}})
			if err == nil {
				var parsed struct {
					Guild *struct {
						Members []struct {
							UUID string `json:"uuid"`
						} `json:"members"`
					} `json:"guild"`
				}
				if json.Unmarshal(guild, &parsed) == nil && parsed.Guild != nil {
					for _, member := range parsed.Guild.Members {
						queue = append(queue, member.UUID)
					}
				}
			}
		}

		fmt.Printf("[%d/%d] %s: %d profiles\n", len(seen), limit, uuid, len(profiles.Profiles))
	}

	// A second copy of the seeds' profiles shows whether Hypixel's gzip output is stable between fetches.
	for _, uuid := range queue[:seedCount] {
		path := filepath.Join(dir, "players", uuid, "profiles.again.json")
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := client.save(ctx, "/v2/skyblock/profiles", url.Values{"uuid": {uuid}}, path); err != nil {
			fmt.Printf("profiles again %s: %v\n", uuid, err)
		}
	}

	return nil
}

type hypixelClient struct {
	key  string
	http *http.Client
	last time.Time
}

func (c *hypixelClient) save(ctx context.Context, path string, query url.Values, out string) error {
	body, err := c.get(ctx, path, query)
	if err != nil {
		return err
	}
	return os.WriteFile(out, body, 0o644)
}

// get spaces requests out and waits for the rate limit window to reset when Hypixel says it is nearly used up.
func (c *hypixelClient) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	for attempt := 0; attempt < 3; attempt++ {
		if wait := 250*time.Millisecond - time.Since(c.last); wait > 0 {
			time.Sleep(wait)
		}
		c.last = time.Now()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.hypixel.net"+path+"?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("API-Key", c.key)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		remaining, _ := strconv.Atoi(resp.Header.Get("RateLimit-Remaining"))
		reset, _ := strconv.Atoi(resp.Header.Get("RateLimit-Reset"))
		if resp.StatusCode == http.StatusTooManyRequests || (resp.Header.Get("RateLimit-Remaining") != "" && remaining < 5) {
			fmt.Printf("rate limit: waiting %ds\n", reset+1)
			time.Sleep(time.Duration(reset+1) * time.Second)
			if resp.StatusCode == http.StatusTooManyRequests {
				continue
			}
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}
		return body, nil
	}
	return nil, errors.New("rate limited")
}

func mojangUUID(ctx context.Context, client *http.Client, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.mojang.com/users/profiles/minecraft/"+url.PathEscape(name), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mojang status %d", resp.StatusCode)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	return parsed.ID, nil
}
