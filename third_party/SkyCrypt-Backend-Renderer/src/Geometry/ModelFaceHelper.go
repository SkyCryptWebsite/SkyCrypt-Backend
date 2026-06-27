package geometry

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
)

var FaceVertexIndices = map[data.BlockFaceDirection][4]int{
	data.Down:  {4, 0, 1, 5},
	data.Up:    {3, 7, 6, 2},
	data.North: {2, 1, 0, 3},
	data.South: {7, 4, 5, 6},
	data.West:  {3, 0, 4, 7},
	data.East:  {6, 5, 1, 2},
}

func DefaultFaceUv(from, to data.Vector3, direction data.BlockFaceDirection) data.Vector4 {
	switch direction {
	case data.Down:
		return data.Vector4{from.X, float64(16) - to.Z, to.X, float64(16) - from.Z}
	case data.Up:
		return data.Vector4{from.X, from.Z, to.X, to.Z}
	case data.North:
		return data.Vector4{float64(16) - to.X, float64(16) - to.Y, float64(16) - from.X, float64(16) - from.Y}
	case data.South:
		return data.Vector4{from.X, float64(16) - to.Y, to.X, float64(16) - from.Y}
	case data.West:
		return data.Vector4{from.Z, float64(16) - to.Y, to.Z, float64(16) - from.Y}
	case data.East:
		return data.Vector4{float64(16) - to.Z, float64(16) - to.Y, float64(16) - from.Z, float64(16) - from.Y}
	default:
		return data.Vector4{0, 0, 16, 16}
	}
}

func GetU(uv data.Vector4, rotationQuadrant, vertexIndex int) float64 {
	shifted := (vertexIndex + rotationQuadrant) % 4
	if shifted != 0 && shifted != 1 {
		return uv.Z
	}
	return uv.X
}

func GetV(uv data.Vector4, rotationQuadrant, vertexIndex int) float64 {
	shifted := (vertexIndex + rotationQuadrant) % 4
	if shifted != 0 && shifted != 3 {
		return uv.W
	}
	return uv.Y
}

func CreateUvMap(faceUv data.Vector4, faceRotationDegrees int) []src.Vector2 {
	normalizedAngle := ((faceRotationDegrees % 360) + 360) % 360
	quadrant := 0
	switch normalizedAngle {
	case 90:
		quadrant = 1
	case 180:
		quadrant = 2
	case 270:
		quadrant = 3
	default:
		quadrant = 0
	}

	m := make([]src.Vector2, 4)
	for i := 0; i < 4; i++ {
		u := GetU(faceUv, quadrant, i) / float64(16)
		v := GetV(faceUv, quadrant, i) / float64(16)
		m[i] = src.Vector2{X: u, Y: v}
	}

	return m
}
