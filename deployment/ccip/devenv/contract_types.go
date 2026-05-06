package devenv

import stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"

// Datastore contract type strings for Stellar CCIP devenv deployments (aliases of [github.com/smartcontractkit/chainlink-stellar/deployment/ccip]).
const (
	CcipReceiverContractType         = stellarccip.CcipReceiverContractType
	TokenAdminRegistryContractType   = stellarccip.TokenAdminRegistryContractType
	LockReleaseTokenPoolContractType = stellarccip.LockReleaseTokenPoolContractType
	TestTokenContractType            = stellarccip.TestTokenContractType
	DevenvTestTokenPoolQualifier     = stellarccip.DevenvTestTokenPoolQualifier
)
