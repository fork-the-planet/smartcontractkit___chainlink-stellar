//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	cciprecv "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/ccip_receiver"
	ccvsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/committee_verifier"
	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	rmnproxybindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_proxy"
	rmnremotebindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	vvrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/versioned_verifier_resolver"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
	"github.com/stellar/go-stellar-sdk/strkey"
)

const (
	remoteSourceChain = uint64(99999)

	ccipVerifierVersion0 = byte(0x49)
	ccipVerifierVersion1 = byte(0xff)
	ccipVerifierVersion2 = byte(0x34)
	ccipVerifierVersion3 = byte(0xed)
)

// fullStack holds all deployed contract IDs and clients needed for execute-path tests.
type fullStack struct {
	RmnRemoteID string
	RmnProxyID  string
	CcvID       string
	VvrID       string
	RouterID    string
	OfframpID   string
	ReceiverID  string

	RmnRemoteClient *rmnremotebindings.RmnRemoteClient
	OfframpClient   *offrampbindings.OffRampClient
	RouterClient    *routerbindings.RouterClient
	CcvClient       *ccvsbindings.CommitteeVerifierClient
	VvrClient       *vvrbindings.VersionedVerifierResolverClient

	OnRampWire    []byte
	OffRampSuffix []byte
	ReceiverRaw   []byte

	signerKey     *ecdsa.PrivateKey
	signerAddrPad [32]byte // left-padded 20-byte Ethereum address
}

