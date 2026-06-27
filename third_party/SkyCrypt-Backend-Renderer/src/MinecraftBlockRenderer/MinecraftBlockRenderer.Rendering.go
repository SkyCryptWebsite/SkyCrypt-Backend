package minecraftblockrenderer

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/model"
	"image"
	"image/color"
	"math"
	"sort"
)

var DefaultGuiTransform = &data.TransformDefinition{
	Rotation:    &[]float64{30, 225, 0},
	Translation: &[]float64{0, 0, 0},
	Scale:       &[]float64{0.625, 0.625, 0.625},
}

var DefaultGuiScaleMagnitude = ComputeTransformScaleMagnitude(DefaultGuiTransform)

var DefaultGuiScaleNormalization = float64(1)

func init() {
	if DefaultGuiScaleMagnitude > 1e-6 {
		DefaultGuiScaleNormalization = 1 / DefaultGuiScaleMagnitude
	}
}

var InventoryLightDirection = normalizeVector3([]float64{-0.55, -1, 1.8})

const InventoryDiffuseStrength = 0.8
const InventoryAmbientStrength = 0.2

const DegreesToRadians = math.Pi / 180

// ? SMTH?

type VisibleTriangle struct {
	V1, V2, V3     data.Vector3
	T1, T2, T3     src.Vector2
	Texture        *image.RGBA
	TextureRect    image.Rectangle
	Depth          float64
	Normal         data.Vector3
	Centroid       data.Vector3
	Direction      data.BlockFaceDirection
	ElementIndex   int
	RenderPriority int
	Shading        float64
}

type Bounds struct {
	MinX float64
	MaxX float64
	MinY float64
	MaxY float64
}

