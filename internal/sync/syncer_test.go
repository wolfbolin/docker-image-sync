package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_splitTagSet(t *testing.T) {
	sourceTags := []string{"s1", "m1", "m2", "s2"}
	targetTags := []string{"t1", "m1", "m2", "t2"}

	tagSet := splitTagSet(sourceTags, targetTags)
	assert.ElementsMatch(t, []string{"s1", "s2"}, tagSet.Sync)
	assert.ElementsMatch(t, []string{"t1", "t2"}, tagSet.Over)
	assert.ElementsMatch(t, []string{"m1", "m2"}, tagSet.Same)
}
