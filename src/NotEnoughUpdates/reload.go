package notenoughupdates

import (
	"context"
	"fmt"
	"os/exec"
	"skycrypt/src/db"
	"strings"
	"time"
)

// The process that updates the repository records the commit it parsed here. Prefork children never update the
// repository themselves, so they poll this key and re-parse when it changes.
const (
	repoVersionKey        = "neu:repo_version"
	repoVersionTTLSeconds = 7 * 24 * 60 * 60
	childReloadInterval   = 10 * time.Minute
)

// reloadParsedRepository re-parses the constants and drops the item caches, which were read from the old files.
func reloadParsedRepository() error {
	if err := ParseNEURepository(); err != nil {
		return err
	}
	CACHED_NEU_ITEMS.Clear()
	CACHED_NEU_ITEM_WIKIS.Clear()
	return nil
}

func repoHead() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = "NotEnoughUpdates-REPO"
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// PublishRepositoryVersion records the parsed commit for prefork children.
func PublishRepositoryVersion() {
	head := repoHead()
	if head == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = db.SetContext(ctx, repoVersionKey, head, repoVersionTTLSeconds)
}

// StartChildReloadLoop re-parses the repository in a prefork child whenever the updating process publishes a
// new commit. The child parsed the files on disk at startup, so the version published then is its baseline.
func StartChildReloadLoop() {
	go func() {
		loaded, _ := db.Get(repoVersionKey)
		for range time.Tick(childReloadInterval) {
			published, err := db.Get(repoVersionKey)
			if err != nil || published == "" || published == loaded {
				continue
			}
			if err := reloadParsedRepository(); err != nil {
				fmt.Printf("[NOT-ENOUGH-UPDATES] Reload failed: %v\n", err)
				continue
			}
			loaded = published
		}
	}()
}
