#![no_std]

mod events;
pub mod types;

use soroban_sdk::{
    contract, contractimpl, symbol_short, Address, Bytes, BytesN, Env, Map, Symbol, Vec,
};

use common_guard::ReentrancyGuard;
use common_message::{MessageIdCompute, StellarToAnyMessage};
use common_error::CCIPError as OnRampError;
use events::{
    CCIPMessageSentEvent, ConfigSetEvent, DestChainConfigSetEvent, OwnershipTransferredEvent,
};
use types::{DestChainConfig, DestChainConfigArgs, DynamicConfig, Receipt, StaticConfig};

// ============================================================
// Storage Keys
// ============================================================

const OWNER: Symbol = symbol_short!("OWNER");
const INITIALIZED: Symbol = symbol_short!("INIT");
const STATIC_CONFIG: Symbol = symbol_short!("STATIC");
const DYNAMIC_CONFIG: Symbol = symbol_short!("DYNAMIC");
const DEST_CHAINS: Symbol = symbol_short!("DESTCHNS");

// ============================================================
// Contract
// ============================================================

#[contract]
pub struct OnRampContract;

#[contractimpl]
impl OnRampContract {
    // ========================================
    // Initialization
    // ========================================

    /// Initialize the OnRamp contract with static and dynamic configuration.
    ///
    /// # Arguments
    /// * `owner` - The owner address (typically MCMS, can be the deployer initially)
    /// * `static_config` - Immutable configuration
    /// * `dynamic_config` - Mutable configuration
    ///
    /// # Errors
    /// * `AlreadyInitialized` - If contract is already initialized
    /// * `InvalidConfig` - If configuration is invalid
    pub fn initialize(
        env: Env,
        owner: Address,
        static_config: StaticConfig,
        dynamic_config: DynamicConfig,
    ) -> Result<(), OnRampError> {
        // Check not already initialized
        if env.storage().instance().has(&INITIALIZED) {
            return Err(OnRampError::AlreadyInitialized);
        }

        // Validate static config
        if static_config.chain_selector == 0 || static_config.max_usd_cents_per_message == 0 {
            return Err(OnRampError::InvalidConfig);
        }

        // Validate dynamic config (fee_quoter cannot be zero address equivalent check)
        // Note: In Soroban, we check for valid address by ensuring it's set

        // Store owner
        env.storage().instance().set(&OWNER, &owner);

        // Store static config (immutable after init)
        env.storage().instance().set(&STATIC_CONFIG, &static_config);

        // Store dynamic config
        env.storage()
            .instance()
            .set(&DYNAMIC_CONFIG, &dynamic_config);

        // Initialize empty destination chains map
        let dest_chains: Map<u64, DestChainConfig> = Map::new(&env);
        env.storage().persistent().set(&DEST_CHAINS, &dest_chains);

        // Mark as initialized
        env.storage().instance().set(&INITIALIZED, &true);

        // Emit config set event
        ConfigSetEvent {
            static_config,
            dynamic_config,
        }
        .publish(&env);

        Ok(())
    }

    // ========================================
    // Core Messaging Functions
    // ========================================

    /// Get the fee for sending a message to a destination chain.
    ///
    /// This function calculates the total fee including:
    /// - Verifier fees from all CCVs
    /// - Token pool fees (if applicable)
    /// - Executor fees
    /// - Network protocol fee
    ///
    /// # Arguments
    /// * `dest_chain_selector` - The destination chain identifier
    /// * `message` - The message to be sent
    ///
    /// # Returns
    /// The total fee amount in the fee token's smallest denomination
    ///
    /// # Errors
    /// * `NotInitialized` - If contract is not initialized
    /// * `DestinationChainNotSupported` - If destination chain is not configured
    /// * `CanOnlySendOneTokenPerMessage` - If more than one token is specified
    pub fn get_fee(
        env: Env,
        dest_chain_selector: u64,
        message: StellarToAnyMessage,
    ) -> Result<i128, OnRampError> {
        Self::require_initialized(&env)?;

        // Get destination chain config
        let dest_config = Self::get_dest_chain_config_internal(&env, dest_chain_selector)?;

        // Validate message
        if message.token_amounts.len() > 1 {
            return Err(OnRampError::CanOnlySendOneTokenPerMessage);
        }

        // TODO: Implement full fee calculation logic
        // This involves:
        // 1. Parse extra args with defaults
        // 2. Get CCVs for pool (if token transfer)
        // 3. Merge CCV lists
        // 4. Query each CCV for fees
        // 5. Query executor for fees
        // 6. Query pool for fees (if token transfer)
        // 7. Add network fee
        // 8. Convert all fees to fee token amount

        // Placeholder: Return a minimal fee for now
        // Real implementation will query FeeQuoter and all CCVs
        let _ = dest_config;
        Ok(0)
    }

