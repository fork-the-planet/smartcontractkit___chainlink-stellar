package stellarutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterRemoteSelectors(t *testing.T) {
	got := FilterRemoteSelectors([]uint64{5, 1, 3, 1, 3, 2, 0}, 1)
	assert.Equal(t, []uint64{2, 3, 5}, got)
}
