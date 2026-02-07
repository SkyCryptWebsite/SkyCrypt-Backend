/*
Minecraft Head Rendering in Rust provided by @mat-1: https://github.com/mat-1/msdsmchr
Rewritten and converted into Go by @DuckySolucky
Original TS version by @DuckySolucky: https://github.com/SkyCryptWebsite/SkyCryptv2/blob/dev/src/lib/server/helper/renderer.ts
Originally Inspired by Crafatar: https://github.com/crafatar/crafatar
*/

package lib

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strconv"
	"strings"
	"sync"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"golang.org/x/sync/singleflight"
)

var textureDir = "assets/static"

var headGroup singleflight.Group

// ========== ITEM RENDERING (Potions & Armor) ==========

func RenderPotion(potionType, hexColor string) ([]byte, error) {
	cachePath := filepath.Join(CACHE_DIR, "potions", fmt.Sprintf("potion_%s_%s.png", potionType, hexColor))
	if _, err := os.Stat(cachePath); err == nil {
		data, err := os.ReadFile(cachePath)
		if err == nil {
			return data, nil
		}

		log.Println("Error reading cached potion texture:", err)
	}

	liquidPath := filepath.Join(textureDir, "potion_overlay.png")

	var bottlePath string
	if potionType == "splash" {
		bottlePath = filepath.Join(textureDir, "splash_potion.png")
	} else {
		bottlePath = filepath.Join(textureDir, "potion.png")
	}

	liquidChan := make(chan imageResult)
	bottleChan := make(chan imageResult)

	go func() {
		img, err := loadImage(liquidPath)
		liquidChan <- imageResult{img, err}
	}()

	go func() {
		img, err := loadImage(bottlePath)
		bottleChan <- imageResult{img, err}
	}()

	liquidResult := <-liquidChan
	bottleResult := <-bottleChan

	if liquidResult.err != nil {
		return nil, fmt.Errorf("failed to load liquid image: %w", liquidResult.err)
	}
	if bottleResult.err != nil {
		return nil, fmt.Errorf("failed to load bottle image: %w", bottleResult.err)
	}

	output, err := renderColoredItem("#"+hexColor, liquidResult.img, bottleResult.img)
	if err != nil {
		return nil, fmt.Errorf("failed to render colored potion: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(CACHE_DIR, "potions"), 0755); err != nil {
		log.Println("Error creating cache directory for potions:", err)
	} else {
		if err := os.WriteFile(cachePath, output, 0644); err != nil {
			log.Println("Error saving potion texture to cache:", err)
		}
	}

	return output, nil
}

func RenderArmor(armorType, hexColor string) ([]byte, error) {
	cachePath := filepath.Join(CACHE_DIR, "leather", fmt.Sprintf("leather_%s_%s.png", armorType, hexColor))
	if _, err := os.Stat(cachePath); err == nil {
		data, err := os.ReadFile(cachePath)
		if err == nil {
			return data, nil
		}

		log.Println("Error reading cached armor texture:", err)
	}

	basePath := filepath.Join(textureDir, fmt.Sprintf("leather_%s.png", armorType))
	overlayPath := filepath.Join(textureDir, fmt.Sprintf("leather_%s_overlay.png", armorType))

	baseChan := make(chan imageResult)
	overlayChan := make(chan imageResult)

	go func() {
		img, err := loadImage(basePath)
		baseChan <- imageResult{img, err}
	}()

	go func() {
		img, err := loadImage(overlayPath)
		overlayChan <- imageResult{img, err}
	}()

	baseResult := <-baseChan
	overlayResult := <-overlayChan

	if baseResult.err != nil {
		return nil, fmt.Errorf("failed to load armor base image: %w", baseResult.err)
	}
	if overlayResult.err != nil {
		return nil, fmt.Errorf("failed to load armor overlay image: %w", overlayResult.err)
	}

	output, err := renderColoredItem("#"+hexColor, baseResult.img, overlayResult.img)
	if err != nil {
		return nil, fmt.Errorf("failed to render colored armor: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(CACHE_DIR, "leather"), 0755); err != nil {
		log.Println("Error creating cache directory for leather armor:", err)
	} else {
		if err := os.WriteFile(cachePath, output, 0644); err != nil {
			log.Println("Error saving leather armor to cache:", err)
		}
	}

	return output, nil
}

func RenderHead(textureId string) []byte {
	cachePath := filepath.Join(CACHE_DIR, "heads", textureId+".png")
	if data, err := os.ReadFile(cachePath); err == nil {
		return data
	}

	result, err, _ := headGroup.Do(textureId, func() (interface{}, error) {
		if data, err := os.ReadFile(cachePath); err == nil {
			return data, nil
		}

		response, err := api.HTTPClient.Get("https://textures.minecraft.net/texture/" + textureId)
		if err != nil {
			log.Println("Error fetching texture:", err)
			return nil, err
		}
		defer func() {
			_ = response.Body.Close()
		}()

		if response.StatusCode != http.StatusOK {
			log.Println("Error fetching texture, status code:", response.StatusCode)
			return nil, fmt.Errorf("texture fetch failed: status %d", response.StatusCode)
		}

		img, err := png.Decode(response.Body)
		if err != nil {
			log.Println("Error decoding texture:", err)
			return nil, err
		}

		var rgbaImg *image.RGBA
		switch v := img.(type) {
		case *image.RGBA:
			rgbaImg = v
		default:
			bounds := img.Bounds()
			rgbaImg = image.NewRGBA(bounds)
			draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)
		}

		head := To3DHead(rgbaImg)

		var buf bytes.Buffer
		if err := png.Encode(&buf, head); err != nil {
			log.Println("Error encoding head to PNG:", err)
			return nil, err
		}
		data := buf.Bytes()

		if err := os.MkdirAll(filepath.Join(CACHE_DIR, "heads"), 0755); err == nil {
			if err := os.WriteFile(cachePath, data, 0644); err != nil {
				log.Println("Error saving head to cache:", err)
			}
		} else {
			log.Println("Error creating cache directory:", err)
		}

		return data, nil
	})

	if err != nil {
		log.Printf("Error rendering head for texture %s: %v", textureId, err)
		return nil
	}

	if result == nil {
		return nil
	}

	b, ok := result.([]byte)
	if !ok {
		log.Printf("Error: unexpected type %T from head render for texture %s", result, textureId)
		return nil
	}
	return b
}

type imageResult struct {
	img image.Image
	err error
}

func renderColoredItem(hexColor string, baseImage, overlayImage image.Image) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 16, 16))

	col, err := parseHexColor(hexColor)
	if err != nil {
		return nil, fmt.Errorf("invalid color format: %w", err)
	}

	fillRect(canvas, col)

	multiplyBlend(canvas, baseImage)

	destinationInBlend(canvas, baseImage)

	draw.Draw(canvas, canvas.Bounds(), overlayImage, image.Point{}, draw.Over)

	return imageToPNGBuffer(canvas)
}

