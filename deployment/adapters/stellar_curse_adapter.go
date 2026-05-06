package adapters

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"

	chainsel "github.com/smartcontractkit/chain-selectors"
	routeroperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v1_2_0/operations/router"
	api "github.com/smartcontractkit/chainlink-ccip/deployment/fastcurse"
	datastore_utils "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	rmnremotebindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	rmnremoteops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/rmn_remote"
	stellarsequences "github.com/smartcontractkit/chainlink-stellar/deployment/sequences"
)

var (
	_ api.CurseAdapter        = (*StellarCurseAdapter)(nil)
	_ api.CurseSubjectAdapter = (*StellarCurseAdapter)(nil)
)

// StellarCurseAdapter implements both CurseAdapter and CurseSubjectAdapter for Stellar.
type StellarCurseAdapter struct {
	rmnContractID    map[uint64]string
	routerContractID map[uint64]string
}

func NewStellarCurseAdapter() *StellarCurseAdapter {
	return &StellarCurseAdapter{}
}

func (a *StellarCurseAdapter) Initialize(e cldf.Environment, selector uint64) error {
	if _, ok := e.BlockChains.StellarChains()[selector]; !ok {
		return fmt.Errorf("stellar chain %d not found in environment", selector)
	}

	if a.rmnContractID == nil {
		a.rmnContractID = make(map[uint64]string)
	}
	if a.routerContractID == nil {
		a.routerContractID = make(map[uint64]string)
	}

	if _, exists := a.rmnContractID[selector]; !exists {
		addr, err := stellarContractIDOnChain(e, selector,
			datastore.ContractType(rmnremoteops.ContractType),
			stellarops.ContractDeploymentVersion)
		if err != nil {
			return fmt.Errorf("resolve RMN Remote on chain %d: %w", selector, err)
		}
		a.rmnContractID[selector] = addr
	}

	if _, exists := a.routerContractID[selector]; !exists {
		addr, err := stellarContractIDOnChain(e, selector,
			datastore.ContractType(routeroperations.ContractType),
			routeroperations.Version)
		if err != nil {
			return fmt.Errorf("resolve Router on chain %d: %w", selector, err)
		}
		a.routerContractID[selector] = addr
	}
	return nil
}

func (a *StellarCurseAdapter) IsSubjectCursedOnChain(e cldf.Environment, selector uint64, subject api.Subject) (bool, error) {
	rmnID, ok := a.rmnContractID[selector]
	if !ok {
		return false, fmt.Errorf("no RMN Remote cached for chain %d", selector)
	}
	ch, ok := e.BlockChains.StellarChains()[selector]
	if !ok {
		return false, fmt.Errorf("stellar chain %d not in environment", selector)
	}
	dep, err := stellardeployment.NewDeployerFromChain(ch)
	if err != nil {
		return false, err
	}
	client := rmnremotebindings.NewRmnRemoteClient(dep, rmnID)
	return client.IsCursedBySubject(e.GetContext(), subject)
}

func (a *StellarCurseAdapter) IsChainConnectedToTargetChain(e cldf.Environment, selector uint64, targetSel uint64) (bool, error) {
	routerID, ok := a.routerContractID[selector]
	if !ok {
		return false, fmt.Errorf("no Router cached for chain %d", selector)
	}
	ch, ok := e.BlockChains.StellarChains()[selector]
	if !ok {
		return false, fmt.Errorf("stellar chain %d not in environment", selector)
	}
	dep, err := stellardeployment.NewDeployerFromChain(ch)
	if err != nil {
		return false, err
	}
	client := routerbindings.NewRouterClient(dep, routerID)
	return client.IsChainSupported(e.GetContext(), targetSel)
}

func (a *StellarCurseAdapter) IsCurseEnabledForChain(_ cldf.Environment, selector uint64) (bool, error) {
	_, ok := a.rmnContractID[selector]
	return ok, nil
}

