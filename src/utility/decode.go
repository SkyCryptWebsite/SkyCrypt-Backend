package utility

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"skycrypt/src/models"
	"sync"

	"github.com/Tnze/go-mc/nbt"
	"github.com/klauspost/compress/gzip"
)

var gzipReaderPool = sync.Pool{
	New: func() any {
		return &gzip.Reader{}
	},
}

var inflateBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func DecodeInventory(inventoryData *string) (*models.DecodedInventory, error) {
	if *inventoryData == "" {
		return &models.DecodedInventory{}, nil
	}

	decodedData, err := base64.StdEncoding.DecodeString(*inventoryData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	reader := gzipReaderPool.Get().(*gzip.Reader)
	defer gzipReaderPool.Put(reader)

	err = reader.Reset(bytes.NewReader(decodedData))
	if err != nil {
		return nil, fmt.Errorf("failed to reset gzip reader: %w", err)
	}

	// Inflate first: the NBT decoder reads byte by byte, and a gzip reader has no ReadByte, so decoding straight
	// from it made a gzip Read call per byte (about 20% slower on real inventories).
	inflated := inflateBufferPool.Get().(*bytes.Buffer)
	defer inflateBufferPool.Put(inflated)
	inflated.Reset()
	if _, err := io.Copy(inflated, reader); err != nil {
		return nil, fmt.Errorf("failed to inflate inventory: %w", err)
	}

	var nbtData models.DecodedInventory
	decoder := nbt.NewDecoder(bytes.NewReader(inflated.Bytes()))
	_, err = decoder.Decode(&nbtData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse NBT data: %w", err)
	}

	return &nbtData, nil
}
