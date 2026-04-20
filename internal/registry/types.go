package registry

type SourceTag struct {
	Name      string
	MediaType string
	Digest    string
}

type SourceClient interface {
	ListTags(repository string) ([]SourceTag, error)
	GetManifestMediaType(repository, tag string) (string, error)
	SetProxy(proxyURL string)
}

type HarborArtifact struct {
	Digest    string      `json:"digest"`
	Tags      []HarborTag `json:"tags"`
	MediaType string      `json:"media_type"`
}

type HarborTag struct {
	Name string `json:"name"`
}

type HarborTagInfo struct {
	Name   string
	Digest string
}

type HarborRepoListResponse []HarborRepository

type HarborRepository struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
