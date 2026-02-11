use soroban_sdk::{contractevent, Address};

// ============================================================
// Events
// ============================================================

/// Emitted when the RMN implementation address is updated.
#[contractevent(topics = ["rmn_proxy_RmnSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RmnSetEvent {
    /// The new RMN implementation address
    pub rmn: Address,
}
