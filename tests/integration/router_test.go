//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	cciprecv "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/ccip_receiver"
	ccvsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/committee_verifier"
	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	vvrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/versioned_verifier_resolver"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
	"github.com/stellar/go-stellar-sdk/strkey"
)

func TestRouter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, _ := GetSharedTestEnv(ctx, t)

	t.Run("deploy and initialize router", func(t *testing.T) {
		t.Log("Deploying Router contract...")
		salt := deployment.GenerateDeterministicSalt(deployerKP.Address(), "router")
		wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "router.wasm")
		contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
		if err != nil {
			t.Fatalf("Failed to deploy Router: %v", err)
		}
		t.Logf("Router deployed at: %s", contractID)

		mockRmnProxy := helpers.GenerateMockContractID(t, deployerKP.Address(), "rmn-proxy-router")
		client := routerbindings.NewRouterClient(deployer, contractID)

		if err := client.Initialize(ctx, deployerKP.Address(), mockRmnProxy); err != nil {
			t.Fatalf("Failed to initialize Router: %v", err)
		}

		cfg, err := client.GetConfig(ctx)
		if err != nil {
			t.Fatalf("GetConfig: %v", err)
		}
		if cfg == nil {
			t.Fatal("GetConfig returned nil")
		}
		if cfg.RmnProxy != mockRmnProxy {
			t.Fatalf("RmnProxy mismatch: want %q, got %q", mockRmnProxy, cfg.RmnProxy)
		}
		t.Log("Router initialized; config matches")
	})

	// End-to-end: OffRamp execute → internal route_message → Router → example ccip_receiver.
	// CommitteeVerifier.validate_signatures is currently a stub (always OK when signature config exists).
	t.Run("offramp execute routes via router to ccip_receiver", func(t *testing.T) {
		const (
			localDestChain       = uint64(12345)
			remoteSourceChain    = uint64(99999)
			ccipVerifierVersion0 = byte(0x49)
			ccipVerifierVersion1 = byte(0xff)
			ccipVerifierVersion2 = byte(0x34)
			ccipVerifierVersion3 = byte(0xed)
		)

		deploy := func(name, wasmFile string) string {
			t.Helper()
			salt := deployment.GenerateDeterministicSalt(deployerKP.Address(), name)
			p := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", wasmFile)
			id, err := deployer.DeployContract(ctx, p, salt)
			if err != nil {
				t.Fatalf("deploy %s: %v", name, err)
			}
			return id
		}

		rmnRemoteID := deploy("router-int-rmn-remote", "rmn_remote.wasm")
		rmnProxyID := deploy("router-int-rmn-proxy", "rmn_proxy.wasm")
		ccvID := deploy("router-int-ccv", "ccvs_committee_verifier.wasm")
		vvrID := deploy("router-int-vvr", "ccvs_versioned_verifier_resolver.wasm")
		routerID := deploy("router-int-router", "router.wasm")
		offrampID := deploy("router-int-offramp", "offramp.wasm")
		receiverID := deploy("router-int-ccip-recv", "ccip_receiver_example.wasm")

		ccvClient := ccvsbindings.NewCommitteeVerifierClient(deployer, ccvID)
		mockFeeAgg := helpers.GenerateMockContractID(t, deployerKP.Address(), "fee-agg-router-int")
		if err := initialize(ctx, t, deployer, deployerKP, rmnRemoteID, rmnProxyID, ccvClient, mockFeeAgg); err != nil {
			t.Fatal(err)
		}

		signerPubKey, signerPrivKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("ed25519.GenerateKey: %v", err)
		}
		var signerPubKeyArr [32]byte
		copy(signerPubKeyArr[:], signerPubKey)
		if err := ccvClient.ApplySignatureConfigs(ctx, nil, []ccvsbindings.SignatureQuorumConfig{
			{SourceChainSelector: remoteSourceChain, Threshold: 1, Signers: [][32]byte{signerPubKeyArr}},
		}); err != nil {
			t.Fatalf("ApplySignatureConfigs: %v", err)
		}

		vvrClient := vvrbindings.NewVersionedVerifierResolverClient(deployer, vvrID)
		if err := vvrClient.Initialize(ctx, deployerKP.Address(), mockFeeAgg); err != nil {
			t.Fatalf("VVR Initialize: %v", err)
		}
		verAddr := ccvID
		if err := vvrClient.ApplyInboundImplUpdates(ctx, []vvrbindings.InboundImplementationUpdate{
			{
				Verifier: &verAddr,
				Version:  [4]byte{ccipVerifierVersion0, ccipVerifierVersion1, ccipVerifierVersion2, ccipVerifierVersion3},
			},
		}); err != nil {
			t.Fatalf("ApplyInboundImplUpdates: %v", err)
		}

		routerClient := routerbindings.NewRouterClient(deployer, routerID)
		if err := routerClient.Initialize(ctx, deployerKP.Address(), rmnProxyID); err != nil {
			t.Fatalf("Router Initialize: %v", err)
		}

		mockTokenAdminReg := helpers.GenerateMockContractID(t, deployerKP.Address(), "token-admin-router-int")
		offrampClient := offrampbindings.NewOffRampClient(deployer, offrampID)
		if err := offrampClient.Initialize(ctx, deployerKP.Address(), offrampbindings.StaticConfig{
			ChainSelector:      localDestChain,
			RmnProxy:           rmnProxyID,
			TokenAdminRegistry: mockTokenAdminReg,
		}); err != nil {
			t.Fatalf("OffRamp Initialize: %v", err)
		}

		recvClient := cciprecv.NewExampleCcipReceiverClient(deployer, receiverID)
		if err := recvClient.Initialize(ctx, routerID); err != nil {
			t.Fatalf("CcipReceiver Initialize: %v", err)
		}

		if err := routerClient.AddOfframp(ctx, remoteSourceChain, offrampID); err != nil {
			t.Fatalf("AddOfframp: %v", err)
		}

		onRampWire := bytes.Repeat([]byte{0xab}, 32)
		offRampSuffix, err := contractAddressScValSuffix32(offrampID)
		if err != nil {
			t.Fatalf("offramp scval suffix: %v", err)
		}
		receiverRaw, err := strkey.Decode(strkey.VersionByteContract, receiverID)
		if err != nil {
			t.Fatalf("decode receiver contract: %v", err)
		}
		if len(receiverRaw) != 32 {
			t.Fatalf("receiver raw len %d, want 32", len(receiverRaw))
		}

		if err := offrampClient.ApplySourceChainCfgUpdates(ctx, []offrampbindings.SourceChainConfigArgs{
			{
				SourceChainSelector: remoteSourceChain,
				Router:              routerID,
				IsEnabled:           true,
				OnRamps:             [][]byte{onRampWire},
				DefaultCcvs:         []string{vvrID},
				LaneMandatedCcvs:    nil,
			},
		}); err != nil {
			t.Fatalf("ApplySourceChainCfgUpdates: %v", err)
		}

		msgData := []byte("ccip-router-integration")
		sender := bytes.Repeat([]byte{0xcd}, 20)
		var ccvHashZero [32]byte
		encoded, err := encodeCcipMessageV1(ccipV1Wire{
			SourceChainSelector: remoteSourceChain,
			DestChainSelector:   localDestChain,
			SequenceNumber:      1,
			ExecutionGasLimit:   500_000,
			CcipReceiveGasLimit: 200_000,
			Finality:            0,
			CcvExecutorHash:     ccvHashZero,
			OnRampAddress:       onRampWire,
			OffRampAddress:      offRampSuffix,
			Sender:              sender,
			Receiver:            receiverRaw,
			DestBlob:            nil,
			TokenTransfer:       nil,
			Data:                msgData,
		})
		if err != nil {
			t.Fatalf("encodeCcipMessageV1: %v", err)
		}

		msgID := keccak256MessageID(encoded)
		versionTag := [4]byte{ccipVerifierVersion0, ccipVerifierVersion1, ccipVerifierVersion2, ccipVerifierVersion3}
		var signedPayload []byte
		signedPayload = append(signedPayload, versionTag[:]...)
		signedPayload = append(signedPayload, msgID[:]...)
		signedHash := crypto.Keccak256(signedPayload)
		sig := ed25519.Sign(signerPrivKey, signedHash)
		const perSigBytes = 32 + 64
		var verifierBlob []byte
		verifierBlob = append(verifierBlob, versionTag[:]...)
		verifierBlob = binary.BigEndian.AppendUint16(verifierBlob, perSigBytes)
		verifierBlob = append(verifierBlob, signerPubKey...)
		verifierBlob = append(verifierBlob, sig...)
		if err := offrampClient.Execute(ctx, encoded, []string{vvrID}, [][]byte{verifierBlob}, 0); err != nil {
			t.Fatalf("OffRamp Execute: %v", err)
		}

		state, err := offrampClient.GetExecutionState(ctx, msgID)
		if err != nil {
			t.Fatalf("GetExecutionState: %v", err)
		}
		if state != offrampbindings.MessageExecutionStateSuccess {
			t.Fatalf("execution state = %d, want Success (%d)", state, offrampbindings.MessageExecutionStateSuccess)
		}

		stored, err := recvClient.LastMessageId(ctx)
		if err != nil {
			t.Fatalf("LastMessageId: %v", err)
		}
		if stored != msgID {
			t.Fatalf("receiver last_message_id mismatch: got %x want %x", stored, msgID)
		}
		t.Log("OffRamp → Router → ccip_receive succeeded; receiver persisted message id")
	})
}
