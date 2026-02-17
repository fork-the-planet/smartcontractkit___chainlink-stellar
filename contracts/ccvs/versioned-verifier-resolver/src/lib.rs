#![no_std]

//! # Versioned Verifier Resolver
//!
//! A Soroban contract that maps verifier versions (4-byte prefixes) to inbound verifier
//! contract addresses, and destination chain selectors to outbound verifier contract addresses.
//!
//! This is the Stellar equivalent of the Solidity `VersionedVerifierResolver` contract.
//!
//! ## Storage Layout
//!
//! - `VERSION_TO_INBOUND` (Map<BytesN<4>, Address>): Maps a 4-byte version prefix to an inbound
//!   verifier contract address.
//! - `DEST_CHAIN_TO_OUTBOUND` (Map<u64, Address>): Maps a destination chain selector to an
//!   outbound verifier contract address.
//! - `SUPPORTED_VERSIONS` (Vec<BytesN<4>>): Set of registered verifier version prefixes.
//! - `SUPPORTED_DEST_CHAINS` (Vec<u64>): Set of supported destination chain selectors.
//! - `FEE_AGGREGATOR` (Address): The fee aggregator address.

pub mod error;
pub mod events;

use soroban_sdk::{
    contract, contractimpl, contracttype, symbol_short, Address, Bytes, BytesN, Env, Map, Symbol,
    Vec,
};

use common_authorization::Ownable;
use error::VerifierResolverError;
use events::{
    FeeAggregatorSetEvent, InboundImplRemovedEvent, InboundImplSetEvent,
    OutboundImplRemovedEvent, OutboundImplSetEvent,
};

// ============================================================
// Storage Keys
// ============================================================

const INITIALIZED: Symbol = symbol_short!("INIT");
const VER_INBOUND: Symbol = symbol_short!("VERINB");
const DEST_OUTBND: Symbol = symbol_short!("DESTOUT");
const SUP_VERS: Symbol = symbol_short!("SUPVERS");
const SUP_DESTS: Symbol = symbol_short!("SUPDEST");
const FEE_AGG: Symbol = symbol_short!("FEEAGG");

// ============================================================
// Types
// ============================================================

/// Arguments for updating an inbound implementation.
/// Maps a 4-byte verifier version prefix to a verifier contract address.
///
/// - If `verifier` is `None`, the implementation for this version is removed.
/// - If `verifier` is `Some(address)`, the implementation is set/updated.
///
/// This mirrors the EVM pattern where `address(0)` signals deletion.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InboundImplementationUpdate {
    /// 4-byte verifier version prefix (equivalent to Solidity bytes4)
    pub version: BytesN<4>,
    /// Address of the verifier contract, or None to remove
    pub verifier: Option<Address>,
}

/// Return type for querying inbound implementations (always has an address).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InboundImplementationArgs {
    /// 4-byte verifier version prefix
    pub version: BytesN<4>,
    /// Address of the verifier contract
    pub verifier: Address,
}

/// Arguments for updating an outbound implementation.
/// Maps a destination chain selector to a verifier contract address.
///
/// - If `verifier` is `None`, the implementation for this chain is removed.
/// - If `verifier` is `Some(address)`, the implementation is set/updated.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OutboundImplementationUpdate {
    /// Destination chain selector
    pub dest_chain_selector: u64,
    /// Address of the verifier contract, or None to remove
    pub verifier: Option<Address>,
}

/// Return type for querying outbound implementations (always has an address).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OutboundImplementationArgs {
    /// Destination chain selector
    pub dest_chain_selector: u64,
    /// Address of the verifier contract
    pub verifier: Address,
}

// ============================================================
// Contract
// ============================================================

#[contract]
pub struct VersionedVerifierResolverContract;

#[contractimpl]
impl VersionedVerifierResolverContract {
    // ========================================
    // Initialization
    // ========================================

    /// Initialize the VersionedVerifierResolver contract.
    ///
    /// # Arguments
    /// * `owner` - The owner address
    /// * `fee_aggregator` - The fee aggregator address
    ///
    /// # Errors
    /// * `AlreadyInitialized` - If contract is already initialized
    pub fn initialize(
        env: Env,
        owner: Address,
        fee_aggregator: Address,
    ) -> Result<(), VerifierResolverError> {
        if env.storage().instance().has(&INITIALIZED) {
            return Err(VerifierResolverError::AlreadyInitialized);
        }

        Ownable::init(&env, &owner);

        // Initialize empty mappings
        let inbound_map: Map<BytesN<4>, Address> = Map::new(&env);
        env.storage().instance().set(&VER_INBOUND, &inbound_map);

        let outbound_map: Map<u64, Address> = Map::new(&env);
        env.storage().instance().set(&DEST_OUTBND, &outbound_map);

        let supported_versions: Vec<BytesN<4>> = Vec::new(&env);
        env.storage().instance().set(&SUP_VERS, &supported_versions);

        let supported_dests: Vec<u64> = Vec::new(&env);
        env.storage().instance().set(&SUP_DESTS, &supported_dests);

        env.storage().instance().set(&FEE_AGG, &fee_aggregator);

        env.storage().instance().set(&INITIALIZED, &true);

        Ok(())
    }

