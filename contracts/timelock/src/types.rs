//! On-chain types for RBACTimelock-Stellar.

use soroban_sdk::{contracttype, symbol_short, Bytes, BytesN, Symbol, Vec};

/// Sentinel timestamp stored when an operation has been executed.
pub const DONE_TIMESTAMP: u64 = 1;

/// Standard role constants — mirrors RBACTimelock.sol role keccak constants.
pub const ADMIN_ROLE: Symbol = symbol_short!("ADMIN");
pub const PROPOSER_ROLE: Symbol = symbol_short!("PROPOSER");
pub const EXECUTOR_ROLE: Symbol = symbol_short!("EXECUTOR");
pub const CANCELLER_ROLE: Symbol = symbol_short!("CANCELLER");
pub const BYPASSER_ROLE: Symbol = symbol_short!("BYPASSER");

/// A single call to be executed by the timelock.
///
/// Mirrors `RBACTimelock.Call` but without the `value` field — native XLM
/// attachment is not supported in this version.
///
/// `data` is XDR-encoded as `ScVec([ScSymbol(fn_name), arg0, arg1, ...])` —
/// the same encoding used by MCMS `StellarOp.data`.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Call {
    pub to: BytesN<32>,
    pub data: Bytes,
}

/// Wrapper so exported contract methods accept `Vec<Call>` (Soroban ABI restriction
/// on bare `Vec<ContractType>` arguments — same pattern as `SignatureVec` in mcms).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Calls {
    pub inner: Vec<Call>,
}
