//go:build integration

package integration

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/onramp"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	tokenpoolbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_pool"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestTokenPool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, rpcClient, networkPassphrase, friendbotURL := GetSharedTestEnv(ctx, t)
	deployerAddr := deployerKP.Address()

	t.Run("deploy and initialize lock-release pool", func(t *testing.T) {
		wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "pools_lock_release_pool.wasm")
		salt := deployment.GenerateDeterministicSalt(deployerAddr, "test-lock-release-pool")
		contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
		if err != nil {
			t.Fatalf("Deploy LockRelease pool: %v", err)
		}
		t.Logf("Pool deployed at: %s", contractID)

		mockToken := helpers.GenerateMockContractID(t, deployerAddr, "pool-test-token")
		client := tokenpoolbindings.NewTokenPoolClient(deployer, contractID)

		if err := client.Initialize(ctx, deployerAddr, mockToken); err != nil {
			t.Fatalf("Initialize pool: %v", err)
		}

		gotToken, err := client.GetToken(ctx)
		if err != nil {
			t.Fatalf("GetToken: %v", err)
		}
		if gotToken != mockToken {
			t.Fatalf("token mismatch: want %s, got %s", mockToken, gotToken)
		}

		supported, err := client.IsSupportedToken(ctx, mockToken)
		if err != nil {
			t.Fatalf("IsSupportedToken: %v", err)
		}
		if !supported {
			t.Fatal("expected token to be supported")
		}
		t.Log("Pool deployed, initialized, and token verified")
	})

	t.Run("apply chain updates", func(t *testing.T) {
		wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "pools_lock_release_pool.wasm")
		salt := deployment.GenerateDeterministicSalt(deployerAddr, "test-pool-chain-updates")
		contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
		if err != nil {
			t.Fatalf("Deploy pool: %v", err)
		}

		mockToken := helpers.GenerateMockContractID(t, deployerAddr, "pool-chain-test-token")
		client := tokenpoolbindings.NewTokenPoolClient(deployer, contractID)
		if err := client.Initialize(ctx, deployerAddr, mockToken); err != nil {
			t.Fatalf("Initialize pool: %v", err)
		}

		remoteChain := uint64(54321)
		remotePool := make([]byte, 20)
		remoteToken := make([]byte, 20)
		err = client.ApplyChainUpdates(ctx, []tokenpoolbindings.ChainUpdate{
			{
				RemoteChainSelector: remoteChain,
				RemotePoolAddresses: remotePool,
				RemoteTokenAddress:  remoteToken,
			},
		}, nil)
		if err != nil {
			t.Fatalf("ApplyChainUpdates: %v", err)
		}

		supported, err := client.IsSupportedChain(ctx, remoteChain)
		if err != nil {
			t.Fatalf("IsSupportedChain: %v", err)
		}
		if !supported {
			t.Fatal("expected remote chain to be supported after ApplyChainUpdates")
		}
		t.Logf("Chain %d supported after update", remoteChain)
	})

	t.Run("deploy full stack with token pool", func(t *testing.T) {
		const destChain = uint64(11111)
		stack := deployFullStack(ctx, t, projectRoot, deployer, deployerAddr, destChain, "token-pool-stack")

		mockToken := helpers.GenerateMockContractID(t, deployerAddr, "stack-test-token")
		stack.deployTokenPool(ctx, t, projectRoot, deployer, deployerAddr, "token-pool-stack", mockToken)

		if stack.TokenAdminRegistryID == "" {
			t.Fatal("TokenAdminRegistryID not set after deployTokenPool")
		}
		if stack.TokenPoolID == "" {
			t.Fatal("TokenPoolID not set after deployTokenPool")
		}

		pool, err := stack.TokenAdminRegistryClient.GetPool(ctx, mockToken)
		if err != nil {
			t.Fatalf("GetPool: %v", err)
		}
		if pool == nil || *pool != stack.TokenPoolID {
			t.Fatalf("pool mismatch: want %s, got %v", stack.TokenPoolID, pool)
		}
		t.Log("Full stack with token pool: TokenAdminRegistry correctly maps token to pool")
	})

	t.Run("router ccip_send with lock-release pool token amount", func(t *testing.T) {
		const localChain = uint64(11111)
		const remoteDestChain = uint64(22222)
		const tokenTransferAmount = int64(1_000_000) // 0.1 INTG at 7 decimals (same scale as ccv/chain E2E test token)

		stack := deployFullStack(ctx, t, projectRoot, deployer, deployerAddr, localChain, "ccip-token-send")

		// Real SAC + minted balance on deployer (classic issue + trustline), like a mock ERC20 on EVM.
		sacToken := deployIntegrationTestSAC(ctx, t, rpcClient, deployer, deployerAddr, networkPassphrase, friendbotURL, "ccip-token-send")
		mockFeeToken := helpers.GenerateMockContractID(t, deployerAddr, "ccip-send-fee-token")

		stack.deployTokenPool(ctx, t, projectRoot, deployer, deployerAddr, "ccip-token-send", sacToken)

		remotePool := make([]byte, 20)
		remoteToken := make([]byte, 20)
		for i := range remotePool {
			remotePool[i] = 0x11
			remoteToken[i] = 0x22
		}
		if err := stack.TokenPoolClient.ApplyChainUpdates(ctx, []tokenpoolbindings.ChainUpdate{{
			RemoteChainSelector: remoteDestChain,
			RemotePoolAddresses: remotePool,
			RemoteTokenAddress:  remoteToken,
		}}, nil); err != nil {
			t.Fatalf("TokenPool ApplyChainUpdates: %v", err)
		}

		outWire := deployOutboundSendWire(ctx, t, projectRoot, deployer, deployerAddr, "ccip-token-send", stack,
			localChain, remoteDestChain, mockFeeToken, []string{sacToken})
		// Must match deployOutboundSendWire: 20-byte placeholder off-ramp address on dest chain.
		offRampRawPlaceholder := make([]byte, 20)

		defaultExecutor := helpers.GenerateMockContractID(t, deployerAddr, "ccip-token-send-executor")
		extraArgs, err := encodeOnrampExtraArgsV3(onrampbindings.GenericExtraArgsV3{
			Ccvs:               []string{stack.VvrID},
			CcvArgs:            [][]byte{{}},
			Executor:           defaultExecutor,
			ExecutorArgs:       []byte{},
			GasLimit:           0,
			BlockConfirmations: 0,
			TokenReceiver:      []byte{},
			TokenArgs:          []byte{},
		})
		if err != nil {
			t.Fatalf("encode extra args: %v", err)
		}

		evmReceiver := make([]byte, 20)
		for i := range evmReceiver {
			evmReceiver[i] = 0x33
		}

		msg := routerbindings.StellarToAnyMessage{
			Receiver:     evmReceiver,
			Data:         []byte("integration token ccip_send"),
			FeeToken:     mockFeeToken,
			ExtraArgs:    extraArgs,
			TokenAmounts: []routerbindings.TokenAmount{{Token: sacToken, Amount: tokenTransferAmount}},
		}

		requiredFee, err := stack.RouterClient.GetFee(ctx, remoteDestChain, msg)
		if err != nil {
			t.Fatalf("Router GetFee: %v", err)
		}
		if requiredFee <= 0 {
			t.Fatalf("expected positive fee for token message, got %d", requiredFee)
		}
		t.Logf("quoted fee (fee token base units): %d", requiredFee)

		// Successful send (no tokens): Router returns a message ID and emits CCIPSendRequested.
		msgNoTokens := msg
		msgNoTokens.TokenAmounts = nil
		msgNoTokens.Data = []byte("integration ccip_send without tokens")

		feeNoTokens, err := stack.RouterClient.GetFee(ctx, remoteDestChain, msgNoTokens)
		if err != nil {
			t.Fatalf("Router GetFee (no tokens): %v", err)
		}
		if feeNoTokens <= 0 {
			t.Fatalf("expected positive fee, got %d", feeNoTokens)
		}

		latest, err := rpcClient.GetLatestLedger(ctx)
		if err != nil {
			t.Fatalf("GetLatestLedger: %v", err)
		}
		startLedger := latest.Sequence

		messageID, err := stack.RouterClient.CcipSend(ctx, deployerAddr, remoteDestChain, msgNoTokens, feeNoTokens)
		if err != nil {
			t.Fatalf("Router CcipSend (no tokens): %v", err)
		}
		if messageID == [32]byte{} {
			t.Fatal("CcipSend returned empty message_id")
		}
		t.Logf("message_id: %x", messageID)

		const eventWait = 30 * time.Second
		sendEvt, err := stack.RouterClient.WaitForCCIPSendRequestedEvent(ctx, startLedger, eventWait,
			func(e *routerbindings.CCIPSendRequestedEvent) bool {
				if e.DestChainSelector != remoteDestChain || e.Sender != deployerAddr {
					return false
				}
				return bytes.Equal(e.MessageId[:], messageID[:])
			})
		if err != nil {
			t.Fatalf("WaitForCCIPSendRequestedEvent: %v", err)
		}
		if !bytes.Equal(sendEvt.MessageId[:], messageID[:]) {
			t.Fatalf("event message_id %x != return value %x", sendEvt.MessageId, messageID)
		}
		t.Logf("CCIPSendRequested at ledger %d tx %s", sendEvt.Ledger, sendEvt.TxHash)

		senderBefore := sacBalanceOrFatal(ctx, t, deployer, sacToken, deployerAddr)
		poolBefore := sacBalanceOrFatal(ctx, t, deployer, sacToken, stack.TokenPoolID)
		t.Logf("SAC balances before token send: sender=%d pool=%d", senderBefore, poolBefore)

		// Token transfer: deployer authorizes SAC transfer into the pool via simulation-derived auth (see deployment.Deployer).
		seqForTokenSend, err := outWire.OnRampClient.GetExpectedNextMessageNumber(ctx, remoteDestChain)
		if err != nil {
			t.Fatalf("GetExpectedNextMessageNumber before token send: %v", err)
		}

		tokenXferEncoded, err := EncodeCcipTokenTransferV1(
			tokenTransferAmount,
			stack.TokenPoolID,
			sacToken,
			remoteToken,
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("EncodeCcipTokenTransferV1: %v", err)
		}

		latest2, err := rpcClient.GetLatestLedger(ctx)
		if err != nil {
			t.Fatalf("GetLatestLedger (token send): %v", err)
		}
		tokenMsgID, err := stack.RouterClient.CcipSend(ctx, deployerAddr, remoteDestChain, msg, requiredFee)
		if err != nil {
			t.Fatalf("Router CcipSend (with SAC token): %v", err)
		}
		if tokenMsgID == [32]byte{} {
			t.Fatal("CcipSend (token) returned empty message_id")
		}
		t.Logf("token transfer message_id: %x", tokenMsgID)

		tokenEvt, err := stack.RouterClient.WaitForCCIPSendRequestedEvent(ctx, latest2.Sequence, eventWait,
			func(e *routerbindings.CCIPSendRequestedEvent) bool {
				if e.DestChainSelector != remoteDestChain || e.Sender != deployerAddr {
					return false
				}
				return bytes.Equal(e.MessageId[:], tokenMsgID[:])
			})
		if err != nil {
			t.Fatalf("WaitForCCIPSendRequestedEvent (token send): %v", err)
		}
		t.Logf("token CCIPSendRequested at ledger %d tx %s", tokenEvt.Ledger, tokenEvt.TxHash)

		predictedID, err := PredictStellarOnrampMessageID(StellarOnrampMessageIDInput{
			SourceChainSelector: localChain,
			DestChainSelector:   remoteDestChain,
			SequenceNumber:      seqForTokenSend,
			GasLimit:            0,
			BlockConfirmations:  0,
			Ccvs:                []string{stack.VvrID},
			Executor:            defaultExecutor,
			OnRampContractID:    outWire.OnRampID,
			OffRampRawBytes:     offRampRawPlaceholder,
			SenderStrkey:        deployerAddr,
			Receiver:            evmReceiver,
			Data:                msg.Data,
			TokenTransfer:       tokenXferEncoded,
		})
		if err != nil {
			t.Fatalf("PredictStellarOnrampMessageID: %v", err)
		}
		if predictedID != tokenMsgID {
			t.Fatalf("off-chain message_id %x != Router CcipSend %x", predictedID, tokenMsgID)
		}
		if predictedID != tokenEvt.MessageId {
			t.Fatalf("off-chain message_id %x != CCIPSendRequested event %x", predictedID, tokenEvt.MessageId)
		}

		senderAfter := sacBalanceOrFatal(ctx, t, deployer, sacToken, deployerAddr)
		poolAfter := sacBalanceOrFatal(ctx, t, deployer, sacToken, stack.TokenPoolID)
		t.Logf("SAC balances after token send: sender=%d pool=%d", senderAfter, poolAfter)

		if got := senderBefore - senderAfter; got != tokenTransferAmount {
			t.Fatalf("sender SAC balance should drop by %d; before=%d after=%d (delta=%d)",
				tokenTransferAmount, senderBefore, senderAfter, got)
		}
		if got := poolAfter - poolBefore; got != tokenTransferAmount {
			t.Fatalf("pool SAC balance should increase by %d; before=%d after=%d (delta=%d)",
				tokenTransferAmount, poolBefore, poolAfter, got)
		}
	})
}

// sacBalanceOrFatal reads an SPL/SAC balance for a holder (G… account or C… contract) via simulate-only contract call.
func sacBalanceOrFatal(ctx context.Context, t *testing.T, deployer *deployment.Deployer, sacContract, holderStrkey string) int64 {
	t.Helper()
	args := []xdr.ScVal{scval.AddressToScVal(holderStrkey)}
	res, err := deployer.SimulateContract(ctx, sacContract, "balance", args)
	if err != nil {
		t.Fatalf("SAC balance(holder=%s): %v", holderStrkey, err)
	}
	if res == nil {
		t.Fatalf("SAC balance(holder=%s): nil result", holderStrkey)
	}
	bal, err := scval.I128FromScVal(*res)
	if err != nil {
		t.Fatalf("parse SAC balance: %v", err)
	}
	return bal
}
