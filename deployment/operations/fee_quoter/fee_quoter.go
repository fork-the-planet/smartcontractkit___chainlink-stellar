package fee_quoter

import (
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	fqbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// ContractType labels FeeQuoter.
const ContractType = "FeeQuoter"

// Deploy uploads fee_quoter.wasm.
var Deploy = stellarops.NewDeployOperation("fee-quoter:deploy", "Deploys the FeeQuoter Soroban contract from WASM")

// InitializeInput configures FeeQuoter static config and price updaters.
type InitializeInput struct {
	ContractID        string                  `json:"contract_id"`
	Owner             string                  `json:"owner"`
	StaticConfig      fqbindings.StaticConfig `json:"static_config"`
	AuthorizedCallers []string                `json:"authorized_callers"`
}

// Initialize calls FeeQuoter `initialize`.
var Initialize = cldfops.NewOperation(
	"fee-quoter:initialize",
	stellarops.ContractDeploymentVersion,
	"Initializes FeeQuoter with owner, static config, and authorized callers",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in InitializeInput) (stellarops.Void, error) {
		c := fqbindings.NewFeeQuoterClient(d.Invoker, in.ContractID)
		if err := c.Initialize(b.GetContext(), in.Owner, in.StaticConfig, in.AuthorizedCallers); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)

// ApplyDestChainConfigsInput applies destination gas/fee defaults on FeeQuoter.
type ApplyDestChainConfigsInput struct {
	ContractID string                           `json:"contract_id"`
	Configs    []fqbindings.DestChainConfigArgs `json:"configs"`
}

// ApplyDestChainConfigs calls FeeQuoter `apply_dest_chain_configs`.
var ApplyDestChainConfigs = cldfops.NewOperation(
	"fee-quoter:apply-dest-chain-configs",
	stellarops.ContractDeploymentVersion,
	"Applies FeeQuoter destination chain configurations",
	func(b cldfops.Bundle, d stellardeps.StellarDeps, in ApplyDestChainConfigsInput) (stellarops.Void, error) {
		c := fqbindings.NewFeeQuoterClient(d.Invoker, in.ContractID)
		if err := c.ApplyDestChainConfigs(b.GetContext(), in.Configs); err != nil {
			return stellarops.Void{}, err
		}
		return stellarops.Void{}, nil
	},
)
