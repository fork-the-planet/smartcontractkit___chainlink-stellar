package stellarutil

import (
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccipOffchain "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/offchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSignersFromTopology(t *testing.T) {
	evm := chainsel.FamilyEVM

	t.Run("nil_topology", func(t *testing.T) {
		signers, th := ResolveSignersFromTopology(nil, 1, evm)
		assert.Nil(t, signers)
		assert.Equal(t, uint32(0), th)
	})

	t.Run("nil_NOPTopology", func(t *testing.T) {
		topo := &ccipOffchain.EnvironmentTopology{NOPTopology: nil}
		signers, th := ResolveSignersFromTopology(topo, 1, evm)
		assert.Nil(t, signers)
		assert.Equal(t, uint32(0), th)
	})

	t.Run("no_matching_chain", func(t *testing.T) {
		topo := &ccipOffchain.EnvironmentTopology{
			NOPTopology: &ccipOffchain.NOPTopology{
				NOPs: []ccipOffchain.NOPConfig{
					{Alias: "a", SignerAddressByFamily: map[string]string{evm: "1111111111111111111111111111111111111111"}},
				},
				Committees: map[string]ccipOffchain.CommitteeConfig{
					"c": {
						ChainConfigs: map[string]ccipOffchain.ChainCommitteeConfig{
							"999": {NOPAliases: []string{"a"}, Threshold: 1},
						},
					},
				},
			},
		}
		signers, th := ResolveSignersFromTopology(topo, 12345, evm)
		assert.Nil(t, signers)
		assert.Equal(t, uint32(0), th)
	})

	t.Run("sorts_signers_and_returns_threshold", func(t *testing.T) {
		topo := &ccipOffchain.EnvironmentTopology{
			NOPTopology: &ccipOffchain.NOPTopology{
				NOPs: []ccipOffchain.NOPConfig{
					{Alias: "nopA", SignerAddressByFamily: map[string]string{evm: "1111111111111111111111111111111111111111"}},
					{Alias: "nopB", SignerAddressByFamily: map[string]string{evm: "2222222222222222222222222222222222222222"}},
				},
				Committees: map[string]ccipOffchain.CommitteeConfig{
					"only": {
						ChainConfigs: map[string]ccipOffchain.ChainCommitteeConfig{
							"12345": {
								NOPAliases: []string{"nopB", "nopA"},
								Threshold:  7,
							},
						},
					},
				},
			},
		}
		signers, th := ResolveSignersFromTopology(topo, 12345, evm)
		require.Len(t, signers, 2)
		assert.Equal(t, uint32(7), th)
		var wantA, wantB [32]byte
		copy(wantA[12:], []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11})
		copy(wantB[12:], []byte{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22})
		assert.Equal(t, wantA, signers[0])
		assert.Equal(t, wantB, signers[1])
	})

	t.Run("skips_invalid_and_missing_evm_signer", func(t *testing.T) {
		topo := &ccipOffchain.EnvironmentTopology{
			NOPTopology: &ccipOffchain.NOPTopology{
				NOPs: []ccipOffchain.NOPConfig{
					{Alias: "badhex", SignerAddressByFamily: map[string]string{evm: "not-hex"}},
					{Alias: "wronglen", SignerAddressByFamily: map[string]string{evm: "111111"}},
					{Alias: "nosvm", SignerAddressByFamily: map[string]string{"other": "1111111111111111111111111111111111111111"}},
					{Alias: "good", SignerAddressByFamily: map[string]string{evm: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
				},
				Committees: map[string]ccipOffchain.CommitteeConfig{
					"only": {
						ChainConfigs: map[string]ccipOffchain.ChainCommitteeConfig{
							"1": {
								NOPAliases: []string{"badhex", "wronglen", "nosvm", "missing", "good"},
								Threshold:  3,
							},
						},
					},
				},
			},
		}
		signers, th := ResolveSignersFromTopology(topo, 1, evm)
		require.Len(t, signers, 1)
		assert.Equal(t, uint32(3), th)
	})
}