    // ========================================
    // View Functions
    // ========================================

    /// Returns the inbound verifier implementation for the given verifier results.
    ///
    /// The first 4 bytes of `verifier_results` are used as the version key
    /// to look up the corresponding verifier contract address.
    ///
    /// # Arguments
    /// * `verifier_results` - The verifier results bytes (must be at least 4 bytes)
    ///
    /// # Returns
    /// The address of the inbound verifier contract
    ///
    /// # Errors
    /// * `InvalidVerifierResultsLength` - If verifier_results is shorter than 4 bytes
    /// * `InboundImplementationNotFound` - If no implementation is registered for the version
    pub fn get_inbound_implementation(
        env: Env,
        verifier_results: Bytes,
    ) -> Result<Address, VerifierResolverError> {
        Self::require_initialized(&env)?;

        if verifier_results.len() < 4 {
            return Err(VerifierResolverError::InvalidVerifierResultsLength);
        }

        // Extract first 4 bytes as version
        let version = Self::extract_version(&env, &verifier_results);

        let inbound_map: Map<BytesN<4>, Address> = env
            .storage()
            .instance()
            .get(&VER_INBOUND)
            .unwrap_or(Map::new(&env));

        inbound_map
            .get(version)
            .ok_or(VerifierResolverError::InboundImplementationNotFound)
    }

    /// Returns all registered inbound implementations.
    ///
    /// # Returns
    /// A vector of `InboundImplementationArgs` containing all version-to-verifier mappings
    pub fn get_all_inbound_implementations(env: Env) -> Vec<InboundImplementationArgs> {
        let supported_versions: Vec<BytesN<4>> = env
            .storage()
            .instance()
            .get(&SUP_VERS)
            .unwrap_or(Vec::new(&env));

        let inbound_map: Map<BytesN<4>, Address> = env
            .storage()
            .instance()
            .get(&VER_INBOUND)
            .unwrap_or(Map::new(&env));

        let mut result: Vec<InboundImplementationArgs> = Vec::new(&env);

        for version in supported_versions.iter() {
            if let Some(verifier) = inbound_map.get(version.clone()) {
                result.push_back(InboundImplementationArgs {
                    version: version.clone(),
                    verifier,
                });
            }
        }

        result
    }

    /// Returns the outbound verifier implementation for the given destination chain.
    ///
    /// # Arguments
    /// * `dest_chain_selector` - The destination chain selector
    /// * `extra_args` - Additional arguments (reserved for future use, currently ignored)
    ///
    /// # Returns
    /// The address of the outbound verifier contract
    ///
    /// # Errors
    /// * `OutboundImplementationNotFound` - If no implementation is registered for the chain
    pub fn get_outbound_implementation(
        env: Env,
        dest_chain_selector: u64,
        _extra_args: Bytes,
    ) -> Result<Address, VerifierResolverError> {
        Self::require_initialized(&env)?;

        let outbound_map: Map<u64, Address> = env
            .storage()
            .instance()
            .get(&DEST_OUTBND)
            .unwrap_or(Map::new(&env));

        outbound_map
            .get(dest_chain_selector)
            .ok_or(VerifierResolverError::OutboundImplementationNotFound)
    }

    /// Returns all registered outbound implementations.
    ///
    /// # Returns
    /// A vector of `OutboundImplementationArgs` containing all chain-to-verifier mappings
    pub fn get_all_outbound_implementations(env: Env) -> Vec<OutboundImplementationArgs> {
        let supported_dests: Vec<u64> = env
            .storage()
            .instance()
            .get(&SUP_DESTS)
            .unwrap_or(Vec::new(&env));

        let outbound_map: Map<u64, Address> = env
            .storage()
            .instance()
            .get(&DEST_OUTBND)
            .unwrap_or(Map::new(&env));

        let mut result: Vec<OutboundImplementationArgs> = Vec::new(&env);

        for selector in supported_dests.iter() {
            if let Some(verifier) = outbound_map.get(selector) {
                result.push_back(OutboundImplementationArgs {
                    dest_chain_selector: selector,
                    verifier,
                });
            }
        }

        result
    }

