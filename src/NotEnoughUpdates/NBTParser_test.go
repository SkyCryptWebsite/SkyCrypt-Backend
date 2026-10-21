package notenoughupdates

import (
	"reflect"
	"testing"
)

const aspectOfTheEndNBT = `{ExtraAttributes:{id:"ASPECT_OF_THE_END"},HideFlags:254,Unbreakable:1B,display:{Lore:[0:"§7Damage: §c+100",1:"§8This item can be reforged!",2:"§9§lRARE SWORD"],Name:"§9Aspect of the End"}}`

func TestParseNBT(t *testing.T) {
	got := ParseNBT(aspectOfTheEndNBT)
	want := map[string]any{
		"ExtraAttributes": map[string]any{"id": "ASPECT_OF_THE_END"},
		"HideFlags":       int32(254),
		"Unbreakable":     int8(1),
		"display": map[string]any{
			"Lore": []any{"§7Damage: §c+100", "§8This item can be reforged!", "§9§lRARE SWORD"},
			"Name": "§9Aspect of the End",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}

	got = ParseNBT(`{ a : "x\"y\\z", b:[1s,2L,-3.5d,4f,0.5,7], "c d":{}}`)
	want = map[string]any{
		"a":   `x"y\z`,
		"b":   []any{int16(1), int64(2), -3.5, float32(4), 0.5, int32(7)},
		"c d": map[string]any{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseNBTRejectsBadInput(t *testing.T) {
	for _, input := range []string{`{a:"unterminated}`, `{a:"bad\q"}`, `{a:`} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%q: expected a parse error", input)
				}
			}()
			ParseNBT(input)
		}()
	}
}

func BenchmarkParseNBT(b *testing.B) {
	for b.Loop() {
		ParseNBT(aspectOfTheEndNBT)
	}
}
