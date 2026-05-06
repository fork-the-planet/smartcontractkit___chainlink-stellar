package adapters

import (
	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	seqcore "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"

	"github.com/smartcontractkit/chainlink-stellar/deployment/sequences"
)

// StellarMCMSDeployer implements deploy.Deployer MCMS-related methods for Soroban.
// DeployChainContracts and SetOCR3Config are nil (CCIP 2.0 uses DeployChainContractsAdapter).
type StellarMCMSDeployer struct{}

var _ deploy.Deployer = (*StellarMCMSDeployer)(nil)

func (StellarMCMSDeployer) DeployChainContracts() *cldfops.Sequence[deploy.ContractDeploymentConfigPerChainWithAddress, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return nil
}

func (StellarMCMSDeployer) SetOCR3Config() *cldfops.Sequence[deploy.SetOCR3ConfigInput, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return nil
}

func (StellarMCMSDeployer) DeployMCMS() *cldfops.Sequence[deploy.MCMSDeploymentConfigPerChainWithAddress, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return sequences.DeployStellarMCMS
}

func (StellarMCMSDeployer) FinalizeDeployMCMS() *cldfops.Sequence[deploy.MCMSDeploymentConfigPerChainWithAddress, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return sequences.FinalizeStellarDeployMCMS
}

func (StellarMCMSDeployer) GrantAdminRoleToTimelock() *cldfops.Sequence[deploy.GrantAdminRoleToTimelockConfigPerChainWithSelector, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return sequences.GrantAdminRoleToTimelockStellar
}

func (StellarMCMSDeployer) UpdateMCMSConfig() *cldfops.Sequence[deploy.UpdateMCMSConfigInputPerChainWithSelector, seqcore.OnChainOutput, cldfchain.BlockChains] {
	return sequences.UpdateStellarMCMSConfig
}
