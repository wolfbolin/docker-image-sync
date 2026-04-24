package hub

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewContainersClient(t *testing.T) {
	ctx := context.Background()
	client := NewContainersClient()

	image := ParseImage("registry.aliyuncs.com/google_containers/pause")
	tags, err := client.ImageTags(ctx, image)
	assert.Nil(t, err)
	fmt.Println(tags)

	digest, err := client.ImageTagDigest(ctx, image, tags[0])
	assert.Nil(t, err)
	fmt.Println(digest)

	mediaType, err := client.ImageMediaType(ctx, image, tags[0])
	assert.Nil(t, err)
	fmt.Println(mediaType)
}
