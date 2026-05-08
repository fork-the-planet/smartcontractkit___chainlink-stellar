package stellarutil

import (
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateContractAddress(t *testing.T) {
	networkPassphrase := "Test SDF Network ; September 2015"

	addr := GenerateContractAddress("test-contract", networkPassphrase)
	assert.Len(t, addr, 32)

	addr2 := GenerateContractAddress("test-contract", networkPassphrase)
	assert.Equal(t, addr, addr2)

	addr3 := GenerateContractAddress("other-contract", networkPassphrase)
	assert.NotEqual(t, addr, addr3)

	addr4 := GenerateContractAddress("test-contract", "Public Global Stellar Network ; September 2015")
	assert.NotEqual(t, addr, addr4)
}

func TestParseFeeAggregatorAddress(t *testing.T) {
	t.Run("account_strkey", func(t *testing.T) {
		kp, err := keypair.FromRawSeed([32]byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0})
		require.NoError(t, err)
		got, err := ParseFeeAggregatorAddress(kp.Address())
		require.NoError(t, err)
		require.Equal(t, kp.Address(), got)
	})
	t.Run("hex_roundtrip", func(t *testing.T) {
		kp, err := keypair.FromRawSeed([32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		require.NoError(t, err)
		hexAddr, err := StrkeyToHex(kp.Address())
		require.NoError(t, err)
		got, err := ParseFeeAggregatorAddress(hexAddr)
		require.NoError(t, err)
		require.Equal(t, kp.Address(), got)
	})
	t.Run("empty", func(t *testing.T) {
		_, err := ParseFeeAggregatorAddress("  ")
		require.Error(t, err)
	})
}

func TestStrkeyToHex(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := StrkeyToHex("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported strkey prefix")
	})

	t.Run("unsupported_prefix", func(t *testing.T) {
		_, err := StrkeyToHex("Xnotvalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported strkey prefix")
	})

	t.Run("contract_prefix_invalid_strkey", func(t *testing.T) {
		_, err := StrkeyToHex("CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "decode strkey") || strings.Contains(err.Error(), "decode"))
	})

	t.Run("valid_contract_strkey", func(t *testing.T) {
		id := MustGenerateMockContractID("G-test-deployer", "strkey-hex-test")
		got, err := StrkeyToHex(id)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(got, "0x"))
		assert.Len(t, got, 2+64) // 32 bytes hex
	})

	t.Run("valid_account_strkey", func(t *testing.T) {
		kp, err := keypair.FromRawSeed([32]byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0})
		require.NoError(t, err)
		got, err := StrkeyToHex(kp.Address())
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(got, "0x"))
		assert.Len(t, got, 2+64)
	})
}
