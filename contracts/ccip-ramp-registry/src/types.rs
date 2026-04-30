use common_error::CCIPError;
use common_helpers::map_updater::{MapUpdate, MapUpdater, StorageKind};
use soroban_sdk::{contracttype, Address, Env, Map, Symbol, Vec};

use crate::{
    events::{OffRampAddedEvent, OffRampRemovedEvent, OnRampRemovedEvent, OnRampSetEvent},
    OFFRAMPS, OFFRAMP_KEYS, ONRAMPS, ONRAMP_KEYS,
};

/// OnRamp mapping for a destination chain (same shape as router `OnRampEntry`).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OnRampEntry {
    pub dest_chain_selector: u64,
    pub onramp: Address,
}

/// OffRamp mapping for a source chain (same shape as router `OffRampEntry`).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampEntry {
    pub source_chain_selector: u64,
    pub offramp: Address,
}

/// Composite key identifying a single (source_chain, offramp) registration.
///
/// Storing offramps as `Map<OffRampKey, ()>` lets the shared `MapUpdater`
/// helper drive add/remove with one map entry per registration.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampKey {
    pub source_chain_selector: u64,
    pub offramp: Address,
}

/// Batch update for the onramp map.
///
/// - `onramp = Some(addr)` sets/updates the entry for `dest_chain_selector`.
/// - `onramp = None` removes the entry for `dest_chain_selector`.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OnRampUpdate {
    pub dest_chain_selector: u64,
    pub onramp: Option<Address>,
}

impl MapUpdate for OnRampUpdate {
    type Key = u64;
    type Value = Address;

    fn key(&self) -> Self::Key {
        self.dest_chain_selector
    }

    fn value(&self) -> Option<Self::Value> {
        self.onramp.clone()
    }
}

type OnRampMap = Map<u64, Address>;
impl MapUpdater<OnRampUpdate, u64, Address> for OnRampMap {
    const MAP_NAME: Symbol = ONRAMPS;
    const KEY_SET_NAME: Symbol = ONRAMP_KEYS;
    const STORAGE_KIND: StorageKind = StorageKind::Persistent;
    type Error = CCIPError;

    fn save_changes(
        &self,
        env: &Env,
        key_set: &Vec<u64>,
        map: &Map<u64, Address>,
    ) -> Result<(), Self::Error> {
        env.storage().persistent().set(&ONRAMP_KEYS, key_set);
        env.storage().persistent().set(&ONRAMPS, map);
        Ok(())
    }

    fn validate_update(&self, update: &OnRampUpdate) -> Result<(), Self::Error> {
        if update.onramp.is_some() && update.dest_chain_selector == 0 {
            return Err(CCIPError::InvalidChainSelector);
        }
        Ok(())
    }

    fn emit_set_event(&self, env: &Env, update: &OnRampUpdate) {
        if let Some(onramp) = &update.onramp {
            OnRampSetEvent {
                dest_chain_selector: update.dest_chain_selector,
                onramp: onramp.clone(),
            }
            .publish(env);
        }
    }

    fn emit_remove_event(&self, env: &Env, update: &OnRampUpdate) {
        OnRampRemovedEvent {
            dest_chain_selector: update.dest_chain_selector,
        }
        .publish(env);
    }
}

/// Batch update for the offramp map.
///
/// Each update targets a single (source_chain, offramp) pair.
/// - `enabled = true` registers the offramp for the source chain.
/// - `enabled = false` removes that registration.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OffRampUpdate {
    pub source_chain_selector: u64,
    pub offramp: Address,
    pub enabled: bool,
}

impl MapUpdate for OffRampUpdate {
    type Key = OffRampKey;
    type Value = ();

    fn key(&self) -> Self::Key {
        OffRampKey {
            source_chain_selector: self.source_chain_selector,
            offramp: self.offramp.clone(),
        }
    }

    fn value(&self) -> Option<Self::Value> {
        if self.enabled {
            Some(())
        } else {
            None
        }
    }
}

type OffRampMap = Map<OffRampKey, ()>;
impl MapUpdater<OffRampUpdate, OffRampKey, ()> for OffRampMap {
    const MAP_NAME: Symbol = OFFRAMPS;
    const KEY_SET_NAME: Symbol = OFFRAMP_KEYS;
    const STORAGE_KIND: StorageKind = StorageKind::Persistent;
    type Error = CCIPError;

    fn save_changes(
        &self,
        env: &Env,
        key_set: &Vec<OffRampKey>,
        map: &Map<OffRampKey, ()>,
    ) -> Result<(), Self::Error> {
        env.storage().persistent().set(&OFFRAMP_KEYS, key_set);
        env.storage().persistent().set(&OFFRAMPS, map);
        Ok(())
    }

    fn validate_update(&self, update: &OffRampUpdate) -> Result<(), Self::Error> {
        if update.enabled && update.source_chain_selector == 0 {
            return Err(CCIPError::InvalidChainSelector);
        }
        Ok(())
    }

    fn emit_set_event(&self, env: &Env, update: &OffRampUpdate) {
        OffRampAddedEvent {
            source_chain_selector: update.source_chain_selector,
            offramp: update.offramp.clone(),
        }
        .publish(env);
    }

    fn emit_remove_event(&self, env: &Env, update: &OffRampUpdate) {
        OffRampRemovedEvent {
            source_chain_selector: update.source_chain_selector,
            offramp: update.offramp.clone(),
        }
        .publish(env);
    }
}
