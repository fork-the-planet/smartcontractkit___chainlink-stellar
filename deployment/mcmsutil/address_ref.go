package mcmsutil

import (
	"fmt"

	frameworkdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	"github.com/smartcontractkit/chainlink-ccip/deployment/utils"
	ccipdatastore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	mcmsutils "github.com/smartcontractkit/chainlink-ccip/deployment/utils/mcms"

	mcmsops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/mcms"
)

// MCMSRefLookupOrder returns datastore contract types to try when resolving a Stellar MCMS address.
func MCMSRefLookupOrder(action mcmstypes.TimelockAction) ([]cldf.ContractType, error) {
	var primary cldf.ContractType
	switch action {
	case mcmstypes.TimelockActionSchedule:
		primary = cldf.ContractType(utils.ProposerManyChainMultisig)
	case mcmstypes.TimelockActionBypass:
		primary = cldf.ContractType(utils.BypasserManyChainMultisig)
	case mcmstypes.TimelockActionCancel:
		primary = cldf.ContractType(utils.CancellerManyChainMultisig)
	default:
		return nil, fmt.Errorf("unsupported timelock action: %s", action)
	}
	return []cldf.ContractType{
		primary,
		cldf.ContractType(utils.ProposerManyChainMultisig),
		cldf.ContractType(utils.BypasserManyChainMultisig),
		cldf.ContractType(utils.CancellerManyChainMultisig),
		cldf.ContractType(mcmsops.ContractType),
	}, nil
}

// FindStellarMCMSAddressRef resolves the MCMS contract from the environment datastore.
func FindStellarMCMSAddressRef(e cldf.Environment, chainSelector uint64, input mcmsutils.Input) (frameworkdatastore.AddressRef, error) {
	order, err := MCMSRefLookupOrder(input.TimelockAction)
	if err != nil {
		return frameworkdatastore.AddressRef{}, err
	}
	refs := e.DataStore.Addresses().Filter()
	v := deploy.MCMSVersion
	for _, addrType := range order {
		ref := ccipdatastore.GetAddressRef(refs, chainSelector, addrType, v, input.Qualifier)
		if ref.Address != "" {
			return ref, nil
		}
	}
	return frameworkdatastore.AddressRef{}, fmt.Errorf("no Stellar MCMS address found for chain %d qualifier %q (expected EVM-style MCM alias types or %s)",
		chainSelector, input.Qualifier, mcmsops.ContractType)
}

// FindStellarTimelockAddressRef resolves RBACTimelock from the datastore, or falls back to the
// MCMS contract when no timelock row exists (pre-timelock Stellar deployments).
func FindStellarTimelockAddressRef(e cldf.Environment, chainSelector uint64, input mcmsutils.Input) (frameworkdatastore.AddressRef, error) {
	refs := e.DataStore.Addresses().Filter()
	v := deploy.MCMSVersion
	ref := ccipdatastore.GetAddressRef(refs, chainSelector, utils.RBACTimelock, v, input.Qualifier)
	if ref.Address != "" {
		return ref, nil
	}
	return FindStellarMCMSAddressRef(e, chainSelector, input)
}
