package stellarutil

import (
	"bytes"
	"encoding/hex"
	"sort"
	"strconv"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/offchain"
	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
)

// ResolveSignersFromTopology extracts signer addresses and threshold for a
// given source chain selector from the environment topology.
//
// Committee verifier contracts store 20-byte ETH-style ECDSA addresses, left-padded to 32
// bytes on Soroban. Topology may record those under [chainsel.FamilyEVM] and/or
// [chainsel.FamilyStellar] (devenv enriches NOPs per impl factory family).
//
// preferredFamily is tried first for each NOP (e.g. [chainsel.FamilyStellar] when the source
// lane is Stellar), then [chainsel.FamilyEVM], then [chainsel.FamilyStellar], without duplicates.
func ResolveSignersFromTopology(topology *ccvdeployment.EnvironmentTopology, sourceChainSelector uint64, preferredFamily string) ([][32]byte, uint32) {
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
			addrHex := committeeVerifierSignerHex(nop.SignerAddressByFamily, preferredFamily)
			if addrHex == "" {
				continue
			}
			signer, ok := decodeCommitteeVerifierSigner(addrHex)
			if !ok {
				continue
			}
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

// ResolveSignersFromOffchainTopology is the same as [ResolveSignersFromTopology] but uses the chainlink-ccip
// offchain topology type carried on DeployChainContractsInput.
func ResolveSignersFromOffchainTopology(topology *offchain.EnvironmentTopology, sourceChainSelector uint64, preferredFamily string) ([][32]byte, uint32) {
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
			addrHex := committeeVerifierSignerHex(nop.SignerAddressByFamily, preferredFamily)
			if addrHex == "" {
				continue
			}
			signer, ok := decodeCommitteeVerifierSigner(addrHex)
			if !ok {
				continue
			}
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

// committeeVerifierSignerHex picks a 40-nibble hex string (20-byte ECDSA) from SignerAddressByFamily.
func committeeVerifierSignerHex(signerByFamily map[string]string, preferredFamily string) string {
	if len(signerByFamily) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	order := []string{preferredFamily, chainsel.FamilyEVM, chainsel.FamilyStellar}
	for _, fam := range order {
		if fam == "" {
			continue
		}
		if _, dup := seen[fam]; dup {
			continue
		}
		seen[fam] = struct{}{}
		if h := signerByFamily[fam]; h != "" {
			return h
		}
	}
	return ""
}

func decodeCommitteeVerifierSigner(addrHex string) ([32]byte, bool) {
	decoded, err := hex.DecodeString(addrHex)
	if err != nil || len(decoded) != 20 {
		return [32]byte{}, false
	}
	var signer [32]byte
	copy(signer[32-len(decoded):], decoded)
	return signer, true
}
