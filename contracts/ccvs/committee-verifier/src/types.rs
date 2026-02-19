use common_authorization::allowlist::AllowListUpdateInterface;
use common_verifier::base_verifier::RemoteChainConfigInterface;
use soroban_sdk::{contracttype, Address, Vec};
use common_helpers::validation::Validatable;
use common_error::CCIPError as BaseVerifierError;

/// Dynamic config mirrored from EVM CommitteeVerifier.DynamicConfig.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DynamicConfig {
    /// Destination for withdrawn fee tokens.
    pub fee_aggregator: Option<Address>,
    /// Optional allowlist admin, owner still has full access.
    pub allowlist_admin: Option<Address>,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RemoteChainConfig {
    pub remote_chain_selector: u64,
    pub router: Option<Address>,
    pub allowlist_enabled: bool,
    pub fee_usd_cents: u32,
    pub gas_for_verification: u32,
    pub payload_size_bytes: u32,
}

impl RemoteChainConfigInterface for RemoteChainConfig {
    fn get_fee_data(&self) -> (u32, u32, u32) {
        (self.fee_usd_cents, self.gas_for_verification, self.payload_size_bytes)
    }

    fn remote_chain_selector(&self) -> u64 {
        self.remote_chain_selector
    }
}

impl Validatable for RemoteChainConfig {
    fn validate(&self) -> Result<(), BaseVerifierError> {
        if self.remote_chain_selector == 0 || self.gas_for_verification == 0 {
            return Err(BaseVerifierError::InvalidConfig);
        }

        if self.router.is_none() && self.allowlist_enabled {
            return Err(BaseVerifierError::InvalidConfig);
        }

        // TODO: add other validation rules here

        Ok(())
    }
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AllowListUpdate {
    pub dest_chain_selector: u64,
    // pub allowlist_enabled: bool,
    pub added_allowlisted_senders: Vec<Address>,
    pub removed_allowlisted_senders: Vec<Address>,
}

impl Validatable for AllowListUpdate {
    fn validate(&self) -> Result<(), BaseVerifierError> {
        // TODO: add validation rules here
        Ok(())
    }
}

impl AllowListUpdateInterface for AllowListUpdate {
    fn key(&self) -> u64 {
        self.dest_chain_selector
    }

    fn get_allowlist_addresses_to_add(&self) -> Vec<Address> {
        self.added_allowlisted_senders.clone()
    }

    fn get_allowlist_addresses_to_remove(&self) -> Vec<Address> {
        self.removed_allowlisted_senders.clone()
    }
}