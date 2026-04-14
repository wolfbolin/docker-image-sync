package registry

type DockerHubTag struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Digest    string `json:"digest"`
}

type DockerHubTagsResponse struct {
	Count   int            `json:"count"`
	Results []DockerHubTag `json:"results"`
}

type HarborArtifact struct {
	Digest string      `json:"digest"`
	Tags   []HarborTag `json:"tags"`
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
