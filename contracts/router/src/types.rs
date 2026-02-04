use soroban_sdk::{contracttype, Address};

// ============================================================
// Types & Structs
// ============================================================

/// Static configuration for the Router contract.
/// Set during initialization and cannot be changed.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RouterConfig {
    /// RMN proxy contract address for curse checking
    pub rmn_proxy: Address,
}

/// Represents an OnRamp configuration entry.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OnRampEntry {
    /// Destination chain selector
    pub dest_chain_selector: u64,
    /// OnRamp contract address
    pub onramp: Address,
}

/// Represents an OffRamp configuration entry.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampEntry {
    /// Source chain selector
    pub source_chain_selector: u64,
    /// OffRamp contract address
    pub offramp: Address,
}
