package src

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"image"
	"image/color"
	"math"
	"sort"
)

// Face represents the 6 faces of a cube
type Face string

const (
	FaceRight  Face = "right"
	FaceLeft   Face = "left"
	FaceTop    Face = "top"
	FaceBottom Face = "bottom"
	FaceFront  Face = "front"
	FaceBack   Face = "back"
)

type Vector2 struct {
	X float64
	Y float64
}

type Rectangle struct {
	X      int
	Y      int
	Width  int
	Height int
}

type RenderOptions struct {
	Size               int
	YawInDegrees       float64
	PitchInDegrees     float64
	RollInDegrees      float64
	PerspectiveAmount  float64
	ShowOverlay        bool
	EnableAntiAliasing bool
}

func NewRenderOptions(size int, yaw float64, pitch float64, roll float64) *RenderOptions {
	return &RenderOptions{
		Size:               size,
		YawInDegrees:       yaw,
		PitchInDegrees:     pitch,
		RollInDegrees:      roll,
		PerspectiveAmount:  0,
		ShowOverlay:        true,
		EnableAntiAliasing: true,
	}
}

type IsometricSide int

const (
	IsometricSideLeft IsometricSide = iota
	IsometricSideRight
)

type IsometricRenderOptions struct {
	Size               int
	Side               IsometricSide
	ShowOverlay        bool
	EnableAntiAliasing bool
}

func NewIsometricRenderOptions(size int) *IsometricRenderOptions {
	return &IsometricRenderOptions{
		Size:               size,
		Side:               IsometricSideRight,
		ShowOverlay:        true,
		EnableAntiAliasing: true,
	}
}

var BaseMappings = map[Face]Rectangle{
	FaceRight:  {X: 0, Y: 8, Width: 8, Height: 8},
	FaceFront:  {X: 8, Y: 8, Width: 8, Height: 8},
	FaceLeft:   {X: 16, Y: 8, Width: 8, Height: 8},
	FaceBack:   {X: 24, Y: 8, Width: 8, Height: 8},
	FaceTop:    {X: 8, Y: 0, Width: 8, Height: 8},
	FaceBottom: {X: 16, Y: 0, Width: 8, Height: 8},
}

var OverlayMappings = map[Face]Rectangle{
	FaceRight:  {X: 32, Y: 8, Width: 8, Height: 8},
	FaceFront:  {X: 40, Y: 8, Width: 8, Height: 8},
	FaceLeft:   {X: 48, Y: 8, Width: 8, Height: 8},
	FaceBack:   {X: 56, Y: 8, Width: 8, Height: 8},
	FaceTop:    {X: 40, Y: 0, Width: 8, Height: 8},
	FaceBottom: {X: 48, Y: 0, Width: 8, Height: 8},
}

var InventoryLightDirection = normalizeVector3(data.Vector3{X: 0.55, Y: -1, Z: 1.8})

const (
	InventoryAmbientStrength = 0.2
	InventoryDiffuseStrength = 0.8
)

func normalizeVector3(v data.Vector3) data.Vector3 {
	length := float64(math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z))
	if length == 0 {
		return v
	}
	return data.Vector3{X: v.X / length, Y: v.Y / length, Z: v.Z / length}
}

var Vertices = []data.Vector3{
	// Back face vertices (z = -0.5)
	{X: -0.5, Y: -0.5, Z: -0.5}, // 0: bottom-left-back
	{X: 0.5, Y: -0.5, Z: -0.5},  // 1: bottom-right-back
	{X: 0.5, Y: 0.5, Z: -0.5},   // 2: top-right-back
	{X: -0.5, Y: 0.5, Z: -0.5},  // 3: top-left-back

	// Front face vertices (z = 0.5)
	{X: -0.5, Y: -0.5, Z: 0.5}, // 4: bottom-left-front
	{X: 0.5, Y: -0.5, Z: 0.5},  // 5: bottom-right-front
	{X: 0.5, Y: 0.5, Z: 0.5},   // 6: top-right-front
	{X: -0.5, Y: 0.5, Z: 0.5},  // 7: top-left-front
}

var StandardUvMap = []Vector2{
	{X: 1, Y: 0}, {X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1},
}