// deployFullStack deploys and wires the complete contract stack needed for
// OffRamp execute tests. The saltPrefix differentiates contract instances so
// multiple stacks can coexist in the same shared Stellar environment.
func deployFullStack(
	ctx context.Context,
	t *testing.T,
	projectRoot string,
	deployer *deployment.Deployer,
	deployerAddr string,
	destChainSelector uint64,
	saltPrefix string,
) *fullStack {
	t.Helper()

	deploy := func(name, wasmFile string) string {
		t.Helper()
		salt := deployment.GenerateDeterministicSalt(deployerAddr, saltPrefix+"-"+name)
		p := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", wasmFile)
		id, err := deployer.DeployContract(ctx, p, salt)
		if err != nil {
			t.Fatalf("deploy %s: %v", name, err)
		}
		return id
	}

	s := &fullStack{}

	s.RmnRemoteID = deploy("rmn-remote", "rmn_remote.wasm")
	s.RmnProxyID = deploy("rmn-proxy", "rmn_proxy.wasm")
	s.CcvID = deploy("ccv", "ccvs_committee_verifier.wasm")
	s.VvrID = deploy("vvr", "ccvs_versioned_verifier_resolver.wasm")
	s.RouterID = deploy("router", "router.wasm")
	s.OfframpID = deploy("offramp", "offramp.wasm")
	s.ReceiverID = deploy("ccip-recv", "ccip_receiver_example.wasm")

	s.RmnRemoteClient = rmnremotebindings.NewRmnRemoteClient(deployer, s.RmnRemoteID)
	s.CcvClient = ccvsbindings.NewCommitteeVerifierClient(deployer, s.CcvID)
	s.VvrClient = vvrbindings.NewVersionedVerifierResolverClient(deployer, s.VvrID)
	s.RouterClient = routerbindings.NewRouterClient(deployer, s.RouterID)
	s.OfframpClient = offrampbindings.NewOffRampClient(deployer, s.OfframpID)

	// 1. RMN Remote
	if err := s.RmnRemoteClient.Initialize(ctx, deployerAddr, destChainSelector); err != nil {
		t.Fatalf("RMN Remote Initialize: %v", err)
	}

	// 2. RMN Proxy
	rmnProxyClient := rmnproxybindings.NewRmnProxyClient(deployer, s.RmnProxyID)
	if err := rmnProxyClient.Initialize(ctx, deployerAddr, s.RmnRemoteID); err != nil {
		t.Fatalf("RMN Proxy Initialize: %v", err)
	}

	// 3. CommitteeVerifier
	mockFeeAgg := helpers.GenerateMockContractID(t, deployerAddr, saltPrefix+"-fee-agg")
	if err := s.CcvClient.Initialize(ctx, deployerAddr, ccvsbindings.DynamicConfig{
		FeeAggregator: &mockFeeAgg,
	}, [][]byte{}, s.RmnProxyID); err != nil {
		t.Fatalf("CommitteeVerifier Initialize: %v", err)
	}

	// 4. Signature quorum for remote source chain with secp256k1 ECDSA keypair
	ecdsaKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey: %v", err)
	}
	s.signerKey = ecdsaKey
	ethAddr := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	copy(s.signerAddrPad[12:], ethAddr.Bytes())
	if err := s.CcvClient.ApplySignatureConfigs(ctx, nil, []ccvsbindings.SignatureQuorumConfig{
		{SourceChainSelector: remoteSourceChain, Threshold: 1, Signers: [][32]byte{s.signerAddrPad}},
	}); err != nil {
		t.Fatalf("ApplySignatureConfigs: %v", err)
	}

	// 5. VVR
	if err := s.VvrClient.Initialize(ctx, deployerAddr, mockFeeAgg); err != nil {
		t.Fatalf("VVR Initialize: %v", err)
	}

	// 6. Register CommitteeVerifier as inbound implementation on VVR
	verAddr := s.CcvID
	if err := s.VvrClient.ApplyInboundImplUpdates(ctx, []vvrbindings.InboundImplementationUpdate{
		{
			Verifier: &verAddr,
			Version:  [4]byte{ccipVerifierVersion0, ccipVerifierVersion1, ccipVerifierVersion2, ccipVerifierVersion3},
		},
	}); err != nil {
		t.Fatalf("ApplyInboundImplUpdates: %v", err)
	}

	// 7. Router
	if err := s.RouterClient.Initialize(ctx, deployerAddr, s.RmnProxyID); err != nil {
		t.Fatalf("Router Initialize: %v", err)
	}

	// 8. OffRamp
	mockTokenAdminReg := helpers.GenerateMockContractID(t, deployerAddr, saltPrefix+"-token-admin")
	if err := s.OfframpClient.Initialize(ctx, deployerAddr, offrampbindings.StaticConfig{
		ChainSelector:      destChainSelector,
		RmnProxy:           s.RmnProxyID,
		TokenAdminRegistry: mockTokenAdminReg,
	}); err != nil {
		t.Fatalf("OffRamp Initialize: %v", err)
	}

	// 9. CCIP Receiver
	recvClient := cciprecv.NewExampleCcipReceiverClient(deployer, s.ReceiverID)
	if err := recvClient.Initialize(ctx, s.RouterID); err != nil {
		t.Fatalf("CcipReceiver Initialize: %v", err)
	}

	// 10. Register OffRamp on Router for remote source chain
	if err := s.RouterClient.AddOfframp(ctx, remoteSourceChain, s.OfframpID); err != nil {
		t.Fatalf("AddOfframp: %v", err)
	}

	// 11. Source chain config on OffRamp
	s.OnRampWire = bytes.Repeat([]byte{0xab}, 32)
	if err := s.OfframpClient.ApplySourceChainCfgUpdates(ctx, []offrampbindings.SourceChainConfigArgs{
		{
			SourceChainSelector: remoteSourceChain,
			Router:              s.RouterID,
			IsEnabled:           true,
			OnRamps:             [][]byte{s.OnRampWire},
			DefaultCcvs:         []string{s.VvrID},
			LaneMandatedCcvs:    nil,
		},
	}); err != nil {
		t.Fatalf("ApplySourceChainCfgUpdates: %v", err)
	}

	// Pre-compute commonly needed byte representations
	s.OffRampSuffix, err = contractAddressScValSuffix32(s.OfframpID)
	if err != nil {
		t.Fatalf("offramp scval suffix: %v", err)
	}
	s.ReceiverRaw, err = strkey.Decode(strkey.VersionByteContract, s.ReceiverID)
	if err != nil {
		t.Fatalf("decode receiver contract: %v", err)
	}

	return s
}