// ========== 3D HEAD RENDERING ==========

const (
	SKEW_A       = 26.0 / 45.0
	SKEW_B       = SKEW_A * 2.0
	SECTION_SIZE = 8
)

var (
	TRANSFORM_TOP_BOTTOM_MATRIX = [9]float32{
		1.0, 1.0, 0.0,
		-SKEW_A, SKEW_A, 0.0,
		0.0, 0.0, 1.0,
	}
	TRANSFORM_FRONT_BACK_MATRIX = [9]float32{
		1.0, 0.0, 0.0,
		-SKEW_A, SKEW_B, SKEW_A,
		0.0, 0.0, 1.0,
	}
	TRANSFORM_RIGHT_LEFT_MATRIX = [9]float32{
		1.0, 0.0, 0.0,
		SKEW_A, SKEW_B, 0.0,
		0.0, 0.0, 1.0,
	}
)

type OverlaySectionOptions struct {
	Size       uint32
	Scale      float32
	X          uint32
	Y          uint32
	Matrix     [9]float32
	TranslateX float32
	TranslateY float32
	Flip       bool
}

type Matrix [9]float32

type Point2D struct {
	X, Y float32
}

func To3DHead(img image.Image) *image.RGBA {
	size := uint32(128)

	out := image.NewRGBA(image.Rect(0, 0, int(size), int(size)))

	// Bottom (base)
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          2,
		Y:          0,
		Matrix:     TRANSFORM_TOP_BOTTOM_MATRIX,
		TranslateX: float32(size)*(-145.0/256.0)/(8.0/8.1) + float32(size)*(10.0/256.0),
		TranslateY: float32(size) * (177.0 / 256.0),
		Flip:       false,
		Scale:      float32(size / 20),
	})

	// Back (base)
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          3,
		Y:          1,
		Matrix:     TRANSFORM_FRONT_BACK_MATRIX,
		TranslateX: float32(size)*(26.0/256.0)*(8.0/9.0) + 10,
		TranslateY: float32(size)*(70.0/256.0)*(8.0/9.0) + 12,
		Flip:       false,
		Scale:      float32(size / 20),
	})

	// Left (base)
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          0,
		Y:          1,
		Matrix:     TRANSFORM_RIGHT_LEFT_MATRIX,
		TranslateX: float32(size)*(231.0/256.0)/(8.0/8.1) - float32(size)*(10.0/256.0) - 6,
		TranslateY: float32(size)*(-56.0/256.0) + 9,
		Flip:       true,
		Scale:      float32(size / 20),
	})

	// Bottom overlay
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          6,
		Y:          0,
		Matrix:     TRANSFORM_TOP_BOTTOM_MATRIX,
		TranslateX: float32(size) * (-145.0 / 256.0),
		TranslateY: float32(size) * (177.0 / 256.0),
		Flip:       false,
		Scale:      float32(size / 20),
	})

	// Back overlay
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          7,
		Y:          1,
		Matrix:     TRANSFORM_FRONT_BACK_MATRIX,
		TranslateX: float32(size) * (26.0 / 256.0),
		TranslateY: float32(size) * (70.0 / 256.0),
		Flip:       false,
		Scale:      float32(size/20) * (9.0 / 8.0),
	})

	// Left overlay
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          6,
		Y:          1,
		Matrix:     TRANSFORM_RIGHT_LEFT_MATRIX,
		TranslateX: float32(size) * (231.0 / 256.0) * (8.0 / 8.1),
		TranslateY: float32(size) * (-56.0 / 256.0),
		Flip:       true,
		Scale:      float32(size/20) * (9.0 / 8.0),
	})

	// Top (base)
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          1,
		Y:          0,
		Matrix:     TRANSFORM_TOP_BOTTOM_MATRIX,
		TranslateX: float32(size) * (-40.0 / 256.0),
		TranslateY: float32(size) * (83.0 / 256.0),
		Flip:       false,
		Scale:      float32(size / 20),
	})

	// Front
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          1,
		Y:          1,
		Matrix:     TRANSFORM_FRONT_BACK_MATRIX,
		TranslateX: float32(size) * (132.5 / 256.0),
		TranslateY: float32(size) * (177.5 / 256.0),
		Flip:       false,
		Scale:      float32(size / 20),
	})

	// Right
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          2,
		Y:          1,
		Matrix:     TRANSFORM_RIGHT_LEFT_MATRIX,
		TranslateX: float32(size) * (121.0 / 256.0),
		TranslateY: float32(size) * (52.0 / 256.0),
		Flip:       true,
		Scale:      float32(size / 20),
	})

	// Front overlay
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          5,
		Y:          1,
		Matrix:     TRANSFORM_FRONT_BACK_MATRIX,
		TranslateX: float32(size) * (132.5 / 256.0) * (8.1 / 8.0),
		TranslateY: float32(size) * (177.5 / 256.0),
		Flip:       false,
		Scale:      float32(size/20) * (9.0 / 8.0),
	})

	// Right overlay
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          4,
		Y:          1,
		Matrix:     TRANSFORM_RIGHT_LEFT_MATRIX,
		TranslateX: float32(size) * (26.0 / 256.0) * (8.0 / 8.1),
		TranslateY: float32(size) * (52.0 / 256.0),
		Flip:       false,
		Scale:      float32(size/20) * (9.0 / 8.0),
	})

	// Top overlay
	overlay3DSection(out, img, &OverlaySectionOptions{
		Size:       size,
		X:          5,
		Y:          0,
		Matrix:     TRANSFORM_TOP_BOTTOM_MATRIX,
		TranslateX: float32(size) * (-40.0 / 256.0) * (8.0 / 8.1),
		TranslateY: float32(size) * (83.0 / 256.0) * (8.0 / 9.0),
		Flip:       false,
		Scale:      float32(size/20) * (9.0 / 8.0),
	})

	return out
}

