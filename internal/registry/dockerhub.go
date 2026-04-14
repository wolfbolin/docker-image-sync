package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/wolfbolin/sync-docker/internal/logger"
)

type DockerHubClient struct {
	httpClient *http.Client
	mu         sync.Mutex
	cache      map[string][]DockerHubTag
}

func NewDockerHubClient(proxy string) *DockerHubClient {
	transport := &http.Transport{}
	if proxy != "" {
		if proxyURL, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &DockerHubClient{
		httpClient: &http.Client{Transport: transport},
		cache:      make(map[string][]DockerHubTag),
	}
}

func (c *DockerHubClient) ListTags(project, name string) ([]DockerHubTag, error) {
	cacheKey := project + "/" + name

	c.mu.Lock()
	if tags, ok := c.cache[cacheKey]; ok {
		c.mu.Unlock()
		return tags, nil
	}
	c.mu.Unlock()

	var allTags []DockerHubTag

	for page := 1; ; page++ {
		apiURL := fmt.Sprintf("https://hub.docker.com/v2/namespaces/%s/repositories/%s/tags?page=%d&page_size=100",
			project, name, page)

		logger.Debug("GET %s", apiURL)

		resp, err := c.httpClient.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("request docker hub tags: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("docker hub API returned status %d", resp.StatusCode)
		}

		var result DockerHubTagsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		allTags = append(allTags, result.Results...)

		if page*100 >= result.Count {
			break
		}
	}

	c.mu.Lock()
	c.cache[cacheKey] = allTags
	c.mu.Unlock()

	return allTags, nil
}
