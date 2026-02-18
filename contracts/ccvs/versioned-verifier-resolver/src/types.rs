use soroban_sdk::{Address, BytesN, Env, Map, Vec, contracttype};
use common_helpers::map_updater::{MapUpdate, MapUpdater};

use crate::{DEST_OUTBND, SUP_DESTS, SUP_VERS, VER_INBOUND, error::VerifierResolverError};

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

impl MapUpdate for InboundImplementationUpdate {
    type Key = BytesN<4>;
    type Value = Address;

    fn key(&self) -> Self::Key {
        self.version.clone()
    }

    fn value(&self) -> Option<Self::Value> {
        self.verifier.clone()
    }
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

type InboundMap = Map<BytesN<4>, Address>;
impl MapUpdater<InboundImplementationUpdate, BytesN<4>, Address> for InboundMap {
    type Error = VerifierResolverError;

    fn get_map(&self, env: &Env) -> Result<InboundMap, Self::Error> {
        env.storage().instance()
            .get(&VER_INBOUND)
            .ok_or(VerifierResolverError::NotInitialized)
    }

    fn get_key_set(&self, env: &Env) -> Result<Vec<BytesN<4>>, Self::Error> {
        env.storage().instance()
            .get(&SUP_VERS)
            .ok_or(VerifierResolverError::NotInitialized)
    }

    fn save_changes(&self, env: &Env, key_set: &Vec<BytesN<4>>, map: &Map<BytesN<4>, Address>) -> Result<(), Self::Error> {

    }
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

impl MapUpdate for OutboundImplementationUpdate {
    type Key = u64;
    type Value = Address;

    fn key(&self) -> Self::Key {
        self.dest_chain_selector
    }

    fn value(&self) -> Option<Self::Value> {
        self.verifier.clone()
    }
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

type OutboundMap = Map<u64, Address>;
type SupportedDestinations = Vec<u64>;

impl MapUpdater<OutboundImplementationUpdate, u64, Address> for OutboundMap {
    type Error = VerifierResolverError;

    fn get_map(&self, env: &Env) -> Result<OutboundMap, Self::Error> {
        env.storage().instance()
            .get(&DEST_OUTBND)
            .ok_or(VerifierResolverError::NotInitialized)
    }

    fn get_key_set(&self, env: &Env) -> Result<SupportedDestinations, Self::Error> {
        env.storage().instance()
            .get(&SUP_DESTS)
            .ok_or(VerifierResolverError::NotInitialized)
    }

    fn save_changes(&self, env: &Env, key_set: &Vec<BytesN<4>>, map: &Map<BytesN<4>, Address>) -> Result<(), Self::Error> {
        
    }
}