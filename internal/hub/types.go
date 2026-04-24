package hub

import (
	"context"
	"net/url"

	"github.com/containers/image/v5/types"
)

type Client interface {
	SystemCtx() *types.SystemContext
	SetProxy(proxy *url.URL)
	UnsetProxy()
	ImageTags(ctx context.Context, image *Image) ([]string, error)
	ImageTagDigest(ctx context.Context, image *Image, tag string) (string, error)
	ImageMediaType(ctx context.Context, image *Image, tag string) (string, error)
}