// ========== SHARED UTILITY FUNCTIONS ==========

var imageCache sync.Map

func loadImage(path string) (image.Image, error) {
	if cached, ok := imageCache.Load(path); ok {
		return cached.(image.Image), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}

	imageCache.Store(path, img)
	return img, nil
}

func parseHexColor(hexColor string) (color.RGBA, error) {
	if len(hexColor) != 7 || hexColor[0] != '#' {
		return color.RGBA{}, fmt.Errorf("invalid hex color format: %s", hexColor)
	}

	rVal, err := strconv.ParseUint(hexColor[1:3], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	gVal, err := strconv.ParseUint(hexColor[3:5], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	bVal, err := strconv.ParseUint(hexColor[5:7], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}

	return color.RGBA{uint8(rVal), uint8(gVal), uint8(bVal), 255}, nil
}

func fillRect(img *image.RGBA, col color.RGBA) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, col)
		}
	}
}

func multiplyBlend(canvas *image.RGBA, src image.Image) {
	bounds := canvas.Bounds()
	srcBounds := src.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if x < srcBounds.Max.X && y < srcBounds.Max.Y {
				canvasColor := canvas.RGBAAt(x, y)
				srcColor := color.RGBAModel.Convert(src.At(x, y)).(color.RGBA)

				// Multiply blend: (canvas * src) / 255
				r := uint8((uint16(canvasColor.R) * uint16(srcColor.R)) / 255)
				g := uint8((uint16(canvasColor.G) * uint16(srcColor.G)) / 255)
				b := uint8((uint16(canvasColor.B) * uint16(srcColor.B)) / 255)
				a := srcColor.A // Keep source alpha

				canvas.Set(x, y, color.RGBA{r, g, b, a})
			}
		}
	}
}

