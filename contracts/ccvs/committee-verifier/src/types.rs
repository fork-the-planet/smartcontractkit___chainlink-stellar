use soroban_sdk::{contracttype, Address, BytesN, Vec};

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
pub struct SignatureConfig {
    pub source_chain_selector: u64,
    pub threshold: u32,
    /// TODO: confirm signer encoding from offchain verifier signer set format.
    /// Using 32-byte Ed25519 pubkeys as scaffold.
    pub signers: Vec<BytesN<32>>,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignatureConfigState {
    pub threshold: u32,
    pub signers: Vec<BytesN<32>>,
}
