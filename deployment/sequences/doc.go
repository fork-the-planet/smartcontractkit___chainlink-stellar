// Package sequences defines CLDF deployment sequences for Stellar CCIP.
//
// Outer sequences use cldf_chain.BlockChains as the dependency type to match
// chainlink-ccip deployment/v2_0_0 adapter contracts. Inner sequences use
// deployment/operations/stellardeps.StellarDeps, mirroring how EVM wraps
// BlockChains in adapters then runs inner sequences with evm.Chain.
// DeployStellarCCIPInnerInput carries ExistingAddresses from the CCIP changeset
// input so deploy can seed the in-memory datastore like EVM ExistingAddresses.
// AllSelectors lists every chain selector in the environment (outer sequence derives it from BlockChains).
// The inner sequence passes the same CLDF bundle as the parent ExecuteSequence into [github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellardeploy].
package sequences