func destinationInBlend(canvas *image.RGBA, src image.Image) {
	bounds := canvas.Bounds()
	srcBounds := src.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if x < srcBounds.Max.X && y < srcBounds.Max.Y {
				canvasColor := canvas.RGBAAt(x, y)
				srcColor := color.RGBAModel.Convert(src.At(x, y)).(color.RGBA)

				// Destination-in: keep canvas color but use source alpha
				canvas.Set(x, y, color.RGBA{
					canvasColor.R,
					canvasColor.G,
					canvasColor.B,
					srcColor.A, // Use source alpha as mask
				})
			} else {
				// Outside source bounds, make transparent
				canvas.Set(x, y, color.RGBA{0, 0, 0, 0})
			}
		}
	}
}
func imageToPNGBuffer(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ========== 3D HEAD RENDERING HELPER FUNCTIONS ==========

func cropSection(img image.Image, x, y uint32) image.Image {
	bounds := img.Bounds()
	x1 := int(x * SECTION_SIZE)
	y1 := int(y * SECTION_SIZE)

	cropped := image.NewRGBA(image.Rect(0, 0, SECTION_SIZE, SECTION_SIZE))
	for dy := 0; dy < SECTION_SIZE; dy++ {
		for dx := 0; dx < SECTION_SIZE; dx++ {
			srcX := x1 + dx
			srcY := y1 + dy

			if srcX >= bounds.Min.X && srcX < bounds.Max.X && srcY >= bounds.Min.Y && srcY < bounds.Max.Y {
				cropped.Set(dx, dy, img.At(srcX, srcY))
			}
		}
	}

	return cropped
}

func (m Matrix) Transform(p Point2D) Point2D {
	x := m[0]*p.X + m[1]*p.Y + m[2]
	y := m[3]*p.X + m[4]*p.Y + m[5]
	w := m[6]*p.X + m[7]*p.Y + m[8]

	if w != 0 {
		x /= w
		y /= w
	}

	return Point2D{X: x, Y: y}
}

func (m Matrix) Multiply(other Matrix) Matrix {
	var result Matrix
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				result[i*3+j] += m[i*3+k] * other[k*3+j]
			}
		}
	}
	return result
}

