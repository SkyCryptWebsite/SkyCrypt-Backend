package minecraftblockrenderer

import (
	"fmt"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src"
	geometry "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/Geometry"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/model"
	"image"
)

func (_minecraftBlockRenderer *MinecraftBlockRenderer) BuildTriangles(blockMModel *data.BlockModelInstance, transform model.Matrix4, applyInventoryLighting bool, blockName string) []VisibleTriangle {
	triangles := make([]VisibleTriangle, 0, len(blockMModel.Elements)*12)

	// Debug: print incoming transform
	// fmt.Printf("DEBUG BuildTriangles transform: %v\n", transform)

	// Console.WriteLine($"Building triangles for block '{blockName}' with {model.Elements.Count} elements.");
	// fmt.Printf("Building triangles for block '%s' with %d elements.\n", blockName, len(blockMModel.Elements))

	for elementIndex, element := range blockMModel.Elements {
		elementTriangles := _minecraftBlockRenderer.BuildTrianglesForElement(blockMModel, element, transform, elementIndex, applyInventoryLighting, blockName)

		// Console.WriteLine($"Built {elementTriangles.Count} triangles for element {elementIndex} of block '{blockName}'.");
		// fmt.Printf("Built %d triangles for element %d of block '%s'.\n", len(elementTriangles), elementIndex, blockName)

		triangles = append(triangles, elementTriangles...)
	}

	return triangles
}
func (_minecraftBlockRenderer *MinecraftBlockRenderer) BuildTrianglesForElement(blockMModel *data.BlockModelInstance, element data.ModelElement, transform model.Matrix4, elementIndex int, applyInventoryLighting bool, blockName string) []VisibleTriangle {
	vertices := BuildElementVertices(element)
	_minecraftBlockRenderer.ApplyElementRotation(element, &vertices)
	results := make([]VisibleTriangle, 0, len(element.Faces)*2)

	for direction, face := range element.Faces {
		textureId := ResolveTexture(face.Texture, blockMModel)
		var texture *image.RGBA

		renderPriority := 0
		if face.TintIndex != nil {
			renderPriority = 1
		}

		if face.TintIndex != nil {
			constantTint := _minecraftBlockRenderer.TryGetConstantTint(textureId, &blockName)
			if constantTint != nil {
				texture = _minecraftBlockRenderer._textureRepository.GetTintedTexture(textureId, *constantTint, float64(ConstantTintStrength), float64(1.0))
			} else if biomeKind := _minecraftBlockRenderer.TryGetBiomeTintKind(textureId, blockName); biomeKind != nil {
				texture = _minecraftBlockRenderer.GetBiomeTintedTexture(textureId, *biomeKind)
			} else {
				fallbackTint := _minecraftBlockRenderer.GetColorFromBlockName(blockName)
				if fallbackTint == nil {
					fallbackTint = _minecraftBlockRenderer.GetColorFromBlockName(textureId)
				}
				if fallbackTint != nil {
					texture = _minecraftBlockRenderer._textureRepository.GetTintedTexture(textureId, *fallbackTint, 1, ColorTintBlend)
				} else {
					texture = _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
				}
			}
		} else {
			texture = _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
		}
		if _minecraftBlockRenderer._textureRepository.IsMissingTexture(texture) {
			fmt.Printf("warning: model %q face %q could not resolve texture %q; using missing texture placeholder\n", blockName, data.BlockFaceDirectionToString(direction), textureId)
		}

		faceUv := _minecraftBlockRenderer.GetFaceUv(face, direction, element)

		// fmt.Printf("Face UV for element %d of block '%s', direction %v: %v\n", elementIndex, blockName, direction, faceUv)

		if face.Rotation == nil {
			n := 0
			face.Rotation = &n
		}

		uvMap := geometry.CreateUvMap(faceUv, *face.Rotation)
		// for (var i = 0; i < 4; i++) {
		// 	Console.WriteLine($"UV {i} for element {elementIndex} of block '{blockName}', direction {direction}: {uvMap[i]}");
		// }
		// for i := 0; i < 4; i++ {
		// 	fmt.Printf("UV %d for element %d of block '%s', direction %v: %v\n", i, elementIndex, blockName, direction, uvMap[i])
		// }

		textureRect := _minecraftBlockRenderer.ComputeTextureRectangle(uvMap, texture)

		rectMinU := float64(textureRect.Min.X) / float64(texture.Bounds().Dx())
		rectRangeU := float64(textureRect.Dx()) / float64(texture.Bounds().Dx())
		rectMinV := float64(textureRect.Min.Y) / float64(texture.Bounds().Dy())
		rectRangeV := float64(textureRect.Dy()) / float64(texture.Bounds().Dy())
		for i := 0; i < 4; i++ {
			if rectRangeU > 1e-6 {
				uvMap[i].X = (uvMap[i].X - rectMinU) / rectRangeU
			} else {
				uvMap[i].X = 0
			}
			if rectRangeV > 1e-6 {
				uvMap[i].Y = (uvMap[i].Y - rectMinV) / rectRangeV
			} else {
				uvMap[i].Y = 0
			}
		}

		indices := geometry.FaceVertexIndices[direction]
		localFace := [4]data.Vector3{}
		for i := 0; i < 4; i++ {
			localFace[i] = vertices[indices[i]]
		}

		// fmt.Printf("Local face vertices for element %d of block '%s', direction %s:\n", elementIndex, blockName, data.BlockFaceDirectionToString(direction))
		// for i := 0; i < 4; i++ {
		// 	fmt.Printf("  Vertex %d: %v (UV: %v)\n", i, localFace[i], uvMap[i])
		// }

		transformed := [4]data.Vector3{}
		for i := 0; i < 4; i++ {
			// 			Console.WriteLine("Passing in: ");
			// for (var j = 0; j < 4; j++) {
			// 	Console.WriteLine($"  Vertex {j}: {localFace[j]} (UV: {uvMap[j]})");
			// }

			// Console.WriteLine($"Using transform matrix for element {elementIndex} of block '{blockName}':");
			// for (var row = 0; row < 4; row++) {
			// 	Console.WriteLine($"  {transform.M11 * (row == 0 ? 1 : 0)} {transform.M12 * (row == 0 ? 1 : 0)} {transform.M13 * (row == 0 ? 1 : 0)} {transform.M14 * (row == 0 ? 1 : 0)}");
			// 	Console.WriteLine($"  {transform.M21 * (row == 1 ? 1 : 0)} {transform.M22 * (row == 1 ? 1 : 0)} {transform.M23 * (row == 1 ? 1 : 0)} {transform.M24 * (row == 1 ? 1 : 0)}");
			// 	Console.WriteLine($"  {transform.M31 * (row == 2 ? 1 : 0)} {transform.M32 * (row == 2 ? 1 : 0)} {transform.M33 * (row == 2 ? 1 : 0)} {transform.M34 * (row == 2 ? 1 : 0)}");
			// 	Console.WriteLine($"  {transform.M41 * (row == 3 ? 1 : 0)} {transform.M42 * (row == 3 ? 1 : 0)} {transform.M43 * (row == 3 ? 1	 : 0)} {transform.M44 * (row == 3 ? 1 : 0)}");
			// }
			// fmt.Printf("Passing in:\n")
			// for j := 0; j < 4; j++ {
			// 	fmt.Printf("  Vertex %d: %v (UV: %v)\n", j, localFace[j], uvMap[j])
			// }
			// fmt.Printf("Using transform matrix for element %d of block '%s':\n", elementIndex, blockName)
			// for row := 0; row < 4; row++ {
			// 	fmt.Printf("  %f %f %f %f\n", transform[row][0], transform[row][1], transform[row][2], transform[row][3])
			// }

			transformed[i] = model.Transform(localFace[i], transform)

			// Console.WriteLine($"Transformed vertex {i} for element {elementIndex} of block '{blockName}', direction {direction}: {transformed[i]} (UV: {uvMap[i]})");
			// fmt.Printf("Transformed vertex %d for element %d of block '%s', direction %s: %v (UV: %v)\n", i, elementIndex, blockName, data.BlockFaceDirectionToString(direction), transformed[i], uvMap[i])
		}

		// Console.WriteLine($"Transofrmed vertices for element {elementIndex} of block '{blockName}':");
		// for (var i = 0; i < 4; i++) {
		// 	Console.WriteLine($"  Vertex {i}: {transformed[i]} (UV: {uvMap[i]})");
		// }
		// fmt.Printf("Transofrmed vertices for element %d of block '%s':\n", elementIndex, blockName)
		// for i := 0; i < 4; i++ {
		// 	fmt.Printf("  Vertex %d: %v (UV: %v)\n", i, transformed[i], uvMap[i])
		// }

		depth := (transformed[0].Z + transformed[1].Z + transformed[2].Z + transformed[3].Z) * 0.25
		triangle1Normal := data.Cross(data.Sub(transformed[1], transformed[0]), data.Sub(transformed[2], transformed[0]))
		triangle2Normal := data.Cross(data.Sub(transformed[2], transformed[0]), data.Sub(transformed[3], transformed[0]))

		// Debug: if normals are effectively zero, print vertices for investigation
		const normalEpsilon = 1e-6
		if triangle1Normal.X*triangle1Normal.X+triangle1Normal.Y*triangle1Normal.Y+triangle1Normal.Z*triangle1Normal.Z <= normalEpsilon {
			// fmt.Printf("DEBUG zero normal triangle1. localFace=%v transformed=%v\n", localFace, transformed)
		}
		if triangle2Normal.X*triangle2Normal.X+triangle2Normal.Y*triangle2Normal.Y+triangle2Normal.Z*triangle2Normal.Z <= normalEpsilon {
			// fmt.Printf("DEBUG zero normal triangle2. localFace=%v transformed=%v\n", localFace, transformed)
		}
		triangle1Centroid := data.Scale(data.Add(data.Add(transformed[0], transformed[1]), transformed[2]), 1.0/3.0)
		triangle2Centroid := data.Scale(data.Add(data.Add(transformed[0], transformed[2]), transformed[3]), 1.0/3.0)
		shadingEnabled := applyInventoryLighting && element.Shade
		triangle1Shading := float64(1)
		if shadingEnabled {
			triangle1Shading = _minecraftBlockRenderer.ComputeInventoryLightingIntensity(triangle1Normal)
		}
		triangle2Shading := float64(1)
		if shadingEnabled {
			triangle2Shading = _minecraftBlockRenderer.ComputeInventoryLightingIntensity(triangle2Normal)
		}

		results = append(results, VisibleTriangle{
			V1:             transformed[0],
			V2:             transformed[1],
			V3:             transformed[2],
			T1:             src.Vector2{X: uvMap[0].X, Y: uvMap[0].Y},
			T2:             src.Vector2{X: uvMap[1].X, Y: uvMap[1].Y},
			T3:             src.Vector2{X: uvMap[2].X, Y: uvMap[2].Y},
			Texture:        texture,
			TextureRect:    textureRect,
			Depth:          depth,
			Normal:         triangle1Normal,
			Centroid:       triangle1Centroid,
			Direction:      direction,
			ElementIndex:   elementIndex,
			RenderPriority: renderPriority,
			Shading:        triangle1Shading,
		})

		// Console.WriteLine($"Added triangle 1 for element {elementIndex} of block '{blockName}' with normal {triangle1Normal} and shading {triangle1Shading}.");
		// fmt.Printf("Added triangle 1 for element %d of block '%s' with normal %v and shading %f.\n", elementIndex, blockName, triangle1Normal, triangle1Shading)

		results = append(results, VisibleTriangle{
			V1:             transformed[0],
			V2:             transformed[2],
			V3:             transformed[3],
			T1:             src.Vector2{X: uvMap[0].X, Y: uvMap[0].Y},
			T2:             src.Vector2{X: uvMap[2].X, Y: uvMap[2].Y},
			T3:             src.Vector2{X: uvMap[3].X, Y: uvMap[3].Y},
			Texture:        texture,
			TextureRect:    textureRect,
			Depth:          depth,
			Normal:         triangle2Normal,
			Centroid:       triangle2Centroid,
			Direction:      direction,
			ElementIndex:   elementIndex,
			RenderPriority: renderPriority,
			Shading:        triangle2Shading,
		})

		// Console.WriteLine($"Added triangle 2 for element {elementIndex} of block '{blockName}' with normal {triangle2Normal} and shading {triangle2Shading}.");
		// fmt.Printf("Added triangle 2 for element %d of block '%s' with normal %v and shading %f.\n", elementIndex, blockName, triangle2Normal, triangle2Shading)
	}

	return results
}

