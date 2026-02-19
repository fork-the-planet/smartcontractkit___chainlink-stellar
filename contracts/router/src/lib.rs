#![no_std]

mod events;
mod types;

use soroban_sdk::{contract, contractimpl, symbol_short, Address, BytesN, Env, Map, Symbol, Vec};

use common_authorization::Ownable;
use common_error::CCIPError as RouterError;
use common_interfaces::onramp::OnRampClient;
use common_interfaces::rmn_proxy::RmnProxyClient;
use common_message::StellarToAnyMessage;
use events::{CCIPSendRequestedEvent, OffRampAddedEvent, OffRampRemovedEvent, OnRampSetEvent};
use types::{OffRampEntry, OnRampEntry, RouterConfig};
use common_guard::initializable::Initializable;

// ============================================================
// Storage Keys
// ============================================================

const INITIALIZED: Symbol = symbol_short!("INIT");
const OWNER: Symbol = symbol_short!("OWNER");
const PENDING_OWNER: Symbol = symbol_short!("PNDGOWNR");
const CONFIG: Symbol = symbol_short!("CONFIG");
const ONRAMPS: Symbol = symbol_short!("ONRAMPS");
const OFFRAMPS: Symbol = symbol_short!("OFFRAMPS");

// ============================================================
// Contract
// ============================================================

#[contract]
pub struct RouterContract;

#[contractimpl]
impl Ownable for RouterContract {
    const OWNER: Symbol = OWNER;
    const PENDING_OWNER: Symbol = PENDING_OWNER;
}

#[contractimpl]
impl Initializable for RouterContract {
    const INITIALIZED: Symbol = INITIALIZED;
}

#[contractimpl]
impl RouterContract {
    // ========================================
    // Initialization
    // ========================================

    /// Initialize the Router contract.
    ///
    /// # Arguments
    /// * `owner` - The owner address (typically MCMS)
    /// * `rmn_proxy` - The RMN proxy contract address for curse checking
    ///
    /// # Errors
    /// * `AlreadyInitialized` - If contract is already initialized
    pub fn initialize(env: Env, owner: Address, rmn_proxy: Address) -> Result<(), RouterError> {
        <Self as Initializable>::require_not_initialized(&env)?;

        // Initialize owner via shared authorization lib (two-step ownership)
        <Self as Ownable>::init(&env, &owner);
        <Self as Initializable>::init(&env)?;

        // Store config
        let config = RouterConfig { rmn_proxy };
        env.storage().instance().set(&CONFIG, &config);

        // Initialize empty OnRamps map
        let onramps: Map<u64, Address> = Map::new(&env);
        env.storage().persistent().set(&ONRAMPS, &onramps);

        // Initialize empty OffRamps map (source_chain_selector -> Vec<Address>)
        let offramps: Map<u64, Vec<Address>> = Map::new(&env);
        env.storage().persistent().set(&OFFRAMPS, &offramps);

        // Mark as initialized
        env.storage().instance().set(&INITIALIZED, &true);

        Ok(())
    }

    // ========================================
    // Core Messaging Functions
    // ========================================

    /// Get the fee for sending a message to a destination chain.
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
    /// * `UnsupportedDestinationChain` - If destination chain is not configured
    pub fn get_fee(
        env: Env,
        dest_chain_selector: u64,
        message: StellarToAnyMessage,
    ) -> Result<i128, RouterError> {
        <Self as Initializable>::require_initialized(&env)?;

        // Get OnRamp for destination
        let onramp = Self::get_onramp_internal(&env, dest_chain_selector)?;

        // Cross-contract call to OnRamp to get fee
        let onramp_client = OnRampClient::new(&env, &onramp);
        let fee = onramp_client.get_fee(&dest_chain_selector, &message);

        Ok(fee)
    }