    /// Returns the fee aggregator address.
    pub fn get_fee_aggregator(env: Env) -> Result<Address, VerifierResolverError> {
        Self::require_initialized(&env)?;

        env.storage()
            .instance()
            .get(&FEE_AGG)
            .ok_or(VerifierResolverError::NotInitialized)
    }

    /// Returns the current owner address.
    pub fn owner(env: Env) -> Result<Address, VerifierResolverError> {
        Ownable::get_owner(&env).ok_or(VerifierResolverError::NotInitialized)
    }

    // ========================================
    // Admin Functions (Owner Only)
    // ========================================

    /// Apply a batch of inbound implementation updates atomically.
    ///
    /// For each entry in `implementations`:
    /// - If `verifier` is `None`: removes the mapping for that version
    /// - If `verifier` is `Some(address)`: sets/updates the mapping
    ///
    /// This mirrors the EVM `applyInboundImplementationUpdates` function.
    ///
    /// # Arguments
    /// * `implementations` - A vector of updates to apply
    ///
    /// # Errors
    /// * `NotInitialized` - If contract is not initialized
    /// * `Unauthorized` - If caller is not the owner
    /// * `InvalidVersion` - If version is all zeros when setting (not removing)
    pub fn apply_inbound_impl_updates(
        env: Env,
        implementations: Vec<InboundImplementationUpdate>,
    ) -> Result<(), VerifierResolverError> {
        Self::require_initialized(&env)?;
        Ownable::require_owner(&env).map_err(|_| VerifierResolverError::Unauthorized)?;

        let mut inbound_map: Map<BytesN<4>, Address> = env
            .storage()
            .instance()
            .get(&VER_INBOUND)
            .unwrap_or(Map::new(&env));

        let mut supported_versions: Vec<BytesN<4>> = env
            .storage()
            .instance()
            .get(&SUP_VERS)
            .unwrap_or(Vec::new(&env));

        let zero_version = BytesN::from_array(&env, &[0u8; 4]);

        for update in implementations.iter() {
            match update.verifier {
                None => {
                    // Remove: verifier is None (equivalent to address(0) in Solidity)
                    inbound_map.remove(update.version.clone());
                    supported_versions =
                        Self::vec_remove_version(&env, &supported_versions, &update.version);

                    InboundImplRemovedEvent {
                        version: update.version,
                    }
                    .publish(&env);
                }
                Some(verifier) => {
                    // Set/Update: verifier is provided
                    if update.version == zero_version {
                        return Err(VerifierResolverError::InvalidVersion);
                    }

                    // Add to supported versions set if not already present
                    if !Self::vec_contains_version(&supported_versions, &update.version) {
                        supported_versions.push_back(update.version.clone());
                    }

                    inbound_map.set(update.version.clone(), verifier.clone());

                    InboundImplSetEvent {
                        version: update.version,
                        verifier,
                    }
                    .publish(&env);
                }
            }
        }

        env.storage().instance().set(&VER_INBOUND, &inbound_map);
        env.storage().instance().set(&SUP_VERS, &supported_versions);

        Ok(())
    }

    /// Apply a batch of outbound implementation updates atomically.
    ///
    /// For each entry in `implementations`:
    /// - If `verifier` is `None`: removes the mapping for that dest chain
    /// - If `verifier` is `Some(address)`: sets/updates the mapping
    ///
    /// This mirrors the EVM `applyOutboundImplementationUpdates` function.
    ///
    /// # Arguments
    /// * `implementations` - A vector of updates to apply
    ///
    /// # Errors
    /// * `NotInitialized` - If contract is not initialized
    /// * `Unauthorized` - If caller is not the owner
    /// * `InvalidChainSelector` - If dest_chain_selector is 0 when setting (not removing)
    pub fn apply_outbound_impl_updates(
        env: Env,
        implementations: Vec<OutboundImplementationUpdate>,
    ) -> Result<(), VerifierResolverError> {
        Self::require_initialized(&env)?;
        Ownable::require_owner(&env).map_err(|_| VerifierResolverError::Unauthorized)?;

        let mut outbound_map: Map<u64, Address> = env
            .storage()
            .instance()
            .get(&DEST_OUTBND)
            .unwrap_or(Map::new(&env));

        let mut supported_dests: Vec<u64> = env
            .storage()
            .instance()
            .get(&SUP_DESTS)
            .unwrap_or(Vec::new(&env));

        for update in implementations.iter() {
            match update.verifier {
                None => {
                    // Remove: verifier is None
                    outbound_map.remove(update.dest_chain_selector);
                    supported_dests =
                        Self::vec_remove_u64(&env, &supported_dests, update.dest_chain_selector);

                    OutboundImplRemovedEvent {
                        dest_chain_selector: update.dest_chain_selector,
                    }
                    .publish(&env);
                }
                Some(verifier) => {
                    // Set/Update: verifier is provided
                    if update.dest_chain_selector == 0 {
                        return Err(VerifierResolverError::InvalidChainSelector);
                    }

                    // Add to supported dest chains if not already present
                    if !Self::vec_contains_u64(&supported_dests, update.dest_chain_selector) {
                        supported_dests.push_back(update.dest_chain_selector);
                    }

                    outbound_map.set(update.dest_chain_selector, verifier.clone());

                    OutboundImplSetEvent {
                        dest_chain_selector: update.dest_chain_selector,
                        verifier,
                    }
                    .publish(&env);
                }
            }
        }

        env.storage().instance().set(&DEST_OUTBND, &outbound_map);
        env.storage().instance().set(&SUP_DESTS, &supported_dests);

        Ok(())
    }