func Scale(sx, sy float32) Matrix {
	return Matrix{
		sx, 0, 0,
		0, sy, 0,
		0, 0, 1,
	}
}

func Translate(tx, ty float32) Matrix {
	return Matrix{
		1, 0, tx,
		0, 1, ty,
		0, 0, 1,
	}
}

func blendRGBA(bottom, top color.RGBA) color.RGBA {
	if top.A == 0 {
		return bottom
	}
	if top.A == 255 {
		return top
	}

	alpha := float64(top.A) / 255.0
	invAlpha := 1.0 - alpha

	return color.RGBA{
		R: uint8(float64(top.R)*alpha + float64(bottom.R)*invAlpha),
		G: uint8(float64(top.G)*alpha + float64(bottom.G)*invAlpha),
		B: uint8(float64(top.B)*alpha + float64(bottom.B)*invAlpha),
		A: uint8(math.Min(255, float64(top.A)+float64(bottom.A)*invAlpha)),
	}
}

func fastOverlay(bottom *image.RGBA, top *image.RGBA) {
	bounds := bottom.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			topColor := top.RGBAAt(x, y)

			if topColor.A == 0 {
				continue
			}

			bottomColor := bottom.RGBAAt(x, y)
			blended := blendRGBA(bottomColor, topColor)
			bottom.SetRGBA(x, y, blended)
		}
	}
}

func warpImage(src *image.RGBA, matrix Matrix, outputSize uint32) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, int(outputSize), int(outputSize)))

	det := matrix[0]*(matrix[4]*matrix[8]-matrix[5]*matrix[7]) -
		matrix[1]*(matrix[3]*matrix[8]-matrix[5]*matrix[6]) +
		matrix[2]*(matrix[3]*matrix[7]-matrix[4]*matrix[6])

	if math.Abs(float64(det)) < 1e-10 {
		return dst
	}

	invMatrix := Matrix{
		(matrix[4]*matrix[8] - matrix[5]*matrix[7]) / det,
		(matrix[2]*matrix[7] - matrix[1]*matrix[8]) / det,
		(matrix[1]*matrix[5] - matrix[2]*matrix[4]) / det,
		(matrix[5]*matrix[6] - matrix[3]*matrix[8]) / det,
		(matrix[0]*matrix[8] - matrix[2]*matrix[6]) / det,
		(matrix[2]*matrix[3] - matrix[0]*matrix[5]) / det,
		(matrix[3]*matrix[7] - matrix[4]*matrix[6]) / det,
		(matrix[1]*matrix[6] - matrix[0]*matrix[7]) / det,
		(matrix[0]*matrix[4] - matrix[1]*matrix[3]) / det,
	}

	srcBounds := src.Bounds()

	for y := 0; y < int(outputSize); y++ {
		for x := 0; x < int(outputSize); x++ {
			srcPoint := invMatrix.Transform(Point2D{X: float32(x), Y: float32(y)})

			srcX := int(math.Round(float64(srcPoint.X)))
			srcY := int(math.Round(float64(srcPoint.Y)))

			if srcX >= srcBounds.Min.X && srcX < srcBounds.Max.X &&
				srcY >= srcBounds.Min.Y && srcY < srcBounds.Max.Y {
				dst.Set(x, y, src.At(srcX, srcY))
			}
		}
	}

	return dst
}

func overlay3DSection(out *image.RGBA, skin image.Image, opts *OverlaySectionOptions) {
	section := cropSection(skin, opts.X, opts.Y)

	sectionRGBA, ok := section.(*image.RGBA)
	if !ok {
		bounds := section.Bounds()
		sectionRGBA = image.NewRGBA(bounds)
		draw.Draw(sectionRGBA, bounds, section, bounds.Min, draw.Src)
	}

	baseMatrix := Matrix(opts.Matrix)
	translateMatrix := Translate(opts.TranslateX, opts.TranslateY)

	scaleX := opts.Scale
	if opts.Flip {
		scaleX = -scaleX
	}
	scaleMatrix := Scale(scaleX, opts.Scale)

	finalMatrix := baseMatrix.Multiply(translateMatrix).Multiply(scaleMatrix)

	sectionWarped := warpImage(sectionRGBA, finalMatrix, opts.Size)

	fastOverlay(out, sectionWarped)
}

