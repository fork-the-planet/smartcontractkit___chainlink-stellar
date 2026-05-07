package mcmsutil

import (
	"os"
	"path/filepath"
)

// DefaultMCMSWasmRelative is the release artifact path relative to the chainlink-stellar repo root.
const DefaultMCMSWasmRelative = "target/wasm32v1-none/release/mcms.wasm"

// DefaultTimelockWasmRelative is the timelock WASM path relative to the chainlink-stellar repo root.
const DefaultTimelockWasmRelative = "target/wasm32v1-none/release/timelock.wasm"

// ResolveMCMSWasmPath returns the path to mcms.wasm.
// Order: STELLAR_MCMS_WASM (full path), then CHAINLINK_STELLAR_ROOT + DefaultMCMSWasmRelative, then cwd-relative DefaultMCMSWasmRelative.
func ResolveMCMSWasmPath() (string, error) {
	if p := os.Getenv("STELLAR_MCMS_WASM"); p != "" {
		return p, nil
	}
	if root := os.Getenv("CHAINLINK_STELLAR_ROOT"); root != "" {
		p := filepath.Join(root, DefaultMCMSWasmRelative)
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, DefaultMCMSWasmRelative), nil
}

// ResolveTimelockWasmPath returns the path to timelock.wasm.
// Order: STELLAR_TIMELOCK_WASM (full path), then CHAINLINK_STELLAR_ROOT + DefaultTimelockWasmRelative, then cwd-relative DefaultTimelockWasmRelative.
func ResolveTimelockWasmPath() (string, error) {
	if p := os.Getenv("STELLAR_TIMELOCK_WASM"); p != "" {
		return p, nil
	}
	if root := os.Getenv("CHAINLINK_STELLAR_ROOT"); root != "" {
		p := filepath.Join(root, DefaultTimelockWasmRelative)
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, DefaultTimelockWasmRelative), nil
}
