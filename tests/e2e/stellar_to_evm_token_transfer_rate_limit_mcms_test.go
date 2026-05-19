package e2e_tests

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/require"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	lrpbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/lock_release_pool"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
)

// TestStellarToEVMTokenTransferRateLimitViaMCMS exercises governance-controlled rate limiting
// on the Stellar lock-release token pool during Stellar→EVM token transfers.
//
// Phase 1: baseline token transfer succeeds (no rate limit).
// Phase 2: deploy MCMS + timelock wired for MCMS-mediated governance.
// (Phases 3–5: ownership transfer, rate-limit config via MCMS, blocked transfer — follow-up.)
//
// Prerequisites:
//
//	make build
//	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv
//
// Run:
//
//	go test -v -timeout 15m ./tests/e2e/... -run TestStellarToEVMTokenTransferRateLimitViaMCMS
func TestStellarToEVMTokenTransferRateLimitViaMCMS(t *testing.T) {
	configOutputPath := "../env/env-stellar-evm-out.toml"

	stellarChainID := chainsel.STELLAR_LOCALNET.ChainID
	stellarSelector := chainsel.STELLAR_LOCALNET.Selector

	ctx := ccv.Plog.WithContext(t.Context())
	l := zerolog.Ctx(ctx)

	env := helpers.NewE2ETestEnv(t, ctx, l, configOutputPath, stellarChainID, stellarSelector)
	stellarDetails := env.SourceChainDetails
	evmDetails := env.DestChainDetails

	stellarChain := env.Chains[stellarDetails.ChainSelector]
	require.NotNil(t, stellarChain, "Stellar chain not found")

	evmChain := env.Chains[evmDetails.ChainSelector]
	require.NotNil(t, evmChain, "EVM chain not found")

	stellarCcvChain, ok := stellarChain.(*ccvchain.Chain)
	require.True(t, ok, "Stellar chain must be *ccvchain.Chain")

	tokenAddr, err := stellarCcvChain.GetTokenAddress()
	require.NoError(t, err, "test token must be deployed")
	l.Info().Str("tokenAddress", tokenAddr).Msg("Using test SAC token")

	tokenRaw, err := strkey.Decode(strkey.VersionByteContract, tokenAddr)
	require.NoError(t, err)

	poolRef, err := env.DataStore.Addresses().Get(
		stellarccip.LockReleasePoolDevenvDatastoreRef().AddressRefKey(stellarDetails.ChainSelector),
	)
	require.NoError(t, err)
	require.NotEmpty(t, poolRef.Address)
	poolContractID, err := scval.HexToContractStrkey(poolRef.Address)
	require.NoError(t, err)
	poolClient := lrpbindings.NewLockReleasePoolClient(env.Deployer, poolContractID)
	l.Info().Str("poolContractID", poolContractID).Msg("Using lock-release token pool")

	senderAddr, err := stellarChain.GetSenderAddress()
	require.NoError(t, err)

	evmReceiver, err := evmChain.GetEOAReceiverAddress()
	require.NoError(t, err)

	t.Run("phase1_baseline_transfer_succeeds", func(t *testing.T) {
		balBefore, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(tokenRaw))
		require.NoError(t, err)
		l.Info().Str("balance", balBefore.String()).Msg("Sender token balance before baseline transfer")
		require.True(t, balBefore.Int64() >= tokenTransferAmount,
			"sender must have enough tokens; balance=%s, need=%d", balBefore, tokenTransferAmount)

		seqNo, err := stellarChain.GetExpectedNextSequenceNumber(ctx, evmDetails.ChainSelector)
		require.NoError(t, err)

		sendResult, err := stellarChain.SendMessage(ctx, evmDetails.ChainSelector,
			cciptestinterfaces.MessageFields{
				Receiver: evmReceiver,
				Data:     []byte("rate-limit-mcms-baseline"),
				TokenAmount: cciptestinterfaces.TokenAmount{
					Amount:       big.NewInt(tokenTransferAmount),
					TokenAddress: protocol.UnknownAddress(tokenRaw),
				},
			},
			cciptestinterfaces.MessageOptions{},
			messageV3Version,
		)
		require.NoError(t, err)
		l.Info().
			Str("messageID", hex.EncodeToString(sendResult.MessageID[:])).
			Msg("Baseline token transfer sent from Stellar")

		sentEvent, err := stellarChain.ConfirmSendOnSource(ctx, evmDetails.ChainSelector,
			cciptestinterfaces.MessageEventKey{SeqNum: seqNo}, tokenTransferSentTimeout)
		require.NoError(t, err)
		l.Info().
			Str("messageID", hex.EncodeToString(sentEvent.MessageID[:])).
			Msg("Baseline sent event confirmed")

		balAfter, err := stellarChain.GetTokenBalance(ctx, senderAddr, protocol.UnknownAddress(tokenRaw))
		require.NoError(t, err)

		locked := new(big.Int).Sub(balBefore, balAfter)
		require.Equal(t, tokenTransferAmount, locked.Int64(),
			"sender balance should decrease by exactly the transfer amount")

		l.Info().Int64("lockedAmount", locked.Int64()).Msg("Phase 1 complete: baseline transfer locked tokens on source")
	})

	t.Run("phase2_deploy_mcms_and_timelock", func(t *testing.T) {
		gov := helpers.DeployMCMSAndTimelock(t, ctx, env, stellarDetails.ChainSelector, "e2e-rate-limit-mcms")
		l.Info().
			Str("mcmsID", gov.MCMSID).
			Str("timelockID", gov.TimelockID).
			Uint64("minDelaySec", gov.MinDelaySec).
			Msg("MCMS and timelock deployed")

		owner, err := poolClient.Owner(ctx)
		require.NoError(t, err)
		require.NotNil(t, owner)
		require.Equal(t, env.DeployerKP.Address(), *owner,
			"pool should still be owned by deployer before governance transfer")

		// Sanity: outbound rate limit is disabled for the EVM dest chain (devenv default).
		state, err := poolClient.GetCurrentRateLimiterState(ctx, evmDetails.ChainSelector, false)
		require.NoError(t, err)
		require.False(t, state.Outbound.IsEnabled, "outbound rate limit should start disabled")

		l.Info().Msg("Phase 2 complete: MCMS + timelock ready for ownership transfer and rate-limit config")
	})
}
