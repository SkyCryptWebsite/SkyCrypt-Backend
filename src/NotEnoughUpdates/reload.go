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

// reloadParsedRepository drops the item caches, which were read from the old files, then re-parses the constants.
// The caches go first because parsing reads items through them (attribute shards do).
func reloadParsedRepository() error {
	CACHED_NEU_ITEMS.Clear()
	CACHED_NEU_ITEM_WIKIS.Clear()
	return ParseNEURepository()
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

// StartInitialUpdate pulls the repository in the background once the first parse is done, then parses again.
// Updating while that first parse read the same files could hand it a half-written file.
func StartInitialUpdate() {
	go updateAndReload()
}

// updateAndReload pulls the repository and, only if HEAD moved, re-parses it and publishes the new commit.
// An unchanged HEAD needs no publish: the commit already published (or loaded by children) is still current.
func updateAndReload() {
	before := repoHead()
	if err := UpdateNEURepository(); err != nil {
		fmt.Printf("[NOT-ENOUGH-UPDATES] Failed to update repository: %v\n", err)
		return
	}
	if before != "" && repoHead() == before {
		return
	}
	if err := reloadParsedRepository(); err != nil {
		fmt.Printf("[NOT-ENOUGH-UPDATES] Failed to parse updated repository: %v\n", err)
		return
	}
	PublishRepositoryVersion()
}

// ParseWithRetry parses the repository, retrying a few times. Prefork children parse while the main process may be
// updating the files, so a parse can catch one mid-write.
func ParseWithRetry() error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = ParseNEURepository(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return err
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

// PublishedRepositoryVersion returns the commit the updating process last published, or "".
func PublishedRepositoryVersion() string {
	version, _ := db.Get(repoVersionKey)
	return version
}

// StartChildReloadLoop re-parses the repository in a prefork child whenever the updating process publishes a
// new commit. baseline must be read before the child's first parse: read after, a version published in between
// would be taken as already loaded while the child holds the older data.
func StartChildReloadLoop(baseline string) {
	go func() {
		loaded := baseline
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