var BackFaceUvMap = []Vector2{
	{X: 0, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 0}, {X: 0, Y: 0},
}

var BottomFaceUvMap = []Vector2{
	{X: 1, Y: 1}, {X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0},
}

type FaceData struct {
	Face     Face
	Vertices []data.Vector3
	UvMap    []Vector2
}

// Face definitions for the cube
var FaceDefinitions = []FaceData{
	// Front face (+Z)
	{Face: FaceFront, Vertices: []data.Vector3{Vertices[7], Vertices[6], Vertices[5], Vertices[4]}, UvMap: StandardUvMap},
	// Back face (-Z)
	{Face: FaceBack, Vertices: []data.Vector3{Vertices[0], Vertices[1], Vertices[2], Vertices[3]}, UvMap: BackFaceUvMap},
	// Right face (+X)
	{Face: FaceRight, Vertices: []data.Vector3{Vertices[6], Vertices[2], Vertices[1], Vertices[5]}, UvMap: StandardUvMap},
	// Left face (-X)
	{Face: FaceLeft, Vertices: []data.Vector3{Vertices[3], Vertices[7], Vertices[4], Vertices[0]}, UvMap: StandardUvMap},
	// Top face (+Y)
	{Face: FaceTop, Vertices: []data.Vector3{Vertices[3], Vertices[2], Vertices[6], Vertices[7]}, UvMap: StandardUvMap},
	// Bottom face (-Y)
	{Face: FaceBottom, Vertices: []data.Vector3{Vertices[4], Vertices[5], Vertices[1], Vertices[0]}, UvMap: BottomFaceUvMap},
}

type VisibleTriangle struct {
	V1          data.Vector3
	V2          data.Vector3
	V3          data.Vector3
	T1          Vector2
	T2          Vector2
	T3          Vector2
	TextureRect Rectangle
	Depth       float64
	Shading     float64
	IsOverlay   bool
}

type BarycentricData struct {
	V0    Vector2
	V1    Vector2
	D00   float64
	D01   float64
	D11   float64
	Denom float64
}

type PerspectiveParams struct {
	Amount         float64
	CameraDistance float64
	FocalLength    float64
}

func (_minecraftHeadRenderer *RenderOptions) RenderIsometricHead(options *IsometricRenderOptions, skin image.RGBA) image.RGBA {
	// Isometric view: showing front, right, and top faces (or left if specified)
	const isometricRightYaw = -135.0
	const isometricLeftYaw = 45.0
	const isometricPitch = 30.0
	const isometricRoll = 0.0

	fullOptions := &RenderOptions{
		Size:               options.Size,
		YawInDegrees:       isometricRightYaw,
		PitchInDegrees:     isometricPitch,
		RollInDegrees:      isometricRoll,
		ShowOverlay:        options.ShowOverlay,
		EnableAntiAliasing: options.EnableAntiAliasing,
	}

	if options.Side == IsometricSideLeft {
		fullOptions.YawInDegrees = isometricLeftYaw
	}

	return _minecraftHeadRenderer.RenderHead(fullOptions, skin)
}

