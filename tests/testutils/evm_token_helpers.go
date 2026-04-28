package helpers

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

// evmTokenContractTypes lists EVM BurnMint drip token types stored in the devenv
// datastore. Order reflects deployment likelihood.
//
// The devenv deploys one BurnMint ERC-20 per token combination; in practice the
// first match on a given chain is the correct test token for cross-family tests.
var evmTokenContractTypes = []string{
	"BurnMintERC20WithDripToken",
	"BurnMintERC20WithDrip",
	"BurnMintERC20Token",
}

// ResolveEVMTestToken finds the first BurnMint test token deployed on the given
// EVM chain from the merged datastore. It returns the raw token address as a
// protocol.UnknownAddress suitable for SendMessage.TokenAmount.TokenAddress and
// GetTokenBalance.
func ResolveEVMTestToken(ds datastore.DataStore, evmChainSelector uint64) (protocol.UnknownAddress, error) {
	allRefs, err := ds.Addresses().Fetch()
	if err != nil {
		return nil, fmt.Errorf("fetch datastore addresses: %w", err)
	}
	for _, ct := range evmTokenContractTypes {
		for _, ref := range allRefs {
			if ref.ChainSelector == evmChainSelector && string(ref.Type) == ct {
				addr, decErr := hexToEVMAddress(ref.Address)
				if decErr != nil {
					return nil, fmt.Errorf("decode address for %s on chain %d: %w", ct, evmChainSelector, decErr)
				}
				return addr, nil
			}
		}
	}
	return nil, fmt.Errorf("no EVM test token found for chain selector %d", evmChainSelector)
}

func hexToEVMAddress(hexAddr string) (protocol.UnknownAddress, error) {
	raw := strings.TrimPrefix(strings.TrimPrefix(hexAddr, "0x"), "0X")
	b, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid hex address %s: %w", hexAddr, err)
	}
	if len(b) != 20 {
		return nil, fmt.Errorf("expected 20 bytes for EVM address, got %d: %s", len(b), hexAddr)
	}
	return protocol.UnknownAddress(b), nil
}
