use soroban_sdk::{contracttype, Address};

/// OnRamp mapping for a destination chain (same shape as router `OnRampEntry`).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OnRampEntry {
    pub dest_chain_selector: u64,
    pub onramp: Address,
}

/// OffRamp mapping for a source chain (same shape as router `OffRampEntry`).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampEntry {
    pub source_chain_selector: u64,
    pub offramp: Address,
}
