#![no_std]

mod error;
mod events;
pub mod types;

use soroban_sdk::{contract, contractimpl, symbol_short, Address, Env, Map, Symbol, Vec};

use error::FeeQuoterError;
use events::{
    AuthorizedCallerAddedEvent, AuthorizedCallerRemovedEvent, DestChainAddedEvent,
    DestChainConfigUpdatedEvent, FeeTokenAddedEvent, FeeTokenRemovedEvent,
    OwnershipTransferredEvent, TokenFeeConfigDeletedEvent, TokenFeeConfigUpdatedEvent,
    UsdPerTokenUpdatedEvent, UsdPerUnitGasUpdatedEvent,
};
use types::{
    DestChainConfig, DestChainConfigArgs, GasQuoteResult, PriceUpdates, StaticConfig,
    TimestampedPrice, TokenFeeConfigArgs, TokenFeeConfigRemoveArgs, TokenTransferFeeConfig,
    TokenTransferFeeResult,
};

// ============================================================
// Storage Keys
// ============================================================

const OWNER: Symbol = symbol_short!("OWNER");
const INITIALIZED: Symbol = symbol_short!("INIT");
const STATIC_CFG: Symbol = symbol_short!("STATIC");
const AUTH_CALL: Symbol = symbol_short!("AUTHCALL");
const TOKEN_PRC: Symbol = symbol_short!("TOKENPRC");
const GAS_PRC: Symbol = symbol_short!("GASPRC");
const FEE_TKNS: Symbol = symbol_short!("FEETKNS");
const DEST_CFG: Symbol = symbol_short!("DESTCFG");
const TKN_FEES: Symbol = symbol_short!("TKNFEES");

/// Minimum bytes overhead for token pool return data (matching EVM Pool.CCIP_LOCK_OR_BURN_V1_RET_BYTES).
const CCIP_LOCK_OR_BURN_V1_RET_BYTES: u32 = 32;

// ============================================================
// Contract
// ============================================================

#[contract]
pub struct FeeQuoterContract;

#[contractimpl]
impl FeeQuoterContract {
    // ========================================
    // Initialization
    // ========================================

    /// Initialize the FeeQuoter contract.
    ///
    /// # Arguments
    /// * `owner` - The owner address (typically MCMS)
    /// * `static_config` - Static configuration (immutable after init)
    /// * `authorized_callers` - Initial list of authorized price updaters
    ///
    /// # Errors
    /// * `AlreadyInitialized` - If contract is already initialized
    /// * `InvalidStaticConfig` - If static config is invalid
    pub fn initialize(
        env: Env,
        owner: Address,
        static_config: StaticConfig,
        authorized_callers: Vec<Address>,
    ) -> Result<(), FeeQuoterError> {
        // Check not already initialized
        if env.storage().instance().has(&INITIALIZED) {
            return Err(FeeQuoterError::AlreadyInitialized);
        }

        // Validate static config
        if static_config.max_fee_juels_per_msg <= 0 {
            return Err(FeeQuoterError::InvalidStaticConfig);
        }

        // Store owner
        env.storage().instance().set(&OWNER, &owner);

        // Store static config
        env.storage().instance().set(&STATIC_CFG, &static_config);

        // Store authorized callers
        env.storage().instance().set(&AUTH_CALL, &authorized_callers);

        // Initialize empty maps
        let token_prices: Map<Address, TimestampedPrice> = Map::new(&env);
        env.storage().persistent().set(&TOKEN_PRC, &token_prices);

        let gas_prices: Map<u64, TimestampedPrice> = Map::new(&env);
        env.storage().persistent().set(&GAS_PRC, &gas_prices);

        let fee_tokens: Vec<Address> = Vec::new(&env);
        env.storage().persistent().set(&FEE_TKNS, &fee_tokens);

        // Store dest configs as a Vec of tuples to avoid complex Map type issues
        let dest_selectors: Vec<u64> = Vec::new(&env);
        let dest_configs: Vec<DestChainConfig> = Vec::new(&env);
        env.storage()
            .persistent()
            .set(&DEST_CFG, &(dest_selectors, dest_configs));

        // Token transfer fees use a nested structure
        // We store as Vec of tuples: (dest_chain_selector, token, config)
        let token_fees: Vec<(u64, Address, TokenTransferFeeConfig)> = Vec::new(&env);
        env.storage().persistent().set(&TKN_FEES, &token_fees);

        // Mark as initialized
        env.storage().instance().set(&INITIALIZED, &true);

        Ok(())
    }

