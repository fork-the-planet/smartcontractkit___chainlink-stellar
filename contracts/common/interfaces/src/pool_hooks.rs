/// Cross-contract interface for advanced pool hooks — Stellar analogue of
/// EVM [`IAdvancedPoolHooks`](https://github.com/smartcontractkit/chainlink-ccip/blob/develop/chains/evm/contracts/interfaces/IAdvancedPoolHooks.sol).
///
/// An optional external contract that pools call before `lock_or_burn`
/// (preflight) and before `release_or_mint` (postflight) for additional
/// security checks such as:
/// - Sender allowlisting
/// - CCV (cross-chain verifier) configuration
/// - Policy engine validation
///
/// The hooks contract must authorize the calling pool via `require_auth`.
#[soroban_sdk::contractclient(name = "PoolHooksClient")]
pub trait PoolHooksInterface {
    /// Called before lock_or_burn. Revert (return Err) to block the transfer.
    ///
    /// # Arguments
    /// * `original_sender` — the user initiating the cross-chain send
    /// * `remote_chain_selector` — destination chain
    /// * `amount` — token amount (after any fee deduction)
    /// * `requested_finality` — finality config from the sender
    fn preflight_check(
        env: soroban_sdk::Env,
        original_sender: soroban_sdk::Address,
        remote_chain_selector: u64,
        amount: i128,
        requested_finality: u32,
    ) -> Result<(), CCIPError>;

    /// Called before release_or_mint. Revert (return Err) to block the transfer.
    ///
    /// # Arguments
    /// * `source_chain_selector` — the source chain
    /// * `receiver` — the local recipient
    /// * `amount` — local token amount to be released/minted
    /// * `requested_finality` — finality config from the message
    fn postflight_check(
        env: soroban_sdk::Env,
        source_chain_selector: u64,
        receiver: soroban_sdk::Address,
        amount: i128,
        requested_finality: u32,
    ) -> Result<(), CCIPError>;
}

#[soroban_sdk::contracterror(export = false)]
#[derive(Debug, Copy, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub enum CCIPError {
    NotInitialized = 1,
    Unauthorized = 3,
    CallerNotAuthorized = 6,
    SenderNotAllowed = 49,
    InvalidConfig = 52,
}
