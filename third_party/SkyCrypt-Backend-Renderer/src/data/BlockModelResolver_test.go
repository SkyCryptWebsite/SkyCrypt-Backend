package data

import "testing"

func TestResolveInternalChildElementsReplaceParentElements(t *testing.T) {
	resolver := &BlockModelResolver{
		Definitions: map[string]BlockModelDefinition{
			"item/template_skull": {
				Textures: map[string]string{"parent": "minecraft:item/parent"},
				Elements: []ElementDefinition{{
					From: []float64{0, 0, 0},
					To:   []float64{16, 16, 16},
					Faces: map[string]FaceDefinition{
						"north": {Texture: "#parent"},
					},
				}},
			},
			"helmet_icon:item/crown_of_avarice_model": {
				Parent:   stringPtr("item/template_skull"),
				Textures: map[string]string{"0": "helmet_icon:item/crown_of_avarice_model"},
				Elements: []ElementDefinition{{
					From: []float64{4, 0, 4},
					To:   []float64{12, 8, 12},
					Faces: map[string]FaceDefinition{
						"north": {Texture: "#0"},
					},
				}},
			},
		},
		_cache: make(map[string]BlockModelInstance),
	}

	model := resolver.Resolve("helmet_icon:item/crown_of_avarice_model")
	if model == nil {
		t.Fatal("resolved model is nil")
	}
	if len(model.Elements) != 1 {
		t.Fatalf("resolved element count = %d, want child element only", len(model.Elements))
	}
	if model.Elements[0].From.X != 4 || model.Elements[0].To.X != 12 {
		t.Fatalf("resolved element = %+v, want child crown element", model.Elements[0])
	}
	if model.Textures["parent"] == "" || model.Textures["0"] == "" {
		t.Fatalf("expected inherited and child textures, got %+v", model.Textures)
	}
}

func stringPtr(value string) *string {
	return &value
}
