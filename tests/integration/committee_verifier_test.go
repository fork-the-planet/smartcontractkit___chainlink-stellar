package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

<<<<<<< HEAD
	ccvsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/committee_verifier"
	rmnproxybindings "github.com/smartcontractkit/chainlink-stellar/bindings/rmn_proxy"
	vvrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/versioned_verifier_resolver"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

func TestCommitteeVerifier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, _ := helpers.SetupTestEnv(ctx, t)

	// Deploy the CommitteeVerifier contract
	t.Log("Deploying CommitteeVerifier contract...")
	salt := deployment.GenerateDeterministicSalt(deployerKP.Address(), "committee-verifier")
	wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "ccvs_committee_verifier.wasm")

	contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy CommitteeVerifier: %v", err)
	}
	t.Logf("CommitteeVerifier deployed at: %s", contractID)

	// Deploy the RMN Proxy contract
	t.Log("Deploying RMN Proxy contract...")
	salt = deployment.GenerateDeterministicSalt(deployerKP.Address(), "rmn-proxy")
	wasmPath = filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "rmn_proxy.wasm")
	rmnProxyContractID, err := deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy RMN Proxy: %v", err)
	}
	t.Logf("RMN Proxy deployed at: %s", rmnProxyContractID)

	// Create client
	client := ccvsbindings.NewCommiteeVerifierClient(deployer, contractID)

	// Generate mock addresses
	mockFeeAggregator := helpers.GenerateMockContractID(t, deployerKP.Address(), "fee-aggregator")
	mockInboundVerifier := helpers.GenerateMockContractID(t, deployerKP.Address(), "inbound-verifier")
	mockOutboundVerifier := helpers.GenerateMockContractID(t, deployerKP.Address(), "outbound-verifier")

	t.Run("initialize", func(t *testing.T) {
		// Initialize RMN Proxy
		rmnProxyClient := rmnproxybindings.NewRMNProxyClient(deployer, rmnProxyContractID)
		mockRmnRemote := helpers.GenerateMockContractID(t, deployerKP.Address(), "rmn-remote")
		err := rmnProxyClient.Initialize(ctx, deployerKP.Address(), mockRmnRemote)
		if err != nil {
			t.Fatalf("Failed to initialize RMN Proxy: %v", err)
		}
		t.Log("RMN Proxy initialized successfully")

		// Initialize CommitteeVerifier
		err = client.Initialize(ctx, deployerKP.Address(), ccvsbindings.DynamicConfig{
			FeeAggregator: mockFeeAggregator,
		}, [][]byte{}, rmnProxyContractID)
		if err != nil {
			t.Fatalf("Failed to initialize CommitteeVerifier: %v", err)
		}

		t.Log("CommitteeVerifier initialized successfully with RMN Proxy")
	})

	t.Run("verify owner", func(t *testing.T) {
		owner, err := client.Owner(ctx)
		if err != nil {
			t.Fatalf("Failed to get owner: %v", err)
		}
		if owner != deployerKP.Address() {
			t.Errorf("Owner mismatch: expected %s, got %s", deployerKP.Address(), owner)
		}
		t.Logf("Owner verified: %s", owner)
	})

	t.Run("verify fee aggregator", func(t *testing.T) {
		feeAgg, err := client.GetFeeAggregator(ctx)
		if err != nil {
			t.Fatalf("Failed to get fee aggregator: %v", err)
		}
		if feeAgg != mockFeeAggregator {
			t.Errorf("FeeAggregator mismatch: expected %s, got %s", mockFeeAggregator, feeAgg)
		}
		t.Logf("FeeAggregator verified: %s", feeAgg)
	})

	t.Run("apply inbound implementation updates", func(t *testing.T) {
		version := [4]byte{0x00, 0x01, 0x00, 0x00} // version 1.0

		updates := []vvrbindings.InboundImplementationUpdate{
			{
				Version:  version,
				Verifier: &mockInboundVerifier,
			},
		}

		err := client.ApplyInboundImplUpdates(ctx, updates)
		if err != nil {
			t.Fatalf("Failed to apply inbound impl updates: %v", err)
		}
		t.Log("Inbound implementation update applied")

		// Verify via get_inbound_implementation
		// The first 4 bytes of verifierResults are the version
		verifierResults := version[:]
		addr, err := client.GetInboundImplementation(ctx, verifierResults)
		if err != nil {
			t.Fatalf("Failed to get inbound implementation: %v", err)
		}
		if addr != mockInboundVerifier {
			t.Errorf("Inbound implementation mismatch: expected %s, got %s", mockInboundVerifier, addr)
		}
		t.Logf("Inbound implementation verified: %s", addr)
	})

	t.Run("apply outbound implementation updates", func(t *testing.T) {
		destChainSelector := uint64(12345)

		updates := []vvrbindings.OutboundImplementationUpdate{
			{
				DestChainSelector: destChainSelector,
				Verifier:          &mockOutboundVerifier,
			},
		}

		err := client.ApplyOutboundImplUpdates(ctx, updates)
		if err != nil {
			t.Fatalf("Failed to apply outbound impl updates: %v", err)
		}
		t.Log("Outbound implementation update applied")

		// Verify via get_outbound_implementation
		addr, err := client.GetOutboundImplementation(ctx, destChainSelector, []byte{})
		if err != nil {
			t.Fatalf("Failed to get outbound implementation: %v", err)
		}
		if addr != mockOutboundVerifier {
			t.Errorf("Outbound implementation mismatch: expected %s, got %s", mockOutboundVerifier, addr)
		}
		t.Logf("Outbound implementation verified: %s", addr)
	})

	t.Run("update fee aggregator", func(t *testing.T) {
		newFeeAggregator := helpers.GenerateMockContractID(t, deployerKP.Address(), "new-fee-aggregator")

		err := client.SetFeeAggregator(ctx, newFeeAggregator)
		if err != nil {
			t.Fatalf("Failed to set fee aggregator: %v", err)
		}

		feeAgg, err := client.GetFeeAggregator(ctx)
		if err != nil {
			t.Fatalf("Failed to get fee aggregator: %v", err)
		}
		if feeAgg != newFeeAggregator {
			t.Errorf("FeeAggregator mismatch after update: expected %s, got %s", newFeeAggregator, feeAgg)
		}
		t.Logf("Fee aggregator updated and verified: %s", feeAgg)
	})

	t.Run("remove inbound implementation", func(t *testing.T) {
		version := [4]byte{0x00, 0x01, 0x00, 0x00} // same version as set above

		// Remove by passing nil verifier (maps to Soroban Option::None)
		updates := []vvrbindings.InboundImplementationUpdate{
			{
				Version:  version,
				Verifier: nil,
			},
		}

		err := client.ApplyInboundImplUpdates(ctx, updates)
		if err != nil {
			t.Fatalf("Failed to remove inbound impl: %v", err)
		}
		t.Log("Inbound implementation removed")
	})

	t.Run("remove outbound implementation", func(t *testing.T) {
		destChainSelector := uint64(12345) // same as set above

		// Remove by passing nil verifier (maps to Soroban Option::None)
		updates := []vvrbindings.OutboundImplementationUpdate{
			{
				DestChainSelector: destChainSelector,
				Verifier:          nil,
			},
		}

		err := client.ApplyOutboundImplUpdates(ctx, updates)
		if err != nil {
			t.Fatalf("Failed to remove outbound impl: %v", err)
		}
		t.Log("Outbound implementation removed")
	})

	t.Log("VersionedVerifierResolver test passed!")
