package config

import (
	"testing"
	"time"

	chain_selectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/chainlink-stellar/relayer/txm"
	"github.com/stretchr/testify/require"
)

func TestNewDecodedTOMLConfig_TxManagerOverrides(t *testing.T) {
	t.Parallel()

	raw := `
ChainID = "` + chain_selectors.STELLAR_TESTNET.ChainID + `"
[[Nodes]]
Name = "primary"
URL = "https://example.invalid"

[TxManager]
BroadcastChanSize = 77
`
	cfg, err := NewDecodedTOMLConfig(raw)
	require.NoError(t, err)
	require.NotNil(t, cfg.TxManager.BroadcastChanSize)
	require.Equal(t, uint(77), *cfg.TxManager.BroadcastChanSize)

	txmCfg := cfg.TxManager
	txmCfg.Resolve()
	require.Equal(t, uint(77), *txmCfg.BroadcastChanSize)
	require.Equal(t, *txm.DefaultConfigSet.MaxInclusionFee, *txmCfg.MaxInclusionFee,
		"unset TxManager fields should still resolve to txm defaults")
}

func TestSetDefaults_MultiNode(t *testing.T) {
	t.Parallel()

	t.Run("fills Stellar-appropriate defaults", func(t *testing.T) {
		raw := `
ChainID = "` + chain_selectors.STELLAR_TESTNET.ChainID + `"
[[Nodes]]
Name = "primary"
URL = "https://example.invalid"
`
		cfg, err := NewDecodedTOMLConfig(raw)
		require.NoError(t, err)

		m := &cfg.MultiNode
		require.Equal(t, "HighestHead", m.SelectionMode())
		require.Equal(t, 3*time.Second, m.NewHeadsPollInterval())
		require.True(t, m.VerifyChainID())
		// Single-finality model: no finality tag, finalized == latest.
		require.False(t, m.FinalityTagEnabled())
		require.Equal(t, uint32(0), m.FinalityDepth())
		// Read unconditionally by the node lifecycle, so must be non-nil even though unused.
		require.NotNil(t, cfg.MultiNode.MultiNode.NoNewFinalizedHeadsThreshold)
	})

	t.Run("respects explicit overrides", func(t *testing.T) {
		raw := `
ChainID = "` + chain_selectors.STELLAR_TESTNET.ChainID + `"
[[Nodes]]
Name = "primary"
URL = "https://example.invalid"

[MultiNode]
SelectionMode = "RoundRobin"
NewHeadsPollInterval = "1s"
`
		cfg, err := NewDecodedTOMLConfig(raw)
		require.NoError(t, err)
		require.Equal(t, "RoundRobin", cfg.MultiNode.SelectionMode())
		require.Equal(t, 1*time.Second, cfg.MultiNode.NewHeadsPollInterval())
	})
}
