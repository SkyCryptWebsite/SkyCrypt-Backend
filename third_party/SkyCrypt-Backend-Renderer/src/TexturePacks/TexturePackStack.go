package texturepacks

type TexturePackStack struct {
	Packs        []RegisteredResourcePack
	OverlayRoots []PackOverlayRoot
	Fingerprint  string

	SupportsCit bool
}

type PackOverlayRoot struct {
	Path   string
	PackId string
}
