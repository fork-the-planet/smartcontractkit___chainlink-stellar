package ccip

// Datastore contract type strings for Stellar CCIP deployments.
const (
	CcipReceiverContractType              = "CcipReceiverExample"
	TokenAdminRegistryContractType        = "token_admin_registry"
	LockReleaseTokenPoolContractType      = "lock_release_token_pool"
	SiloedLockReleaseTokenPoolContractType = "siloed_lock_release_token_pool"
	TestTokenContractType                 = "sac_token"

	// DevenvTestTokenPoolQualifier is the datastore qualifier for the siloed test pool used in E2E transfers.
	DevenvTestTokenPoolQualifier = "TEST"

	// DevenvLegacyLockReleasePoolQualifier is the datastore qualifier for the legacy lock-release pool
	// deployed alongside the siloed pool (e.g. MCMS / lock-release-specific tests).
	DevenvLegacyLockReleasePoolQualifier = "LEGACY_LR"
)
