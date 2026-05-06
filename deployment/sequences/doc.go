// Package sequences defines CLDF deployment sequences for Stellar CCIP.
//
// Deploy chain contracts uses cldf_chain.BlockChains as the dependency type to match
// chainlink-ccip deployment/v2_0_0 adapter contracts. The sequence builds
// deployment/operations/stellardeps.StellarDeps and a CCIP devenv host from the CLDF Stellar chain entry,
// then runs [RunStellarCCIPFullDeploy] with nil topology on this adapter path (signer resolution falls back per full-deploy logic); [github.com/smartcontractkit/chainlink-stellar/deployment/sequences.RunStellarCCIPFullDeployForCCV] passes real topology from ccv. The sequence return type matches chainlink-ccip/deployment/v2_0_0/adapters.DeployChainContractsAdapter for the module version in go.mod.
// DeployStellarCCIPInnerInput carries ExistingAddresses from the CCIP changeset
// input so deploy can seed the in-memory datastore like EVM ExistingAddresses.
// AllSelectors lists every chain selector in the environment (derived from BlockChains in the adapter sequence).
package sequences
