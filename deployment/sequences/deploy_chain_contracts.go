package sequences

// DeployChainContractsInnerInput is an alias for [DeployStellarCCIPInnerInput], matching EVM/Solana naming.
type DeployChainContractsInnerInput = DeployStellarCCIPInnerInput

// DeployChainContractsInner deploys the full Stellar CCIP Soroban stack for one chain.
// It is the same sequence as [DeployStellarCCIPInner] (same ID and handler); use this name when aligning with
// EVM `deploy-chain-contracts` / Solana `DeployChainContracts` conventions.
var DeployChainContractsInner = DeployStellarCCIPInner