    // ========================================
    // Price Query Functions
    // ========================================

    /// Get the price for a token (may be stale or zero).
    ///
    /// # Arguments
    /// * `token` - Token address
    ///
    /// # Returns
    /// Timestamped price (value may be 0 if not set)
    pub fn get_token_price(env: Env, token: Address) -> Result<TimestampedPrice, FeeQuoterError> {
        Self::require_initialized(&env)?;

        let token_prices: Map<Address, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&TOKEN_PRC)
            .unwrap_or(Map::new(&env));

        Ok(token_prices.get(token).unwrap_or(TimestampedPrice {
            value: 0,
            timestamp: 0,
        }))
    }

    /// Get prices for multiple tokens.
    ///
    /// # Arguments
    /// * `tokens` - Vector of token addresses
    ///
    /// # Returns
    /// Vector of timestamped prices
    pub fn get_token_prices(
        env: Env,
        tokens: Vec<Address>,
    ) -> Result<Vec<TimestampedPrice>, FeeQuoterError> {
        Self::require_initialized(&env)?;

        let token_prices: Map<Address, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&TOKEN_PRC)
            .unwrap_or(Map::new(&env));

        let mut result: Vec<TimestampedPrice> = Vec::new(&env);

        for token in tokens.iter() {
            let price = token_prices.get(token).unwrap_or(TimestampedPrice {
                value: 0,
                timestamp: 0,
            });
            result.push_back(price);
        }

        Ok(result)
    }

    /// Get the validated price for a token (reverts if not set).
    ///
    /// # Arguments
    /// * `token` - Token address
    ///
    /// # Returns
    /// Price value in USD with 18 decimals
    ///
    /// # Errors
    /// * `TokenNotSupported` - If token price is not set
    pub fn get_validated_token_price(env: Env, token: Address) -> Result<u128, FeeQuoterError> {
        Self::require_initialized(&env)?;

        let token_prices: Map<Address, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&TOKEN_PRC)
            .unwrap_or(Map::new(&env));

        let price = token_prices
            .get(token)
            .ok_or(FeeQuoterError::TokenNotSupported)?;

        // Price must be set at least once
        if price.timestamp == 0 || price.value == 0 {
            return Err(FeeQuoterError::TokenNotSupported);
        }

        Ok(price.value)
    }

    /// Get the gas price for a destination chain.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - Destination chain selector
    ///
    /// # Returns
    /// Timestamped gas price
    pub fn get_dest_chain_gas_price(
        env: Env,
        dest_chain_selector: u64,
    ) -> Result<TimestampedPrice, FeeQuoterError> {
        Self::require_initialized(&env)?;

        let gas_prices: Map<u64, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&GAS_PRC)
            .unwrap_or(Map::new(&env));

        Ok(gas_prices
            .get(dest_chain_selector)
            .unwrap_or(TimestampedPrice {
                value: 0,
                timestamp: 0,
            }))
    }

    // ========================================
    // Fee Token Functions
    // ========================================

    /// Get the list of fee tokens.
    pub fn get_fee_tokens(env: Env) -> Result<Vec<Address>, FeeQuoterError> {
        Self::require_initialized(&env)?;

        Ok(env
            .storage()
            .persistent()
            .get(&FEE_TKNS)
            .unwrap_or(Vec::new(&env)))
    }

    /// Remove fee tokens (owner only).
    /// This also clears their prices.
    ///
    /// # Arguments
    /// * `tokens` - Tokens to remove from fee tokens
    pub fn remove_fee_tokens(env: Env, tokens: Vec<Address>) -> Result<(), FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        let mut fee_tokens: Vec<Address> = env
            .storage()
            .persistent()
            .get(&FEE_TKNS)
            .unwrap_or(Vec::new(&env));

        let mut token_prices: Map<Address, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&TOKEN_PRC)
            .unwrap_or(Map::new(&env));

        for token in tokens.iter() {
            // Remove from fee tokens
            let mut new_fee_tokens: Vec<Address> = Vec::new(&env);
            for ft in fee_tokens.iter() {
                if ft != token {
                    new_fee_tokens.push_back(ft);
                }
            }
            fee_tokens = new_fee_tokens;

            // Remove price
            token_prices.remove(token.clone());

            FeeTokenRemovedEvent {
                fee_token: token.clone(),
            }
            .publish(&env);
        }

        env.storage().persistent().set(&FEE_TKNS, &fee_tokens);
        env.storage().persistent().set(&TOKEN_PRC, &token_prices);

        Ok(())
    }

    // ========================================
    // Price Update Functions
    // ========================================

    /// Update token and gas prices. Only callable by authorized callers.
    ///
    /// # Arguments
    /// * `price_updates` - Token and gas price updates
    pub fn update_prices(env: Env, price_updates: PriceUpdates) -> Result<(), FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::require_authorized_caller(&env)?;

        let timestamp = env.ledger().timestamp();

        // Update token prices
        let mut token_prices: Map<Address, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&TOKEN_PRC)
            .unwrap_or(Map::new(&env));

        let mut fee_tokens: Vec<Address> = env
            .storage()
            .persistent()
            .get(&FEE_TKNS)
            .unwrap_or(Vec::new(&env));

        for update in price_updates.token_price_updates.iter() {
            let price = TimestampedPrice {
                value: update.usd_per_token,
                timestamp,
            };
            token_prices.set(update.token.clone(), price);

            // Add to fee tokens if not already present
            let mut found = false;
            for ft in fee_tokens.iter() {
                if ft == update.token {
                    found = true;
                    break;
                }
            }
            if !found {
                fee_tokens.push_back(update.token.clone());
                FeeTokenAddedEvent {
                    fee_token: update.token.clone(),
                }
                .publish(&env);
            }

            UsdPerTokenUpdatedEvent {
                token: update.token.clone(),
                value: update.usd_per_token,
                timestamp,
            }
            .publish(&env);
        }

        env.storage().persistent().set(&TOKEN_PRC, &token_prices);
        env.storage().persistent().set(&FEE_TKNS, &fee_tokens);

        // Update gas prices
        let mut gas_prices: Map<u64, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&GAS_PRC)
            .unwrap_or(Map::new(&env));

        for update in price_updates.gas_price_updates.iter() {
            let price = TimestampedPrice {
                value: update.usd_per_unit_gas,
                timestamp,
            };
            gas_prices.set(update.dest_chain_selector, price);

            UsdPerUnitGasUpdatedEvent {
                dest_chain_selector: update.dest_chain_selector,
                value: update.usd_per_unit_gas,
                timestamp,
            }
            .publish(&env);
        }

        env.storage().persistent().set(&GAS_PRC, &gas_prices);

        Ok(())
    }

    // ========================================
    // Fee Calculation Functions
    // ========================================

    /// Quote gas for execution on a destination chain.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - Destination chain selector
    /// * `non_calldata_gas` - Non-calldata gas to be used
    /// * `calldata_size` - Size of calldata in bytes
    /// * `fee_token` - Fee token address
    ///
    /// # Returns
    /// GasQuoteResult with total gas, cost, and fee token price
    pub fn quote_gas_for_exec(
        env: Env,
        dest_chain_selector: u64,
        non_calldata_gas: u32,
        calldata_size: u32,
        fee_token: Address,
    ) -> Result<GasQuoteResult, FeeQuoterError> {
        Self::require_initialized(&env)?;

        let dest_config = Self::get_dest_chain_config_internal(&env, dest_chain_selector)?;

        if !dest_config.is_enabled {
            return Err(FeeQuoterError::DestinationChainNotEnabled);
        }

        // Calculate total gas
        let total_gas = non_calldata_gas + calldata_size * dest_config.dest_gas_per_payload_byte;

        if total_gas > dest_config.max_per_msg_gas_limit {
            return Err(FeeQuoterError::MessageGasLimitTooHigh);
        }

        if calldata_size > dest_config.max_data_bytes {
            return Err(FeeQuoterError::MessageTooLarge);
        }

        // Get gas price
        let gas_prices: Map<u64, TimestampedPrice> = env
            .storage()
            .persistent()
            .get(&GAS_PRC)
            .unwrap_or(Map::new(&env));

        let gas_price = gas_prices
            .get(dest_chain_selector)
            .ok_or(FeeQuoterError::NoGasPriceAvailable)?;

        if gas_price.timestamp == 0 {
            return Err(FeeQuoterError::NoGasPriceAvailable);
        }

        // Gas cost in USD cents (gas_price is in 1e18 USD, we want cents which is 1e2)
        // So we divide by 1e16 to go from 1e18 to 1e2
        // Round up to ensure we never reach zero fee
        let gas_cost_usd_cents =
            ((total_gas as u128) * gas_price.value + (10_u128.pow(16) - 1)) / 10_u128.pow(16);

        // Get fee token price
        let fee_token_price = Self::get_validated_token_price(env.clone(), fee_token.clone())?;

        // Apply premium/discount based on fee token
        let static_config: StaticConfig = env
            .storage()
            .instance()
            .get(&STATIC_CFG)
            .ok_or(FeeQuoterError::NotInitialized)?;

        let premium_multiplier = if fee_token == static_config.link_token {
            dest_config.link_premium_percent
        } else {
            100 // No discount for non-LINK tokens
        };

        Ok(GasQuoteResult {
            total_gas,
            gas_cost_usd_cents,
            fee_token_price,
            premium_multiplier,
        })
    }

    /// Get token transfer fee components.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - Destination chain selector
    /// * `token` - Token address
    ///
    /// # Returns
    /// TokenTransferFeeResult with fee, gas overhead, and bytes overhead
    pub fn get_token_transfer_fee(
        env: Env,
        dest_chain_selector: u64,
        token: Address,
    ) -> Result<TokenTransferFeeResult, FeeQuoterError> {
        Self::require_initialized(&env)?;

        // Try to get token-specific config
        let token_fees: Vec<(u64, Address, TokenTransferFeeConfig)> = env
            .storage()
            .persistent()
            .get(&TKN_FEES)
            .unwrap_or(Vec::new(&env));

        for (selector, tkn, config) in token_fees.iter() {
            if selector == dest_chain_selector && tkn == token && config.is_enabled {
                return Ok(TokenTransferFeeResult {
                    fee_usd_cents: config.fee_usd_cents,
                    dest_gas_overhead: config.dest_gas_overhead,
                    dest_bytes_overhead: config.dest_bytes_overhead,
                });
            }
        }

        // Fall back to destination chain defaults
        let dest_config = Self::get_dest_chain_config_internal(&env, dest_chain_selector)?;

        Ok(TokenTransferFeeResult {
            fee_usd_cents: dest_config.default_token_fee_usd,
            dest_gas_overhead: dest_config.default_token_dest_gas,
            dest_bytes_overhead: CCIP_LOCK_OR_BURN_V1_RET_BYTES,
        })
    }

    /// Convert a token amount to another token.
    ///
    /// # Arguments
    /// * `from_token` - Source token address
    /// * `from_token_amount` - Amount in source token
    /// * `to_token` - Target token address
    ///
    /// # Returns
    /// Amount in target token
    pub fn convert_token_amount(
        env: Env,
        from_token: Address,
        from_token_amount: i128,
        to_token: Address,
    ) -> Result<i128, FeeQuoterError> {
        Self::require_initialized(&env)?;

        let from_price = Self::get_validated_token_price(env.clone(), from_token)?;
        let to_price = Self::get_validated_token_price(env, to_token)?;

        // (fromTokenAmount * fromTokenPrice) / toTokenPrice
        // To avoid overflow, we use a scaled calculation:
        // result = (amount / scale) * (from_price / to_price) * scale + 
        //          (amount % scale) * (from_price / to_price)
        // Or more simply: divide first, then multiply
        // price_ratio = from_price / to_price (loses some precision)
        // For better precision with large numbers, we scale down first
        let amount = from_token_amount as u128;
        
        // Use a safe approach: divide amount by to_price first, then multiply by from_price
        // This loses less precision when from_price > to_price
        if from_price >= to_price {
            // from_price / to_price won't overflow when multiplied by amount
            let price_ratio = from_price / to_price;
            let remainder = from_price % to_price;
            // result = amount * price_ratio + (amount * remainder) / to_price
            let base = amount.saturating_mul(price_ratio);
            let extra = (amount / to_price).saturating_mul(remainder);
            Ok((base.saturating_add(extra)) as i128)
        } else {
            // to_price > from_price, compute ratio in other direction
            // result = amount * from_price / to_price
            // = (amount / to_price) * from_price + (amount % to_price) * from_price / to_price
            let quotient = amount / to_price;
            let remainder = amount % to_price;
            let base = quotient.saturating_mul(from_price);
            let extra = (remainder.saturating_mul(from_price)) / to_price;
            Ok((base.saturating_add(extra)) as i128)
        }
    }

    // ========================================
    // Destination Chain Config Functions
    // ========================================

    /// Get configuration for a destination chain.
    pub fn get_dest_chain_config(
        env: Env,
        dest_chain_selector: u64,
    ) -> Result<DestChainConfig, FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::get_dest_chain_config_internal(&env, dest_chain_selector)
    }

    /// Get all destination chain configurations.
    pub fn get_all_dest_configs(
        env: Env,
    ) -> Result<(Vec<u64>, Vec<DestChainConfig>), FeeQuoterError> {
        Self::require_initialized(&env)?;

        let (selectors, configs): (Vec<u64>, Vec<DestChainConfig>) = env
            .storage()
            .persistent()
            .get(&DEST_CFG)
            .unwrap_or((Vec::new(&env), Vec::new(&env)));

        Ok((selectors, configs))
    }

    /// Apply destination chain config updates (owner only).
    pub fn apply_dest_chain_configs(
        env: Env,
        config_args: Vec<DestChainConfigArgs>,
    ) -> Result<(), FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        let (mut selectors, mut configs): (Vec<u64>, Vec<DestChainConfig>) = env
            .storage()
            .persistent()
            .get(&DEST_CFG)
            .unwrap_or((Vec::new(&env), Vec::new(&env)));

        for args in config_args.iter() {
            // Validate config
            if args.dest_chain_selector == 0
                || args.config.default_tx_gas_limit == 0
                || args.config.default_tx_gas_limit > args.config.max_per_msg_gas_limit
            {
                return Err(FeeQuoterError::InvalidDestChainConfig);
            }

            // Check if this is a new chain or an update
            let mut found_idx: Option<u32> = None;
            for i in 0..selectors.len() {
                if selectors.get(i).unwrap() == args.dest_chain_selector {
                    found_idx = Some(i);
                    break;
                }
            }

            match found_idx {
                Some(idx) => {
                    // Update existing
                    configs.set(idx, args.config.clone());
                    DestChainConfigUpdatedEvent {
                        dest_chain_selector: args.dest_chain_selector,
                        is_enabled: args.config.is_enabled,
                        max_data_bytes: args.config.max_data_bytes,
                    }
                    .publish(&env);
                }
                None => {
                    // Add new
                    selectors.push_back(args.dest_chain_selector);
                    configs.push_back(args.config.clone());
                    DestChainAddedEvent {
                        dest_chain_selector: args.dest_chain_selector,
                        is_enabled: args.config.is_enabled,
                        max_data_bytes: args.config.max_data_bytes,
                    }
                    .publish(&env);
                }
            }
        }

        env.storage()
            .persistent()
            .set(&DEST_CFG, &(selectors, configs));

        Ok(())
    }

    // ========================================
    // Token Transfer Fee Config Functions
    // ========================================

    /// Get token transfer fee configuration.
    pub fn get_token_fee_config(
        env: Env,
        dest_chain_selector: u64,
        token: Address,
    ) -> Result<TokenTransferFeeConfig, FeeQuoterError> {
        Self::require_initialized(&env)?;

        let token_fees: Vec<(u64, Address, TokenTransferFeeConfig)> = env
            .storage()
            .persistent()
            .get(&TKN_FEES)
            .unwrap_or(Vec::new(&env));

        for (selector, tkn, config) in token_fees.iter() {
            if selector == dest_chain_selector && tkn == token {
                return Ok(config);
            }
        }

        // Return default (not enabled)
        Ok(TokenTransferFeeConfig {
            fee_usd_cents: 0,
            dest_gas_overhead: 0,
            dest_bytes_overhead: 0,
            is_enabled: false,
        })
    }

    /// Apply token transfer fee config updates (owner only).
    pub fn apply_token_fee_configs(
        env: Env,
        config_args: Vec<TokenFeeConfigArgs>,
        remove_args: Vec<TokenFeeConfigRemoveArgs>,
    ) -> Result<(), FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        let mut token_fees: Vec<(u64, Address, TokenTransferFeeConfig)> = env
            .storage()
            .persistent()
            .get(&TKN_FEES)
            .unwrap_or(Vec::new(&env));

        // Apply updates
        for args in config_args.iter() {
            // Validate bytes overhead
            if args.config.dest_bytes_overhead < CCIP_LOCK_OR_BURN_V1_RET_BYTES {
                return Err(FeeQuoterError::InvalidDestBytesOverhead);
            }

            // Find existing or add new
            let mut found_idx: Option<u32> = None;
            for i in 0..token_fees.len() {
                let (selector, tkn, _) = token_fees.get(i).unwrap();
                if selector == args.dest_chain_selector && tkn == args.token {
                    found_idx = Some(i);
                    break;
                }
            }

            match found_idx {
                Some(idx) => {
                    token_fees.set(
                        idx,
                        (
                            args.dest_chain_selector,
                            args.token.clone(),
                            args.config.clone(),
                        ),
                    );
                }
                None => {
                    token_fees.push_back((
                        args.dest_chain_selector,
                        args.token.clone(),
                        args.config.clone(),
                    ));
                }
            }

            TokenFeeConfigUpdatedEvent {
                dest_chain_selector: args.dest_chain_selector,
                token: args.token.clone(),
                fee_usd_cents: args.config.fee_usd_cents,
                dest_gas_overhead: args.config.dest_gas_overhead,
                dest_bytes_overhead: args.config.dest_bytes_overhead,
            }
            .publish(&env);
        }

        // Apply removals
        for args in remove_args.iter() {
            let mut new_token_fees: Vec<(u64, Address, TokenTransferFeeConfig)> = Vec::new(&env);
            let mut removed = false;

            for (selector, tkn, config) in token_fees.iter() {
                if selector == args.dest_chain_selector && tkn == args.token {
                    removed = true;
                } else {
                    new_token_fees.push_back((selector, tkn, config));
                }
            }

            if removed {
                TokenFeeConfigDeletedEvent {
                    dest_chain_selector: args.dest_chain_selector,
                    token: args.token.clone(),
                }
                .publish(&env);
            }

            token_fees = new_token_fees;
        }

        env.storage().persistent().set(&TKN_FEES, &token_fees);

        Ok(())
    }

    // ========================================
    // Static Config Functions
    // ========================================

    /// Get the static configuration.
    pub fn get_static_config(env: Env) -> Result<StaticConfig, FeeQuoterError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&STATIC_CFG)
            .ok_or(FeeQuoterError::NotInitialized)
    }

    // ========================================
    // Authorized Caller Management
    // ========================================

    /// Add an authorized caller (owner only).
    pub fn add_authorized_caller(env: Env, caller: Address) -> Result<(), FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        let mut authorized: Vec<Address> = env
            .storage()
            .instance()
            .get(&AUTH_CALL)
            .unwrap_or(Vec::new(&env));

        // Check if already exists
        for ac in authorized.iter() {
            if ac == caller {
                return Err(FeeQuoterError::AuthorizedCallerAlreadyExists);
            }
        }

        authorized.push_back(caller.clone());
        env.storage().instance().set(&AUTH_CALL, &authorized);

        AuthorizedCallerAddedEvent { caller }.publish(&env);

        Ok(())
    }

    /// Remove an authorized caller (owner only).
    pub fn remove_authorized_caller(env: Env, caller: Address) -> Result<(), FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        let authorized: Vec<Address> = env
            .storage()
            .instance()
            .get(&AUTH_CALL)
            .unwrap_or(Vec::new(&env));

        let mut new_authorized: Vec<Address> = Vec::new(&env);
        let mut found = false;

        for ac in authorized.iter() {
            if ac == caller {
                found = true;
            } else {
                new_authorized.push_back(ac);
            }
        }

        if !found {
            return Err(FeeQuoterError::AuthorizedCallerNotFound);
        }

        env.storage().instance().set(&AUTH_CALL, &new_authorized);

        AuthorizedCallerRemovedEvent { caller }.publish(&env);

        Ok(())
    }

    /// Get all authorized callers.
    pub fn get_authorized_callers(env: Env) -> Result<Vec<Address>, FeeQuoterError> {
        Self::require_initialized(&env)?;
        Ok(env
            .storage()
            .instance()
            .get(&AUTH_CALL)
            .unwrap_or(Vec::new(&env)))
    }

    // ========================================
    // Owner Management
    // ========================================

    /// Transfer ownership to a new address.
    pub fn transfer_ownership(env: Env, new_owner: Address) -> Result<(), FeeQuoterError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        env.storage().instance().set(&OWNER, &new_owner);

        OwnershipTransferredEvent {
            new_owner: new_owner.clone(),
        }
        .publish(&env);

        Ok(())
    }

    /// Get the current owner.
    pub fn owner(env: Env) -> Result<Address, FeeQuoterError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&OWNER)
            .ok_or(FeeQuoterError::NotInitialized)
    }

    // ========================================
    // Internal Helper Functions
    // ========================================

    fn require_initialized(env: &Env) -> Result<(), FeeQuoterError> {
        if !env.storage().instance().has(&INITIALIZED) {
            return Err(FeeQuoterError::NotInitialized);
        }
        Ok(())
    }

    fn require_owner(env: &Env) -> Result<(), FeeQuoterError> {
        let owner: Address = env
            .storage()
            .instance()
            .get(&OWNER)
            .ok_or(FeeQuoterError::NotInitialized)?;
        owner.require_auth();
        Ok(())
    }

    fn require_authorized_caller(env: &Env) -> Result<(), FeeQuoterError> {
        let authorized: Vec<Address> = env
            .storage()
            .instance()
            .get(&AUTH_CALL)
            .unwrap_or(Vec::new(env));

        // Check if any authorized caller has signed
        for caller in authorized.iter() {
            // Try to require auth - if it succeeds, caller is authorized
            // Note: In Soroban, we check if the caller provided auth
            caller.require_auth();
            return Ok(());
        }

        Err(FeeQuoterError::CallerNotAuthorized)
    }

    fn get_dest_chain_config_internal(
        env: &Env,
        dest_chain_selector: u64,
    ) -> Result<DestChainConfig, FeeQuoterError> {
        let (selectors, configs): (Vec<u64>, Vec<DestChainConfig>) = env
            .storage()
            .persistent()
            .get(&DEST_CFG)
            .ok_or(FeeQuoterError::DestinationChainNotEnabled)?;

        for i in 0..selectors.len() {
            if selectors.get(i).unwrap() == dest_chain_selector {
                return configs.get(i).ok_or(FeeQuoterError::DestinationChainNotEnabled);
            }
        }

        Err(FeeQuoterError::DestinationChainNotEnabled)
    }
}

mod test;
