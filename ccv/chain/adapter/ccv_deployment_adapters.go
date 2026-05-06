package adapter

import (
	"fmt"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/committee_verifier"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/proxy"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/versioned_verifier_resolver"
	dsutils "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	ccipdevenvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccvdeploymentadapters "github.com/smartcontractkit/chainlink-ccv/deployment/adapters"
	"github.com/smartcontractkit/chainlink-ccv/executor"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

// StellarCCVDeploymentAggregatorConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.AggregatorConfigAdapter
// for the devenv (datastore-only verifier address resolution). On-chain committee
// state for GenerateAggregatorConfig lives on StellarCCVCommitteeVerifierOnchainAdapter.
type StellarCCVDeploymentAggregatorConfigAdapter struct{}

var _ ccvdeploymentadapters.AggregatorConfigAdapter = (*StellarCCVDeploymentAggregatorConfigAdapter)(nil)

func (a *StellarCCVDeploymentAggregatorConfigAdapter) ResolveVerifierAddress(
	ds datastore.DataStore,
	chainSelector uint64,
	qualifier string,
) (string, error) {
	return dsutils.FindAndFormatFirstRef(ds, chainSelector,
		func(r datastore.AddressRef) (string, error) { return r.Address, nil },
		datastore.AddressRef{
			Type:      datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType),
			Qualifier: qualifier,
		},
		datastore.AddressRef{
			Type:      datastore.ContractType(committee_verifier.ContractType),
			Qualifier: qualifier,
		},
	)
}

// StellarCCVDeploymentExecutorConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.ExecutorConfigAdapter.
type StellarCCVDeploymentExecutorConfigAdapter struct{}

var _ ccvdeploymentadapters.ExecutorConfigAdapter = (*StellarCCVDeploymentExecutorConfigAdapter)(nil)

func (a *StellarCCVDeploymentExecutorConfigAdapter) GetDeployedChains(ds datastore.DataStore, qualifier string) []uint64 {
	if ds == nil {
		return nil
	}
	refs := ds.Addresses().Filter(
		datastore.AddressRefByQualifier(qualifier),
		datastore.AddressRefByType(datastore.ContractType(proxy.ContractType)),
	)
	seen := make(map[uint64]struct{}, len(refs))
	chains := make([]uint64, 0, len(refs))
	for _, ref := range refs {
		if _, exists := seen[ref.ChainSelector]; exists {
			continue
		}
		family, err := chainsel.GetSelectorFamily(ref.ChainSelector)
		if err != nil || family != chainsel.FamilyStellar {
			continue
		}
		seen[ref.ChainSelector] = struct{}{}
		chains = append(chains, ref.ChainSelector)
	}
	return chains
}

func (a *StellarCCVDeploymentExecutorConfigAdapter) BuildChainConfig(
	ds datastore.DataStore,
	chainSelector uint64,
	qualifier string,
) (executor.ChainConfiguration, error) {
	cfg, err := (&StellarExecutorConfigAdapter{}).BuildChainConfig(ds, chainSelector, qualifier)
	if err != nil {
		return executor.ChainConfiguration{}, err
	}
	return executor.ChainConfiguration{
		DestinationChainConfig: chainaccess.DestinationChainConfig{
			OffRampAddress: cfg.OffRampAddress,
			RmnAddress:     cfg.RmnAddress,
		},
		DefaultExecutorAddress: cfg.ExecutorProxyAddress,
	}, nil
}

// StellarCCVDeploymentVerifierConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.VerifierConfigAdapter.
type StellarCCVDeploymentVerifierConfigAdapter struct{}

var _ ccvdeploymentadapters.VerifierConfigAdapter = (*StellarCCVDeploymentVerifierConfigAdapter)(nil)

func (a *StellarCCVDeploymentVerifierConfigAdapter) GetSignerAddressFamily() string {
	return chainsel.FamilyStellar
}

func (a *StellarCCVDeploymentVerifierConfigAdapter) ResolveVerifierContractAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	committeeQualifier string,
	executorQualifier string,
) (*ccvdeploymentadapters.VerifierContractAddresses, error) {
	addrs, err := (&StellarVerifierConfigAdapter{}).ResolveVerifierContractAddresses(
		ds, chainSelector, committeeQualifier, executorQualifier)
	if err != nil {
		return nil, err
	}
	return &ccvdeploymentadapters.VerifierContractAddresses{
		CommitteeVerifierAddress: addrs.CommitteeVerifierAddress,
		OnRampAddress:            addrs.OnRampAddress,
		ExecutorProxyAddress:     addrs.ExecutorProxyAddress,
		RMNRemoteAddress:         addrs.RMNRemoteAddress,
	}, nil
}

// StellarCCVDeploymentIndexerConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.IndexerConfigAdapter.
type StellarCCVDeploymentIndexerConfigAdapter struct{}

var _ ccvdeploymentadapters.IndexerConfigAdapter = (*StellarCCVDeploymentIndexerConfigAdapter)(nil)

func (a *StellarCCVDeploymentIndexerConfigAdapter) ResolveVerifierAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	qualifier string,
	kind ccvdeploymentadapters.VerifierKind,
) ([]string, error) {
	switch kind {
	case ccvdeploymentadapters.CommitteeVerifierKind:
		return (&StellarIndexerConfigAdapter{}).ResolveVerifierAddresses(
			ds, chainSelector, qualifier, ccipdevenvadapters.VerifierKind(kind))
	default:
		return nil, fmt.Errorf("Stellar does not support verifier kind %q", kind)
	}
}

// StellarCCVDeploymentTokenVerifierConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.TokenVerifierConfigAdapter.
type StellarCCVDeploymentTokenVerifierConfigAdapter struct{}

var _ ccvdeploymentadapters.TokenVerifierConfigAdapter = (*StellarCCVDeploymentTokenVerifierConfigAdapter)(nil)

func (a *StellarCCVDeploymentTokenVerifierConfigAdapter) ResolveTokenVerifierAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	cctpQualifier string,
	lombardQualifier string,
) (*ccvdeploymentadapters.TokenVerifierChainAddresses, error) {
	addrs, err := (&StellarTokenVerifierConfigAdapter{}).ResolveTokenVerifierAddresses(
		ds, chainSelector, cctpQualifier, lombardQualifier)
	if err != nil {
		return nil, err
	}
	return &ccvdeploymentadapters.TokenVerifierChainAddresses{
		OnRampAddress:                  addrs.OnRampAddress,
		RMNRemoteAddress:               addrs.RMNRemoteAddress,
		CCTPVerifierAddress:            addrs.CCTPVerifierAddress,
		CCTPVerifierResolverAddress:    addrs.CCTPVerifierResolverAddress,
		LombardVerifierResolverAddress: addrs.LombardVerifierResolverAddress,
	}, nil
}
