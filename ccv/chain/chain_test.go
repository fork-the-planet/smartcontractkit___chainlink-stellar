package ccvchain

import (
	"strings"
	"testing"

	"github.com/rs/zerolog"
	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/ccv/chains/evm/deployment/v1_7_0/versioned_verifier_resolver"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChain(t *testing.T) {
	logger := zerolog.Nop()
	chain := New(logger, 1)
	require.NotNil(t, chain)
	assert.Equal(t, chainsel.FamilyStellar, chain.ChainFamily())
}

func TestGenerateContractAddress(t *testing.T) {
	networkPassphrase := "Test SDF Network ; September 2015"

	// Test that generateContractAddress creates proper 32-byte addresses
	addr := generateContractAddress("test-contract", networkPassphrase)
	assert.Len(t, addr, stellarAddressLen)

	// Test determinism - same inputs should produce same output
	addr2 := generateContractAddress("test-contract", networkPassphrase)
	assert.Equal(t, addr, addr2)

	// Test that different names produce different addresses
	addr3 := generateContractAddress("other-contract", networkPassphrase)
	assert.NotEqual(t, addr, addr3)

	// Test that different network passphrases produce different addresses
	addr4 := generateContractAddress("test-contract", "Public Global Stellar Network ; September 2015")
	assert.NotEqual(t, addr, addr4)
}

func TestGenerateAccountAddress(t *testing.T) {
	// Test that generateAccountAddress creates valid Stellar addresses
	addr, err := generateAccountAddress("test-seed")
	require.NoError(t, err)

	// Stellar account addresses start with 'G'
	assert.True(t, len(addr) == 56, "Stellar address should be 56 characters")
	assert.Equal(t, byte('G'), addr[0], "Stellar account address should start with G")

	// Test determinism - same seed should produce same address
	addr2, err := generateAccountAddress("test-seed")
	require.NoError(t, err)
	assert.Equal(t, addr, addr2)

	// Test that different seeds produce different addresses
	addr3, err := generateAccountAddress("other-seed")
	require.NoError(t, err)
	assert.NotEqual(t, addr, addr3)
}

func TestGetConnectionProfile(t *testing.T) {
	logger := zerolog.Nop()
	selector := uint64(12345)
	chain := New(logger, selector)

	chainDef, cvConfig, err := chain.GetConnectionProfile(nil, selector)
	require.NoError(t, err)

	t.Run("chain definition has correct selector and address length", func(t *testing.T) {
		assert.Equal(t, selector, chainDef.Selector)
		assert.Equal(t, uint8(stellarAddressLen), chainDef.AddressBytesLength)
	})

	t.Run("base execution gas cost is set", func(t *testing.T) {
		assert.Equal(t, uint32(100_000), chainDef.BaseExecutionGasCost)
	})

	t.Run("default inbound CCVs reference the VVR contract", func(t *testing.T) {
		require.Len(t, chainDef.DefaultInboundCCVs, 1)
		ref := chainDef.DefaultInboundCCVs[0]
		assert.Equal(t, datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType), ref.Type)
		assert.Equal(t, versioned_verifier_resolver.Version, ref.Version)
		assert.Equal(t, selector, ref.ChainSelector)
		assert.Equal(t, devenvcommon.DefaultCommitteeVerifierQualifier, ref.Qualifier)
	})

	t.Run("default outbound CCVs reference the VVR contract", func(t *testing.T) {
		require.Len(t, chainDef.DefaultOutboundCCVs, 1)
		ref := chainDef.DefaultOutboundCCVs[0]
		assert.Equal(t, datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType), ref.Type)
		assert.Equal(t, versioned_verifier_resolver.Version, ref.Version)
		assert.Equal(t, selector, ref.ChainSelector)
		assert.Equal(t, devenvcommon.DefaultCommitteeVerifierQualifier, ref.Qualifier)
	})

	t.Run("committee verifier config has verification gas set", func(t *testing.T) {
		assert.Equal(t, uint32(10_000), cvConfig.GasForVerification)
	})

	t.Run("uses the provided selector, not the chain's own", func(t *testing.T) {
		otherSelector := uint64(99999)
		def, _, err := chain.GetConnectionProfile(nil, otherSelector)
		require.NoError(t, err)
		assert.Equal(t, otherSelector, def.Selector)
		assert.Equal(t, otherSelector, def.DefaultInboundCCVs[0].ChainSelector)
		assert.Equal(t, otherSelector, def.DefaultOutboundCCVs[0].ChainSelector)
	})
}

func TestPostConnect(t *testing.T) {
	logger := zerolog.Nop()
	chain := New(logger, 1)

	err := chain.PostConnect(nil, 1, []uint64{2, 3})
	assert.NoError(t, err, "PostConnect is a no-op stub and should always succeed")
}

func TestStellarAdapter(t *testing.T) {
	adapter := NewChainFamilyAdapter()
	require.NotNil(t, adapter)

	t.Run("decodes contract address (C...)", func(t *testing.T) {
		contractAddr := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
		ref := datastore.AddressRef{Address: contractAddr}
		decoded, err := adapter.AddressRefToBytes(ref)
		require.NoError(t, err)
		assert.Len(t, decoded, 32)
	})

	t.Run("decodes account address (G...)", func(t *testing.T) {
		accountAddr := "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7"
		ref := datastore.AddressRef{Address: accountAddr}
		decoded, err := adapter.AddressRefToBytes(ref)
		require.NoError(t, err)
		assert.Len(t, decoded, 32)
	})

	t.Run("decodes 32-byte hex address", func(t *testing.T) {
		hexAddr := "0x" + strings.Repeat("ab", 32)
		ref := datastore.AddressRef{Address: hexAddr}
		decoded, err := adapter.AddressRefToBytes(ref)
		require.NoError(t, err)
		assert.Len(t, decoded, 32)
	})

	t.Run("decodes 32-byte hex address without 0x prefix", func(t *testing.T) {
		hexAddr := strings.Repeat("cd", 32)
		ref := datastore.AddressRef{Address: hexAddr}
		decoded, err := adapter.AddressRefToBytes(ref)
		require.NoError(t, err)
		assert.Len(t, decoded, 32)
	})

	t.Run("fails on invalid address", func(t *testing.T) {
		ref := datastore.AddressRef{Address: "not-a-valid-address"}
		_, err := adapter.AddressRefToBytes(ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode Stellar address")
	})

	t.Run("fails on short hex (not 32 bytes)", func(t *testing.T) {
		ref := datastore.AddressRef{Address: "0xabcdef"}
		_, err := adapter.AddressRefToBytes(ref)
		require.Error(t, err)
	})
}
