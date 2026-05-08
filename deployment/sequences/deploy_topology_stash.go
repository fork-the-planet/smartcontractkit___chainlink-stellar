package sequences

import (
	"sync"

	"github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/offchain"
)

// stellarDeployOffchainTopologyBySelector bridges chainlink-ccv's PreDeployContractsForSelector
// (which receives *ccvdeployment.EnvironmentTopology) to StellarDeployChainContracts (which
// only receives chainlink-ccip DeployChainContractsInput, with no topology field on the current pin).
// CCV calls Register in pre-deploy, the CLDF adapter sequence calls Take before RunStellarCCIPFullDeploy,
// and PostDeploy clears any leftover entry after the changeset returns.
// The stashed topology must be non-nil with NOP data: RunStellarCCIPFullDeploy applies committee
// verifier signature quorum config from it; a missing stash causes the adapter sequence to error.
var stellarDeployOffchainTopologyBySelector sync.Map // uint64 -> *offchain.EnvironmentTopology

// RegisterStellarDeployOffchainTopologyForSelector records offchain topology for the Stellar chain
// selector. It is overwritten if already present. nil topo is a no-op.
func RegisterStellarDeployOffchainTopologyForSelector(selector uint64, topo *offchain.EnvironmentTopology) {
	if topo == nil {
		return
	}
	stellarDeployOffchainTopologyBySelector.Store(selector, topo)
}

// TakeStellarDeployOffchainTopologyForSelector loads and removes the topology for selector.
// The second return is false if nothing was registered.
func TakeStellarDeployOffchainTopologyForSelector(selector uint64) (*offchain.EnvironmentTopology, bool) {
	v, ok := stellarDeployOffchainTopologyBySelector.LoadAndDelete(selector)
	if !ok {
		return nil, false
	}
	t, ok := v.(*offchain.EnvironmentTopology)
	if !ok || t == nil {
		return nil, false
	}
	return t, true
}

// ClearStellarDeployOffchainTopologyForSelector deletes a stashed topology without returning it.
// Call from post-deploy after Take may have already run (idempotent cleanup).
func ClearStellarDeployOffchainTopologyForSelector(selector uint64) {
	stellarDeployOffchainTopologyBySelector.Delete(selector)
}
