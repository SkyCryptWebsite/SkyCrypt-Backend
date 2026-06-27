package assets

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDirectoryResourceProviderTextRelativeSortedAndTraversal(t *testing.T) {
	root := t.TempDir()
	mustWriteText(t, filepath.Join(root, "dir", "b.txt"), "b")
	mustWriteText(t, filepath.Join(root, "dir", "a.txt"), "a")

	provider, err := NewDirectoryResourceProvider(root)
	if err != nil {
		t.Fatal(err)
	}

	text, err := provider.ReadAllText("dir/a.txt")
	if err != nil || text != "a" {
		t.Fatalf("ReadAllText = %q, %v", text, err)
	}
	relative, err := provider.GetRelativePath("dir/a.txt", "dir")
	if err != nil || relative != "a.txt" {
		t.Fatalf("GetRelativePath = %q, %v", relative, err)
	}
	files, err := provider.EnumerateFiles("dir", "*.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"dir/a.txt", "dir/b.txt"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
	if provider.FileExists("../outside.txt") {
		t.Fatal("traversal path unexpectedly exists")
	}
	if _, err := provider.OpenRead("../outside.txt"); err == nil {
		t.Fatal("OpenRead traversal succeeded")
	}
}

func TestZipResourceProviderTextRelativeSortedAndTraversal(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "pack.zip")
	createZip(t, zipPath, map[string]string{
		"dir/b.txt":        "b",
		"dir/a.txt":        "a",
		"../escaped.txt":   "bad",
		"dir/nested/c.txt": "c",
	})

	provider := NewZipResourceProvider(zipPath)
	text, err := provider.ReadAllText("dir/a.txt")
	if err != nil || text != "a" {
		t.Fatalf("ReadAllText = %q, %v", text, err)
	}
	relative, err := provider.GetRelativePath("dir/a.txt", "dir")
	if err != nil || relative != "a.txt" {
		t.Fatalf("GetRelativePath = %q, %v", relative, err)
	}
	files, err := provider.EnumerateFiles("dir", "*.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"dir/a.txt", "dir/b.txt"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
	if provider.FileExists("../escaped.txt") {
		t.Fatal("unsafe zip entry was indexed")
	}
}

func TestSubPathResourceProviderScopesAndRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	mustWriteText(t, filepath.Join(root, "assets", "minecraft", "models", "x.txt"), "x")
	mustWriteText(t, filepath.Join(root, "assets", "other.txt"), "other")

	inner, err := NewDirectoryResourceProvider(root)
	if err != nil {
		t.Fatal(err)
	}
	provider := NewSubPathResourceProvider(inner, "assets/minecraft")

	if !provider.FileExists("models/x.txt") {
		t.Fatal("scoped file was not found")
	}
	if provider.FileExists("../other.txt") {
		t.Fatal("subpath traversal unexpectedly exists")
	}
	files, err := provider.EnumerateFiles("models", "*.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"models/x.txt"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
}

func TestCatsResourceProviderTextRelativeSortedAndTraversal(t *testing.T) {
	cats, err := NewCatsFile(buildCatsArchive(t, map[string]string{
		"dir/b.txt": "b",
		"dir/a.txt": "a",
		"../x.txt":  "bad",
	}))
	if err != nil {
		t.Fatal(err)
	}
	provider := NewCatsResourceProvider(cats, "memory.cats")

	text, err := provider.ReadAllText("dir/a.txt")
	if err != nil || text != "a" {
		t.Fatalf("ReadAllText = %q, %v", text, err)
	}
	relative, err := provider.GetRelativePath("dir/a.txt", "dir")
	if err != nil || relative != "a.txt" {
		t.Fatalf("GetRelativePath = %q, %v", relative, err)
	}
	files, err := provider.EnumerateFiles("dir", "*.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"dir/a.txt", "dir/b.txt"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
	if provider.FileExists("../x.txt") {
		t.Fatal("unsafe cats entry was indexed")
	}
}

func mustWriteText(t *testing.T, path string, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func createZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	defer archive.Close()
	for name, text := range entries {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(text)); err != nil {
			t.Fatal(err)
		}
	}
}

func buildCatsArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var header bytes.Buffer
	binary.Write(&header, binary.BigEndian, Magic)
	header.WriteByte(CurrentVersion)
	writeCatsDirectory(t, &header, "", entries)

	offsets := make(map[string]int)
	var data bytes.Buffer
	names := []string{"dir/a.txt", "dir/b.txt", "../x.txt"}
	for _, name := range names {
		if text, ok := entries[name]; ok {
			offsets[name] = data.Len()
			data.WriteString(text)
		}
	}

	var final bytes.Buffer
	binary.Write(&final, binary.BigEndian, Magic)
	final.WriteByte(CurrentVersion)
	writeCatsDirectoryWithOffsets(t, &final, "", entries, offsets)
	final.Write(data.Bytes())
	return final.Bytes()
}

func writeCatsDirectory(t *testing.T, buf *bytes.Buffer, prefix string, entries map[string]string) {
	t.Helper()
	writeCatsDirectoryWithOffsets(t, buf, prefix, entries, map[string]int{})
}

func writeCatsDirectoryWithOffsets(t *testing.T, buf *bytes.Buffer, prefix string, entries map[string]string, offsets map[string]int) {
	t.Helper()
	if prefix == "" {
		binary.Write(buf, binary.BigEndian, uint16(2))
		writeCatsFile(t, buf, "../x.txt", offsets["../x.txt"], len(entries["../x.txt"]))
		buf.WriteByte(0x01)
		buf.WriteByte(byte(len("dir")))
		buf.WriteString("dir")
		writeCatsDirectoryWithOffsets(t, buf, "dir", entries, offsets)
		return
	}
	binary.Write(buf, binary.BigEndian, uint16(2))
	writeCatsFile(t, buf, "a.txt", offsets["dir/a.txt"], len(entries["dir/a.txt"]))
	writeCatsFile(t, buf, "b.txt", offsets["dir/b.txt"], len(entries["dir/b.txt"]))
}

func writeCatsFile(t *testing.T, buf *bytes.Buffer, name string, offset int, size int) {
	t.Helper()
	buf.WriteByte(0x00)
	buf.WriteByte(byte(len(name)))
	buf.WriteString(name)
	binary.Write(buf, binary.BigEndian, int32(offset))
	binary.Write(buf, binary.BigEndian, int32(size))
	buf.WriteByte(CompressionNone)
}
