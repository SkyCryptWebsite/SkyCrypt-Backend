package store

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/klauspost/compress/gzip"
)

func gzipBase64(t *testing.T, content string) string {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write([]byte(content))
	_ = writer.Close()
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func payloadCount(t *testing.T, container []byte) uint64 {
	t.Helper()
	count, n := binary.Uvarint(container)
	if n <= 0 {
		t.Fatal("bad container header")
	}
	return count
}

func gunzipBase64(t *testing.T, value string) string {
	t.Helper()
	compressed, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestPackUnpackRoundTrip(t *testing.T) {
	inventory := gzipBase64(t, strings.Repeat("inventory nbt ", 40))
	vault := gzipBase64(t, strings.Repeat("vault nbt ", 40))
	notGzip := "H4sI" + strings.Repeat("A", 60)
	// Hypixel writes base64 padding as \u003d.
	escaped := strings.ReplaceAll(gzipBase64(t, strings.Repeat("escaped nbt ", 41)), "=", `\u003d`)
	if !strings.Contains(escaped, `\u003d`) {
		t.Fatal("test payload needs padding")
	}
	body := `{"members":{"a":{"inventory":{"inv_contents":{"type":0,"data":"` + inventory + `"},` +
		`"personal_vault_contents":{"type":0,"data":"` + vault + `"},` +
		`"ender_chest_contents":{"type":0,"data":"` + escaped + `"}}},` +
		`"b":{"note":"H4sI quoted \" text","fake":"` + notGzip + `","escaped":"a\\u0001b"}}}`

	for _, level := range []int{gzip.NoCompression, gzip.BestSpeed} {
		packed, err := Pack([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if got := payloadCount(t, packed); got != 3 {
			t.Fatalf("extracted %d payloads, want 3", got)
		}

		unpacked, err := Unpack(packed, level)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(unpacked) {
			t.Fatalf("unpacked body is not valid JSON: %s", unpacked)
		}
		if strings.Count(string(unpacked), `\u003d`) != strings.Count(escaped, `\u003d`) {
			t.Fatal("escaped padding was not written back")
		}

		var a, b map[string]map[string]map[string]any
		_ = json.Unmarshal([]byte(body), &a)
		_ = json.Unmarshal(unpacked, &b)
		if b["members"]["b"]["fake"] != notGzip || b["members"]["b"]["escaped"] != `a\u0001b` {
			t.Fatalf("non-NBT strings changed: %v", b["members"]["b"])
		}
		for _, name := range []string{"inv_contents", "personal_vault_contents", "ender_chest_contents"} {
			want := gunzipBase64(t, a["members"]["a"]["inventory"].(map[string]any)[name].(map[string]any)["data"].(string))
			got := gunzipBase64(t, b["members"]["a"]["inventory"].(map[string]any)[name].(map[string]any)["data"].(string))
			if got != want {
				t.Fatalf("%s payload changed", name)
			}
		}

		again, err := Pack(unpacked)
		if err != nil || !SameContent(again, packed) {
			t.Fatalf("packing the unpacked body gave a different container")
		}
	}
}

func TestPackLeavesUnusualStringsAlone(t *testing.T) {
	valid := gzipBase64(t, strings.Repeat("x", 100))
	cases := map[string]string{
		"escaped slash": strings.Replace(valid, "/", `\/`, 1),
		"trailing data": base64.StdEncoding.EncodeToString(append(mustDecode(t, valid), 0)),
		"no padding":    strings.TrimRight(valid, "="),
	}
	for name, value := range cases {
		if !strings.Contains(value, `\/`) && name == "escaped slash" {
			continue
		}
		body := `{"data":"` + value + `"}`
		packed, err := Pack([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := payloadCount(t, packed); got != 0 && name != "no padding" {
			t.Fatalf("%s: extracted %d payloads, want 0", name, got)
		}
		unpacked, err := Unpack(packed, gzip.BestSpeed)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if payloadCount(t, packed) == 0 && string(unpacked) != body {
			t.Fatalf("%s: body changed", name)
		}
	}
}

func TestPackStoresBodiesWithMarkerBytesVerbatim(t *testing.T) {
	body := "{\"a\":\"\x01\",\"data\":\"" + gzipBase64(t, "nbt nbt nbt nbt nbt nbt nbt") + "\"}"
	packed, err := Pack([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got := payloadCount(t, packed); got != 0 {
		t.Fatalf("extracted %d payloads from a body with a marker byte", got)
	}
	unpacked, err := Unpack(packed, gzip.BestSpeed)
	if err != nil || string(unpacked) != body {
		t.Fatalf("body changed: %q", unpacked)
	}
}

func mustDecode(t *testing.T, value string) []byte {
	t.Helper()
	out, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSplitProfileDropsRequesterFields(t *testing.T) {
	raw := []byte(`{"profile_id":"abc","selected":true,"cute_name":"Apple","members":{"x":{"a":[1,{"b":"}"}]}}}`)
	id, cuteName, object, err := SplitProfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if id != "abc" || cuteName != "Apple" {
		t.Fatalf("id = %q, cute name = %q", id, cuteName)
	}
	want := `{"profile_id":"abc","members":{"x":{"a":[1,{"b":"}"}]}}}`
	if string(object) != want {
		t.Fatalf("object = %s", object)
	}
}
