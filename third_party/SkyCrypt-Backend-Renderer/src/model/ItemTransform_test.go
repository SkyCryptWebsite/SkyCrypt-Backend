package model

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"math"
	"testing"
)

func TestTransformAppliesTranslationLikeSystemNumerics(t *testing.T) {
	matrix := CreateTranslation(1.25, -2.5, 3.75)
	got := Transform(data.Vector3{X: 2, Y: 4, Z: 8}, matrix)

	assertClose(t, got.X, 3.25)
	assertClose(t, got.Y, 1.5)
	assertClose(t, got.Z, 11.75)
}

func TestItemTransformBuildMatrixAppliesDisplayTranslation(t *testing.T) {
	transform := ItemTransform{
		Rotation:    data.Vector3{X: 0, Y: 0, Z: 0},
		Translation: data.Vector3{X: 4, Y: -8, Z: 12},
		Scale:       data.Vector3{X: 1, Y: 1, Z: 1},
	}

	got := Transform(data.Vector3{}, transform.BuildMatrix(false))

	assertClose(t, got.X, 0.25)
	assertClose(t, got.Y, -0.5)
	assertClose(t, got.Z, 0.75)
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %f, want %f", got, want)
	}
}