func (a *StellarCurseAdapter) SubjectToSelector(subject api.Subject) (uint64, error) {
	return api.GenericSubjectToSelector(subject)
}

func (a *StellarCurseAdapter) SelectorToSubject(selector uint64) api.Subject {
	return api.GenericSelectorToSubject(selector)
}

func (a *StellarCurseAdapter) DeriveCurseAdapterVersion(_ cldf.Environment, _ uint64) (*semver.Version, error) {
	return stellarops.ContractDeploymentVersion, nil
}

func (a *StellarCurseAdapter) Curse() *cldf_ops.Sequence[api.CurseInput, seqcore.OnChainOutput, cldf_chain.BlockChains] {
	return wrapCurseSequence(a, stellarsequences.StellarCurse)
}

func (a *StellarCurseAdapter) Uncurse() *cldf_ops.Sequence[api.CurseInput, seqcore.OnChainOutput, cldf_chain.BlockChains] {
	return wrapCurseSequence(a, stellarsequences.StellarUncurse)
}

// wrapCurseSequence adapts a StellarCurseInput sequence to the api.CurseInput interface
// by injecting the cached RMN contract ID.
func wrapCurseSequence(
	a *StellarCurseAdapter,
	inner *cldf_ops.Sequence[stellarsequences.StellarCurseInput, seqcore.OnChainOutput, cldf_chain.BlockChains],
) *cldf_ops.Sequence[api.CurseInput, seqcore.OnChainOutput, cldf_chain.BlockChains] {
	return cldf_ops.NewSequence(
		inner.ID(),
		stellarops.ContractDeploymentVersion,
		inner.Description(),
		func(b cldf_ops.Bundle, chains cldf_chain.BlockChains, in api.CurseInput) (seqcore.OnChainOutput, error) {
			rmnID, ok := a.rmnContractID[in.ChainSelector]
			if !ok {
				return seqcore.OnChainOutput{}, fmt.Errorf("no RMN Remote cached for chain %d", in.ChainSelector)
			}
			report, err := cldf_ops.ExecuteSequence(b, inner, chains, stellarsequences.StellarCurseInput{
				CurseInput:    in,
				RMNContractID: rmnID,
			})
			if err != nil {
				return seqcore.OnChainOutput{}, err
			}
			return report.Output, nil
		},
	)
}

func (a *StellarCurseAdapter) ListConnectedChains(e cldf.Environment, selector uint64) ([]uint64, error) {
	routerID, ok := a.routerContractID[selector]
	if !ok {
		return nil, fmt.Errorf("no Router cached for chain %d", selector)
	}
	ch, ok := e.BlockChains.StellarChains()[selector]
	if !ok {
		return nil, fmt.Errorf("stellar chain %d not in environment", selector)
	}
	dep, err := stellardeployment.NewDeployerFromChain(ch)
	if err != nil {
		return nil, err
	}
	client := routerbindings.NewRouterClient(dep, routerID)
	offRamps, err := client.GetOfframps(e.GetContext())
	if err != nil {
		return nil, fmt.Errorf("get offramps on chain %d: %w", selector, err)
	}
	var connected []uint64
	for _, entry := range offRamps {
		if entry.Offramp == "" {
			continue
		}
		family, err := chainsel.GetSelectorFamily(entry.SourceChainSelector)
		if err != nil {
			continue
		}
		if !api.GetCurseRegistry().IsFamilyRegistered(family) {
			continue
		}
		if !slices.Contains(connected, entry.SourceChainSelector) {
			connected = append(connected, entry.SourceChainSelector)
		}
	}
	return connected, nil
}

func stellarContractIDOnChain(e cldf.Environment, selector uint64, ct datastore.ContractType, version *semver.Version) (string, error) {
	toAddress := func(ref datastore.AddressRef) (string, error) { return ref.Address, nil }
	return datastore_utils.FindAndFormatRef(e.DataStore, datastore.AddressRef{
		Type:    ct,
		Version: version,
	}, selector, toAddress)
}
