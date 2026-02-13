package helpers

import (
	"encoding/hex"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stretchr/testify/require"
)

func KeypairFromPrivateKeyHex(t *testing.T, privateKeyHex string) *keypair.Full {
	seedBytes, err := hex.DecodeString(privateKeyHex)
	require.NoError(t, err)

	var seed [32]byte
	copy(seed[:], seedBytes)

	kp, err := keypair.FromRawSeed(seed)
	require.NoError(t, err)

	return kp
}