func (_minecraftHeadRenderer *RenderOptions) RenderHead(options *RenderOptions, skin image.RGBA) image.RGBA {
	deg2Rad := float64(math.Pi / 180)
	transform := _minecraftHeadRenderer.createRotationMatrix(
		options.YawInDegrees*deg2Rad,
		options.PitchInDegrees*deg2Rad,
		options.RollInDegrees*deg2Rad,
	)

	initialCapacity := len(FaceDefinitions) * 4
	if !options.ShowOverlay {
		initialCapacity = len(FaceDefinitions) * 2
	}
	visibleTriangles := make([]VisibleTriangle, 0, initialCapacity)

	_minecraftHeadRenderer.processFaces(FaceDefinitions, transform, false, &visibleTriangles)

	if options.ShowOverlay {
		overlayTransform := _minecraftHeadRenderer.createRotationMatrix(
			options.YawInDegrees*deg2Rad,
			options.PitchInDegrees*deg2Rad,
			options.RollInDegrees*deg2Rad,
		)
		for i := range overlayTransform {
			for j := range overlayTransform[i] {
				overlayTransform[i][j] *= 1.125
			}
		}
		_minecraftHeadRenderer.processFaces(FaceDefinitions, overlayTransform, true, &visibleTriangles)
	}

	// Sort triangles by depth (back to front)
	sort.Slice(visibleTriangles, func(i, j int) bool {
		return visibleTriangles[i].Depth > visibleTriangles[j].Depth
	})

	canvas := image.NewRGBA(image.Rect(0, 0, options.Size, options.Size))
	scale := float64(options.Size) / 1.75
	offset := Vector2{X: float64(options.Size) / 2, Y: float64(options.Size) / 2}
	depthBuffer := make([]float64, options.Size*options.Size)
	for i := range depthBuffer {
		depthBuffer[i] = float64(math.Inf(1))
	}

	triangleOrder := 0
	const DepthBiasPerTriangle = 1e-4

	var perspectiveParams *PerspectiveParams
	if options.PerspectiveAmount > 0.01 {
		perspectiveParams = &PerspectiveParams{
			Amount:         options.PerspectiveAmount,
			CameraDistance: 10,
			FocalLength:    10,
		}
	}

	for _, tri := range visibleTriangles {
		p1 := _minecraftHeadRenderer.projectToScreen(tri.V1, scale, offset, perspectiveParams)
		p2 := _minecraftHeadRenderer.projectToScreen(tri.V2, scale, offset, perspectiveParams)
		p3 := _minecraftHeadRenderer.projectToScreen(tri.V3, scale, offset, perspectiveParams)

		depthBias := float64(triangleOrder) * DepthBiasPerTriangle
		triangleOrder++

		_minecraftHeadRenderer.rasterizeTriangle(
			canvas,
			depthBuffer,
			depthBias,
			tri.V1.Z,
			tri.V2.Z,
			tri.V3.Z,
			p1,
			p2,
			p3,
			tri.T1,
			tri.T2,
			tri.T3,
			skin,
			tri.TextureRect,
			tri.Shading,
			tri.IsOverlay)
	}

	if options.EnableAntiAliasing {
		ApplyFxaa(canvas)
	}

	return *canvas
}

func (_minecraftHeadRenderer *RenderOptions) createRotationMatrix(yaw, pitch, roll float64) [4][4]float64 {
	cosY := float64(math.Cos(float64(yaw)))
	sinY := float64(math.Sin(float64(yaw)))
	cosP := float64(math.Cos(float64(pitch)))
	sinP := float64(math.Sin(float64(pitch)))
	cosR := float64(math.Cos(float64(roll)))
	sinR := float64(math.Sin(float64(roll)))

	return [4][4]float64{
		{cosY*cosR + sinY*sinP*sinR, -cosY*sinR + sinY*sinP*cosR, sinY * cosP, 0},
		{cosP * sinR, cosP * cosR, -sinP, 0},
		{-sinY*cosR + cosY*sinP*sinR, sinY*sinR + cosY*sinP*cosR, cosY * cosP, 0},
		{0, 0, 0, 1},
	}
}

func (_minecraftHeadRenderer *RenderOptions) processFaces(faces []FaceData, transform [4][4]float64, isOverlay bool, triangles *[]VisibleTriangle) {
	mappings := BaseMappings
	if isOverlay {
		mappings = OverlayMappings
	}

	for _, face := range faces {
		texRect := mappings[face.Face]

		transformed := make([]data.Vector3, 4)
		for i := 0; i < 4; i++ {
			transformed[i] = _minecraftHeadRenderer.transformVector3(face.Vertices[i], transform)
		}

		v1 := vector3Subtract(transformed[1], transformed[0])
		v2 := vector3Subtract(transformed[2], transformed[0])
		normal := vector3Cross(v1, v2)
		shading := _minecraftHeadRenderer.ComputeInventoryLightingIntensity(normal)

		if !isOverlay && normal.Z < 0 {
			continue
		}

		depth := (transformed[0].Z + transformed[1].Z + transformed[2].Z + transformed[3].Z) * 0.25

		*triangles = append(*triangles, VisibleTriangle{
			V1:          transformed[0],
			V2:          transformed[1],
			V3:          transformed[2],
			T1:          face.UvMap[0],
			T2:          face.UvMap[1],
			T3:          face.UvMap[2],
			TextureRect: texRect,
			Depth:       depth,
			Shading:     shading,
			IsOverlay:   isOverlay,
		})

		*triangles = append(*triangles, VisibleTriangle{
			V1:          transformed[0],
			V2:          transformed[2],
			V3:          transformed[3],
			T1:          face.UvMap[0],
			T2:          face.UvMap[2],
			T3:          face.UvMap[3],
			TextureRect: texRect,
			Depth:       depth,
			Shading:     shading,
			IsOverlay:   isOverlay,
		})
	}
}

