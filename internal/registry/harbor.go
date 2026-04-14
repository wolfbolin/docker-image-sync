package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/wolfbolin/sync-docker/internal/logger"
)

type HarborClient struct {
	httpClient *http.Client
	registry   string
}

func NewHarborClient(registry string, proxy string) *HarborClient {
	transport := &http.Transport{}
	if proxy != "" {
		if proxyURL, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &HarborClient{
		httpClient: &http.Client{Transport: transport},
		registry:   registry,
	}
}

func (c *HarborClient) ListTagsWithDigest(project, name string) ([]HarborTagInfo, error) {
	var allTags []HarborTagInfo
	encodedName := strings.ReplaceAll(name, "/", "%252F")

	for page := 1; ; page++ {
		apiURL := fmt.Sprintf("https://%s/api/v2.0/projects/%s/repositories/%s/artifacts?page=%d&page_size=100",
			c.registry, project, encodedName, page)

		logger.Debug("GET %s", apiURL)

		resp, err := c.httpClient.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("request harbor artifacts: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("harbor API returned status %d", resp.StatusCode)
		}

		var artifacts []HarborArtifact
		if err := json.NewDecoder(resp.Body).Decode(&artifacts); err != nil {
			return allTags, nil
		}

		for _, a := range artifacts {
			for _, t := range a.Tags {
				allTags = append(allTags, HarborTagInfo{Name: t.Name, Digest: a.Digest})
			}
		}

		if len(artifacts) < 100 {
			break
		}
	}

	return allTags, nil
}

func (c *HarborClient) ListTags(project, name string) ([]string, error) {
	tags, err := c.ListTagsWithDigest(project, name)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names, nil
}

func (c *HarborClient) ListRepositories(project string) ([]string, error) {
	var repos []string

	for page := 1; ; page++ {
		apiURL := fmt.Sprintf("https://%s/api/v2.0/projects/%s/repositories?page=%d&page_size=100",
			c.registry, project, page)

		logger.Debug("GET %s", apiURL)

		resp, err := c.httpClient.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("request harbor repositories: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("harbor API returned status %d", resp.StatusCode)
		}

		var result HarborRepoListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return repos, nil
		}

		for _, r := range result {
			repos = append(repos, r.Name)
		}

		if len(result) < 100 {
			break
		}
	}

	return repos, nil
}
