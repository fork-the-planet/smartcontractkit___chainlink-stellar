package helpers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	stellar_devenv "github.com/smartcontractkit/chainlink-ccv/devenv/stellar"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/strkey"
)

// findProjectRoot finds the root of the chainlink-stellar project.
func FindProjectRoot(t *testing.T) string {
	// Start from the current working directory
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up until we find Cargo.toml
	for {
		cargoPath := filepath.Join(dir, "Cargo.toml")
		if _, err := os.Stat(cargoPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding Cargo.toml
			t.Fatal("Could not find project root (Cargo.toml)")
		}
		dir = parent
	}
}

// generateMockContractID generates a deterministic mock contract ID for testing.
func GenerateMockContractID(t *testing.T, deployerAddress, contractName string) string {
	// Generate a deterministic salt
	salt := stellar_devenv.GenerateDeterministicSalt(deployerAddress, contractName)

	// Encode as a Stellar contract address
	encoded, err := strkey.Encode(strkey.VersionByteContract, salt[:])
	if err != nil {
		t.Fatalf("Failed to encode mock contract ID: %v", err)
	}
	return encoded
}

// waitForFriendbot waits for the friendbot service to be ready.
// The Stellar quickstart container starts multiple services and friendbot
// initializes after the main RPC endpoint is ready.
func WaitForFriendbot(ctx context.Context, friendbotBaseURL string, timeout time.Duration) error {
	// Generate a test address to use for the health check
	testKP, err := keypair.Random()
	if err != nil {
		return fmt.Errorf("failed to generate test keypair: %w", err)
	}
	testURL := fmt.Sprintf("%s?addr=%s", friendbotBaseURL, testKP.Address())

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
			if err != nil {
				lastErr = err
				continue
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil // Friendbot is ready
			}

			lastErr = fmt.Errorf("friendbot returned status %d", resp.StatusCode)
		}
	}

	return fmt.Errorf("friendbot not ready after %v: %w", timeout, lastErr)
}
