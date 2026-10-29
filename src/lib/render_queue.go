package lib

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"skycrypt/src/db"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

// Under prefork, items that need a live render are rendered by the master alone. Every process used to build its
// own renderer per pack stack it saw (each loads every model and item definition), so a child kept 150-200 MB of
// renderers, rebuilt them as stacks were evicted, and all of it was repeated in every child.
//
// A child that misses looks for the master's result in Redis and otherwise queues the item once; until the result
// is there the item gets its fallback texture, which is cached for a minute only. A single process (no prefork)
// still renders inline.
const (
	renderQueueKey      = "render:queue"
	renderPendingPrefix = "render:pending:"
	renderResultPrefix  = "render:result:"

	// Rendered files are named after what they show and never change, so a result stays good until the cache is
	// rebuilt. A miss is kept shorter, so a pack or renderer fix shows up.
	renderResultTTL  = 24 * time.Hour
	renderMissTTL    = 10 * time.Minute
	renderPendingTTL = 2 * time.Minute
	// renderQueueLimit drops new jobs while the master is this far behind, rather than growing Redis without bound.
	renderQueueLimit = 20000

	renderWorkers      = 2
	renderLookupBudget = 300 * time.Millisecond
)

// renderJob is everything the render and its acceptance check depend on.
type renderJob struct {
	Input          ItemTextureInput `json:"input"`
	EnabledPackIDs []string         `json:"enabledPackIds"`
}

type renderResult struct {
	Texture AppliedItemTexture `json:"texture"`
	Reason  string             `json:"reason"`
	OK      bool               `json:"ok"`
}

// renderInMaster reports whether this process hands live renders to the prefork master.
var renderInMaster = fiber.IsChild

func renderJobKey(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:16])
}

// queuedRuntimeRender returns the master's render of input once it is done, and queues it otherwise.
func queuedRuntimeRender(input ItemTextureInput, textureCtx TextureApplyContext) (AppliedItemTexture, string, bool) {
	client, err := db.Redis()
	if err != nil {
		return AppliedItemTexture{}, "", false
	}
	payload, err := json.Marshal(renderJob{Input: input, EnabledPackIDs: textureCtx.EnabledPackIDs})
	if err != nil {
		return AppliedItemTexture{}, "", false
	}
	key := renderJobKey(payload)

	ctx, cancel := context.WithTimeout(context.Background(), renderLookupBudget)
	defer cancel()
	if raw, err := client.Get(ctx, renderResultPrefix+key).Bytes(); err == nil {
		var result renderResult
		if json.Unmarshal(raw, &result) == nil {
			return result.Texture, result.Reason, result.OK
		}
	}

	// Queue it once, whatever the number of requests that miss it meanwhile.
	if queued, err := client.SetNX(ctx, renderPendingPrefix+key, 1, renderPendingTTL).Result(); err != nil || !queued {
		return AppliedItemTexture{}, "", false
	}
	if length, err := client.LLen(ctx, renderQueueKey).Result(); err == nil && length >= renderQueueLimit {
		_ = client.Del(ctx, renderPendingPrefix+key).Err()
		return AppliedItemTexture{}, "", false
	}
	_ = client.RPush(ctx, renderQueueKey, payload).Err()
	return AppliedItemTexture{}, "", false
}

// StartRenderWorkers renders the items prefork children queue. Master only; in a single process the queue stays
// empty and the workers just wait.
func StartRenderWorkers() {
	for range renderWorkers {
		go renderWorker()
	}
}

func renderWorker() {
	for {
		client, err := db.Redis()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		popped, err := client.BLPop(context.Background(), 5*time.Second, renderQueueKey).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil || len(popped) != 2 {
			time.Sleep(time.Second)
			continue
		}
		renderQueuedJob(client, []byte(popped[1]))
	}
}

func renderQueuedJob(client *redis.Client, payload []byte) {
	key := renderJobKey(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	defer func() { _ = client.Del(context.WithoutCancel(ctx), renderPendingPrefix+key).Err() }()
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Printf("[CUSTOM_RESOURCES] queued render panicked: %v\n", recovered)
		}
	}()

	var job renderJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return
	}
	if err := ensureCustomResourceRenderer(); err != nil {
		return
	}
	renderer := currentCustomResourceRenderer()
	if renderer == nil {
		return
	}

	textureCtx := normalizeTextureApplyContext(NewTextureApplyContext(job.EnabledPackIDs))
	texture, reason, ok := runtimeRenderTexture(renderer, job.Input, textureCtx, nil)
	result, err := json.Marshal(renderResult{Texture: texture, Reason: reason, OK: ok})
	if err != nil {
		return
	}
	ttl := renderResultTTL
	if !ok {
		ttl = renderMissTTL
	}
	_ = client.Set(ctx, renderResultPrefix+key, result, ttl).Err()
}
