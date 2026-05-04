//! CCIP ramp registry client (matches on-chain `RampRegistryContract`).
//!
//! Off-chain operators keep this registry aligned with the Router ramp tables so token pools
//! can authorize ramp callers without calling back into the Router during outbound sends.
//!
//! Trait signatures use `soroban_sdk::Env` / `soroban_sdk::Address` paths so `bindings/generator`
//! (`parseFunctions`) can parse this file the same way as generated router interfaces.

use common_error::CCIPError;

/// Same XDR layout as on-chain `ccip-ramp-registry` / Router ramp entries.
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OnRampEntry {
    pub dest_chain_selector: u64,
    pub onramp: soroban_sdk::Address,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OffRampEntry {
    pub source_chain_selector: u64,
    pub offramp: soroban_sdk::Address,
}

/// Batch update for the onramp map.
///
/// - `onramp = Some(addr)` sets/updates the entry for `dest_chain_selector`.
/// - `onramp = None` removes the entry for `dest_chain_selector`.
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OnRampUpdate {
    pub dest_chain_selector: u64,
    pub onramp: Option<soroban_sdk::Address>,
}

/// Batch update for the offramp map. Each update targets a single
/// (source_chain, offramp) pair: `enabled = true` registers it,
/// `enabled = false` removes it.
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OffRampUpdate {
    pub source_chain_selector: u64,
    pub offramp: soroban_sdk::Address,
    pub enabled: bool,
}

#[soroban_sdk::contractargs(name = "RampRegistryArgs")]
#[soroban_sdk::contractclient(name = "RampRegistryClient")]
pub trait RampRegistryInterface {
    fn initialize(env: soroban_sdk::Env, owner: soroban_sdk::Address) -> Result<(), CCIPError>;
    fn type_and_version(env: soroban_sdk::Env) -> soroban_sdk::String;
    fn get_onramp(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
    ) -> Result<soroban_sdk::Address, CCIPError>;
    fn is_offramp(
        env: soroban_sdk::Env,
        source_chain_selector: u64,
        offramp: soroban_sdk::Address,
    ) -> Result<bool, CCIPError>;
    fn get_onramps(env: soroban_sdk::Env) -> Result<soroban_sdk::Vec<OnRampEntry>, CCIPError>;
    fn get_offramps(env: soroban_sdk::Env) -> Result<soroban_sdk::Vec<OffRampEntry>, CCIPError>;
    fn apply_onramp_updates(
        env: soroban_sdk::Env,
        updates: soroban_sdk::Vec<OnRampUpdate>,
    ) -> Result<(), CCIPError>;
    fn apply_offramp_updates(
        env: soroban_sdk::Env,
        updates: soroban_sdk::Vec<OffRampUpdate>,
    ) -> Result<(), CCIPError>;
}
