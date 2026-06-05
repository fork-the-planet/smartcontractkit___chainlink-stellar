package config

import (
	"testing"

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
