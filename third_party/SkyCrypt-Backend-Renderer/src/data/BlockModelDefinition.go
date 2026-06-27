package data

type BlockModelDefinition struct {
	Parent           *string                        `json:"parent,omitempty"`
	Textures         map[string]string              `json:"textures,omitempty"`
	Display          map[string]TransformDefinition `json:"display,omitempty"`
	Elements         []ElementDefinition            `json:"elements,omitempty"`
	GuiLight         *string                        `json:"gui_light,omitempty"`
	AmbientOcclusion *bool                          `json:"ambientocclusion,omitempty"`
}

type TransformDefinition struct {
	Rotation    *[]float64 `json:"rotation,omitempty"`
	Translation *[]float64 `json:"translation,omitempty"`
	Scale       *[]float64 `json:"scale,omitempty"`
}

type ElementDefinition struct {
	From     []float64                  `json:"from,omitempty"`
	To       []float64                  `json:"to,omitempty"`
	Rotation *ElementRotationDefinition `json:"rotation,omitempty"`
	Faces    map[string]FaceDefinition  `json:"faces,omitempty"`
	Shade    *bool                      `json:"shade,omitempty"`
}

type ElementRotationDefinition struct {
	Angle   float64   `json:"angle"`
	Axis    string    `json:"axis"`
	Origin  []float64 `json:"origin,omitempty"`
	Rescale *bool     `json:"rescale,omitempty"`
}

type FaceDefinition struct {
	Uv        []float64 `json:"uv,omitempty"`
	Texture   string    `json:"texture"`
	Rotation  *int      `json:"rotation,omitempty"`
	TintIndex *int      `json:"tintindex,omitempty"`
	CullFace  *string   `json:"cullface,omitempty"`
}