// signVerifierBlob builds a verifier result blob containing a real secp256k1 ECDSA
// signature in EIP-2098 compact format over the message hash:
//
//	signed_hash = keccak256(VERSION_TAG || message_hash)
//	blob = [4B version_tag][2B sig_payload_len][64B r||yParityAndS]
func (s *fullStack) signVerifierBlob(t *testing.T, messageHash [32]byte) []byte {
	t.Helper()

	versionTag := [4]byte{ccipVerifierVersion0, ccipVerifierVersion1, ccipVerifierVersion2, ccipVerifierVersion3}

	var signedPayload []byte
	signedPayload = append(signedPayload, versionTag[:]...)
	signedPayload = append(signedPayload, messageHash[:]...)
	signedHash := crypto.Keccak256(signedPayload)

	sig65, err := crypto.Sign(signedHash, s.signerKey)
	if err != nil {
		t.Fatalf("crypto.Sign: %v", err)
	}

	compact := ecdsaToEIP2098Compact(t, sig65)

	const perSigBytes = 64
	var blob []byte
	blob = append(blob, versionTag[:]...)
	blob = binary.BigEndian.AppendUint16(blob, perSigBytes)
	blob = append(blob, compact[:]...)

	return blob
}

// ecdsaToEIP2098Compact converts a 65-byte R||S||V signature to 64-byte EIP-2098 compact format.
func ecdsaToEIP2098Compact(t *testing.T, sig65 []byte) [64]byte {
	t.Helper()
	if len(sig65) != 65 {
		t.Fatalf("expected 65-byte signature, got %d", len(sig65))
	}

	var compact [64]byte
	copy(compact[:32], sig65[0:32])  // R
	copy(compact[32:], sig65[32:64]) // S

	v := sig65[64]
	// go-ethereum returns v as 0 or 1; ensure low-S
	s := new(big.Int).SetBytes(compact[32:])
	halfN := new(big.Int).Rsh(crypto.S256().Params().N, 1)
	if s.Cmp(halfN) > 0 {
		t.Fatalf("crypto.Sign should produce low-S signatures")
	}

	// Pack recovery ID into bit 255 of yParityAndS
	if v == 1 {
		compact[32] |= 0x80
	}
	return compact
}

// buildValidMessage constructs an encoded CCIP v1 message targeting the fullStack's
// receiver, using the given sequence number and data payload.
// Returns the encoded message, its keccak256 ID, and a signed verifier blob.
func (s *fullStack) buildValidMessage(t *testing.T, destChainSelector uint64, seqNo uint64, data []byte) (encoded []byte, msgID [32]byte, verifierBlob []byte) {
	t.Helper()

	sender := bytes.Repeat([]byte{0xcd}, 20)
	var ccvHashZero [32]byte
	var err error
	encoded, err = encodeCcipMessageV1(ccipV1Wire{
		SourceChainSelector: remoteSourceChain,
		DestChainSelector:   destChainSelector,
		SequenceNumber:      seqNo,
		ExecutionGasLimit:   500_000,
		CcipReceiveGasLimit: 200_000,
		Finality:            0,
		CcvExecutorHash:     ccvHashZero,
		OnRampAddress:       s.OnRampWire,
		OffRampAddress:      s.OffRampSuffix,
		Sender:              sender,
		Receiver:            s.ReceiverRaw,
		DestBlob:            nil,
		TokenTransfer:       nil,
		Data:                data,
	})
	if err != nil {
		t.Fatalf("encodeCcipMessageV1: %v", err)
	}
	msgID = keccak256MessageID(encoded)
	verifierBlob = s.signVerifierBlob(t, msgID)
	return encoded, msgID, verifierBlob
}

