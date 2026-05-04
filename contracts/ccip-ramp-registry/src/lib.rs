#![no_std]

mod events;
mod types;

use soroban_sdk::{contract, contractimpl, symbol_short, Address, Env, Map, Symbol, Vec};

use common_authorization::Ownable;
use common_error::CCIPError;
use common_guard::initializable::Initializable;
use events::{OffRampAddedEvent, OffRampRemovedEvent, OnRampSetEvent};
use types::{OffRampEntry, OnRampEntry};

const INITIALIZED: Symbol = symbol_short!("INIT");
const OWNER: Symbol = symbol_short!("OWNER");
const PENDING_OWNER: Symbol = symbol_short!("PNDGOWNR");
const ONRAMPS: Symbol = symbol_short!("ONRAMPS");
const OFFRAMPS: Symbol = symbol_short!("OFFRAMPS");

/// Stores CCIP OnRamp / OffRamp configuration separately from the Router so token pools
/// can authorize ramp callers without re-entering the Router during outbound sends.
#[contract]
pub struct RampRegistryContract;

#[contractimpl]
impl Initializable for RampRegistryContract {
    const INITIALIZED: Symbol = INITIALIZED;
}

#[contractimpl(contracttrait)]
impl Ownable for RampRegistryContract {
    const OWNER: Symbol = OWNER;
    const PENDING_OWNER: Symbol = PENDING_OWNER;
}

#[contractimpl]
impl RampRegistryContract {
    /// Initialize the registry. Owner-only mutations mirror the Router ramp admin API.
    pub fn initialize(env: Env, owner: Address) -> Result<(), CCIPError> {
        <Self as Initializable>::require_not_initialized(&env)?;
        <Self as Ownable>::init_owner(&env, &owner)?;
        <Self as Initializable>::init(&env)?;

        let onramps: Map<u64, Address> = Map::new(&env);
        env.storage().persistent().set(&ONRAMPS, &onramps);

        let offramps: Map<u64, Vec<Address>> = Map::new(&env);
        env.storage().persistent().set(&OFFRAMPS, &offramps);

        Ok(())
    }

    pub fn type_and_version(_env: Env) -> soroban_sdk::String {
        soroban_sdk::String::from_str(&_env, "RampRegistry 1.0.0")
    }

    /// Same semantics as the Router `get_onramp` entrypoint.
    pub fn get_onramp(env: Env, dest_chain_selector: u64) -> Result<Address, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        Self::get_onramp_internal(&env, dest_chain_selector)
    }

    /// Same semantics as the Router `is_offramp` entrypoint.
    pub fn is_offramp(
        env: Env,
        source_chain_selector: u64,
        offramp: Address,
    ) -> Result<bool, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        Ok(Self::is_offramp_internal(
            &env,
            source_chain_selector,
            offramp,
        ))
    }

    pub fn get_offramps(env: Env) -> Result<Vec<OffRampEntry>, CCIPError> {
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

    pub fn get_onramps(env: Env) -> Result<Vec<OnRampEntry>, CCIPError> {
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

    pub fn set_onramp(
        env: Env,
        dest_chain_selector: u64,
        onramp: Address,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

        let mut onramps: Map<u64, Address> = env
            .storage()
            .persistent()
            .get(&ONRAMPS)
            .unwrap_or(Map::new(&env));

        onramps.set(dest_chain_selector, onramp.clone());
        env.storage().persistent().set(&ONRAMPS, &onramps);

        OnRampSetEvent {
            dest_chain_selector,
            onramp,
        }
        .publish(&env);

        Ok(())
    }

    pub fn add_offramp(
        env: Env,
        source_chain_selector: u64,
        offramp: Address,
    ) -> Result<(), CCIPError> {
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

        for i in 0..chain_offramps.len() {
            if chain_offramps.get(i) == Some(offramp.clone()) {
                return Err(CCIPError::OffRampAlreadyExists);
            }
        }

        chain_offramps.push_back(offramp.clone());
        offramps.set(source_chain_selector, chain_offramps);
        env.storage().persistent().set(&OFFRAMPS, &offramps);

        OffRampAddedEvent {
            source_chain_selector,
            offramp,
        }
        .publish(&env);

        Ok(())
    }

    pub fn remove_offramp(
        env: Env,
        source_chain_selector: u64,
        offramp: Address,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

        let mut offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(&env));

        let chain_offramps = offramps
            .get(source_chain_selector)
            .ok_or(CCIPError::OffRampMismatch)?;

        let mut found = false;
        let mut new_chain_offramps: Vec<Address> = Vec::new(&env);

        for i in 0..chain_offramps.len() {
            if let Some(addr) = chain_offramps.get(i) {
                if addr == offramp {
                    found = true;
                } else {
                    new_chain_offramps.push_back(addr);
                }
            }
        }

        if !found {
            return Err(CCIPError::OffRampMismatch);
        }

        if new_chain_offramps.is_empty() {
            offramps.remove(source_chain_selector);
        } else {
            offramps.set(source_chain_selector, new_chain_offramps);
        }
        env.storage().persistent().set(&OFFRAMPS, &offramps);

        OffRampRemovedEvent {
            source_chain_selector,
            offramp,
        }
        .publish(&env);

        Ok(())
    }

    pub fn apply_ramp_updates(
        env: Env,
        onramp_updates: Vec<OnRampEntry>,
        offramp_removes: Vec<OffRampEntry>,
        offramp_adds: Vec<OffRampEntry>,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

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

        let mut offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(&env));

        for entry in offramp_removes.iter() {
            let chain_offramps = offramps
                .get(entry.source_chain_selector)
                .ok_or(CCIPError::OffRampMismatch)?;

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
                return Err(CCIPError::OffRampMismatch);
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

        for entry in offramp_adds.iter() {
            let mut chain_offramps = offramps
                .get(entry.source_chain_selector)
                .unwrap_or(Vec::new(&env));

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

    fn get_onramp_internal(env: &Env, dest_chain_selector: u64) -> Result<Address, CCIPError> {
        let onramps: Map<u64, Address> = env
            .storage()
            .persistent()
            .get(&ONRAMPS)
            .ok_or(CCIPError::UnsupportedDestinationChain)?;

        onramps
            .get(dest_chain_selector)
            .ok_or(CCIPError::UnsupportedDestinationChain)
    }

    fn is_offramp_internal(env: &Env, source_chain_selector: u64, offramp: Address) -> bool {
        let offramps: Map<u64, Vec<Address>> = env
            .storage()
            .persistent()
            .get(&OFFRAMPS)
            .unwrap_or(Map::new(env));

        if let Some(chain_offramps) = offramps.get(source_chain_selector) {
            for i in 0..chain_offramps.len() {
                if chain_offramps.get(i) == Some(offramp.clone()) {
                    return true;
                }
            }
        }

        false
    }
}

#[cfg(test)]
mod test;
