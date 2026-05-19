package sequences

import (
	"fmt"

	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	api "github.com/smartcontractkit/chainlink-ccip/deployment/fastcurse"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	rmnremoteops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/rmn_remote"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
)

// StellarCurseInput extends the shared CurseInput with the resolved RMN Remote contract ID.
type StellarCurseInput struct {
	api.CurseInput
	RMNContractID string
}

// StellarCurse curses subjects on a Stellar RMN Remote contract.
var StellarCurse = cldfops.NewSequence(
	"stellar-curse-rmn-remote",
	stellarops.ContractDeploymentVersion,
	"Curse subjects on Stellar RMN Remote",
	func(b cldfops.Bundle, chains cldfchain.BlockChains, in StellarCurseInput) (seqcore.OnChainOutput, error) {
		ch, ok := chains.StellarChains()[in.ChainSelector]
		if !ok {
			return seqcore.OnChainOutput{}, fmt.Errorf("stellar chain %d not found", in.ChainSelector)
		}
		dep, err := stellardeployment.NewDeployerFromChain(ch)
		if err != nil {
			return seqcore.OnChainOutput{}, err
		}
		deps := stellardeps.FromDeployer(dep)
		_, err = cldfops.ExecuteOperation(b, rmnremoteops.Curse, deps, rmnremoteops.CurseInput{
			ContractID: in.RMNContractID,
			Caller:     dep.SignerAddress(),
			Subjects:   in.Subjects,
		})
		if err != nil {
			return seqcore.OnChainOutput{}, fmt.Errorf("curse on chain %d: %w", in.ChainSelector, err)
		}
		return seqcore.OnChainOutput{}, nil
	},
)

// StellarUncurse uncurses subjects on a Stellar RMN Remote contract.
var StellarUncurse = cldfops.NewSequence(
	"stellar-uncurse-rmn-remote",
	stellarops.ContractDeploymentVersion,
	"Uncurse subjects on Stellar RMN Remote",
	func(b cldfops.Bundle, chains cldfchain.BlockChains, in StellarCurseInput) (seqcore.OnChainOutput, error) {
		ch, ok := chains.StellarChains()[in.ChainSelector]
		if !ok {
			return seqcore.OnChainOutput{}, fmt.Errorf("stellar chain %d not found", in.ChainSelector)
		}
		dep, err := stellardeployment.NewDeployerFromChain(ch)
		if err != nil {
			return seqcore.OnChainOutput{}, err
		}
		deps := stellardeps.FromDeployer(dep)
		_, err = cldfops.ExecuteOperation(b, rmnremoteops.Uncurse, deps, rmnremoteops.UncurseInput{
			ContractID: in.RMNContractID,
			Subjects:   in.Subjects,
		})
		if err != nil {
			return seqcore.OnChainOutput{}, fmt.Errorf("uncurse on chain %d: %w", in.ChainSelector, err)
		}
		return seqcore.OnChainOutput{}, nil
	},
)
