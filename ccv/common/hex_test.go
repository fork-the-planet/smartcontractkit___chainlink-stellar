package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHexEncode(t *testing.T) {
	t.Parallel()
	require.Equal(t, "0x", HexEncode(nil))
	require.Equal(t, "0xdeadbeef", HexEncode([]byte{0xde, 0xad, 0xbe, 0xef}))
}