    /// Forward a message from the Router to be sent cross-chain.
    ///
    /// This is the main entry point for sending CCIP messages. It:
    /// 1. Validates the message and caller
    /// 2. Parses extra args and applies defaults
    /// 3. Merges CCV lists (user, lane-mandated, pool-required)
    /// 4. Calculates and distributes fees
    /// 5. Locks or burns tokens (if applicable)
    /// 6. Encodes the message and computes message ID
    /// 7. Calls each verifier
    /// 8. Emits CCIPMessageSent event
    ///
    /// # Arguments
    /// * `dest_chain_selector` - Destination chain identifier
    /// * `message` - The message to send
    /// * `fee_token_amount` - Amount of fee token provided by router
    /// * `original_sender` - The original initiator of the CCIP request
    ///
    /// # Returns
    /// The unique message ID (32-byte hash)
    ///
    /// # Errors
    /// Various errors for validation failures
    pub fn forward_from_router(
        env: Env,
        dest_chain_selector: u64,
        message: StellarToAnyMessage,
        fee_token_amount: i128,
        original_sender: Address,
    ) -> Result<BytesN<32>, OnRampError> {
        Self::require_initialized(&env)?;
        message.validate()?;

        // Enter reentrancy guard (uses temporary storage)
        ReentrancyGuard::enter(&env)?;

        // Get destination chain config
        let mut dest_config = Self::get_dest_chain_config_internal(&env, dest_chain_selector)?;

        // Verify caller is the router
        dest_config.router.require_auth();

        // Validate original sender
        // Note: In Soroban, we can't check for "zero address" the same way as EVM
        // The router must have explicitly set this

        // Validate message
        // TODO: move to message type impl
        if message.token_amounts.len() > 1 {
            // Exit reentrancy guard before returning
            ReentrancyGuard::exit(&env);
            return Err(OnRampError::CanOnlySendOneTokenPerMessage);
        }

        // TODO: Implement full message processing logic:
        // 1. Check RMN curse status
        // 2. Parse extra args with defaults
        // 3. Get pool CCVs if token transfer
        // 4. Merge CCV lists
        // 5. Build MessageV1
        // 6. Get receipts and calculate fees
        // 7. Validate fee amount
        // 8. Distribute fees
        // 9. Lock or burn tokens
        // 10. Encode message and compute message ID
        // 11. Call each verifier
        // 12. Emit event

        // Increment message number (sequence number for this destination)
        dest_config.message_number += 1;
        let sequence_number = dest_config.message_number;
        Self::set_dest_chain_config(&env, dest_chain_selector, &dest_config);

        // Generate message ID (keccak256(messageBytes))
        let message_id = message.compute_message_id(&env);

        // Emit CCIPMessageSent event
        CCIPMessageSentEvent {
            dest_chain_selector,
            sequence_number,
            sender: original_sender,
            message_id: message_id.clone(),
            fee_token: message.fee_token.clone(),
            token_amount_before_fees: message
                .token_amounts
                .get(0)
                .map(|token_amount| token_amount.amount)
                .unwrap_or(0),
            encoded_message: Bytes::new(&env),
            receipts: Vec::new(&env),
            verifier_blobs: Vec::new(&env),
        }
        .publish(&env);

        // Exit reentrancy guard
        ReentrancyGuard::exit(&env);

        // TODO: Placeholder to use fee_token_amount
        let _ = fee_token_amount;

        // TODO: do we need to keep track of message IDs in storage for idempotency?

        Ok(message_id)
    }

    /// Get the expected next message number for a destination chain.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - The destination chain identifier
    ///
    /// # Returns
    /// The next message number that will be used
    pub fn get_expected_next_message_number(
        env: Env,
        dest_chain_selector: u64,
    ) -> Result<u64, OnRampError> {
        Self::require_initialized(&env)?;
        let dest_config = Self::get_dest_chain_config_internal(&env, dest_chain_selector)?;
        Ok(dest_config.message_number + 1)
    }

    // ========================================
    // Token Pool Functions
    // ========================================

