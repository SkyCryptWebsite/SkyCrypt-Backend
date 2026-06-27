package assets

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

// private const uint Magic = 0x43415453; // "CATS" in ASCII
// private const byte CurrentVersion = 0x01;
// private const byte CompressionNone = 0xFF;
// private const byte CompressionGzip = 0xFE;
const (
	Magic           uint32 = 0x43415453 // "Cats" in ASCII
	CurrentVersion  byte   = 0x01
	CompressionNone byte   = 0xFF
	CompressionGzip byte   = 0xFE
)

type CatsFile struct {
	_data       []byte
	_dataOffset int
	_root       *CatsDirectoryEntry
}

type CatsEntry interface {
	GetName() string
}

type CatsFileEntry struct {
	Name        string
	Offset      int
	Size        int
	Compression byte
}

func (e *CatsFileEntry) GetName() string {
	return e.Name
}

type CatsDirectoryEntry struct {
	Name     string
	Children map[string]CatsEntry
}

func (d *CatsDirectoryEntry) GetName() string {
	return d.Name
}

func NewCatsFile(data []byte) (*CatsFile, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("data is too small to be a valid .cats file")
	}

	catsFile := &CatsFile{
		_data: data,
	}

	offset := 0

	magic := catsFile.readUInt32BigEndian(&offset)
	if magic != Magic {
		return nil, fmt.Errorf("invalid .cats file: incorrect magic number (expected 0x%X, got 0x%X)", Magic, magic)
	}

	version := catsFile.readByte(&offset)
	if version != CurrentVersion {
		return nil, fmt.Errorf("unsupported .cats file version: expected 0x%X, got 0x%X", CurrentVersion, version)
	}

	catsFile._root = catsFile.ParseDirectory(&offset)
	catsFile._dataOffset = offset

	return catsFile, nil
}

func (c *CatsFile) GetEntry(path string) CatsEntry {
	trimmed := strings.TrimLeft(path, "/")
	if trimmed == "" {
		return c._root
	}

	parts := strings.Split(trimmed, "/")
	current := c._root

	for i, part := range parts {
		child, exists := current.Children[part]
		if !exists {
			return nil
		}

		if i == len(parts)-1 {
			// Last segment: return it
			return child
		}

		// Intermediate segment must be a directory
		dir, ok := child.(*CatsDirectoryEntry)
		if !ok {
			return nil
		}

		current = dir
	}

	return nil
}

func (c *CatsFile) OpenStream(entry *CatsFileEntry) (io.ReadSeeker, error) {
	if entry == nil {
		return nil, fmt.Errorf("entry cannot be null")
	}

	absoluteOffset := c._dataOffset + entry.Offset
	if absoluteOffset < 0 || absoluteOffset+entry.Size > len(c._data) {
		return nil, fmt.Errorf(
			"file entry data range [%d..%d] is out of bounds (archive size: %d)",
			absoluteOffset, absoluteOffset+entry.Size, len(c._data))
	}

	entryData := c._data[absoluteOffset : absoluteOffset+entry.Size]

	if entry.Compression == CompressionGzip {
		gzipReader, err := gzip.NewReader(bytes.NewReader(entryData))
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()

		decompressed, err := io.ReadAll(gzipReader)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(decompressed), nil
	}

	return bytes.NewReader(entryData), nil
}

func (c *CatsFile) ParseDirectory(offset *int) *CatsDirectoryEntry {
	entryCount := c.readUInt16BigEndian(offset)
	children := make(map[string]CatsEntry, entryCount)

	for i := 0; i < int(entryCount); i++ {
		entryType := c.readByte(offset)
		nameLength := c.readByte(offset)
		name := c.readAsciiString(offset, int(nameLength))

		switch entryType {
		case 0x00: // File
			fileOffset := c.readInt32BigEndian(offset)
			fileSize := c.readInt32BigEndian(offset)
			compression := c.readByte(offset)
			children[name] = &CatsFileEntry{
				Name:        name,
				Offset:      int(fileOffset),
				Size:        int(fileSize),
				Compression: compression,
			}
		case 0x01: // Directory
			dirEntry := c.ParseDirectory(offset)
			dirEntry.Name = name
			children[name] = dirEntry
		default:
			panic(fmt.Sprintf("Unknown entry type: 0x%X.", entryType))
		}
	}

	return &CatsDirectoryEntry{Children: children}
}

func (c *CatsFile) readUInt32BigEndian(offset *int) uint32 {
	if *offset+4 > len(c._data) {
		panic("Unexpected end of data while reading UInt32.")
	}

	value := uint32(c._data[*offset])<<24 | uint32(c._data[*offset+1])<<16 | uint32(c._data[*offset+2])<<8 | uint32(c._data[*offset+3])
	*offset += 4
	return value
}

func (c *CatsFile) readInt32BigEndian(offset *int) int32 {
	if *offset+4 > len(c._data) {
		panic("Unexpected end of data while reading Int32.")
	}

	value := int32(c._data[*offset])<<24 | int32(c._data[*offset+1])<<16 | int32(c._data[*offset+2])<<8 | int32(c._data[*offset+3])
	*offset += 4
	return value
}

func (c *CatsFile) readUInt16BigEndian(offset *int) uint16 {
	if *offset+2 > len(c._data) {
		panic("Unexpected end of data while reading UInt16.")
	}

	value := uint16(c._data[*offset])<<8 | uint16(c._data[*offset+1])
	*offset += 2
	return value
}

func (c *CatsFile) readByte(offset *int) byte {
	if *offset >= len(c._data) {
		panic("Unexpected end of data while reading byte.")
	}

	value := c._data[*offset]
	*offset++
	return value
}

func (c *CatsFile) readAsciiString(offset *int, length int) string {
	if *offset+length > len(c._data) {
		panic("Unexpected end of data while reading string.")
	}

	value := string(c._data[*offset : *offset+length])
	*offset += length
	return value
}
