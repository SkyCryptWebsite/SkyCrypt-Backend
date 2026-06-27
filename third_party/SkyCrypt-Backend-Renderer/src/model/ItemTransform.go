package model

import (
	"math"

	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
)

type Matrix4 [4][4]float64

func IdentityMatrix() Matrix4 {
	var m Matrix4
	for i := 0; i < 4; i++ {
		m[i][i] = 1
	}
	return m
}

func MulMatrix(a, b Matrix4) Matrix4 {
	var r Matrix4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			var s float64
			for k := 0; k < 4; k++ {
				s += a[i][k] * b[k][j]
			}
			r[i][j] = s
		}
	}
	return r
}

func CreateTranslation(tx, ty, tz float64) Matrix4 {
	m := IdentityMatrix()
	m[3][0] = tx
	m[3][1] = ty
	m[3][2] = tz
	return m
}

func CreateScale(s data.Vector3) Matrix4 {
	m := IdentityMatrix()
	m[0][0] = s.X
	m[1][1] = s.Y
	m[2][2] = s.Z
	return m
}

func CreateScaleWithFloat(s float64) Matrix4 {
	m := IdentityMatrix()
	m[0][0] = s
	m[1][1] = s
	m[2][2] = s
	return m
}

func CreateRotationX(rad float64) Matrix4 {
	m := IdentityMatrix()
	c := float64(math.Cos((rad)))
	s := float64(math.Sin((rad)))
	// Use transposed form to match the project's matrix/vector convention
	m[1][1] = c
	m[2][1] = -s
	m[1][2] = s
	m[2][2] = c
	return m
}

func CreateRotationY(rad float64) Matrix4 {
	m := IdentityMatrix()
	c := float64(math.Cos((rad)))
	s := float64(math.Sin((rad)))
	// Transposed form
	m[0][0] = c
	m[0][2] = -s
	m[2][0] = s
	m[2][2] = c
	return m
}

func CreateRotationZ(rad float64) Matrix4 {
	m := IdentityMatrix()
	c := float64(math.Cos((rad)))
	s := float64(math.Sin((rad)))
	// Transposed form
	m[0][0] = c
	m[1][0] = -s
	m[0][1] = s
	m[1][1] = c
	return m
}

type ItemTransform struct {
	Rotation    data.Vector3
	Translation data.Vector3
	Scale       data.Vector3
}

var NoTransform = ItemTransform{Rotation: data.Vector3{0, 0, 0}, Translation: data.Vector3{0, 0, 0}, Scale: data.Vector3{1, 1, 1}}

func (t ItemTransform) equals(o ItemTransform) bool {
	return t.Rotation == o.Rotation && t.Translation == o.Translation && t.Scale == o.Scale
}

// BuildMatrix returns a 4x4 transformation matrix equivalent to the C# implementation.
func (t ItemTransform) BuildMatrix(isLeftHand bool) Matrix4 {
	if t.equals(NoTransform) {
		return IdentityMatrix()
	}

	translationX := t.Translation.X
	rotationY := t.Rotation.Y
	rotationZ := t.Rotation.Z
	if isLeftHand {
		translationX = -translationX
		rotationY = -rotationY
		rotationZ = -rotationZ
	}

	translationMatrix := CreateTranslation(translationX/16.0, t.Translation.Y/16.0, t.Translation.Z/16.0)

	degToRad := float64(math.Pi / 180.0)
	rotationMatrix := MulMatrix(CreateRotationZ(rotationZ*degToRad), MulMatrix(CreateRotationY(rotationY*degToRad), CreateRotationX(t.Rotation.X*degToRad)))

	scaleMatrix := CreateScale(t.Scale)

	// scale * rotation * translation (row-major order)
	return MulMatrix(scaleMatrix, MulMatrix(rotationMatrix, translationMatrix))
}

func MultiplyMatrix(a, b Matrix4) Matrix4 {
	var r Matrix4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			var s float64
			for k := 0; k < 4; k++ {
				s += a[i][k] * b[k][j]
			}
			r[i][j] = s
		}
	}
	return r
}

func CreateFromAxisAngle(axis data.Vector3, angle float64) Matrix4 {
	c := float64(math.Cos((angle)))
	s := float64(math.Sin((angle)))
	t := 1 - c

	x := axis.X
	y := axis.Y
	z := axis.Z

	m := IdentityMatrix()
	// Use the transposed form to match the C# matrix/vector convention
	m[0][0] = t*x*x + c
	m[1][0] = t*x*y - s*z
	m[2][0] = t*x*z + s*y

	m[0][1] = t*x*y + s*z
	m[1][1] = t*y*y + c
	m[2][1] = t*y*z - s*x

	m[0][2] = t*x*z - s*y
	m[1][2] = t*y*z + s*x
	m[2][2] = t*z*z + c

	return m
}

// Summary:
//
//	Transforms a vector by the specified Quaternion rotation value.
//
// Parameters:
//
//	value:
//	  The vector to rotate.
//
//	rotation:
//	  The rotation to apply.
//
// Returns:
//
//	The transformed vector.
func Transform(relative data.Vector3, rotationMatrix Matrix4) data.Vector3 {
	x := relative.X*rotationMatrix[0][0] + relative.Y*rotationMatrix[1][0] + relative.Z*rotationMatrix[2][0] + rotationMatrix[3][0]
	y := relative.X*rotationMatrix[0][1] + relative.Y*rotationMatrix[1][1] + relative.Z*rotationMatrix[2][1] + rotationMatrix[3][1]
	z := relative.X*rotationMatrix[0][2] + relative.Y*rotationMatrix[1][2] + relative.Z*rotationMatrix[2][2] + rotationMatrix[3][2]
	return data.Vector3{X: x, Y: y, Z: z}
}
