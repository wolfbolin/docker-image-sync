package hub

import (
	"context"
	"net/url"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/wolfbolin/bolbox/pkg/log"
)

type ContainersClient struct {
	proxyUrl  *url.URL
	systemCtx *types.SystemContext

	imageTagsCache    sync.Map
	imageDigestsCache sync.Map
}

func NewContainersClient() *ContainersClient {
	return &ContainersClient{}
}

func (c *ContainersClient) SystemCtx() *types.SystemContext {
	return c.systemCtx
}

func (c *ContainersClient) SetProxy(proxy *url.URL) {
	c.proxyUrl = proxy
	c.systemCtx = c.sysCtx()
}

func (c *ContainersClient) UnsetProxy() {
	c.proxyUrl = nil
	c.systemCtx = c.sysCtx()
}

func (c *ContainersClient) sysCtx() *types.SystemContext {
	sysCtx := &types.SystemContext{}
	if c.proxyUrl != nil {
		sysCtx.DockerProxyURL = c.proxyUrl
	}
	return sysCtx
}

func (c *ContainersClient) ImageTags(ctx context.Context, image *Image) ([]string, error) {
	imageUrl := image.ToUrl()
	log.Debugf("Get container image[%s] tags", imageUrl)
	if tagNames, exist := c.imageTagsCache.Load(imageUrl); exist {
		return tagNames.([]string), nil
	}

	refStr := "docker://" + image.ToUrl()
	ref, err := alltransports.ParseImageName(refStr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tagNames, err := docker.GetRepositoryTags(ctx, c.systemCtx, ref)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c.imageTagsCache.Store(imageUrl, tagNames)
	return tagNames, nil
}

func (c *ContainersClient) ImageTagDigest(ctx context.Context, image *Image, tag string) (string, error) {
	imageTagUrl := image.ToTagUrl(tag)
	log.Debugf("Get container image tag[%s] digests", imageTagUrl)
	if digest, exist := c.imageDigestsCache.Load(imageTagUrl); exist {
		return digest.(string), nil
	}

	tagRef, err := alltransports.ParseImageName("docker://" + imageTagUrl)
	if err != nil {
		return "", errors.WithStack(err)
	}
	digest, err := docker.GetDigest(ctx, c.systemCtx, tagRef)
	if err != nil {
		return "", errors.WithStack(err)
	}

	digestStr := digest.String()
	c.imageDigestsCache.Store(imageTagUrl, digestStr)

	return digestStr, nil
}

func (c *ContainersClient) ImageMediaType(ctx context.Context, image *Image, tag string) (string, error) {
	refStr := "docker://" + image.ToTagUrl(tag)
	ref, err := alltransports.ParseImageName(refStr)
	if err != nil {
		return "", errors.WithStack(err)
	}

	src, err := ref.NewImageSource(ctx, c.systemCtx)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer src.Close()

	_, mimeType, err := src.GetManifest(context.Background(), nil)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return mimeType, nil
}
