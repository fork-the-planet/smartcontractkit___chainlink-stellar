#![no_std]

pub mod initializable;

use common_error::CCIPError;
use soroban_sdk::{symbol_short, Env, Symbol};

/// Storage key for the reentrancy guard flag.
/// Uses temporary storage which is cleared after each transaction.
const REENTRANCY_GUARD: Symbol = symbol_short!("RE_GUARD");

/// A reentrancy guard that prevents recursive calls into protected functions.
///
/// This implementation uses temporary storage, which is automatically cleared
/// at the end of each transaction. This provides reentrancy protection without
/// persisting any state.
///
/// # Usage Patterns
///
/// ## Simple (single guarded function per contract)
/// When you have only one guarded function, you don't need to call `exit()`
/// since temporary storage is cleared automatically after the transaction:
///
/// ```ignore
/// pub fn protected_function(env: Env) -> Result<(), CCIPError> {
///     ReentrancyGuard::enter(&env)?;
///     // ... protected logic (may call external contracts) ...
///     Ok(())
///     // No exit() needed - temporary storage clears automatically
/// }
/// ```
///
/// ## Multiple guarded functions in sequence
/// If the same transaction may call multiple guarded functions sequentially,
/// use `exit()` or `with_guard()` so each can acquire the lock:
///
/// ```ignore
/// pub fn function_a(env: Env) -> Result<(), CCIPError> {
///     ReentrancyGuard::enter(&env)?;
///     // ... logic ...
///     ReentrancyGuard::exit(&env);  // Allow function_b to be called next
///     Ok(())
/// }
/// ```
pub struct ReentrancyGuard;

impl ReentrancyGuard {
    /// Enter the guarded section.
    ///
    /// Sets the guard flag to prevent reentrant calls within the same transaction.
    /// Any nested call to `enter()` will fail with `ReentrantCall` error.
    ///
    /// # Note
    /// For single-guarded-function contracts, you don't need to call `exit()`
    /// since temporary storage is cleared automatically after the transaction.
    pub fn enter(env: &Env) -> Result<(), CCIPError> {
        if env.storage().temporary().has(&REENTRANCY_GUARD) {
            return Err(CCIPError::ReentrantCall);
        }
        env.storage().temporary().set(&REENTRANCY_GUARD, &true);
        Ok(())
    }

    /// Exit the guarded section (optional for single-function guards).
    ///
    /// Clears the guard flag, allowing subsequent calls to `enter()` within
    /// the same transaction. Only needed if multiple guarded functions may
    /// be called sequentially in one transaction.
    pub fn exit(env: &Env) {
        env.storage().temporary().remove(&REENTRANCY_GUARD);
    }

    /// Execute a closure within a guarded section with automatic cleanup.
    ///
    /// Useful when you need explicit scope control or have multiple guarded
    /// functions that may be called sequentially.
    pub fn with_guard<F, T, E>(env: &Env, f: F) -> Result<T, E>
    where
        F: FnOnce() -> Result<T, E>,
        E: From<CCIPError>,
    {
        Self::enter(env)?;
        let result = f();
        Self::exit(env);
        result
    }

    /// Check if the guard is currently active.
    pub fn is_entered(env: &Env) -> bool {
        env.storage().temporary().has(&REENTRANCY_GUARD)
    }
}

mod test;
