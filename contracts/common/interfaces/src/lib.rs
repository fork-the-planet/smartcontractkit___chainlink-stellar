//! Cross-contract client interfaces for the CCIP Stellar workspace.
//!
//! This crate defines shared interface traits using `#[contractclient]`,
//! following the [Stellar workspace pattern](https://developers.stellar.org/docs/build/smart-contracts/example-contracts/workspace).
//!
//! Each module defines a trait for a contract's cross-contract interface and
//! auto-generates a typed client via `#[contractclient]`. Other contracts
//! (e.g. the Router) import these clients to make cross-contract calls
//! without depending on the full contract implementation — avoiding WASM
//! export symbol collisions.
//!
//! To regenerate or verify these interfaces against compiled contracts:
//! ```sh
//! make generate-interfaces
//! ```
#![no_std]

pub mod ccip_receiver;
pub mod committee_verifier;
pub mod fee_quoter;
pub mod offramp;
pub mod onramp;
pub mod pool_hooks;
pub mod ramp_registry;
pub mod rmn_proxy;
pub mod rmn_remote;
pub mod router;
pub mod siloed_lock_release_pool;
pub mod token_admin_registry;
pub mod token_lock_box;
pub mod token_pool;
pub mod versioned_verifier_resolver;

/// Convert the generated `ramp_registry::CCIPError` into [`common_error::CCIPError`].
///
/// The bindings duplicate the on-chain `contracterror` discriminant space; the generated
/// enum may use a smaller ABI than `common_error::CCIPError` (`#[repr(u32)]`), so we widen
/// via `as u32` and reinterpret as the canonical type.
impl From<ramp_registry::CCIPError> for common_error::CCIPError {
    #[inline]
    fn from(e: ramp_registry::CCIPError) -> Self {
        let code = e as u32;
        // SAFETY: `common_error::CCIPError` is `#[repr(u32)]` and `code` comes from the
        // interface copy of the same on-chain error table (valid discriminant).
        unsafe { core::mem::transmute::<u32, Self>(code) }
    }
}