    /// Send a cross-chain message via CCIP.
    ///
    /// This is the main entry point for sending CCIP messages. It:
    /// 1. Verifies the sender's authorization
    /// 2. Checks RMN curse status
    /// 3. Looks up the OnRamp for the destination chain
    /// 4. Gets fee quote from OnRamp
    /// 5. Validates the fee token amount
    /// 6. Calls forwardFromRouter on OnRamp
    /// 7. Returns the message ID
    ///
    /// # Arguments
    /// * `sender` - The original sender of the message (must authorize)
    /// * `dest_chain_selector` - Destination chain identifier
    /// * `message` - The message to send (includes receiver, data, tokens, fee_token, extra_args)
    /// * `fee_token_amount` - Amount of fee tokens to pay
    ///
    /// # Returns
    /// The unique message ID (32-byte hash)
    ///
    /// # Errors
    /// * `NotInitialized` - If contract is not initialized
    /// * `UnsupportedDestinationChain` - If destination is not configured
    /// * `BadRMNSignal` - If the network is cursed
    /// * `InsufficientFeeTokenAmount` - If fee provided is less than required
    /// * `OnRampError` - If the OnRamp returns an error
    pub fn ccip_send(
        env: Env,
        sender: Address,
        dest_chain_selector: u64,
        message: StellarToAnyMessage,
        fee_token_amount: i128,
    ) -> Result<BytesN<32>, RouterError> {
        // Verify the sender's identity (Soroban equivalent of EVM's msg.sender)
        sender.require_auth();

        <Self as Initializable>::require_initialized(&env)?;
        Self::require_not_cursed(&env)?;

        // Get OnRamp for destination
        let onramp = Self::get_onramp_internal(&env, dest_chain_selector)?;

        // Get fee from OnRamp and validate fee_token_amount >= required_fee
        let onramp_client = OnRampClient::new(&env, &onramp);
        let required_fee = onramp_client.get_fee(&dest_chain_selector, &message);
        if fee_token_amount < required_fee {
            return Err(RouterError::InsufficientFeeTokenAmount);
        }

        // TODO: Transfer fee tokens from sender to OnRamp.
        // This requires the sender to have authorized the token transfer as part of
        // the transaction tree (sub-invocations). Will be implemented when token pool
        // and fee quoter integration is complete.

        // Call OnRamp.forward_from_router to process the message
        let message_id = onramp_client.forward_from_router(
            &dest_chain_selector,
            &message,
            &fee_token_amount,
            &sender,
        );

        // Emit Router-level event for tracking
        CCIPSendRequestedEvent {
            message_id: message_id.clone(),
            dest_chain_selector,
            sender,
        }
        .publish(&env);

        Ok(message_id)
    }

    // ========================================
    // Query Functions
    // ========================================

    /// Check if a destination chain is supported.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - The destination chain identifier
    ///
    /// # Returns
    /// True if an OnRamp is configured for this destination
    pub fn is_chain_supported(env: Env, dest_chain_selector: u64) -> Result<bool, RouterError> {
        <Self as Initializable>::require_initialized(&env)?;

        let onramps: Map<u64, Address> = env
            .storage()
            .persistent()
            .get(&ONRAMPS)
            .unwrap_or(Map::new(&env));

        Ok(onramps.contains_key(dest_chain_selector))
    }

    /// Get the OnRamp address for a destination chain.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - The destination chain identifier
    ///
    /// # Returns
    /// The OnRamp contract address
    pub fn get_onramp(env: Env, dest_chain_selector: u64) -> Result<Address, RouterError> {
        <Self as Initializable>::require_initialized(&env)?;
        Self::get_onramp_internal(&env, dest_chain_selector)
    }

    /// Check if an address is a valid OffRamp for a source chain.
    ///
    /// # Arguments
    /// * `source_chain_selector` - The source chain identifier
    /// * `offramp` - The address to check
    ///
    /// # Returns
    /// True if the address is a configured OffRamp for this source chain
    pub fn is_offramp(
        env: Env,
        source_chain_selector: u64,
        offramp: Address,
    ) -> Result<bool, RouterError> {
        <Self as Initializable>::require_initialized(&env)?;

        let offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(&env));

        if let Some(chain_offramps) = offramps.get(source_chain_selector) {
            // Check if offramp is in the list
            for i in 0..chain_offramps.len() {
                if chain_offramps.get(i) == Some(offramp.clone()) {
                    return Ok(true);
                }
            }
        }

