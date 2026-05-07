package mcmsutil

import (
	"fmt"
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

// StellarDeployerPrivateKeyHex returns the 32-byte seed hex for the account that signs MCMS deploy txs.
// Must match the Stellar chain signer used in the environment. Set STELLAR_DEPLOYER_PRIVATE_KEY.
func StellarDeployerPrivateKeyHex() (string, error) {
	k := os.Getenv("STELLAR_DEPLOYER_PRIVATE_KEY")
	if k == "" {
		return "", fmt.Errorf("STELLAR_DEPLOYER_PRIVATE_KEY is required for Stellar MCMS deploy/update sequences (32-byte hex seed, same signer as the Stellar chain in CLDF)")
	}
	return k, nil
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
