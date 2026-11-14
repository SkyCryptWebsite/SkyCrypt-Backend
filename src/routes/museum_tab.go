package routes

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"skycrypt/src/constants"
	"skycrypt/src/db"
	"skycrypt/src/models"
	statsItems "skycrypt/src/stats/items"
	"skycrypt/src/utility"
	"sort"
	"time"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

// A museum changes only when its owner donates, but the inventory response expires every five minutes, and each
// rebuild decoded every museum entry again (its own gzipped NBT blob each), about half of an inventory build. The
// built tab is kept in Redis under a hash of the museum's data, so a rebuild with an unchanged museum reads it
// back, in whichever prefork process it runs. The TTL bounds how long a tab outlives a change the key can't see,
// such as a refreshed item list.
const (
	museumTabCacheTTLSeconds = 60 * 60
	museumTabCacheVersion    = "1"
)

// museumTabCacheKey names the tab built from museum for enabledPacks by this build of the server.
func museumTabCacheKey(museum *skycrypttypes.Museum, enabledPacks []string) string {
	h := sha256.New()
	writeString := func(value string) {
		_ = binary.Write(h, binary.LittleEndian, uint32(len(value)))
		_, _ = h.Write([]byte(value))
	}
	writeItem := func(item skycrypttypes.MuseumItem) {
		_ = binary.Write(h, binary.LittleEndian, item.DonatedTime)
		_ = binary.Write(h, binary.LittleEndian, item.Borrowing)
		_ = binary.Write(h, binary.LittleEndian, int64(item.Items.Type))
		writeString(item.FeaturedSlot)
		writeString(item.Items.Data)
	}

	writeString(museumTabCacheVersion)
	writeString(os.Getenv("SOURCE_COMMIT"))
	writeString(utility.GetDomain())
	writeString(enabledPacksCachePart(enabledPacks))
	_ = binary.Write(h, binary.LittleEndian, museum.Value)
	_ = binary.Write(h, binary.LittleEndian, museum.Appraisal)

	if museum.Items != nil {
		ids := make([]string, 0, len(*museum.Items))
		for id := range *museum.Items {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		_ = binary.Write(h, binary.LittleEndian, uint32(len(ids)))
		for _, id := range ids {
			writeString(id)
			writeItem((*museum.Items)[id])
		}
	}
	if museum.Special != nil {
		_ = binary.Write(h, binary.LittleEndian, uint32(len(*museum.Special)))
		for _, item := range *museum.Special {
			writeItem(item)
		}
	}
	return "museum_tab:" + hex.EncodeToString(h.Sum(nil))
}

// museumTab returns the museum tab of the inventory response as JSON, from the cache when the same museum was
// built before. strip is how long stripping the items took, 0 on a cache hit.
func museumTab(ctx context.Context, museum *skycrypttypes.Museum, enabledPacks []string) (body []byte, strip time.Duration, err error) {
	caching := museum != nil && processedResponseCacheEnabled()
	var cacheKey string
	if caching {
		cacheKey = museumTabCacheKey(museum, enabledPacks)
		if cached, err := db.GetBytesContext(ctx, cacheKey); err == nil && len(cached) > 0 && cached[0] == '{' {
			return cached, 0, nil
		}
	}

	museumItems := statsItems.GetMuseum(museum, enabledPacks)
	stripStart := time.Now()
	strippedItems := statsItems.StripItems(&museumItems)
	strip = time.Since(stripStart)

	inventoryData := constants.INVENTORY["museum"]
	body, err = marshalBody(models.Inventory{
		Name:           inventoryData.Name,
		Texture:        fmt.Sprintf("%s%s", utility.GetDomain(), inventoryData.Texture),
		SeparatorAfter: inventoryData.SeparatorAfter,
		Items:          strippedItems,
	})
	if err != nil {
		return nil, strip, err
	}

	if caching {
		cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = db.SetContext(cacheCtx, cacheKey, body, museumTabCacheTTLSeconds)
	}
	return body, strip, nil
}