type BarycentricData struct {
	V0    src.Vector2
	V1    src.Vector2
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

type CullTarget struct {
	ElementIndex  int
	FaceDirection data.BlockFaceDirection
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) RenderModel(blockModel *data.BlockModelInstance, options BlockRenderOptions, blockName *string) *image.RGBA {
	// Ensure sensible defaults for fields that commonly get omitted by callers
	if options.AdditionalScale == 0 {
		options.AdditionalScale = 1
	}

	guiTransform := options.OverrideGuiTransform
	if guiTransform == nil && options.UseGuiTransform {
		guiTransform = blockModel.GetDisplayTransform("gui")
	}

	if guiTransform == nil {
		guiTransform = DefaultGuiTransform
	}

	displayTransform := BuildDisplayTransform(guiTransform, true)
	// fmt.Printf("DEBUG | %+v\n", displayTransform)

	displayTransformWithoutScale := BuildDisplayTransform(guiTransform, false)

	additionalRotation := model.MulMatrix(
		model.CreateRotationX((options.RollInDegrees * DegreesToRadians)),
		model.MulMatrix(
			model.CreateRotationY((options.YawInDegrees*DegreesToRadians)),
			model.CreateRotationZ((options.PitchInDegrees*DegreesToRadians)),
		),
	)

	scaleMatrix := model.CreateScaleWithFloat((options.AdditionalScale))
	translationVector := data.Vector3{
		X: options.AdditionalTranslation.X / 16.0,
		Y: options.AdditionalTranslation.Y / 16.0,
		Z: options.AdditionalTranslation.Z / 16.0,
	}
	translationMatrix := model.CreateTranslation(translationVector.X, translationVector.Y, translationVector.Z)

	totalTransform := model.MulMatrix(model.MulMatrix(displayTransform, additionalRotation), model.MulMatrix(scaleMatrix, translationMatrix))

	// Debug: print matrix components used to build totalTransform
	// fmt.Printf("DEBUG displayTransform: %v\n", displayTransform)
	// fmt.Printf("DEBUG additionalRotation: %v\n", additionalRotation)
	// fmt.Printf("DEBUG scaleMatrix: %v\n", scaleMatrix)
	// fmt.Printf("DEBUG translationMatrix: %v\n", translationMatrix)
	// fmt.Printf("DEBUG totalTransform: %v\n", totalTransform)
	referenceTransform := model.MulMatrix(model.MulMatrix(displayTransformWithoutScale, additionalRotation), translationMatrix)

	applyInventoryLighting := options.UseGuiTransform || options.OverrideGuiTransform != nil
	triangles := _minecraftBlockRenderer.BuildTriangles(blockModel, totalTransform, applyInventoryLighting, *blockName)
	// Console.WriteLine($"Built {triangles.Count} triangles for block '{blockName}' before culling.");
	// fmt.Printf("Built %d triangles for block '%s' before culling\n", len(triangles), *blockName)

	triangles = CullBackfaces(triangles)

	// for i, tri := range triangles {
	// 	fmt.Printf("Triangle %d: V1=%v T1=%v, V2=%v T2=%v, V3=%v T3=%v, Depth=%f, Normal=%v, Centroid=%v, Direction=%v, ElementIndex=%d, RenderPriority=%d, Shading=%f\n",
	// 		i, tri.V1, tri.T1, tri.V2, tri.T2, tri.V3, tri.T3, tri.Depth, tri.Normal, tri.Centroid, tri.Direction, tri.ElementIndex, tri.RenderPriority, tri.Shading)
	// }

	// fmt.Printf("Total triangles to render for block '%s': %d\n", *blockName, len(triangles))
	if len(triangles) == 0 {
		return image.NewRGBA(image.Rect(0, 0, options.Size, options.Size))
	}

	sort.SliceStable(triangles, func(i, j int) bool {
		if triangles[i].Depth != triangles[j].Depth {
			return triangles[i].Depth > triangles[j].Depth
		}
		return triangles[i].RenderPriority < triangles[j].RenderPriority
	})

	bounds := ComputeBounds(triangles)
	referenceBounds := ComputeReferenceBounds(referenceTransform)
	padding := options.Padding
	if padding < 0 {
		padding = 0
	} else if padding > 0.4 {
		padding = 0.4
	}

	dimensionX := bounds.MaxX - bounds.MinX
	dimensionY := bounds.MaxY - bounds.MinY
	dimension := dimensionX
	if dimensionY > dimensionX {
		dimension = dimensionY
	}
	if dimension < 1e-5 {
		dimension = 1
	}

	referenceDimensionX := referenceBounds.MaxX - referenceBounds.MinX
	referenceDimensionY := referenceBounds.MaxY - referenceBounds.MinY
	referenceDimension := referenceDimensionX
	if referenceDimensionY > referenceDimensionX {
		referenceDimension = referenceDimensionY
	}
	if referenceDimension < 1e-5 {
		referenceDimension = dimension
	}

	availableSize := (float64(options.Size)) * (1 - padding*2)
	scale := availableSize / float64(referenceDimension) * float64(DefaultGuiScaleNormalization)

	translation := guiTransform.Translation
	if translation == nil {
		translation = &[]float64{0, 0, 0}
	}

	hasExplicitTranslation := false
	for _, t := range *translation {
		if math.Abs(float64(t)) > 0.1 {
			hasExplicitTranslation = true
			break
		}
	}

	center := src.Vector2{X: 0, Y: 0}
	if !hasExplicitTranslation {
		center.X = (bounds.MinX + bounds.MaxX) * 0.5
		center.Y = (bounds.MinY + bounds.MaxY) * 0.5
	}

	offset := src.Vector2{X: float64(options.Size) / 2, Y: float64(options.Size) / 2}

	var perspective *PerspectiveParams
	if options.PerspectiveAmount > 0.01 {
		perspective = &PerspectiveParams{
			Amount:         float64(options.PerspectiveAmount),
			CameraDistance: 10,
			FocalLength:    10,
		}
	}

	canvas := image.NewRGBA(image.Rect(0, 0, options.Size, options.Size))
	depthBuffer := make([]float64, options.Size*options.Size)
	for i := range depthBuffer {
		depthBuffer[i] = float64(math.Inf(-1))
	}

	triangleOrder := 0
	const DepthBiasPerTriangle = 1e-4

	for _, tri := range triangles {
		centeredV1 := data.Vector3{X: tri.V1.X - center.X, Y: tri.V1.Y - center.Y, Z: tri.V1.Z}
		centeredV2 := data.Vector3{X: tri.V2.X - center.X, Y: tri.V2.Y - center.Y, Z: tri.V2.Z}
		centeredV3 := data.Vector3{X: tri.V3.X - center.X, Y: tri.V3.Y - center.Y, Z: tri.V3.Z}

		p1 := ProjectToScreen(centeredV1, float64(scale), offset, perspective)
		p2 := ProjectToScreen(centeredV2, float64(scale), offset, perspective)
		p3 := ProjectToScreen(centeredV3, float64(scale), offset, perspective)

		depthBias := float64(triangleOrder) * DepthBiasPerTriangle
		triangleOrder++

		RasterizeTriangle(
			canvas,
			depthBuffer,
			depthBias,
			centeredV1.Z,
			centeredV2.Z,
			centeredV3.Z,
			p1,
			p2,
			p3,
			tri.T1,
			tri.T2,
			tri.T3,
			tri.Texture,
			tri.TextureRect,
			tri.Shading,
		)
	}

	if options.EnableAntiAliasing {
		src.ApplyFxaa(canvas)
	}

	return canvas
}

func CullBackfaces(triangles []VisibleTriangle) []VisibleTriangle {
	const NormalLengthThreshold = 1e-6
	const DotCullThreshold = 5e-3
	cameraForward := data.Vector3{X: 0, Y: 0, Z: -1}

	for i := len(triangles) - 1; i >= 0; i-- {
		triangle := triangles[i]

		normal := triangle.Normal
		lengthSquared := normal.X*normal.X + normal.Y*normal.Y + normal.Z*normal.Z
		// Console.WriteLine($"normal LengthSquared: {normal.LengthSquared()}, NormalLengthThreshold: {NormalLengthThreshold}");
		// fmt.Printf("normal LengthSquared: %f, NormalLengthThreshold: %f\n", lengthSquared, NormalLengthThreshold)
		if lengthSquared < NormalLengthThreshold {
			continue
		}

		dot := normal.X*cameraForward.X + normal.Y*cameraForward.Y + normal.Z*cameraForward.Z
		// Console.WriteLine($"Dot product of normal and camera forward: {dot}, DotCullThreshold: {DotCullThreshold}");
		// fmt.Printf("Dot product of normal and camera forward: %f, DotCullThreshold: %f\n", dot, DotCullThreshold)
		if dot > DotCullThreshold {
			triangles = append(triangles[:i], triangles[i+1:]...)
		}
	}

	return triangles
}

func BuildDisplayTransform(transform *data.TransformDefinition, includeScale bool) model.Matrix4 {
	if transform == nil {
		// fmt.Printf("Transform is nil, using identity matrix\n")
		return model.IdentityMatrix()
	}

	// Apply C# defaults: if nil, use [0, 0, 0] for rotation/translation, [1, 1, 1] for scale
	rotation := transform.Rotation
	if rotation == nil || len(*rotation) < 3 {
		rotation = &[]float64{0, 0, 0}
	}

	translation := transform.Translation
	if translation == nil || len(*translation) < 3 {
		translation = &[]float64{0, 0, 0}
	}

	scaleComponents := transform.Scale
	if scaleComponents == nil {
		scaleComponents = &[]float64{1, 1, 1}
	}

	scaleX := float64(1)
	if len(*scaleComponents) > 0 {
		scaleX = (*scaleComponents)[0]
	}

	scaleY := scaleX
	if len(*scaleComponents) > 1 {
		scaleY = (*scaleComponents)[1]
	}

	scaleZ := scaleX
	if len(*scaleComponents) > 2 {
		scaleZ = (*scaleComponents)[2]
	}

	if !includeScale {
		scaleX = 1
		scaleY = 1
		scaleZ = 1
	}

	itemTransform := model.ItemTransform{
		Rotation:    data.Vector3{X: (*rotation)[0], Y: (*rotation)[1], Z: (*rotation)[2]},
		Translation: data.Vector3{X: (*translation)[0], Y: (*translation)[1], Z: (*translation)[2]},
		Scale:       data.Vector3{X: scaleX, Y: scaleY, Z: scaleZ},
	}

	// for r := 0; r < 4; r++ {
	// 	fmt.Printf("ItemTransform - Rotation: %v, Translation: %v, Scale: %v\n", itemTransform.Rotation, itemTransform.Translation, itemTransform.Scale)
	// }

	output := itemTransform.BuildMatrix(false)
	// for (int r = 0; r < 4; r++)
	// {
	// 	Console.WriteLine($"Display Transform Matrix Row {r}: {output.M11 * (r == 0 ? 1 : 0)} {output.M12 * (r == 0 ? 1 : 0)} {output.M13 * (r == 0 ? 1 : 0)} {output.M14 * (r == 0 ? 1 : 0)}");
	// 	Console.WriteLine($"Display Transform Matrix Row {r}: {output.M21 * (r == 1 ? 1 : 0)} {output.M22 * (r == 1 ? 1 : 0)} {output.M23 * (r == 1 ? 1 : 0)} {output.M24 * (r == 1 ? 1 : 0)}");
	// 	Console.WriteLine($"Display Transform Matrix Row {r}: {output.M31 * (r == 2 ? 1 : 0)} {output.M32 * (r == 2 ? 1 : 0)} {output.M33 * (r == 2 ? 1 : 0)} {output.M34 * (r == 2 ? 1 : 0)}");
	// 	Console.WriteLine($"Display Transform Matrix Row {r}: {output.M41 * (r == 3 ? 1 : 0)} {output.M42 * (r == 3 ? 1 : 0)} {output.M43 * (r == 3 ? 1	 : 0)} {output.M44 * (r == 3 ? 1 : 0)}");
	// }
	// for r := 0; r < 4; r++ {
	// 	fmt.Printf("Display Transform Matrix Row %d: %f %f %f %f\n", r, output[r][0], output[r][1], output[r][2], output[r][3])
	// }

	return output
}

func ComputeTransformScaleMagnitude(transform *data.TransformDefinition) float64 {
	if transform == nil || len(*transform.Scale) == 0 {
		return 1
	}

	scaleX := math.Abs(float64((*transform.Scale)[0]))
	scaleY := scaleX
	scaleZ := scaleX

	if len(*transform.Scale) > 1 {
		scaleY = math.Abs(float64((*transform.Scale)[1]))
	}
	if len(*transform.Scale) > 2 {
		scaleZ = math.Abs(float64((*transform.Scale)[2]))
	}

	max := math.Max(scaleX, math.Max(scaleY, scaleZ))
	if max > 1e-6 {
		return float64(max)
	}
	return 1
}

func normalizeVector3(vec []float64) []float64 {
	if len(vec) < 3 {
		return []float64{0, 0, 0}
	}
	x := vec[0]
	y := vec[1]
	z := vec[2]
	magnitude := float64(math.Sqrt(float64(x*x + y*y + z*z)))
	if magnitude > 1e-6 {
		return []float64{x / magnitude, y / magnitude, z / magnitude}
	}
	return []float64{0, 0, 0}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ComputeInventoryLightingIntensity(normal data.Vector3) float64 {
	const normalEpsilon = 1e-6
	lengthSquared := normal.X*normal.X + normal.Y*normal.Y + normal.Z*normal.Z
	if lengthSquared <= normalEpsilon {
		return 1
	}

	magnitude := float64(math.Sqrt(float64(lengthSquared)))
	normalized := data.Vector3{
		X: normal.X / magnitude,
		Y: normal.Y / magnitude,
		Z: normal.Z / magnitude,
	}

	if normalized.Y >= 0.6 {
		return 1
	}

	lightContribution0 := normalized.X*InventoryLightDirection[0] + normalized.Y*InventoryLightDirection[1] + normalized.Z*InventoryLightDirection[2]
	if lightContribution0 < 0 {
		lightContribution0 = 0
	}
	intensity := InventoryAmbientStrength + InventoryDiffuseStrength*float64(math.Min(1, float64(lightContribution0)))
	if intensity < 0.2 {
		intensity = 0.2
	} else if intensity > 1 {
		intensity = 1
	}
	return intensity
}

func ComputeBounds(triangles []VisibleTriangle) Bounds {
	minX := float64(math.MaxFloat32)
	minY := float64(math.MaxFloat32)
	maxX := float64(-math.MaxFloat32)
	maxY := float64(-math.MaxFloat32)

	update := func(v data.Vector3) {
		if v.X < minX {
			minX = v.X
		}
		if v.X > maxX {
			maxX = v.X
		}
		if v.Y < minY {
			minY = v.Y
		}
		if v.Y > maxY {
			maxY = v.Y
		}
	}

	for _, tri := range triangles {
		update(tri.V1)
		update(tri.V2)
		update(tri.V3)
	}

	return Bounds{
		MinX: minX,
		MaxX: maxX,
		MinY: minY,
		MaxY: maxY,
	}
}

func ComputeReferenceBounds(transform model.Matrix4) Bounds {
	corners := []data.Vector3{
		{X: -0.5, Y: -0.5, Z: -0.5},
		{X: 0.5, Y: -0.5, Z: -0.5},
		{X: 0.5, Y: 0.5, Z: -0.5},
		{X: -0.5, Y: 0.5, Z: -0.5},
		{X: -0.5, Y: -0.5, Z: 0.5},
		{X: 0.5, Y: -0.5, Z: 0.5},
		{X: 0.5, Y: 0.5, Z: 0.5},
		{X: -0.5, Y: 0.5, Z: 0.5},
	}

	minX := float64(math.MaxFloat32)
	minY := float64(math.MaxFloat32)
	maxX := float64(-math.MaxFloat32)
	maxY := float64(-math.MaxFloat32)

	for _, corner := range corners {
		transformed := model.Transform(corner, transform)
		if transformed.X < minX {
			minX = transformed.X
		}
		if transformed.X > maxX {
			maxX = transformed.X
		}
		if transformed.Y < minY {
			minY = transformed.Y
		}
		if transformed.Y > maxY {
			maxY = transformed.Y
		}
	}

	if math.IsInf(float64(minX), 0) || math.IsInf(float64(minY), 0) || math.IsInf(float64(maxX), 0) || math.IsInf(float64(maxY), 0) {
		return Bounds{
			MinX: -0.5,
			MaxX: 0.5,
			MinY: -0.5,
			MaxY: 0.5,
		}
	}

	return Bounds{
		MinX: minX,
		MaxX: maxX,
		MinY: minY,
		MaxY: maxY,
	}
}

func ProjectToScreen(point data.Vector3, scale float64, offset src.Vector2, perspective *PerspectiveParams) src.Vector2 {
	if perspective == nil {
		return src.Vector2{X: point.X*scale + offset.X, Y: -point.Y*scale + offset.Y}
	}

	perspectiveFactor := perspective.FocalLength / (perspective.CameraDistance - point.Z)
	perspX := point.X * perspectiveFactor
	perspY := point.Y * perspectiveFactor

	orthoX := point.X
	orthoY := point.Y

	finalX := orthoX + (perspX-orthoX)*perspective.Amount
	finalY := orthoY + (perspY-orthoY)*perspective.Amount

	return src.Vector2{X: finalX*scale + offset.X, Y: -finalY*scale + offset.Y}
}

func RasterizeTriangle(
	canvas *image.RGBA,
	depthBuffer []float64,
	depthBias float64,
	z1, z2, z3 float64,
	p1, p2, p3 src.Vector2,
	t1, t2, t3 src.Vector2,
	texture *image.RGBA,
	textureRect image.Rectangle,
	shadingFactor float64,
) {
	area := (p2.X-p1.X)*(p3.Y-p1.Y) - (p3.X-p1.X)*(p2.Y-p1.Y)
	if math.Abs(float64(area)) < 0.01 {
		return
	}

	v0 := src.Vector2{X: p2.X - p1.X, Y: p2.Y - p1.Y}
	v1 := src.Vector2{X: p3.X - p1.X, Y: p3.Y - p1.Y}
	d00 := v0.X*v0.X + v0.Y*v0.Y
	d01 := v0.X*v1.X + v0.Y*v1.Y
	d11 := v1.X*v1.X + v1.Y*v1.Y
	denom := d00*d11 - d01*d01

	if math.Abs(float64(denom)) < 1e-6 {
		return
	}

	baryData := BarycentricData{
		V0:    v0,
		V1:    v1,
		D00:   d00,
		D01:   d01,
		D11:   d11,
		Denom: denom,
	}

	minX := int(math.Max(0, math.Min(math.Min(float64(p1.X), float64(p2.X)), float64(p3.X))))
	minY := int(math.Max(0, math.Min(math.Min(float64(p1.Y), float64(p2.Y)), float64(p3.Y))))
	maxX := int(math.Min(float64(canvas.Bounds().Dx()-1), math.Ceil(math.Max(math.Max(float64(p1.X), float64(p2.X)), float64(p3.X)))))
	maxY := int(math.Min(float64(canvas.Bounds().Dy()-1), math.Ceil(math.Max(math.Max(float64(p1.Y), float64(p2.Y)), float64(p3.Y)))))

	texWidth := textureRect.Dx() - 1
	texHeight := textureRect.Dy() - 1

	width := canvas.Bounds().Dx()
	const depthTestEpsilon = 1e-6
	const alphaThreshold = 10

	for y := minY; y <= maxY; y++ {
		rowOffset := y * width
		for x := minX; x <= maxX; x++ {
			point := src.Vector2{X: float64(x) + 0.5, Y: float64(y) + 0.5}
			bary := GetBarycentric(p1, point, baryData)

			const epsilon = 1e-4
			if bary.X < -epsilon || bary.Y < -epsilon || bary.Z < -epsilon {
				continue
			}

			depth := z1*bary.X + z2*bary.Y + z3*bary.Z - depthBias

			texCoord := src.Vector2{
				X: t1.X*bary.X + t2.X*bary.Y + t3.X*bary.Z,
				Y: t1.Y*bary.X + t2.Y*bary.Y + t3.Y*bary.Z,
			}

			texX := int(math.Max(0, math.Min(float64(texCoord.X*float64(textureRect.Dx())), float64(texWidth))))
			texY := int(math.Max(0, math.Min(float64(texCoord.Y*float64(textureRect.Dy())), float64(texHeight))))

			color := texture.At(textureRect.Min.X+texX, textureRect.Min.Y+texY)
			_, _, _, a := color.RGBA()
			if a>>8 <= alphaThreshold {
				continue
			}

			bufferIndex := rowOffset + x
			if depth <= depthBuffer[bufferIndex]+depthTestEpsilon {
				continue
			}

			depthBuffer[bufferIndex] = depth

			if shadingFactor >= 0.999 && shadingFactor <= 1.001 {
				canvas.Set(x, y, color)
			} else {
				canvas.Set(x, y, ApplyShading(color, shadingFactor))
			}
		}
	}
}

func GetBarycentric(p1, p src.Vector2, bdata BarycentricData) data.Vector3 {
	v2 := src.Vector2{X: p.X - p1.X, Y: p.Y - p1.Y}
	d20 := v2.X*bdata.V0.X + v2.Y*bdata.V0.Y
	d21 := v2.X*bdata.V1.X + v2.Y*bdata.V1.Y

	v := (bdata.D11*d20 - bdata.D01*d21) / bdata.Denom
	w := (bdata.D00*d21 - bdata.D01*d20) / bdata.Denom
	u := 1 - v - w

	return data.Vector3{X: u, Y: v, Z: w}
}

func ApplyShading(original color.Color, shadingFactor float64) color.RGBA {
	factor := shadingFactor
	if factor < 0 {
		factor = 0
	}
	if math.Abs(float64(factor-1)) <= 1e-4 {
		return color.RGBAModel.Convert(original).(color.RGBA)
	}

	r, g, b, a := original.RGBA()
	return color.RGBA{
		R: uint8(float64(r>>8) * factor),
		G: uint8(float64(g>>8) * factor),
		B: uint8(float64(b>>8) * factor),
		A: uint8(a >> 8),
	}
}
