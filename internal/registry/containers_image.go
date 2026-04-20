package registry

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"

	"github.com/wolfbolin/sync-docker/internal/logger"
)

type ContainersImageClient struct {
	mu       sync.Mutex
	cache    map[string][]SourceTag
	proxyURL *url.URL
}

func NewContainersImageClient() *ContainersImageClient {
	return &ContainersImageClient{
		cache: make(map[string][]SourceTag),
	}
}

func (c *ContainersImageClient) SetProxy(proxyURL string) {
	if proxyURL == "" {
		c.mu.Lock()
		c.proxyURL = nil
		c.mu.Unlock()
		return
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		logger.Error("Failed to parse proxy URL %q: %v", proxyURL, err)
		return
	}
	c.mu.Lock()
	c.proxyURL = parsed
	c.mu.Unlock()
}

func (c *ContainersImageClient) buildSysCtx() *types.SystemContext {
	sysCtx := &types.SystemContext{}
	c.mu.Lock()
	proxyURL := c.proxyURL
	c.mu.Unlock()
	if proxyURL != nil {
		sysCtx.DockerProxyURL = proxyURL
	}
	return sysCtx
}

func (c *ContainersImageClient) ListTags(repository string) ([]SourceTag, error) {
	c.mu.Lock()
	if tags, ok := c.cache[repository]; ok {
		c.mu.Unlock()
		return tags, nil
	}
	c.mu.Unlock()

	refStr := "docker://" + repository
	logger.Debug("GetRepositoryTags: %s", refStr)

	ref, err := alltransports.ParseImageName(refStr)
	if err != nil {
		return nil, fmt.Errorf("parse image name %s: %w", refStr, err)
	}

	sysCtx := c.buildSysCtx()
	tagNames, err := docker.GetRepositoryTags(context.Background(), sysCtx, ref)
	if err != nil {
		return nil, fmt.Errorf("get repository tags for %s: %w", repository, err)
	}

	allTags := make([]SourceTag, len(tagNames))
	for i, name := range tagNames {
		allTags[i] = SourceTag{Name: name}
	}

	c.mu.Lock()
	c.cache[repository] = allTags
	c.mu.Unlock()

	return allTags, nil
}

func (c *ContainersImageClient) GetManifestMediaType(repository, tag string) (string, error) {
	refStr := "docker://" + repository + ":" + tag
	logger.Debug("GetManifestMediaType: %s", refStr)

	ref, err := alltransports.ParseImageName(refStr)
	if err != nil {
		return "", fmt.Errorf("parse image name %s: %w", refStr, err)
	}

	sysCtx := c.buildSysCtx()
	src, err := ref.NewImageSource(context.Background(), sysCtx)
	if err != nil {
		return "", fmt.Errorf("open image source for %s: %w", refStr, err)
	}
	defer src.Close()

	_, mimeType, err := src.GetManifest(context.Background(), nil)
	if err != nil {
		return "", fmt.Errorf("get manifest for %s: %w", refStr, err)
	}

	return mimeType, nil
}
