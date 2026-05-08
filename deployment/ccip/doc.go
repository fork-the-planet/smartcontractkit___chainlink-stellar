// Package ccip holds Stellar CCIP deployment helpers: datastore contract types, lane/onramp config,
// topology conversion, the [CCIPDevenvHost] surface for Soroban devenv, post-deploy token pool
// ([DeployLockReleaseTestTokenPool]), CLDF-backed devenv host ([NewCLDFStellarCCIPDevenvHost]), and stellarutil/.
// Full-stack deploy entrypoints that merge CCV topology and run [github.com/smartcontractkit/chainlink-stellar/deployment/sequences.RunStellarCCIPFullDeploy]
// live in package sequences ([github.com/smartcontractkit/chainlink-stellar/deployment/sequences.RunStellarCCIPFullDeployForCCV], [github.com/smartcontractkit/chainlink-stellar/deployment/sequences.DeployStellarCCIPContracts]).
// Those entrypoints require a non-nil CCV environment topology so committee verifier signature quorum
// can be configured from offchain NOP/committee data.
package ccip
