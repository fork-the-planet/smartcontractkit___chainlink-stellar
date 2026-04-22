package e2e_tests

import (
	"testing"

	"github.com/rs/zerolog"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	devenvccipevm "github.com/smartcontractkit/chainlink-ccv/build/devenv/evm"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/tests/composable/messaging"
	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	stellardevenv "github.com/smartcontractkit/chainlink-stellar/ccv/devenv"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

var (
	_ cciptestinterfaces.ChainAsSource      = (*devenvccipevm.CCIP17EVM)(nil)
	_ cciptestinterfaces.ChainAsDestination = (*ccvchain.Chain)(nil)
)

// TestEVMToStellarComposableMessaging runs messaging.BasicMessageTestScenario with
// EVM as ChainAsSource and Stellar as ChainAsDestination (same devenv as
// tests/e2e/evm_to_stellar_test.go and tests/e2e/stellar_evm_messaging_test.go).
//
// Prerequisites:
//
//	make build
//	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv
//
// Then:
//
//	go test -v -timeout 15m ./tests/e2e/... -run TestEVMToStellarComposableMessaging
func TestEVMToStellarComposableMessaging(t *testing.T) {
	stellardevenv.RegisterStellarComponents()

	configOutputPath := "../env/env-stellar-evm-out.toml"
	stellarChainID := chain_selectors.STELLAR_LOCALNET.ChainID
	stellarSelector := chain_selectors.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())

	env := helpers.NewE2ETestEnv(t, ctx, zerolog.Ctx(ctx), configOutputPath, stellarChainID, stellarSelector)

	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarImpl := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarImpl, "Stellar chain not found in chains map")
	evmImpl := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmImpl, "EVM chain not found in chains map")

	src, ok := evmImpl.(cciptestinterfaces.ChainAsSource)
	require.True(t, ok, "EVM chain must implement cciptestinterfaces.ChainAsSource")
	dest, ok := stellarImpl.(cciptestinterfaces.ChainAsDestination)
	require.True(t, ok, "Stellar chain must implement cciptestinterfaces.ChainAsDestination")

	receiver, err := dest.GetEOAReceiverAddress()
	require.NoError(t, err)

	// Match evm_to_stellar_test: EVM OnRamp uses GenericExtraArgsV3 for this lane.
	err = messaging.BasicMessageTestScenario(ctx, t, src, dest, cciptestinterfaces.MessageFields{
		Receiver: receiver,
		Data:     []byte("composable evm→stellar"),
	}, cciptestinterfaces.MessageOptions{
		Version:           3,
		ExecutionGasLimit: 200_000,
	}, nil)
	require.NoError(t, err)
}
