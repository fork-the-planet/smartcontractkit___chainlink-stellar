//go:build integration

package integration

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

const sharedContainerName = "blockchain-stellar-integration-shared"

var (
	sharedEnv     *helpers.SharedTestEnv
	sharedEnvOnce sync.Once
	sharedEnvErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()

	// Teardown: stop the Stellar container after all tests finish
	if sharedEnv != nil && sharedEnv.Output != nil && sharedEnv.Output.Container != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := sharedEnv.Output.Container.Terminate(ctx); err != nil {
			log.Printf("Warning: failed to terminate Stellar container: %v", err)
		}
	}

	os.Exit(code)
}

// GetSharedTestEnv returns the shared test environment (Stellar node, deployer, etc.)
// used across all integration tests. Setup runs once on first use; teardown runs after all tests.
func GetSharedTestEnv(ctx context.Context, t *testing.T) (string, *keypair.Full, *deployment.Deployer, *rpcclient.Client, string) {
	sharedEnvOnce.Do(func() {
		sharedEnv, sharedEnvErr = helpers.SetupTestEnvShared(ctx, sharedContainerName)
	})
	if sharedEnvErr != nil {
		t.Fatalf("Shared test env setup failed: %v", sharedEnvErr)
	}
	return sharedEnv.ProjectRoot, sharedEnv.DeployerKP, sharedEnv.Deployer,
		sharedEnv.RPCClient, sharedEnv.NetworkPassphrase
}
