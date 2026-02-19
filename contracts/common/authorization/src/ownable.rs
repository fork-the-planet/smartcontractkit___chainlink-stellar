//! Ownable trait with default implementation.
//!
//! Ownership management with two-step transfer pattern.
//!
//! Two-step transfer process:
//! 1. Current owner calls `transfer_ownership(new_owner)`
//! 2. New owner calls `accept_ownership()`
//!
//! This prevents accidental transfers to wrong addresses.

use common_error::CCIPError;
use soroban_sdk::{Address, Env, Symbol, contracttrait, symbol_short};

use super::events::{OwnershipTransferStartedEvent, OwnershipTransferredEvent};
use common_guard::initializable::Initializable;

/// Ownership management with two-step transfer pattern.
///
/// Implement this trait for contracts that need single-owner access control.
/// Use the default implementation by providing an empty impl:
/// ```ignore
/// impl Ownable for MyContract {}
/// ```
/// 
/// The `contracttrait` macro is required to export the methods in the concrete contract
/// implementing the trait. Otherwise, it's optional.
#[contracttrait]
pub trait Ownable: Initializable {
    /// Storage key for the owner address.
    const OWNER: Symbol;
    /// Storage key for the pending owner (during two-step transfer).
    const PENDING_OWNER: Symbol;

    /// Initialize the owner. Should be called during contract initialization.
    ///
    /// # Arguments
    /// * `env` - The environment
    /// * `owner` - The initial owner address
    fn init_owner(env: &Env, owner: &Address) -> Result<(), CCIPError> {
        if env.storage().instance().has(&Self::OWNER) {
            return Err(CCIPError::AlreadyInitialized);
        }

        env.storage().instance().set(&Self::OWNER, owner);
        Ok(())
    }

    /// Get the current owner.
    ///
    /// # Returns
    /// The owner address, or None if not initialized.
    fn owner(env: &Env) -> Option<Address> {
        env.storage().instance().get(&Self::OWNER)
    }

    /// Check if an address is the owner.
    fn is_owner(env: &Env, addr: &Address) -> bool {
        match Self::owner(env) {
            Some(owner) => owner == *addr,
            None => false,
        }
    }

    /// Require that the caller is the owner.
    /// This calls `require_auth()` on the owner address.
    ///
    /// # Errors
    /// * `NotInitialized` - Owner has not been set
    fn require_owner(env: &Env) -> Result<Address, CCIPError> {
        let owner = Self::owner(env).ok_or(CCIPError::NotOwner)?;
        owner.require_auth();
        Ok(owner)
    }

    /// Start ownership transfer to a new address (two-step process).
    /// The new owner must call `accept_ownership()` to complete the transfer.
    ///
    /// # Arguments
    /// * `env` - The environment
    /// * `new_owner` - The proposed new owner
    ///
    /// # Errors
    /// * `NotInitialized` - Owner has not been set
    fn transfer_ownership(env: &Env, new_owner: &Address) -> Result<(), CCIPError> {
        let current_owner = Self::require_owner(env)?;

        env.storage()
            .instance()
            .set(&Self::PENDING_OWNER, new_owner);

        OwnershipTransferStartedEvent {
            previous_owner: current_owner,
            new_owner: new_owner.clone(),
        }
        .publish(env);

        Ok(())
    }

    /// Accept pending ownership transfer.
    /// Must be called by the pending new owner.
    ///
    /// # Errors
    /// * `NoPendingOwner` - No ownership transfer is pending
    fn accept_ownership(env: &Env) -> Result<(), CCIPError> {
        let pending: Address = env
            .storage()
            .instance()
            .get(&Self::PENDING_OWNER)
            .ok_or(CCIPError::NoPendingOwner)?;

        // Require the pending owner to authorize
        pending.require_auth();

        let previous_owner = Self::owner(env);

        // Set new owner and clear pending
        env.storage().instance().set(&Self::OWNER, &pending);
        env.storage().instance().remove(&Self::PENDING_OWNER);

        OwnershipTransferredEvent {
            previous_owner: previous_owner.unwrap_or(pending.clone()),
            new_owner: pending,
        }
        .publish(env);

        Ok(())
    }

    /// Get the pending owner (if any).
    fn get_pending_owner(env: &Env) -> Option<Address> {
        env.storage().instance().get(&Self::PENDING_OWNER)
    }

    /// Cancel a pending ownership transfer.
    /// Can only be called by the current owner.
    fn cancel_ownership_transfer(env: &Env) -> Result<(), CCIPError> {
        Self::require_owner(env)?;
        env.storage().instance().remove(&Self::PENDING_OWNER);
        Ok(())
    }

    /// A method to transfer ownership without waiting for the new owner to accept.
    fn set_new_owner(env: &Env, new_owner: &Address) -> Result<(), CCIPError> {
        Self::require_owner(env)?;
        env.storage().instance().set(&Self::OWNER, new_owner);
        Ok(())
    }
}

/// Default implementation of Ownable using standard storage keys.
///
/// Use this when you need to call Ownable methods without a specific contract type
/// (e.g., from AuthorizedCallers or AccessControl).
pub struct DefaultOwnable;

impl Initializable for DefaultOwnable {
    const INITIALIZED: Symbol = symbol_short!("INIT");
}

impl Ownable for DefaultOwnable {
    /// Storage key for the owner address.
    const OWNER: Symbol = symbol_short!("AUTHOWN");
    /// Storage key for the pending owner (during two-step transfer).
    const PENDING_OWNER: Symbol = symbol_short!("PNDGOWNR");
}
