package adapters

import (
	"fmt"

	dsutil "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
)

type StellarTokenVerifierConfigAdapter struct{}

var _ ccvadapters.TokenVerifierConfigAdapter = (*StellarTokenVerifierConfigAdapter)(nil)

func (a *StellarTokenVerifierConfigAdapter) ResolveTokenVerifierAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	_ string,
	_ string,
) (*ccvadapters.TokenVerifierChainAddresses, error) {
	toAddress := func(ref datastore.AddressRef) (string, error) { return ref.Address, nil }

	onRampAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.OnRampDatastoreRef().PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return nil, fmt.Errorf("on ramp address for chain %d: %w", chainSelector, err)
	}

	rmnRemoteAddr, err := dsutil.FindAndFormatRef(ds, stellarccip.RMNRemoteDatastoreRef().PartialAddressRef(), chainSelector, toAddress)
	if err != nil {
		return nil, fmt.Errorf("rmn remote address for chain %d: %w", chainSelector, err)
	}

	return &ccvadapters.TokenVerifierChainAddresses{
		OnRampAddress:    onRampAddr,
		RMNRemoteAddress: rmnRemoteAddr,
	}, nil
}
