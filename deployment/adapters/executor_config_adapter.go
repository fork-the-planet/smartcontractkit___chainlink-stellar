package adapters

import (
	"fmt"

	dsutil "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
)

type StellarExecutorConfigAdapter struct{}

var _ ccvadapters.ExecutorConfigAdapter = (*StellarExecutorConfigAdapter)(nil)

func (a *StellarExecutorConfigAdapter) GetDeployedChains(ds datastore.DataStore, qualifier string) []uint64 {
	if ds == nil {
		return nil
	}
	refs := ds.Addresses().Filter(
		datastore.AddressRefByQualifier(qualifier),
		datastore.AddressRefByType(stellarccip.ExecutorProxyDatastoreRef(qualifier).Type),
	)
	seen := make(map[uint64]struct{}, len(refs))
	chains := make([]uint64, 0, len(refs))
	for _, ref := range refs {
		if _, exists := seen[ref.ChainSelector]; !exists {
			seen[ref.ChainSelector] = struct{}{}
			chains = append(chains, ref.ChainSelector)
		}
	}
	return chains
}

func (a *StellarExecutorConfigAdapter) BuildChainConfig(ds datastore.DataStore, chainSelector uint64, qualifier string) (ccvadapters.ExecutorChainConfig, error) {
	toAddress := func(ref datastore.AddressRef) (string, error) { return ref.Address, nil }

	offRampAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.OffRampDatastoreRef().PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return ccvadapters.ExecutorChainConfig{}, fmt.Errorf("off ramp address for chain %d: %w", chainSelector, err)
	}

	rmnRemoteAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.RMNRemoteDatastoreRef().PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return ccvadapters.ExecutorChainConfig{}, fmt.Errorf("rmn remote address for chain %d: %w", chainSelector, err)
	}

	executorAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.ExecutorProxyDatastoreRef(qualifier).PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return ccvadapters.ExecutorChainConfig{}, fmt.Errorf("executor proxy address for chain %d: %w", chainSelector, err)
	}

	return ccvadapters.ExecutorChainConfig{
		OffRampAddress:       offRampAddr,
		RmnAddress:           rmnRemoteAddr,
		ExecutorProxyAddress: executorAddr,
	}, nil
}
