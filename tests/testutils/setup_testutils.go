package helpers

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	chain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-testing-framework/framework/components/blockchain"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
)

// Sha256 hash of the network passphrase
const STELLAR_LOCALNET_PASSPHRASE = "Standalone Network ; February 2017"

func SetupTestEnv(ctx context.Context, t *testing.T) (string, *keypair.Full, *deployment.Deployer, *rpcclient.Client, string) {
	// Deploy local Stellar network using devenv
	chain := chain.New(zerolog.New(os.Stdout))

	chainID := network.ID(STELLAR_LOCALNET_PASSPHRASE)

	input := &blockchain.Input{
		Type:          "stellar",
		ChainID:       string(chainID[:]),
		ContainerName: "blockchain-stellar",
		Port:          "8055",
		DockerCmdParamsOverrides: []string{
			"--enable-soroban-rpc",
			"--local",
		},
		Image: "stellar/quickstart:testing",
	}

	output, err := chain.DeployLocalNetwork(ctx, input)
	if err != nil {
		t.Fatalf("Failed to deploy local network: %v", err)
	}
	t.Logf("Local network deployed at: %s", output.ContainerName)

	rpcURL := output.Nodes[0].ExternalHTTPUrl
	networkPassphrase := chain.NetworkPassphrase()

	// Create RPC client
	rpcClient := rpcclient.NewClient(rpcURL, &http.Client{Timeout: 60 * time.Second})

	// Wait for Friendbot to be ready - it takes longer than the RPC endpoint
	// The quickstart container starts multiple services and friendbot initializes last
	t.Log("Waiting for Friendbot to be ready (this can take up to 90 seconds)...")
	if err := WaitForFriendbot(
		ctx,
		input.Out.NetworkSpecificData.StellarNetwork.FriendbotURL,
		3*time.Minute,
	); err != nil {
		t.Fatalf("Friendbot not ready: %v", err)
	}
	t.Log("Friendbot is ready")

	deployerKP, err := keypair.Random()
	if err != nil {
		t.Fatalf("Failed to generate deployer keypair: %v", err)
	}

	deployerAddressBytes, err := strkey.Decode(strkey.VersionByteAccountID, deployerKP.Address())
	if err != nil {
		t.Fatalf("Failed to decode deployer address: %v", err)
	}

	err = chain.FundAddresses(ctx, input, []protocol.UnknownAddress{deployerAddressBytes}, nil)
	if err != nil {
		t.Fatalf("Failed to fund deployer account: %v", err)
	}

	deployer := deployment.NewDeployer(rpcClient, networkPassphrase, deployerKP)

	// Find the project root (where Cargo.toml is)
	projectRoot := FindProjectRoot(t)

	return projectRoot, deployerKP, deployer, rpcClient, networkPassphrase
}
