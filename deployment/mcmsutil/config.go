package mcmsutil

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	mcmsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/mcms"
)

// MergeTripleMCMSConfig requires Proposer, Bypasser, and Canceller configs to be equal
// (single Soroban MCMS instance). Returns a copy suitable for ExtractSetConfigInputs.
func MergeTripleMCMSConfig(p, b, c mcmstypes.Config) (*mcmstypes.Config, error) {
	if !p.Equals(&b) || !b.Equals(&c) {
		return nil, fmt.Errorf("stellar MCMS: Proposer, Bypasser, and Canceller configs must be identical for a single MCMS instance")
	}
	return &p, nil
}

// ConfigToStellarSetConfig maps mcms types.Config to Soroban set_config inputs using
// the shared ExtractSetConfigInputs helper from github.com/smartcontractkit/mcms/sdk.
func ConfigToStellarSetConfig(cfg *mcmstypes.Config, clearRoot bool) (mcmsbindings.SignerAddresses, mcmsbindings.SignerGroups, [32]byte, [32]byte, bool, error) {
	if cfg == nil {
		return mcmsbindings.SignerAddresses{}, mcmsbindings.SignerGroups{}, [32]byte{}, [32]byte{}, clearRoot, fmt.Errorf("nil MCMS config")
	}
	gq, gp, addrs, groups, err := mcmssdk.ExtractSetConfigInputs(cfg)
	if err != nil {
		return mcmsbindings.SignerAddresses{}, mcmsbindings.SignerGroups{}, [32]byte{}, [32]byte{}, clearRoot, err
	}
	inner := make([][32]byte, len(addrs))
	for i, a := range addrs {
		inner[i] = evmAddressToPadded32(a)
	}
	groupsInner := make([]uint32, len(groups))
	for i, g := range groups {
		groupsInner[i] = uint32(g)
	}
	return mcmsbindings.SignerAddresses{Inner: inner},
		mcmsbindings.SignerGroups{Inner: groupsInner},
		[32]byte(gq),
		[32]byte(gp),
		clearRoot,
		nil
}

func evmAddressToPadded32(a common.Address) [32]byte {
	var out [32]byte
	copy(out[12:], a.Bytes())
	return out
}
