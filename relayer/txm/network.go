package txm

import (
	"fmt"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/stellar/go-stellar-sdk/network"
)

// NetworkPassphrase returns the Stellar network passphrase for a given
// chain-selectors chain ID. Passphrases are static per network so we resolve
// them from chainID rather than carrying them through TOML config.
//
// Localnet shares testnet's passphrase by convention — the network ID hash is
// what matters for signing, and Stellar localnet is configured against the
// testnet passphrase.
func NetworkPassphrase(chainID string) (string, error) {
	switch chainID {
	case chainsel.STELLAR_MAINNET.ChainID:
		return network.PublicNetworkPassphrase, nil
	case chainsel.STELLAR_TESTNET.ChainID, chainsel.STELLAR_LOCALNET.ChainID:
		return network.TestNetworkPassphrase, nil
	default:
		return "", fmt.Errorf("unknown stellar chain ID %q", chainID)
	}
}
