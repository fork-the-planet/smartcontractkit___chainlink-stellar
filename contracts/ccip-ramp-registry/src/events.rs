use soroban_sdk::{contractevent, Address};

/// Emitted when an OnRamp is configured for a destination chain.
#[contractevent(topics = ["ramp_reg_OnRampSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OnRampSetEvent {
    pub dest_chain_selector: u64,
    pub onramp: Address,
}

/// Emitted when an OnRamp is removed for a destination chain.
#[contractevent(topics = ["ramp_reg_OnRampRemoved"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OnRampRemovedEvent {
    pub dest_chain_selector: u64,
}

/// Emitted when an OffRamp is added for a source chain.
#[contractevent(topics = ["ramp_reg_OffRampAdded"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampAddedEvent {
    pub source_chain_selector: u64,
    pub offramp: Address,
}

/// Emitted when an OffRamp is removed for a source chain.
#[contractevent(topics = ["ramp_reg_OffRampRemoved"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampRemovedEvent {
    pub source_chain_selector: u64,
    pub offramp: Address,
}
