use soroban_sdk::{contractevent, Address, Symbol};

// ============================================================
// Ownership Events
// ============================================================

/// Emitted when ownership transfer is initiated (two-step transfer).
#[contractevent(topics = ["auth_OwnerTransferStart"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OwnershipTransferStartedEvent {
    /// Current owner initiating the transfer.
    pub previous_owner: Address,
    /// Proposed new owner.
    pub new_owner: Address,
}

/// Emitted when ownership is transferred (after acceptance).
#[contractevent(topics = ["auth_OwnerTransferred"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OwnershipTransferredEvent {
    /// Previous owner.
    pub previous_owner: Address,
    /// New owner.
    pub new_owner: Address,
}

// ============================================================
// Authorized Callers Events
// ============================================================

/// Emitted when an authorized caller is added.
#[contractevent(topics = ["auth_CallerAdded"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthorizedCallerAddedEvent {
    /// The caller address that was added.
    pub caller: Address,
}

/// Emitted when an authorized caller is removed.
#[contractevent(topics = ["auth_CallerRemoved"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthorizedCallerRemovedEvent {
    /// The caller address that was removed.
    pub caller: Address,
}

// ============================================================
// Access Control Events
// ============================================================

/// Emitted when a role is granted to an account.
#[contractevent(topics = ["auth_RoleGranted"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RoleGrantedEvent {
    /// The role that was granted.
    pub role: Symbol,
    /// The account that received the role.
    pub account: Address,
    /// The account that granted the role.
    pub sender: Address,
}

/// Emitted when a role is revoked from an account.
#[contractevent(topics = ["auth_RoleRevoked"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RoleRevokedEvent {
    /// The role that was revoked.
    pub role: Symbol,
    /// The account that lost the role.
    pub account: Address,
    /// The account that revoked the role.
    pub sender: Address,
}
