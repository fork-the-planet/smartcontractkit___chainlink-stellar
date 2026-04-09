package stellarutil

import (
	"bytes"
	"encoding/hex"
	"sort"
	"strconv"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccipOffchain "github.com/smartcontractkit/chainlink-ccip/deployment/v1_7_0/offchain"
)

// ResolveSignersFromTopology extracts signer addresses and threshold for a
// given source chain selector from the environment topology. All committee
// verifier DONs sign with ECDSA (secp256k1), so we always look up the EVM
// family signer address (20-byte Ethereum address) and left-pad it to 32
// bytes to match the on-chain Soroban storage format.
func ResolveSignersFromTopology(topology *ccipOffchain.EnvironmentTopology, sourceChainSelector uint64, _ string) ([][32]byte, uint32) {
	if topology == nil || topology.NOPTopology == nil {
		return nil, 0
	}

	selectorStr := strconv.FormatUint(sourceChainSelector, 10)

	for _, committee := range topology.NOPTopology.Committees {
		chainCfg, ok := committee.ChainConfigs[selectorStr]
		if !ok {
			continue
		}

		var signers [][32]byte
		for _, alias := range chainCfg.NOPAliases {
			nop, found := topology.NOPTopology.GetNOP(alias)
			if !found {
				continue
			}
			addrHex := nop.SignerAddressByFamily[chainsel.FamilyEVM]
			if addrHex == "" {
				continue
			}
			decoded, decErr := hex.DecodeString(addrHex)
			if decErr != nil {
				continue
			}
			if len(decoded) != 20 {
				continue
			}
			var signer [32]byte
			copy(signer[32-len(decoded):], decoded)
			signers = append(signers, signer)
		}

		if len(signers) > 0 {
			sort.Slice(signers, func(i, j int) bool {
				return bytes.Compare(signers[i][:], signers[j][:]) < 0
			})
			return signers, uint32(chainCfg.Threshold)
		}
	}

	return nil, 0
}
