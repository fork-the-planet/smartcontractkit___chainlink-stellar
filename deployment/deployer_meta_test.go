package deployment

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractWASMHash(t *testing.T) {
	t.Run("nil meta", func(t *testing.T) {
		_, err := extractWASMHash(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil transaction meta")
	})

	t.Run("unsupported meta version", func(t *testing.T) {
		meta := &xdr.TransactionMeta{V: 2}
		_, err := extractWASMHash(meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported version")
	})
}

func TestExtractContractID(t *testing.T) {
	t.Run("nil meta", func(t *testing.T) {
		_, err := extractContractID(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil transaction meta")
	})

	t.Run("unsupported meta version", func(t *testing.T) {
		meta := &xdr.TransactionMeta{V: 2}
		_, err := extractContractID(meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported version")
	})
}

func TestExtractReturnValue(t *testing.T) {
	t.Run("nil meta", func(t *testing.T) {
		v, err := extractReturnValue(nil)
		require.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("unsupported meta version", func(t *testing.T) {
		meta := &xdr.TransactionMeta{V: 99}
		_, err := extractReturnValue(meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported transaction meta version")
	})
}
