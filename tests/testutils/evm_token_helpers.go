package helpers

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	ccvchain "github.com/smartcontractkit/chainlink-stellar/ccv/chain"
)

// evmTokenContractTypes lists EVM BurnMint drip token types stored in the devenv
// datastore. Order reflects deployment likelihood and is only used as a
// fallback when the explicit EVM-to-Stellar token pair is not present.
var evmTokenContractTypes = []string{
	"BurnMintERC20WithDripToken",
	"BurnMintERC20WithDrip",
	"BurnMintERC20Token",
}

// ResolveEVMTestToken finds the BurnMint test token configured for
// EVM-to-Stellar transfers on the given EVM chain. It returns the raw token
// address as a protocol.UnknownAddress suitable for
// SendMessage.TokenAmount.TokenAddress and GetTokenBalance.
func ResolveEVMTestToken(ds datastore.DataStore, evmChainSelector uint64) (protocol.UnknownAddress, error) {
	allRefs, err := ds.Addresses().Fetch()
	if err != nil {
		return nil, fmt.Errorf("fetch datastore addresses: %w", err)
	}
	_, tokenRef, found := ccvchain.ResolveEVMTokenPoolForStellar(allRefs, evmChainSelector)
	if found {
		addr, decErr := hexToEVMAddress(tokenRef.Address)
		if decErr != nil {
			return nil, fmt.Errorf("decode EVM-to-Stellar token address on chain %d: %w", evmChainSelector, decErr)
		}
		return addr, nil
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