||||||| merged common ancestors
=======
	ccvsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/committee_verifier"
	rmnproxybindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_proxy"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

func TestCommitteeVerifier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, _ := helpers.SetupTestEnv(ctx, t)

	// Deploy the CommitteeVerifier contract
	t.Log("Deploying CommitteeVerifier contract...")
	salt := deployment.GenerateDeterministicSalt(deployerKP.Address(), "committee-verifier")
	wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "ccvs_committee_verifier.wasm")

	contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy CommitteeVerifier: %v", err)
	}
	t.Logf("CommitteeVerifier deployed at: %v", contractID)

	// Deploy the RMN Proxy contract
	t.Log("Deploying RMN Proxy contract...")
	salt = deployment.GenerateDeterministicSalt(deployerKP.Address(), "rmn-proxy")
	wasmPath = filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "rmn_proxy.wasm")
	rmnProxyContractID, err := deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy RMN Proxy: %v", err)
	}
	t.Logf("RMN Proxy deployed at: %v", rmnProxyContractID)

	// Create client
	client := ccvsbindings.NewCommitteeVerifierClient(deployer, contractID)

	// Generate mock addresses
	mockFeeAggregator := helpers.GenerateMockContractID(t, deployerKP.Address(), "fee-aggregator")

	t.Run("initialize", func(t *testing.T) {
		// Initialize RMN Proxy
		rmnProxyClient := rmnproxybindings.NewRmnProxyClient(deployer, rmnProxyContractID)
		mockRmnRemote := helpers.GenerateMockContractID(t, deployerKP.Address(), "rmn-remote")
		err := rmnProxyClient.Initialize(ctx, deployerKP.Address(), mockRmnRemote)
		if err != nil {
			t.Fatalf("Failed to initialize RMN Proxy: %v", err)
		}
		t.Log("RMN Proxy initialized successfully")

		// Initialize CommitteeVerifier
		err = client.Initialize(ctx, deployerKP.Address(), ccvsbindings.DynamicConfig{
			FeeAggregator: &mockFeeAggregator,
		}, [][]byte{}, rmnProxyContractID)
		if err != nil {
			t.Fatalf("Failed to initialize CommitteeVerifier: %v", err)
		}

		t.Log("CommitteeVerifier initialized successfully with RMN Proxy")
	})

	t.Run("verify owner", func(t *testing.T) {
		owner, err := client.Owner(ctx)
		if err != nil {
			t.Fatalf("Failed to get owner: %v", err)
		}
		if *owner != deployerKP.Address() {
			t.Errorf("Owner mismatch: expected %s, got %s", deployerKP.Address(), *owner)
		}
		t.Logf("Owner verified: %v", owner)
	})
>>>>>>> main
}
