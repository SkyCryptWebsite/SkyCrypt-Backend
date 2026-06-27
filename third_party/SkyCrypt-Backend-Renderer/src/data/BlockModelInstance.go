package data

type BlockFaceDirection int

const (
	North BlockFaceDirection = iota
	South
	East
	West
	Up
	Down
)

func BlockFaceDirectionToString(dir BlockFaceDirection) string {
	switch dir {
	case North:
		return "North"
	case South:
		return "South"
	case East:
		return "East"
	case West:
		return "West"
	case Up:
		return "Up"
	case Down:
		return "Down"
	default:
		return "Unknown"
	}
}

type Vector3 struct {
	X float64
	Y float64
	Z float64
}

func Cross(a, b Vector3) Vector3 {
	return Vector3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func Sub(a, b Vector3) Vector3 {
	return Vector3{
		X: a.X - b.X,
		Y: a.Y - b.Y,
		Z: a.Z - b.Z,
	}
}

func Add(a, b Vector3) Vector3 {
	return Vector3{
		X: a.X + b.X,
		Y: a.Y + b.Y,
		Z: a.Z + b.Z,
	}
}

func Scale(v Vector3, s float64) Vector3 {
	return Vector3{
		X: v.X * s,
		Y: v.Y * s,
		Z: v.Z * s,
	}
}

type Vector4 struct {
	X float64
	Y float64
	Z float64
	W float64
}

type BlockModelInstance struct {
	Name        string
	ParentChain []string
	Textures    map[string]string
	Display     map[string]*TransformDefinition
	Elements    []ModelElement
}

func (b *BlockModelInstance) GetDisplayTransform(name string) *TransformDefinition {
	if b == nil || b.Display == nil {
		return nil
	}
	return b.Display[name]
}

type ModelElement struct {
	From     Vector3
	To       Vector3
	Rotation *ElementRotation
	Faces    map[BlockFaceDirection]ModelFace
	Shade    bool
}

type ElementRotation struct {
	AngleInDegrees float64
	Origin         Vector3
	Axis           string
	Rescale        bool
}

type ModelFace struct {
	Texture   string
	Uv        *Vector4
	Rotation  *int
	TintIndex *int
	CullFace  *string
}
