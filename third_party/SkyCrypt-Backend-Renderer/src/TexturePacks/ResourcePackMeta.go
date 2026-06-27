package texturepacks

type ResourcePackMeta struct {
	Id          string
	Name        string
	Version     string
	Description string
	Authors     []string
	DownloadUrl *string
	SupportsCit bool
	PackFormat  *int
}
