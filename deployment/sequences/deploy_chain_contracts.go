package sequences

// DeployChainContractsInnerInput is an alias for [DeployStellarCCIPInnerInput], matching EVM/Solana naming.
type DeployChainContractsInnerInput = DeployStellarCCIPInnerInput

// DeployChainContractsInner is the same sequence as [StellarDeployChainContracts] (single BlockChains-based deploy path).
var DeployChainContractsInner = StellarDeployChainContracts
