//! FeeQuoter cross-contract client interface.
//!
//! Defines the subset of FeeQuoter functions callable by other contracts
//! (primarily the Router and OnRamp). The generated `FeeQuoterClient` makes typed
//! cross-contract calls without importing the full FeeQuoter implementation.

use common_message::StellarToAnyMessage;
use fee_quoter::error::FeeQuoterError;
use fee_quoter::types::{
    DestChainConfig, DestChainConfigArgs, GasQuoteResult, PriceUpdates, StaticConfig,
    TimestampedPrice, TokenFeeConfigArgs, TokenFeeConfigRemoveArgs, TokenTransferFeeConfig,
    TokenTransferFeeResult,
};
use soroban_sdk::{contractclient, Address, Env, Vec};

#[contractclient(name = "FeeQuoterClient")]
pub trait FeeQuoterInterface {
    // ========================================
    // Price Query Functions
    // ========================================

    /// Get the price for a token (may be stale or zero).
    fn get_token_price(env: Env, token: Address) -> Result<TimestampedPrice, FeeQuoterError>;

    /// Get prices for multiple tokens.
    fn get_token_prices(
        env: Env,
        tokens: Vec<Address>,
    ) -> Result<Vec<TimestampedPrice>, FeeQuoterError>;

    /// Get the validated price for a token (reverts if not set).
    fn get_validated_token_price(env: Env, token: Address) -> Result<u128, FeeQuoterError>;

    /// Get the gas price for a destination chain.
    fn get_dest_chain_gas_price(
        env: Env,
        dest_chain_selector: u64,
    ) -> Result<TimestampedPrice, FeeQuoterError>;

    // ========================================
    // Fee Token Functions
    // ========================================

    /// Get the list of fee tokens.
    fn get_fee_tokens(env: Env) -> Result<Vec<Address>, FeeQuoterError>;

    /// Remove fee tokens (owner only).
    fn remove_fee_tokens(env: Env, tokens: Vec<Address>) -> Result<(), FeeQuoterError>;

    // ========================================
    // Price Update Functions
    // ========================================

    /// Update token and gas prices. Only callable by authorized callers.
    fn update_prices(env: Env, price_updates: PriceUpdates) -> Result<(), FeeQuoterError>;

    // ========================================
    // Fee Calculation Functions
    // ========================================

    /// Quote gas for execution on a destination chain.
    fn quote_gas_for_exec(
        env: Env,
        dest_chain_selector: u64,
        non_calldata_gas: u32,
        calldata_size: u32,
        fee_token: Address,
    ) -> Result<GasQuoteResult, FeeQuoterError>;

    /// Get token transfer fee components.
    fn get_token_transfer_fee(
        env: Env,
        dest_chain_selector: u64,
        token: Address,
    ) -> Result<TokenTransferFeeResult, FeeQuoterError>;

    /// Get the fee for sending a CCIP message.
    fn get_message_fee(
        env: Env,
        dest_chain_selector: u64,
        message: StellarToAnyMessage,
    ) -> Result<i128, FeeQuoterError>;

    /// Convert a token amount to another token.
    fn convert_token_amount(
        env: Env,
        from_token: Address,
        from_token_amount: i128,
        to_token: Address,
    ) -> Result<i128, FeeQuoterError>;

    // ========================================
    // Destination Chain Config Functions
    // ========================================

    /// Get configuration for a destination chain.
    fn get_dest_chain_config(
        env: Env,
        dest_chain_selector: u64,
    ) -> Result<DestChainConfig, FeeQuoterError>;

    /// Get all destination chain configurations.
    fn get_all_dest_configs(
        env: Env,
    ) -> Result<(Vec<u64>, Vec<DestChainConfig>), FeeQuoterError>;

    /// Apply destination chain config updates (owner only).
    fn apply_dest_chain_configs(
        env: Env,
        config_args: Vec<DestChainConfigArgs>,
    ) -> Result<(), FeeQuoterError>;

    // ========================================
    // Token Transfer Fee Config Functions
    // ========================================

    /// Get token transfer fee configuration.
    fn get_token_fee_config(
        env: Env,
        dest_chain_selector: u64,
        token: Address,
    ) -> Result<TokenTransferFeeConfig, FeeQuoterError>;

    /// Apply token transfer fee config updates (owner only).
    fn apply_token_fee_configs(
        env: Env,
        config_args: Vec<TokenFeeConfigArgs>,
        remove_args: Vec<TokenFeeConfigRemoveArgs>,
    ) -> Result<(), FeeQuoterError>;

    // ========================================
    // Static Config Functions
    // ========================================

    /// Get the static configuration.
    fn get_static_config(env: Env) -> Result<StaticConfig, FeeQuoterError>;

    // ========================================
    // Authorized Caller Management
    // ========================================

    /// Add an authorized caller (owner only).
    fn add_authorized_caller(env: Env, caller: Address) -> Result<(), FeeQuoterError>;

    /// Remove an authorized caller (owner only).
    fn remove_authorized_caller(env: Env, caller: Address) -> Result<(), FeeQuoterError>;

    /// Get all authorized callers.
    fn get_authorized_callers(env: Env) -> Result<Vec<Address>, FeeQuoterError>;

    // ========================================
    // Owner Management (two-step transfer)
    // ========================================

    /// Start ownership transfer to a new address (two-step process).
    fn transfer_ownership(env: Env, new_owner: Address) -> Result<(), FeeQuoterError>;

    /// Accept a pending ownership transfer.
    fn accept_ownership(env: Env) -> Result<(), FeeQuoterError>;

    /// Get the pending owner (if any).
    fn get_pending_owner(env: Env) -> Result<Option<Address>, FeeQuoterError>;

    /// Get the current owner.
    fn owner(env: Env) -> Result<Address, FeeQuoterError>;
}
