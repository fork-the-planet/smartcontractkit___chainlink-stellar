package ccip

// Datastore contract type strings for Stellar CCIP deployments.
const (
	CcipReceiverContractType         = "CcipReceiverExample"
	TokenAdminRegistryContractType   = "token_admin_registry"
	LockReleaseTokenPoolContractType = "lock_release_token_pool"
	TestTokenContractType            = "sac_token"

	// DevenvTestTokenPoolQualifier is the datastore qualifier for the lock-release test pool.
	DevenvTestTokenPoolQualifier = "TEST"
)