    /// Get the pool address for a specific source token.
    ///
    /// # Arguments
    /// * `source_token` - The token address on this chain
    ///
    /// # Returns
    /// The pool address that handles this token
    pub fn get_pool_by_source_token(
        env: Env,
        source_token: Address,
    ) -> Result<Address, OnRampError> {
        Self::require_initialized(&env)?;

        let static_config: StaticConfig = env
            .storage()
            .instance()
            .get(&STATIC_CONFIG)
            .ok_or(OnRampError::NotInitialized)?;

        // TODO: Call TokenAdminRegistry.getPool(sourceToken)
        // For now, return placeholder
        let _ = static_config.token_admin_registry;
        let _ = source_token;

        Err(OnRampError::UnsupportedToken)
    }

    // ========================================
    // Configuration Functions
    // ========================================

    /// Get the static configuration.
    pub fn get_static_config(env: Env) -> Result<StaticConfig, OnRampError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&STATIC_CONFIG)
            .ok_or(OnRampError::NotInitialized)
    }

    /// Get the dynamic configuration.
    pub fn get_dynamic_config(env: Env) -> Result<DynamicConfig, OnRampError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&DYNAMIC_CONFIG)
            .ok_or(OnRampError::NotInitialized)
    }

    /// Set the dynamic configuration. Only callable by owner.
    ///
    /// # Arguments
    /// * `dynamic_config` - New dynamic configuration
    ///
    /// # Errors
    /// * `Unauthorized` - If caller is not owner
    /// * `InvalidConfig` - If configuration is invalid
    pub fn set_dynamic_config(env: Env, dynamic_config: DynamicConfig) -> Result<(), OnRampError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        env.storage()
            .instance()
            .set(&DYNAMIC_CONFIG, &dynamic_config);

        // Emit event
        let static_config: StaticConfig = env
            .storage()
            .instance()
            .get(&STATIC_CONFIG)
            .ok_or(OnRampError::NotInitialized)?;

        ConfigSetEvent {
            static_config,
            dynamic_config,
        }
        .publish(&env);

        Ok(())
    }

    /// Get configuration for a specific destination chain.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - The destination chain identifier
    pub fn get_dest_chain_config(
        env: Env,
        dest_chain_selector: u64,
    ) -> Result<DestChainConfig, OnRampError> {
        Self::require_initialized(&env)?;
        Self::get_dest_chain_config_internal(&env, dest_chain_selector)
    }

    /// Get all destination chain configurations.
    ///
    /// # Returns
    /// Tuple of (chain selectors, configurations)
    pub fn get_all_dest_chain_configs(
        env: Env,
    ) -> Result<(Vec<u64>, Vec<DestChainConfig>), OnRampError> {
        Self::require_initialized(&env)?;

        let dest_chains: Map<u64, DestChainConfig> = env
            .storage()
            .persistent()
            .get(&DEST_CHAINS)
            .unwrap_or(Map::new(&env));

        let mut selectors: Vec<u64> = Vec::new(&env);
        let mut configs: Vec<DestChainConfig> = Vec::new(&env);

        for (selector, config) in dest_chains.iter() {
            selectors.push_back(selector);
            configs.push_back(config);
        }

        Ok((selectors, configs))
    }

    /// Apply destination chain configuration updates. Only callable by owner.
    ///
    /// # Arguments
    /// * `dest_chain_config_args` - Array of destination chain configurations to apply
    pub fn apply_dest_chain_config_updates(
        env: Env,
        dest_chain_config_args: Vec<DestChainConfigArgs>,
    ) -> Result<(), OnRampError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        let static_config: StaticConfig = env
            .storage()
            .instance()
            .get(&STATIC_CONFIG)
            .ok_or(OnRampError::NotInitialized)?;

        let mut dest_chains: Map<u64, DestChainConfig> = env
            .storage()
            .persistent()
            .get(&DEST_CHAINS)
            .unwrap_or(Map::new(&env));

        for args in dest_chain_config_args.iter() {
            // Validate config
            if args.dest_chain_selector == 0
                || args.dest_chain_selector == static_config.chain_selector
                || args.address_bytes_length == 0
                || args.base_execution_gas_cost == 0
            {
                return Err(OnRampError::InvalidConfig);
            }

            // Validate offRamp length matches address_bytes_length
            if args.off_ramp.len() as u32 != args.address_bytes_length {
                return Err(OnRampError::InvalidDestChainAddress);
            }

            // Ensure at least one default or mandated CCV exists
            if args.default_ccvs.is_empty() && args.lane_mandated_ccvs.is_empty() {
                return Err(OnRampError::InvalidConfig);
            }

            // Get existing config or create new one
            let existing_message_number = dest_chains
                .get(args.dest_chain_selector)
                .map(|c| c.message_number)
                .unwrap_or(0);

            let new_config = DestChainConfig {
                router: args.router.clone(),
                message_number: existing_message_number,
                address_bytes_length: args.address_bytes_length,
                token_receiver_allowed: args.token_receiver_allowed,
                message_network_fee_usd_cents: args.message_network_fee_usd_cents,
                token_network_fee_usd_cents: args.token_network_fee_usd_cents,
                base_execution_gas_cost: args.base_execution_gas_cost,
                default_executor: args.default_executor.clone(),
                lane_mandated_ccvs: args.lane_mandated_ccvs.clone(),
                default_ccvs: args.default_ccvs.clone(),
                off_ramp: args.off_ramp.clone(),
            };

            dest_chains.set(args.dest_chain_selector, new_config.clone());

            // Emit event
            DestChainConfigSetEvent {
                dest_chain_selector: args.dest_chain_selector,
                message_number: existing_message_number,
                config: new_config,
            }
            .publish(&env);
        }

        env.storage().persistent().set(&DEST_CHAINS, &dest_chains);

        Ok(())
    }

    // ========================================
    // Fee Functions
    // ========================================

    /// Withdraw accumulated fee tokens to the fee aggregator.
    /// This function is permissionless as it only sends to the trusted fee aggregator.
    ///
    /// # Arguments
    /// * `fee_tokens` - List of fee token addresses to withdraw
    pub fn withdraw_fee_tokens(env: Env, fee_tokens: Vec<Address>) -> Result<(), OnRampError> {
        Self::require_initialized(&env)?;

        let dynamic_config: DynamicConfig = env
            .storage()
            .instance()
            .get(&DYNAMIC_CONFIG)
            .ok_or(OnRampError::NotInitialized)?;

        // TODO: Implement fee token withdrawal
        // For each token:
        // 1. Get contract's balance
        // 2. Transfer to fee_aggregator
        let _ = fee_tokens;
        let _ = dynamic_config.fee_aggregator;

        Ok(())
    }

    // ========================================
    // Owner Management
    // ========================================

    /// Transfer ownership to a new address. Only callable by current owner.
    pub fn transfer_ownership(env: Env, new_owner: Address) -> Result<(), OnRampError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        env.storage().instance().set(&OWNER, &new_owner);

        OwnershipTransferredEvent {
            new_owner: new_owner.clone(),
        }
        .publish(&env);

        Ok(())
    }

    /// Get the current owner address.
    pub fn owner(env: Env) -> Result<Address, OnRampError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&OWNER)
            .ok_or(OnRampError::NotInitialized)
    }

    // ========================================
    // Internal Helper Functions
    // ========================================

    fn require_initialized(env: &Env) -> Result<(), OnRampError> {
        if !env.storage().instance().has(&INITIALIZED) {
            return Err(OnRampError::NotInitialized);
        }
        Ok(())
    }

    fn require_owner(env: &Env) -> Result<(), OnRampError> {
        let owner: Address = env
            .storage()
            .instance()
            .get(&OWNER)
            .ok_or(OnRampError::NotInitialized)?;
        owner.require_auth();
        Ok(())
    }

    fn get_dest_chain_config_internal(
        env: &Env,
        dest_chain_selector: u64,
    ) -> Result<DestChainConfig, OnRampError> {
        let dest_chains: Map<u64, DestChainConfig> =
            env.storage()
                .persistent()
                .get(&DEST_CHAINS)
                .ok_or(OnRampError::DestinationChainNotSupported)?;

        dest_chains
            .get(dest_chain_selector)
            .ok_or(OnRampError::DestinationChainNotSupported)
    }

    fn set_dest_chain_config(env: &Env, dest_chain_selector: u64, config: &DestChainConfig) {
        let mut dest_chains: Map<u64, DestChainConfig> = env
            .storage()
            .persistent()
            .get(&DEST_CHAINS)
            .unwrap_or(Map::new(env));

        dest_chains.set(dest_chain_selector, config.clone());
        env.storage().persistent().set(&DEST_CHAINS, &dest_chains);
    }
}

mod test;