func (_minecraftHeadRenderer *RenderOptions) transformVector3(v data.Vector3, m [4][4]float64) data.Vector3 {
	x := v.X*m[0][0] + v.Y*m[1][0] + v.Z*m[2][0] + m[3][0]
	y := v.X*m[0][1] + v.Y*m[1][1] + v.Z*m[2][1] + m[3][1]
	z := v.X*m[0][2] + v.Y*m[1][2] + v.Z*m[2][2] + m[3][2]
	return data.Vector3{X: x, Y: y, Z: z}
}

func vector3Subtract(a, b data.Vector3) data.Vector3 {
	return data.Vector3{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}

func vector3Cross(a, b data.Vector3) data.Vector3 {
	return data.Vector3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func (_minecraftHeadRenderer *RenderOptions) ComputeInventoryLightingIntensity(normal data.Vector3) float64 {
	const normalEpsilon = 1e-6
	lengthSquared := normal.X*normal.X + normal.Y*normal.Y + normal.Z*normal.Z
	if lengthSquared <= normalEpsilon {
		return 1
	}

	length := float64(math.Sqrt(float64(lengthSquared)))
	normalized := data.Vector3{X: normal.X / length, Y: normal.Y / length, Z: normal.Z / length}

	if normalized.Y >= 0.6 {
		return 1
	}

	lightContribution0 := math.Max(0, float64(normalized.X*InventoryLightDirection.X+normalized.Y*InventoryLightDirection.Y+normalized.Z*InventoryLightDirection.Z))
	intensity := InventoryAmbientStrength + InventoryDiffuseStrength*math.Min(1, lightContribution0)
	return float64(math.Max(0.2, math.Min(float64(intensity), 1)))
}

func (_minecraftHeadRenderer *RenderOptions) projectToScreen(point data.Vector3, scale float64, offset Vector2, perspectiveParams *PerspectiveParams) Vector2 {
	if perspectiveParams == nil {
		return Vector2{X: point.X*scale + offset.X, Y: -point.Y*scale + offset.Y}
	}

	perspectiveFactor := perspectiveParams.FocalLength / (perspectiveParams.CameraDistance - point.Z)
	perspX := point.X * perspectiveFactor
	perspY := point.Y * perspectiveFactor

	orthoX := point.X
	orthoY := point.Y

	finalX := orthoX + (perspX-orthoX)*perspectiveParams.Amount
	finalY := orthoY + (perspY-orthoY)*perspectiveParams.Amount

	return Vector2{
		X: finalX*scale + offset.X,
		Y: -finalY*scale + offset.Y,
	}
}

func (_minecraftHeadRenderer *RenderOptions) rasterizeTriangle(
	canvas *image.RGBA,
	depthBuffer []float64,
	depthBias float64,
	z1, z2, z3 float64,
	p1, p2, p3 Vector2,
	t1, t2, t3 Vector2,
	skin image.RGBA, textureRect Rectangle,
	shadingFactor float64,
	isOverlay bool) {
	area := (p2.X-p1.X)*(p3.Y-p1.Y) - (p3.X-p1.X)*(p2.Y-p1.Y)
	if math.Abs(float64(area)) < 0.01 {
		return
	}

	// Pre-calculate values that are constant for every pixel in the triangle.
	v0 := Vector2{X: p2.X - p1.X, Y: p2.Y - p1.Y}
	v1 := Vector2{X: p3.X - p1.X, Y: p3.Y - p1.Y}
	d00 := v0.X*v0.X + v0.Y*v0.Y
	d01 := v0.X*v1.X + v0.Y*v1.Y
	d11 := v1.X*v1.X + v1.Y*v1.Y
	denom := d00*d11 - d01*d01

	// If the denominator is zero, the triangle is degenerate (a line or point).
	if math.Abs(float64(denom)) < 1e-6 {
		return
	}

	minX := int(math.Max(0, math.Min(math.Min(float64(p1.X), float64(p2.X)), float64(p3.X))))
	minY := int(math.Max(0, math.Min(math.Min(float64(p1.Y), float64(p2.Y)), float64(p3.Y))))
	maxX := int(math.Min(float64(canvas.Bounds().Dx()-1), math.Ceil(math.Max(math.Max(float64(p1.X), float64(p2.X)), float64(p3.X)))))
	maxY := int(math.Min(float64(canvas.Bounds().Dy()-1), math.Ceil(math.Max(math.Max(float64(p1.Y), float64(p2.Y)), float64(p3.Y)))))

	textWidth := textureRect.Width - 1
	textHeight := textureRect.Height - 1

	width := canvas.Bounds().Dx()
	const depthTestEpsilon = 1e-6
	const alphaThreshold = 10

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			point := Vector2{X: float64(x) + 0.5, Y: float64(y) + 0.5}
			bary := getBarycentric(p1, point, v0, v1, d00, d01, d11, denom)

			const epsilon = 1e-5
			if bary.X < -epsilon || bary.Y < -epsilon || bary.Z < -epsilon {
				continue
			}

			depth := z1*bary.X + z2*bary.Y + z3*bary.Z - depthBias

			texCoord := Vector2{
				X: t1.X*bary.X + t2.X*bary.Y + t3.X*bary.Z,
				Y: t1.Y*bary.X + t2.Y*bary.Y + t3.Y*bary.Z,
			}

			texX := int(math.Max(0, math.Min(float64(texCoord.X*float64(textureRect.Width)), float64(textWidth))))
			texY := int(math.Max(0, math.Min(float64(texCoord.Y*float64(textureRect.Height)), float64(textHeight))))

			sampled := skin.RGBAAt(textureRect.X+texX, textureRect.Y+texY)
			if !isOverlay {
				sampled.A = 255
			}

			isTransparent := sampled.A <= alphaThreshold
			if isTransparent && isOverlay {
				continue
			}

			bufferIndex := y*width + x
			if depth >= depthBuffer[bufferIndex]-depthTestEpsilon {
				continue
			}

			depthBuffer[bufferIndex] = depth
			if shadingFactor >= 0.999 && shadingFactor <= 1.001 {
				canvas.SetRGBA(x, y, sampled)
			} else {
				canvas.SetRGBA(x, y, applyShading(sampled, shadingFactor))
			}
		}
	}

}

func applyShading(original color.RGBA, shadingFactor float64) color.RGBA {
	factor := float64(math.Max(float64(shadingFactor), 0))
	if math.Abs(float64(factor-1)) <= 1e-4 {
		return original
	}

	scaledR := int(math.Round(float64(float64(original.R) * factor)))
	scaledG := int(math.Round(float64(float64(original.G) * factor)))
	scaledB := int(math.Round(float64(float64(original.B) * factor)))

	r := uint8(maxInt(0, minInt(255, scaledR)))
	g := uint8(maxInt(0, minInt(255, scaledG)))
	b := uint8(maxInt(0, minInt(255, scaledB)))

	return color.RGBA{R: r, G: g, B: b, A: original.A}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getBarycentric(p1, p Vector2, v0, v1 Vector2, d00, d01, d11, denom float64) data.Vector3 {
	v2 := Vector2{X: p.X - p1.X, Y: p.Y - p1.Y}
	d20 := v2.X*v0.X + v2.Y*v0.Y
	d21 := v2.X*v1.X + v2.Y*v1.Y

	v := (d11*d20 - d01*d21) / denom
	w := (d00*d21 - d01*d20) / denom
	u := 1.0 - v - w

	return data.Vector3{X: u, Y: v, Z: w}
}