        Ok(false)
    }

    /// Get all configured OffRamps.
    ///
    /// # Returns
    /// Vector of OffRampEntry structs
    pub fn get_offramps(env: Env) -> Result<Vec<OffRampEntry>, RouterError> {
        <Self as Initializable>::require_initialized(&env)?;

        let offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(&env));

        let mut result: Vec<OffRampEntry> = Vec::new(&env);

        for (source_chain_selector, chain_offramps) in offramps.iter() {
            for i in 0..chain_offramps.len() {
                if let Some(offramp) = chain_offramps.get(i) {
                    result.push_back(OffRampEntry {
                        source_chain_selector,
                        offramp,
                    });
                }
            }
        }

        Ok(result)
    }

    /// Get all configured OnRamps.
    ///
    /// # Returns
    /// Vector of OnRampEntry structs
    pub fn get_onramps(env: Env) -> Result<Vec<OnRampEntry>, RouterError> {
        <Self as Initializable>::require_initialized(&env)?;

        let onramps: Map<u64, Address> = env
            .storage()
            .persistent()
            .get(&ONRAMPS)
            .unwrap_or(Map::new(&env));

        let mut result: Vec<OnRampEntry> = Vec::new(&env);

        for (dest_chain_selector, onramp) in onramps.iter() {
            result.push_back(OnRampEntry {
                dest_chain_selector,
                onramp,
            });
        }

        Ok(result)
    }

    /// Get the Router configuration.
    pub fn get_config(env: Env) -> Result<RouterConfig, RouterError> {
        <Self as Initializable>::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&CONFIG)
            .ok_or(RouterError::NotInitialized)
    }

    // ========================================
    // Admin Functions
    // ========================================

    /// Set the OnRamp for a destination chain. Only callable by owner.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - The destination chain identifier
    /// * `onramp` - The OnRamp contract address
    pub fn set_onramp(
        env: Env,
        dest_chain_selector: u64,
        onramp: Address,
    ) -> Result<(), RouterError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

        let mut onramps: Map<u64, Address> = env
            .storage()
            .persistent()
            .get(&ONRAMPS)
            .unwrap_or(Map::new(&env));

        onramps.set(dest_chain_selector, onramp.clone());
        env.storage().persistent().set(&ONRAMPS, &onramps);

        // Emit event
        OnRampSetEvent {
            dest_chain_selector,
            onramp,
        }
        .publish(&env);

        Ok(())
    }

    /// Add an OffRamp for a source chain. Only callable by owner.
    ///
    /// # Arguments
    /// * `source_chain_selector` - The source chain identifier
    /// * `offramp` - The OffRamp contract address
    pub fn add_offramp(
        env: Env,
        source_chain_selector: u64,
        offramp: Address,
    ) -> Result<(), RouterError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

        let mut offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(&env));

        let mut chain_offramps = offramps
            .get(source_chain_selector)
            .unwrap_or(Vec::new(&env));

        // Check if already exists
        for i in 0..chain_offramps.len() {
            if chain_offramps.get(i) == Some(offramp.clone()) {
                return Err(RouterError::OffRampAlreadyExists);
            }
        }

        // Add the new OffRamp
        chain_offramps.push_back(offramp.clone());
        offramps.set(source_chain_selector, chain_offramps);
        env.storage().persistent().set(&OFFRAMPS, &offramps);

        // Emit event
        OffRampAddedEvent {
            source_chain_selector,
            offramp,
        }
        .publish(&env);

        Ok(())
    }

    /// Remove an OffRamp for a source chain. Only callable by owner.
    ///
    /// # Arguments
    /// * `source_chain_selector` - The source chain identifier
    /// * `offramp` - The OffRamp contract address to remove
    pub fn remove_offramp(
        env: Env,
        source_chain_selector: u64,
        offramp: Address,
    ) -> Result<(), RouterError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

        let mut offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(&env));

        let chain_offramps = offramps
            .get(source_chain_selector)
            .ok_or(RouterError::OffRampMismatch)?;

        // Find and remove the OffRamp
        let mut found = false;
        let mut new_chain_offramps: Vec<Address> = Vec::new(&env);

        for i in 0..chain_offramps.len() {
            if let Some(addr) = chain_offramps.get(i) {
                if addr == offramp {
                    found = true;
                    // Skip this one (don't add to new list)
                } else {
                    new_chain_offramps.push_back(addr);
                }
            }
        }

        if !found {
            return Err(RouterError::OffRampMismatch);
        }

        // Update storage
        if new_chain_offramps.is_empty() {
            offramps.remove(source_chain_selector);
        } else {
            offramps.set(source_chain_selector, new_chain_offramps);
        }
        env.storage().persistent().set(&OFFRAMPS, &offramps);

        // Emit event
        OffRampRemovedEvent {
            source_chain_selector,
            offramp,
        }
        .publish(&env);

        Ok(())
    }

    /// Apply batch ramp updates. Only callable by owner.
    /// This allows setting multiple OnRamps and adding/removing multiple OffRamps atomically.
    ///
    /// # Arguments
    /// * `onramp_updates` - OnRamps to set (can include zero address to disable)
    /// * `offramp_removes` - OffRamps to remove
    /// * `offramp_adds` - OffRamps to add
    pub fn apply_ramp_updates(
        env: Env,
        onramp_updates: Vec<OnRampEntry>,
        offramp_removes: Vec<OffRampEntry>,
        offramp_adds: Vec<OffRampEntry>,
    ) -> Result<(), RouterError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

        // Apply OnRamp updates
        let mut onramps: Map<u64, Address> = env
            .storage()
            .persistent()
            .get(&ONRAMPS)
            .unwrap_or(Map::new(&env));

        for entry in onramp_updates.iter() {
            onramps.set(entry.dest_chain_selector, entry.onramp.clone());
            OnRampSetEvent {
                dest_chain_selector: entry.dest_chain_selector,
                onramp: entry.onramp.clone(),
            }
            .publish(&env);
        }

        env.storage().persistent().set(&ONRAMPS, &onramps);

        // Apply OffRamp removes
        let mut offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(&env));

        for entry in offramp_removes.iter() {
            let chain_offramps = offramps
                .get(entry.source_chain_selector)
                .ok_or(RouterError::OffRampMismatch)?;

            let mut found = false;
            let mut new_chain_offramps: Vec<Address> = Vec::new(&env);

            for i in 0..chain_offramps.len() {
                if let Some(addr) = chain_offramps.get(i) {
                    if addr == entry.offramp {
                        found = true;
                    } else {
                        new_chain_offramps.push_back(addr);
                    }
                }
            }

            if !found {
                return Err(RouterError::OffRampMismatch);
            }

            if new_chain_offramps.is_empty() {
                offramps.remove(entry.source_chain_selector);
            } else {
                offramps.set(entry.source_chain_selector, new_chain_offramps);
            }

            OffRampRemovedEvent {
                source_chain_selector: entry.source_chain_selector,
                offramp: entry.offramp.clone(),
            }
            .publish(&env);
        }

        // Apply OffRamp adds
        for entry in offramp_adds.iter() {
            let mut chain_offramps = offramps
                .get(entry.source_chain_selector)
                .unwrap_or(Vec::new(&env));

            // Check for duplicates
            let mut exists = false;
            for i in 0..chain_offramps.len() {
                if chain_offramps.get(i) == Some(entry.offramp.clone()) {
                    exists = true;
                    break;
                }
            }

            if !exists {
                chain_offramps.push_back(entry.offramp.clone());
                offramps.set(entry.source_chain_selector, chain_offramps);

                OffRampAddedEvent {
                    source_chain_selector: entry.source_chain_selector,
                    offramp: entry.offramp.clone(),
                }
                .publish(&env);
            }
        }

        env.storage().persistent().set(&OFFRAMPS, &offramps);

        Ok(())
    }

    // ========================================
    // Internal Helper Functions
    // ========================================

    fn require_not_cursed(env: &Env) -> Result<(), RouterError> {
        let config: RouterConfig = env
            .storage()
            .instance()
            .get(&CONFIG)
            .ok_or(RouterError::NotInitialized)?;

        // Cross-contract call to RMN Proxy to check curse status
        let rmn_proxy_client = RmnProxyClient::new(env, &config.rmn_proxy);
        let is_cursed = rmn_proxy_client.is_cursed();

        if is_cursed {
            return Err(RouterError::BadRMNSignal);
        }

        Ok(())
    }

    fn get_onramp_internal(env: &Env, dest_chain_selector: u64) -> Result<Address, RouterError> {
        let onramps: Map<u64, Address> = env
            .storage()
            .persistent()
            .get(&ONRAMPS)
            .ok_or(RouterError::UnsupportedDestinationChain)?;

        onramps
            .get(dest_chain_selector)
            .ok_or(RouterError::UnsupportedDestinationChain)
    }
}

mod test;
