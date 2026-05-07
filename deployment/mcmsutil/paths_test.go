package mcmsutil

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveTimelockWasmPath_STELLAR_TIMELOCK_WASM(t *testing.T) {
	t.Setenv("STELLAR_TIMELOCK_WASM", "/abs/custom/timelock.wasm")
	t.Setenv("CHAINLINK_STELLAR_ROOT", "/should/be/ignored")

	got, err := ResolveTimelockWasmPath()
	require.NoError(t, err)
	require.Equal(t, "/abs/custom/timelock.wasm", got)
}

func TestResolveTimelockWasmPath_CHAINLINK_STELLAR_ROOT(t *testing.T) {
	t.Setenv("STELLAR_TIMELOCK_WASM", "")
	t.Setenv("CHAINLINK_STELLAR_ROOT", "/repo/root")

	got, err := ResolveTimelockWasmPath()
	require.NoError(t, err)
	want := filepath.Join("/repo/root", DefaultTimelockWasmRelative)
	require.Equal(t, want, got)
}

func TestResolveTimelockWasmPath_fallbackUsesCwdRelative(t *testing.T) {
	t.Setenv("STELLAR_TIMELOCK_WASM", "")
	t.Setenv("CHAINLINK_STELLAR_ROOT", "")

	got, err := ResolveTimelockWasmPath()
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(got, DefaultTimelockWasmRelative), "got %q", got)
}
