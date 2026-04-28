//! RBACTimelock events (mirrors RBACTimelock.sol events 1:1 where applicable).

use soroban_sdk::{contractevent, Address, Bytes, BytesN, Symbol};

/// Emitted when a call is scheduled as part of an operation.
#[contractevent(topics = ["tl_CallScheduled"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CallScheduledEvent {
    pub id: BytesN<32>,
    pub index: u32,
    pub to: BytesN<32>,
    pub data: Bytes,
    pub predecessor: BytesN<32>,
    pub salt: BytesN<32>,
    pub delay: u64,
}

/// Emitted when a scheduled call is executed.
#[contractevent(topics = ["tl_CallExecuted"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CallExecutedEvent {
    pub id: BytesN<32>,
    pub index: u32,
    pub to: BytesN<32>,
    pub data: Bytes,
}

/// Emitted when a call is executed via the bypasser path.
#[contractevent(topics = ["tl_BypCallExec"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BypasserCallExecutedEvent {
    pub index: u32,
    pub to: BytesN<32>,
    pub data: Bytes,
}

/// Emitted when an operation is cancelled.
#[contractevent(topics = ["tl_Cancelled"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CancelledEvent {
    pub id: BytesN<32>,
}

/// Emitted when the minimum delay is changed.
#[contractevent(topics = ["tl_MinDelay"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MinDelayChangeEvent {
    pub old_duration: u64,
    pub new_duration: u64,
}

/// Emitted when a function selector (Soroban Symbol) is blocked.
#[contractevent(topics = ["tl_SelBlocked"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FunctionSelectorBlockedEvent {
    pub selector: Symbol,
}

/// Emitted when a function selector is unblocked.
#[contractevent(topics = ["tl_SelUnblock"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FunctionSelectorUnblockedEvent {
    pub selector: Symbol,
}

/// Emitted when a role is granted.
#[contractevent(topics = ["tl_RoleGranted"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RoleGrantedEvent {
    pub role: Symbol,
    pub account: Address,
    pub sender: Address,
}

/// Emitted when a role is revoked.
#[contractevent(topics = ["tl_RoleRevoked"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RoleRevokedEvent {
    pub role: Symbol,
    pub account: Address,
    pub sender: Address,
}