/*
func main() {

	// Render a red splash potion
	pngData, err := RenderPotion("splash", "FF0000")
	if err != nil {
		fmt.Printf("Error rendering potion: %v\n", err)
		return
	}

	// Save to file
	err = os.WriteFile("red_splash_potion.png", pngData, 0644)
	if err != nil {
		fmt.Printf("Error saving potion file: %v\n", err)
		return
	}

	fmt.Println("Potion rendered successfully!")

	// Render a red leather chestplate
	armorData, err := RenderArmor("chestplate", "0000FF")
	if err != nil {
		fmt.Printf("Error rendering armor: %v\n", err)
		return
	}

	// Save armor to file
	err = os.WriteFile("blue_leather_chestplate.png", armorData, 0644)
	if err != nil {
		fmt.Printf("Error saving armor file: %v\n", err)
		return
	}

	fmt.Println("Armor rendered successfully!")

	// Render a 3D head from a skin file
	headData, err := Render3DHead("input2.png")
	if err != nil {
		fmt.Printf("Error rendering 3D head: %v\n", err)
		return
	}

	// Save 3D head to file
	err = os.WriteFile("3d_head.png", headData, 0644)
	if err != nil {
		fmt.Printf("Error saving 3D head file: %v\n", err)
		return
	}

	fmt.Println("3D head rendered successfully!")
}
*/

type RedirectError struct {
	URL string
}

func (e RedirectError) Error() string {
	return "redirect:" + e.URL
}

func RenderItem(itemID string, disabledPacks []string, returnBarrierIfNone bool) ([]byte, error) {
	damage := 0
	if strings.Contains(itemID, ":") {
		splitId := strings.Split(itemID, ":")
		itemID = splitId[0]
		parsedDmg, err := utility.ParseInt(splitId[1])
		if err == nil {
			damage = parsedDmg
		}
	}

	itemData := constants.ITEMS[strings.ToUpper(itemID)]
	TextureItem := models.TextureItem{
		Damage: &itemData.Damage,
		ID:     &itemData.ItemId,
		Tag: skycrypttypes.TextureItemExtraAttributes{
			ExtraAttributes: map[string]interface{}{
				"id": itemData.SkyblockID,
			},
		},
		RawId:   itemID,
		Texture: itemData.TextureId,
	}

	if damage != 0 {
		TextureItem.Damage = &damage
	}

	appliedTexure := ApplyTexture(TextureItem, disabledPacks)
	if appliedTexure.Texture == "" {
		return nil, fmt.Errorf("couldn't find the texture")
	}

	if !returnBarrierIfNone && strings.HasSuffix(appliedTexure.Texture, "/Vanilla/assets/firmskyblock/models/item/barrier.png") {
		return nil, fmt.Errorf("couldn't find the texture")
	}

	// If output is a redirect path (starts with /api/), return redirect error
	if strings.HasPrefix(appliedTexure.Texture, "/api/") {
		return nil, RedirectError{URL: appliedTexure.Texture}
	}

	// If output is a localhost asset, read from disk (performance optimization)
	if strings.Contains(appliedTexure.Texture, "/assets/") && !strings.Contains(appliedTexure.Texture, "/api/") {
		assetsIdx := strings.Index(appliedTexure.Texture, "/assets/")
		if assetsIdx != -1 {
			localPath := appliedTexure.Texture[assetsIdx+1:] // skip the leading slash
			if _, err := os.Stat(localPath); err == nil {
				data, err := os.ReadFile(localPath)
				if err != nil {
					return nil, fmt.Errorf("error reading local asset: %v", err)
				}
				return data, nil
			} else {
				return nil, fmt.Errorf("local asset not found: %s", localPath)
			}
		}

		return nil, fmt.Errorf("invalid localhost asset path: %s", appliedTexure.Texture)
	}

	// Otherwise, fetch from the URL (this shouldn't ever happen but just as a fallback)
	response, err := api.HTTPClient.Get(appliedTexure.Texture)
	if err != nil {
		return nil, fmt.Errorf("error fetching item texture: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching item texture: %v", err)
	}

	img, err := png.Decode(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error decoding item texture: %v", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("error encoding item texture: %v", err)
	}

	return buf.Bytes(), nil
}
