#![no_std]

pub mod decimals;
pub mod events;
pub mod types;

#[cfg(test)]
mod decimals_tests;

pub use decimals::*;
pub use events::*;
pub use types::*;

use common_error::CCIPError;
use soroban_sdk::{contracttrait, Address, Bytes, Env, Vec};

/// Base token pool trait providing shared pool configuration and chain management.
///
/// Concrete pool contracts (LockRelease, BurnMint) implement this trait
/// alongside their specific `lock_or_burn` / `release_or_mint` logic.
/// Ownership checks (via `Ownable`) are handled by the concrete contract
/// `#[contractimpl]` blocks, not enforced here.
///
/// Modeled after the EVM `TokenPool.sol` shared configuration surface.
#[contracttrait]
pub trait BaseTokenPool {
    // ------------------------------------------------------------------
    // Initialization
    // ------------------------------------------------------------------

    fn init_pool(env: &Env, token: &Address, token_decimals: u32) -> Result<(), CCIPError> {
        if token_decimals > u8::MAX as u32 {
            return Err(CCIPError::InvalidPoolTokenDecimals);
        }
        env.storage().instance().set(&PoolDataKey::Token, token);
        env.storage()
            .instance()
            .set(&PoolDataKey::TokenDecimals, &token_decimals);
        let chains: Vec<u64> = Vec::new(env);
        env.storage()
            .instance()
            .set(&PoolDataKey::SupportedChains, &chains);
        Ok(())
    }

    // ------------------------------------------------------------------
    // View Functions
    // ------------------------------------------------------------------

    fn get_token(env: &Env) -> Result<Address, CCIPError> {
        env.storage()
            .instance()
            .get(&PoolDataKey::Token)
            .ok_or(CCIPError::NotInitialized)
    }

    fn get_token_decimals(env: &Env) -> Result<u32, CCIPError> {
        env.storage()
            .instance()
            .get(&PoolDataKey::TokenDecimals)
            .ok_or(CCIPError::NotInitialized)
    }

    fn is_supported_token(env: &Env, token: &Address) -> Result<bool, CCIPError> {
        let pool_token = Self::get_token(env)?;
        Ok(pool_token == *token)
    }

    fn is_supported_chain(env: &Env, remote_chain_selector: u64) -> Result<bool, CCIPError> {
        let chains: Vec<u64> = env
            .storage()
            .instance()
            .get(&PoolDataKey::SupportedChains)
            .unwrap_or(Vec::new(env));
        for chain in chains.iter() {
            if chain == remote_chain_selector {
                return Ok(true);
            }
        }
        Ok(false)
    }

    fn get_remote_pool(env: &Env, remote_chain_selector: u64) -> Result<Bytes, CCIPError> {
        let config: RemoteChainConfig = env
            .storage()
            .persistent()
            .get(&PoolDataKey::RemoteChainConfig(remote_chain_selector))
            .ok_or(CCIPError::ChainNotSupported)?;
        Ok(config.remote_pool_address)
    }

    fn get_remote_token(env: &Env, remote_chain_selector: u64) -> Result<Bytes, CCIPError> {
        let config: RemoteChainConfig = env
            .storage()
            .persistent()
            .get(&PoolDataKey::RemoteChainConfig(remote_chain_selector))
            .ok_or(CCIPError::ChainNotSupported)?;
        Ok(config.remote_token_address)
    }

    // ------------------------------------------------------------------
    // Chain Configuration (owner check done by caller)
    // ------------------------------------------------------------------

    fn apply_chain_updates(
        env: &Env,
        adds: Vec<ChainUpdate>,
        removes: Vec<u64>,
    ) -> Result<(), CCIPError> {
        let mut chains: Vec<u64> = env
            .storage()
            .instance()
            .get(&PoolDataKey::SupportedChains)
            .unwrap_or(Vec::new(env));

        for selector in removes.iter() {
            env.storage()
                .persistent()
                .remove(&PoolDataKey::RemoteChainConfig(selector));

            let mut new_chains: Vec<u64> = Vec::new(env);
            for c in chains.iter() {
                if c != selector {
                    new_chains.push_back(c);
                }
            }
            chains = new_chains;

            ChainRemovedEvent {
                remote_chain_selector: selector,
            }
            .publish(env);
        }

        for update in adds.iter() {
            let config = RemoteChainConfig {
                remote_pool_address: update.remote_pool_addresses.clone(),
                remote_token_address: update.remote_token_address.clone(),
            };
            env.storage().persistent().set(
                &PoolDataKey::RemoteChainConfig(update.remote_chain_selector),
                &config,
            );

            let mut already_listed = false;
            for c in chains.iter() {
                if c == update.remote_chain_selector {
                    already_listed = true;
                    break;
                }
            }
            if !already_listed {
                chains.push_back(update.remote_chain_selector);
            }

            ChainConfiguredEvent {
                remote_chain_selector: update.remote_chain_selector,
                remote_pool_address: update.remote_pool_addresses.clone(),
                remote_token_address: update.remote_token_address.clone(),
            }
            .publish(env);
        }

        env.storage()
            .instance()
            .set(&PoolDataKey::SupportedChains, &chains);

        Ok(())
    }
}
