package adapters

import (
	"fmt"

	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/committee_verifier"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/versioned_verifier_resolver"
	dsutil "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
)

type StellarVerifierConfigAdapter struct{}

var _ ccvadapters.VerifierConfigAdapter = (*StellarVerifierConfigAdapter)(nil)

func (a *StellarVerifierConfigAdapter) ResolveVerifierContractAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	committeeQualifier string,
	executorQualifier string,
) (*ccvadapters.VerifierContractAddresses, error) {
	toAddress := func(ref datastore.AddressRef) (string, error) { return ref.Address, nil }

	committeeVerifierAddr, err := dsutil.FindAndFormatFirstRef(ds, chainSelector, toAddress,
		datastore.AddressRef{
			Type:      datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType),
			Qualifier: committeeQualifier,
		},
		datastore.AddressRef{
			Type:      datastore.ContractType(committee_verifier.ContractType),
			Qualifier: committeeQualifier,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("committee verifier address for chain %d: %w", chainSelector, err)
	}

	onRampAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.OnRampDatastoreRef().PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return nil, fmt.Errorf("on ramp address for chain %d: %w", chainSelector, err)
	}

	executorAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.ExecutorProxyDatastoreRef(executorQualifier).PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return nil, fmt.Errorf("executor proxy address for chain %d: %w", chainSelector, err)
	}

	rmnRemoteAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.RMNRemoteDatastoreRef().PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return nil, fmt.Errorf("rmn remote address for chain %d: %w", chainSelector, err)
	}

	return &ccvadapters.VerifierContractAddresses{
		CommitteeVerifierAddress: committeeVerifierAddr,
		OnRampAddress:            onRampAddr,
		ExecutorProxyAddress:     executorAddr,
		RMNRemoteAddress:         rmnRemoteAddr,
	}, nil
}
