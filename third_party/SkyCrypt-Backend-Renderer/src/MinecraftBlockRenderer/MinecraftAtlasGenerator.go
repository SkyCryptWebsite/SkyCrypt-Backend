package minecraftblockrenderer

import (
	"fmt"
	"image"
	"image/draw"
	"sort"
)

type MinecraftAtlasGenerator struct {
	Renderer *MinecraftBlockRenderer
}

type MinecraftAtlasOptions struct {
	Names   []string
	Size    int
	Cols    int
	PackIds []string
}

type MinecraftAtlasResult struct {
	Image    *image.RGBA
	Manifest MinecraftAtlasManifest
}

type MinecraftAtlasManifest struct {
	TileSize int                   `json:"tile_size"`
	Columns  int                   `json:"columns"`
	Rows     int                   `json:"rows"`
	Width    int                   `json:"width"`
	Height   int                   `json:"height"`
	Entries  []MinecraftAtlasEntry `json:"entries"`
}

type MinecraftAtlasEntry struct {
	Name       string `json:"name"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	ResourceId string `json:"resource_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

type MinecraftAtlasView struct {
	Manifest MinecraftAtlasManifest
	Image    *image.RGBA
}

func NewMinecraftAtlasGenerator(renderer *MinecraftBlockRenderer) *MinecraftAtlasGenerator {
	return &MinecraftAtlasGenerator{Renderer: renderer}
}

func (generator *MinecraftAtlasGenerator) GenerateBlockAtlas(options MinecraftAtlasOptions) (*MinecraftAtlasResult, error) {
	if generator == nil || generator.Renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	names := normalizeAtlasNames(options.Names, generator.Renderer.GetKnownBlockNames())
	return generator.generateAtlas(names, options, func(name string, renderOptions *BlockRenderOptions) (*RenderedResource, error) {
		img := generator.Renderer.RenderBlock(name, *renderOptions)
		if img == nil {
			return nil, fmt.Errorf("failed to render block")
		}
		id := generator.Renderer.ComputeResourceIdInternal(name, *renderOptions, nil)
		return &RenderedResource{Image: img, ResourceId: *id}, nil
	})
}

func (generator *MinecraftAtlasGenerator) GenerateItemAtlas(options MinecraftAtlasOptions) (*MinecraftAtlasResult, error) {
	if generator == nil || generator.Renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	names := normalizeAtlasNames(options.Names, generator.Renderer.GetKnownItemNames())
	return generator.generateAtlas(names, options, func(name string, renderOptions *BlockRenderOptions) (*RenderedResource, error) {
		rendered := generator.Renderer.RenderGuiItemWithResourceId(name, renderOptions)
		if rendered == nil || rendered.Image == nil {
			return nil, fmt.Errorf("failed to render item")
		}
		return rendered, nil
	})
}

func (generator *MinecraftAtlasGenerator) generateAtlas(names []string, options MinecraftAtlasOptions, render func(string, *BlockRenderOptions) (*RenderedResource, error)) (*MinecraftAtlasResult, error) {
	tileSize := options.Size
	if tileSize <= 0 {
		tileSize = 64
	}
	cols := options.Cols
	if cols <= 0 {
		cols = 16
	}
	rows := 0
	if len(names) > 0 {
		rows = (len(names) + cols - 1) / cols
	}
	if rows == 0 {
		rows = 1
	}

	atlas := image.NewRGBA(image.Rect(0, 0, cols*tileSize, rows*tileSize))
	manifest := MinecraftAtlasManifest{
		TileSize: tileSize,
		Columns:  cols,
		Rows:     rows,
		Width:    atlas.Bounds().Dx(),
		Height:   atlas.Bounds().Dy(),
	}

	for index, name := range names {
		x := (index % cols) * tileSize
		y := (index / cols) * tileSize
		entry := MinecraftAtlasEntry{Name: name, X: x, Y: y, Width: tileSize, Height: tileSize}
		renderOptions := DefaultBlockRenderOptions()
		renderOptions.Size = tileSize
		renderOptions.PackIds = options.PackIds
		rendered, err := render(name, &renderOptions)
		if err != nil {
			entry.Error = err.Error()
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		if rendered.ResourceId.ResourceId != "" {
			entry.ResourceId = rendered.ResourceId.ResourceId
		}
		draw.Draw(atlas, image.Rect(x, y, x+tileSize, y+tileSize), rendered.Image, rendered.Image.Bounds().Min, draw.Over)
		manifest.Entries = append(manifest.Entries, entry)
	}

	return &MinecraftAtlasResult{Image: atlas, Manifest: manifest}, nil
}

func normalizeAtlasNames(requested []string, defaults []string) []string {
	names := requested
	if len(names) == 0 {
		names = defaults
	}
	output := append([]string(nil), names...)
	sort.Strings(output)
	return output
}
