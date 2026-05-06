package accessors

import (
	"context"
	"strconv"
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	sourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactory_GetAccessor_validation(t *testing.T) {
	ctx := context.Background()
	lggr := logger.Test(t)
	stellarSel := chainsel.STELLAR_LOCALNET.Selector
	stellarKey := strconv.FormatUint(stellarSel, 10)

	t.Run("nil config map", func(t *testing.T) {
		f := NewReaderFactory(lggr, nil)
		acc, err := f.GetAccessor(ctx, protocol.ChainSelector(stellarSel))
		require.Error(t, err)
		require.Nil(t, acc)
		assert.Contains(t, err.Error(), "stellar ccip config is not set")
	})

	t.Run("non-stellar chain is rejected", func(t *testing.T) {
		evmSel, err := chainsel.SelectorFromChainId(1)
		require.NoError(t, err)

		f := NewReaderFactory(lggr, map[string]sourcereader.ReaderConfig{
			stellarKey: {SorobanRPCURL: "http://localhost:8000", NetworkPassphrase: "p"},
		})
		acc, err := f.GetAccessor(ctx, protocol.ChainSelector(evmSel))
		require.Error(t, err)
		require.Nil(t, acc)
		assert.Contains(t, err.Error(), "only stellar is supported")
	})

	t.Run("missing config for stellar selector", func(t *testing.T) {
		f := NewReaderFactory(lggr, map[string]sourcereader.ReaderConfig{})
		acc, err := f.GetAccessor(ctx, protocol.ChainSelector(stellarSel))
		require.Error(t, err)
		require.Nil(t, acc)
		assert.Contains(t, err.Error(), "stellar config not found")
	})

	t.Run("empty soroban rpc url deferred to SourceReader", func(t *testing.T) {
		f := NewReaderFactory(lggr, map[string]sourcereader.ReaderConfig{
			stellarKey: {NetworkPassphrase: "pass", SorobanRPCURL: ""},
		})
		acc, err := f.GetAccessor(ctx, protocol.ChainSelector(stellarSel))
		require.NoError(t, err, "GetAccessor should succeed even when SourceReader cannot be built")
		require.NotNil(t, acc)
		_, srErr := acc.SourceReader()
		require.Error(t, srErr)
		assert.Contains(t, srErr.Error(), "soroban rpc url is required")
	})

	t.Run("empty network passphrase deferred to SourceReader", func(t *testing.T) {
		f := NewReaderFactory(lggr, map[string]sourcereader.ReaderConfig{
			stellarKey: {SorobanRPCURL: "http://localhost:8000", NetworkPassphrase: ""},
		})
		acc, err := f.GetAccessor(ctx, protocol.ChainSelector(stellarSel))
		require.NoError(t, err, "GetAccessor should succeed even when SourceReader cannot be built")
		require.NotNil(t, acc)
		_, srErr := acc.SourceReader()
		require.Error(t, srErr)
		assert.Contains(t, srErr.Error(), "network passphrase is required")
	})

	t.Run("destination reader unavailable without dest config", func(t *testing.T) {
		f := NewReaderFactory(lggr, map[string]sourcereader.ReaderConfig{
			stellarKey: {SorobanRPCURL: "http://localhost:8000", NetworkPassphrase: "p"},
		})
		acc, err := f.GetAccessor(ctx, protocol.ChainSelector(stellarSel))
		require.NoError(t, err)
		_, drErr := acc.DestinationReader()
		require.Error(t, drErr)
		assert.ErrorIs(t, drErr, errDestConfigMissing)
		_, ctErr := acc.ContractTransmitter()
		require.Error(t, ctErr)
		assert.ErrorIs(t, ctErr, errDestConfigMissing)
	})

	t.Run("source reader requires keystore even when reader cfg is valid", func(t *testing.T) {
		f := NewReaderFactory(lggr, map[string]sourcereader.ReaderConfig{
			stellarKey: {
				SorobanRPCURL:     "http://localhost:8000",
				NetworkPassphrase: "p",
				OnRampContractID:  "CA1234567890123456789012345678901234567890123456789012345AA",
			},
		})
		acc, err := f.GetAccessor(ctx, protocol.ChainSelector(stellarSel))
		require.NoError(t, err)
		_, srErr := acc.SourceReader()
		require.Error(t, srErr)
		assert.ErrorIs(t, srErr, errKeystoreNotInjected)
	})
}
