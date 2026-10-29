package lib

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"

	"skycrypt/src/db"
)

// A prefork child queues a live render, the master renders it, and the child then serves the master's result,
// the same texture an inline render gives. Needs Redis: SKYCRYPT_TEST_REDIS_ADDR (uses database 3).
func TestQueuedRenderMatchesInlineRender(t *testing.T) {
	addr := os.Getenv("SKYCRYPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("SKYCRYPT_TEST_REDIS_ADDR is not set")
	}
	withRenderItemGlobals(t)
	if err := db.InitRedis(addr, os.Getenv("SKYCRYPT_TEST_REDIS_PASSWORD"), 3); err != nil {
		t.Fatal(err)
	}
	client, err := db.Redis()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.FlushDB(context.Background()).Err() })

	appRoot, err := appRootDir()
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := newRendererForPackIDs(
		filepath.Join(t.TempDir(), "cache"),
		filepath.Join(appRoot, "assets", "resourcepacks"),
		filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft"),
		[]string{"HYPIXEL_PACK"},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	customResourceRenderer = renderer

	// Inventory items carry a typed tag, which reaches the master as JSON.
	input := ItemTextureInput{
		ID:         "minecraft:diamond_sword",
		SkyBlockID: "ASPECT_OF_THE_END",
		Tag:        &skycrypttypes.Tag{ExtraAttributes: &skycrypttypes.ExtraAttributes{Id: "ASPECT_OF_THE_END"}},
	}
	textureCtx := NewTextureApplyContext([]string{"HYPIXEL_PACK"})
	inline, inlineReason, ok := runtimeRenderTexture(renderer, input, normalizeTextureApplyContext(textureCtx), nil)
	if !ok || !strings.Contains(inline.Texture, "/cache/rendered/") {
		t.Fatalf("inline render failed: %#v %s", inline, inlineReason)
	}

	previous := renderInMaster
	renderInMaster = func() bool { return true }
	t.Cleanup(func() { renderInMaster = previous })

	first := NewTextureApplyContext([]string{"HYPIXEL_PACK"})
	fallback := ApplyTextureInput(input, first)
	if fallback.Texture == inline.Texture || first.Stats.RenderAttempts != 0 {
		t.Fatalf("a child rendered itself (%d attempts) or got the render before it was queued: %#v", first.Stats.RenderAttempts, fallback)
	}
	if length := client.LLen(ctx, renderQueueKey).Val(); length != 1 {
		t.Fatalf("queue holds %d jobs, want 1", length)
	}
	// Missing again before the master gets to it queues nothing more.
	ApplyTextureInput(input, NewTextureApplyContext([]string{"HYPIXEL_PACK"}))
	if length := client.LLen(ctx, renderQueueKey).Val(); length != 1 {
		t.Fatalf("a second miss queued it again: %d jobs", length)
	}

	payload, err := client.LPop(ctx, renderQueueKey).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	renderQueuedJob(client, payload)
	if pending := client.Keys(ctx, renderPendingPrefix+"*").Val(); len(pending) != 0 {
		t.Fatalf("pending marker left behind: %v", pending)
	}

	second := NewTextureApplyContext([]string{"HYPIXEL_PACK"})
	served := ApplyTextureInput(input, second)
	if served != inline || second.Stats.RenderAttempts != 0 {
		t.Fatalf("child served %#v (%d render attempts), want the inline render %#v", served, second.Stats.RenderAttempts, inline)
	}
}
