use soroban_sdk::{contracttype, Address};

/// Dynamic config mirrored from EVM CommitteeVerifier.DynamicConfig.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DynamicConfig {
    /// Destination for withdrawn fee tokens.
    pub fee_aggregator: Option<Address>,
    /// Optional allowlist admin, owner still has full access.
    pub allowlist_admin: Option<Address>,
}
