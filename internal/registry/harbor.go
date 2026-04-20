package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wolfbolin/sync-docker/internal/logger"
)

type HarborClient struct {
	httpClient *http.Client
	registry   string
	authHeader string
}

func NewHarborClient(registry string) *HarborClient {
	transport := &http.Transport{
		Proxy: nil,
	}

	authHeader := loadHarborAuth(registry)

	return &HarborClient{
		httpClient: &http.Client{Transport: transport},
		registry:   registry,
		authHeader: authHeader,
	}
}

func loadHarborAuth(registry string) string {
	configPath := dockerConfigPath()
	if configPath == "" {
		return ""
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var cfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	for host, auth := range cfg.Auths {
		if matchRegistry(host, registry) && auth.Auth != "" {
			return "Basic " + auth.Auth
		}
	}

	return ""
}

func dockerConfigPath() string {
	if v := os.Getenv("DOCKER_CONFIG"); v != "" {
		return filepath.Join(v, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".docker", "config.json")
}

func matchRegistry(host, registry string) bool {
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")
	return host == registry
}

func (c *HarborClient) newRequest(method, apiURL string) (*http.Request, error) {
	req, err := http.NewRequest(method, apiURL, nil)
	if err != nil {
		return nil, err
	}
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	return req, nil
}

func (c *HarborClient) ListArtifacts(project, name string) ([]HarborArtifact, error) {
	var allArtifacts []HarborArtifact
	encodedName := strings.ReplaceAll(name, "/", "%252F")

	for page := 1; ; page++ {
		apiURL := fmt.Sprintf("https://%s/api/v2.0/projects/%s/repositories/%s/artifacts?page=%d&page_size=100",
			c.registry, project, encodedName, page)

		logger.Debug("GET %s", apiURL)

		req, err := c.newRequest(http.MethodGet, apiURL)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request harbor artifacts: %w", err)
		}

		artifacts, err := func() ([]HarborArtifact, error) {
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("harbor API returned status %d", resp.StatusCode)
			}

			var artifacts []HarborArtifact
			if err := json.NewDecoder(resp.Body).Decode(&artifacts); err != nil {
				return allArtifacts, nil
			}
			return artifacts, nil
		}()
		if err != nil {
			return nil, err
		}

		allArtifacts = append(allArtifacts, artifacts...)

		if len(artifacts) < 100 {
			break
		}
	}

	return allArtifacts, nil
}

func (c *HarborClient) ListTagsWithDigest(project, name string) ([]HarborTagInfo, error) {
	artifacts, err := c.ListArtifacts(project, name)
	if err != nil {
		return nil, err
	}

	var allTags []HarborTagInfo
	for _, a := range artifacts {
		for _, t := range a.Tags {
			allTags = append(allTags, HarborTagInfo{Name: t.Name, Digest: a.Digest})
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

func (c *HarborClient) DeleteArtifact(project, name, digest string) error {
	encodedName := strings.ReplaceAll(name, "/", "%252F")
	apiURL := fmt.Sprintf("https://%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		c.registry, project, encodedName, digest)

	logger.Debug("DELETE %s", apiURL)

	req, err := c.newRequest(http.MethodDelete, apiURL)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("harbor delete API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *HarborClient) ListRepositories(project string) ([]string, error) {
	var repos []string

	for page := 1; ; page++ {
		apiURL := fmt.Sprintf("https://%s/api/v2.0/projects/%s/repositories?page=%d&page_size=100",
			c.registry, project, page)

		logger.Debug("GET %s", apiURL)

		req, err := c.newRequest(http.MethodGet, apiURL)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request harbor repositories: %w", err)
		}

		result, err := func() (HarborRepoListResponse, error) {
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("harbor API returned status %d", resp.StatusCode)
			}

			var result HarborRepoListResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return result, nil
			}
			return result, nil
		}()
		if err != nil {
			return nil, err
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
