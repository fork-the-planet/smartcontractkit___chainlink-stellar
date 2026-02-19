#![no_std]

mod events;

use soroban_sdk::{contract, contractimpl, symbol_short, Address, Env, Symbol};

use common_authorization::Ownable;
use common_error::CCIPError as RmnProxyError;
use events::RmnSetEvent;

// ============================================================
// Storage Keys
// ============================================================

const INITIALIZED: Symbol = symbol_short!("INIT");
const RMN: Symbol = symbol_short!("RMN");

// ============================================================
// Contract
// ============================================================

/// RMN Proxy contract.
///
/// Modeled after the EVM RMNProxy.sol (setARM/getARM/fallback pattern),
/// adapted for Soroban where we expose explicit methods instead of a fallback.
///
/// The proxy holds a stable address and points to the current RMN implementation.
/// Consumers (Router, TokenPools) call `is_cursed()` on the proxy, which will
/// delegate to the RMN Remote implementation once it exists.
#[contract]
pub struct RmnProxyContract;

#[contractimpl]
impl RmnProxyContract {
    // ========================================
    // Initialization
    // ========================================

    /// Initialize the RMN Proxy contract.
    ///
    /// # Arguments
    /// * `owner` - The owner address (typically MCMS)
    /// * `rmn` - The initial RMN implementation address
    ///
    /// # Errors
    /// * `AlreadyInitialized` - If contract is already initialized
    pub fn initialize(env: Env, owner: Address, rmn: Address) -> Result<(), RmnProxyError> {
        if env.storage().instance().has(&INITIALIZED) {
            return Err(RmnProxyError::AlreadyInitialized);
        }

        // Initialize owner via shared authorization lib
        Ownable::init(&env, &owner);

        // Store the RMN implementation address
        env.storage().instance().set(&RMN, &rmn);

        // Mark as initialized
        env.storage().instance().set(&INITIALIZED, &true);

        Ok(())
    }

    // ========================================
    // RMN Proxy Functions
    // ========================================

    /// Set the RMN implementation address. Only callable by owner.
    ///
    /// Equivalent to EVM's `setARM(address arm)`.
    ///
    /// # Arguments
    /// * `rmn` - The new RMN implementation address
    ///
    /// # Errors
    /// * `NotInitialized` - If contract is not initialized
    /// * `Unauthorized` - If caller is not the owner
    pub fn set_rmn(env: Env, rmn: Address) -> Result<(), RmnProxyError> {
        Self::require_initialized(&env)?;
        Ownable::require_owner(&env)?;

        env.storage().instance().set(&RMN, &rmn);

        RmnSetEvent { rmn }.publish(&env);

        Ok(())
    }

    /// Get the current RMN implementation address.
    ///
    /// Equivalent to EVM's `getARM()`.
    ///
    /// # Returns
    /// The current RMN implementation address
    pub fn get_rmn(env: Env) -> Result<Address, RmnProxyError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&RMN)
            .ok_or(RmnProxyError::NotInitialized)
    }

    /// Check if the network is globally cursed.
    ///
    /// In the EVM architecture, this delegates to the RMN Remote via the proxy's
    /// fallback function. In Soroban, we expose it as an explicit method.
    ///
    /// Currently returns `false` because no RMN Remote contract exists yet.
    /// When the RMN Remote is implemented, this will become a cross-contract call:
    /// ```ignore
    /// let rmn_addr = Self::get_rmn(env.clone())?;
    /// let rmn_client = rmn_remote::RmnRemoteClient::new(&env, &rmn_addr);
    /// Ok(rmn_client.is_cursed())
    /// ```
    ///
    /// # Returns
    /// `true` if the network is cursed and operations should be halted
    pub fn is_cursed(env: Env) -> Result<bool, RmnProxyError> {
        Self::require_initialized(&env)?;

        // TODO: Delegate to RMN Remote when it exists.
        // For now, return false (not cursed) to allow the system to operate.
        // The RMN address is stored and ready for delegation.
        let _rmn: Address = env
            .storage()
            .instance()
            .get(&RMN)
            .ok_or(RmnProxyError::NotInitialized)?;

        Ok(false)
    }

    // ========================================
    // Owner Management (via common-authorization)
    // ========================================

    /// Get the current owner address.
    pub fn owner(env: Env) -> Result<Address, RmnProxyError> {
        Self::require_initialized(&env)?;
        Ownable::get_owner(&env).ok_or(RmnProxyError::NotInitialized)
    }

    /// Start ownership transfer to a new address (two-step process).
    /// The new owner must call `accept_ownership()` to complete the transfer.
    pub fn transfer_ownership(env: Env, new_owner: Address) -> Result<(), RmnProxyError> {
        Self::require_initialized(&env)?;
        Ownable::transfer_ownership(&env, &new_owner)?;
        Ok(())
    }

    /// Accept pending ownership transfer. Must be called by the pending new owner.
    pub fn accept_ownership(env: Env) -> Result<(), RmnProxyError> {
        Self::require_initialized(&env)?;
        Ownable::accept_ownership(&env)?;
        Ok(())
    }

    // ========================================
    // Internal Helper Functions
    // ========================================

    fn require_initialized(env: &Env) -> Result<(), RmnProxyError> {
        if !env.storage().instance().has(&INITIALIZED) {
            return Err(RmnProxyError::NotInitialized);
        }
        Ok(())
    }
}

mod test;
