use soroban_sdk::{contractevent, Address};

// ============================================================
// Events
// ============================================================

/// Emitted when an OnRamp is configured for a destination chain.
#[contractevent(topics = ["router_OnRampSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OnRampSetEvent {
    /// Destination chain selector
    pub dest_chain_selector: u64,
    /// OnRamp contract address (can be zero/None to disable)
    pub onramp: Address,
}

/// Emitted when an OffRamp is added for a source chain.
#[contractevent(topics = ["router_OffRampAdded"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampAddedEvent {
    /// Source chain selector
    pub source_chain_selector: u64,
    /// OffRamp contract address
    pub offramp: Address,
}

/// Emitted when an OffRamp is removed for a source chain.
#[contractevent(topics = ["router_OffRampRemoved"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampRemovedEvent {
    /// Source chain selector
    pub source_chain_selector: u64,
    /// OffRamp contract address
    pub offramp: Address,
}

/// Emitted when a message is executed (routed to receiver).
#[contractevent(topics = ["router_MessageExecuted"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MessageExecutedEvent {
    /// Message ID
    pub message_id: soroban_sdk::BytesN<32>,
    /// Source chain selector
    pub source_chain_selector: u64,
    /// OffRamp that delivered the message
    pub offramp: Address,
}

/// Emitted when ownership is transferred.
#[contractevent(topics = ["router_OwnershipTransferred"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OwnershipTransferredEvent {
    /// New owner address
    pub new_owner: Address,
}