func BuildElementVertices(element data.ModelElement) [8]data.Vector3 {
	min := element.From
	max := element.To

	fx := NormalizeComponent((min.X))
	fy := NormalizeComponent((min.Y))
	fz := NormalizeComponent((min.Z))
	tx := NormalizeComponent((max.X))
	ty := NormalizeComponent((max.Y))
	tz := NormalizeComponent((max.Z))

	return [8]data.Vector3{
		{fx, fy, fz},
		{tx, fy, fz},
		{tx, ty, fz},
		{fx, ty, fz},
		{fx, fy, tz},
		{tx, fy, tz},
		{tx, ty, tz},
		{fx, ty, tz},
	}
}

func NormalizeComponent(value float64) float64 {
	return (value/16.0 - 0.5)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ApplyElementRotation(element data.ModelElement, vertices *[8]data.Vector3) {
	if element.Rotation == nil {
		return
	}

	var axis data.Vector3
	switch element.Rotation.Axis {
	case "x":
		axis = data.Vector3{X: 1, Y: 0, Z: 0}
	case "z":
		axis = data.Vector3{X: 0, Y: 0, Z: 1}
	default:
		axis = data.Vector3{X: 0, Y: 1, Z: 0}
	}

	angle := element.Rotation.AngleInDegrees * DegreesToRadians
	pivot := data.Vector3{
		X: NormalizeComponent(float64(element.Rotation.Origin.X)),
		Y: NormalizeComponent(float64(element.Rotation.Origin.Y)),
		Z: NormalizeComponent(float64(element.Rotation.Origin.Z)),
	}

	rotationMatrix := model.CreateFromAxisAngle(axis, angle)

	for i := range *vertices {
		relative := data.Vector3{
			X: (*vertices)[i].X - pivot.X,
			Y: (*vertices)[i].Y - pivot.Y,
			Z: (*vertices)[i].Z - pivot.Z,
		}
		relative = model.Transform(relative, rotationMatrix)
		(*vertices)[i] = data.Vector3{
			X: relative.X + pivot.X,
			Y: relative.Y + pivot.Y,
			Z: relative.Z + pivot.Z,
		}
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetFaceUv(face data.ModelFace, direction data.BlockFaceDirection, element data.ModelElement) data.Vector4 {
	if face.Uv != nil {
		return *face.Uv
	}

	return geometry.DefaultFaceUv(element.From, element.To, direction)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ComputeTextureRectangle(uvMap []src.Vector2, texture *image.RGBA) image.Rectangle {
	widthFactor := float32(texture.Bounds().Dx())
	heightFactor := float32(texture.Bounds().Dy())

	minU := uvMap[0].X
	maxU := uvMap[0].X
	minV := uvMap[0].Y
	maxV := uvMap[0].Y

	for i := 1; i < len(uvMap); i++ {
		if uvMap[i].X < minU {
			minU = uvMap[i].X
		}
		if uvMap[i].X > maxU {
			maxU = uvMap[i].X
		}
		if uvMap[i].Y < minV {
			minV = uvMap[i].Y
		}
		if uvMap[i].Y > maxV {
			maxV = uvMap[i].Y
		}
	}

	minX := int(float32(minU)*widthFactor + 0.5)
	maxX := int(float32(maxU)*widthFactor + 0.5)
	minY := int(float32(minV)*heightFactor + 0.5)
	maxY := int(float32(maxV)*heightFactor + 0.5)

	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX < minX+1 {
		maxX = minX + 1
	}
	if maxY < minY+1 {
		maxY = minY + 1
	}
	if maxX > texture.Bounds().Dx() {
		maxX = texture.Bounds().Dx()
	}
	if maxY > texture.Bounds().Dy() {
		maxY = texture.Bounds().Dy()
	}

	return image.Rect(minX, minY, maxX, maxY)
}

type Console struct{}

func (c Console) WriteLine(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
