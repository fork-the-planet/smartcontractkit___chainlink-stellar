use soroban_sdk::{contractevent, Address};

// ============================================================
// Events
// ============================================================

/// Emitted when a fee token is added.
#[contractevent(topics = ["fq_FeeTokenAdded"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FeeTokenAddedEvent {
    /// Fee token address.
    pub fee_token: Address,
}

/// Emitted when a fee token is removed.
#[contractevent(topics = ["fq_FeeTokenRemoved"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FeeTokenRemovedEvent {
    /// Fee token address.
    pub fee_token: Address,
}

/// Emitted when USD per unit gas is updated for a destination chain.
#[contractevent(topics = ["fq_UsdPerUnitGasUpdated"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct UsdPerUnitGasUpdatedEvent {
    /// Destination chain selector.
    pub dest_chain_selector: u64,
    /// New gas price value in USD with 18 decimals.
    pub value: u128,
    /// Timestamp of the update.
    pub timestamp: u64,
}

/// Emitted when USD per token is updated.
#[contractevent(topics = ["fq_UsdPerTokenUpdated"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct UsdPerTokenUpdatedEvent {
    /// Token address.
    pub token: Address,
    /// New token price value in USD with 18 decimals.
    pub value: u128,
    /// Timestamp of the update.
    pub timestamp: u64,
}

/// Emitted when token transfer fee config is updated.
#[contractevent(topics = ["fq_TknTransferFeeUpdated"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TokenFeeConfigUpdatedEvent {
    /// Destination chain selector.
    pub dest_chain_selector: u64,
    /// Token address.
    pub token: Address,
    /// Fee in USD cents.
    pub fee_usd_cents: u32,
    /// Gas overhead.
    pub dest_gas_overhead: u32,
    /// Bytes overhead.
    pub dest_bytes_overhead: u32,
}

/// Emitted when token transfer fee config is deleted.
#[contractevent(topics = ["fq_TknTransferFeeDeleted"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TokenFeeConfigDeletedEvent {
    /// Destination chain selector.
    pub dest_chain_selector: u64,
    /// Token address.
    pub token: Address,
}

/// Emitted when a new destination chain is added.
#[contractevent(topics = ["fq_DestChainAdded"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DestChainAddedEvent {
    /// Destination chain selector.
    pub dest_chain_selector: u64,
    /// Whether enabled.
    pub is_enabled: bool,
    /// Max data bytes.
    pub max_data_bytes: u32,
}

/// Emitted when destination chain config is updated.
#[contractevent(topics = ["fq_DestChainConfigUpdated"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DestChainConfigUpdatedEvent {
    /// Destination chain selector.
    pub dest_chain_selector: u64,
    /// Whether enabled.
    pub is_enabled: bool,
    /// Max data bytes.
    pub max_data_bytes: u32,
}
