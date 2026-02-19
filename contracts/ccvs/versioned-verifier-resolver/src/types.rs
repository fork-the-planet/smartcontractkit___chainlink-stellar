use common_helpers::map_updater::{MapUpdate, MapUpdater};
use soroban_sdk::{contracttype, Address, BytesN, Env, IntoVal, Map, Symbol, Vec};
use common_error::CCIPError as VerifierResolverError;

use crate::{
    events::{
        InboundImplRemovedEvent, InboundImplSetEvent, OutboundImplRemovedEvent,
        OutboundImplSetEvent,
    },
    DEST_OUTBND, SUP_DESTS, SUP_VERS, VER_INBOUND,
};

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
    const MAP_NAME: Symbol = VER_INBOUND;
    const KEY_SET_NAME: Symbol = SUP_VERS;
    type Error = VerifierResolverError;

    fn save_changes(
        &self,
        env: &Env,
        key_set: &Vec<BytesN<4>>,
        map: &Map<BytesN<4>, Address>,
    ) -> Result<(), Self::Error> {
        env.storage().instance().set(&SUP_VERS, key_set);
        env.storage().instance().set(&VER_INBOUND, map);
        Ok(())
    }

    fn validate_update(&self, _update: &InboundImplementationUpdate) -> Result<(), Self::Error> {
        Ok(())
    }

    fn emit_set_event(&self, env: &Env, update: &InboundImplementationUpdate) {
        if let Some(verifier) = &update.verifier {
            InboundImplSetEvent {
                version: update.version.clone(),
                verifier: verifier.clone(),
            }
            .publish(env);
        }
    }

    fn emit_remove_event(&self, env: &Env, update: &InboundImplementationUpdate) {
        InboundImplRemovedEvent {
            version: update.version.clone(),
        }
        .publish(env);
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
impl MapUpdater<OutboundImplementationUpdate, u64, Address> for OutboundMap {
    const MAP_NAME: Symbol = DEST_OUTBND;
    const KEY_SET_NAME: Symbol = SUP_DESTS;
    type Error = VerifierResolverError;

    fn save_changes(
        &self,
        env: &Env,
        key_set: &Vec<u64>,
        map: &Map<u64, Address>,
    ) -> Result<(), Self::Error> {
        env.storage().instance().set(&SUP_DESTS, key_set);
        env.storage().instance().set(&DEST_OUTBND, map);
        Ok(())
    }

    fn validate_update(&self, _update: &OutboundImplementationUpdate) -> Result<(), Self::Error> {
        Ok(())
    }

    fn emit_set_event(&self, env: &Env, update: &OutboundImplementationUpdate) {
        if let Some(verifier) = &update.verifier {
            OutboundImplSetEvent {
                dest_chain_selector: update.dest_chain_selector,
                verifier: verifier.clone(),
            }
            .publish(env);
        }
    }

    fn emit_remove_event(&self, env: &Env, update: &OutboundImplementationUpdate) {
        OutboundImplRemovedEvent {
            dest_chain_selector: update.dest_chain_selector,
        }
        .publish(env);
    }
}