    /// Update the fee aggregator address.
    ///
    /// # Arguments
    /// * `fee_aggregator` - The new fee aggregator address
    ///
    /// # Errors
    /// * `NotInitialized` - If contract is not initialized
    /// * `Unauthorized` - If caller is not the owner
    pub fn set_fee_aggregator(
        env: Env,
        fee_aggregator: Address,
    ) -> Result<(), VerifierResolverError> {
        Self::require_initialized(&env)?;
        Ownable::require_owner(&env).map_err(|_| VerifierResolverError::Unauthorized)?;

        env.storage().instance().set(&FEE_AGG, &fee_aggregator);

        FeeAggregatorSetEvent { fee_aggregator }.publish(&env);

        Ok(())
    }

    /// Transfer ownership to a new address (two-step process via common-authorization).
    ///
    /// # Arguments
    /// * `new_owner` - The proposed new owner
    pub fn transfer_ownership(
        env: Env,
        new_owner: Address,
    ) -> Result<(), VerifierResolverError> {
        Self::require_initialized(&env)?;
        Ownable::transfer_ownership(&env, &new_owner)
            .map_err(|_| VerifierResolverError::Unauthorized)?;
        Ok(())
    }

    /// Accept pending ownership transfer.
    pub fn accept_ownership(env: Env) -> Result<(), VerifierResolverError> {
        Ownable::accept_ownership(&env).map_err(|_| VerifierResolverError::Unauthorized)?;
        Ok(())
    }

    // ========================================
    // Internal Helpers
    // ========================================

    fn require_initialized(env: &Env) -> Result<(), VerifierResolverError> {
        if !env.storage().instance().has(&INITIALIZED) {
            return Err(VerifierResolverError::NotInitialized);
        }
        Ok(())
    }

    /// Extract the first 4 bytes from a Bytes value as BytesN<4>.
    fn extract_version(env: &Env, data: &Bytes) -> BytesN<4> {
        let mut version_bytes = [0u8; 4];
        for i in 0..4 {
            version_bytes[i as usize] = data.get(i).unwrap();
        }
        BytesN::from_array(env, &version_bytes)
    }

    /// Check if a Vec<BytesN<4>> contains a specific value.
    fn vec_contains_version(vec: &Vec<BytesN<4>>, value: &BytesN<4>) -> bool {
        for item in vec.iter() {
            if item == *value {
                return true;
            }
        }
        false
    }

    /// Remove a specific BytesN<4> value from a Vec, returning a new Vec.
    fn vec_remove_version(env: &Env, vec: &Vec<BytesN<4>>, value: &BytesN<4>) -> Vec<BytesN<4>> {
        let mut result: Vec<BytesN<4>> = Vec::new(env);
        for item in vec.iter() {
            if item != *value {
                result.push_back(item);
            }
        }
        result
    }

    /// Check if a Vec<u64> contains a specific value.
    fn vec_contains_u64(vec: &Vec<u64>, value: u64) -> bool {
        for item in vec.iter() {
            if item == value {
                return true;
            }
        }
        false
    }

    /// Remove a specific u64 value from a Vec, returning a new Vec.
    fn vec_remove_u64(env: &Env, vec: &Vec<u64>, value: u64) -> Vec<u64> {
        let mut result: Vec<u64> = Vec::new(env);
        for item in vec.iter() {
            if item != value {
                result.push_back(item);
            }
        }
        result
    }
}

#[cfg(test)]
mod test;
