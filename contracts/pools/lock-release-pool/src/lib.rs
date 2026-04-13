#![no_std]

mod events;

use soroban_sdk::{contract, contractimpl, symbol_short, token, Address, Bytes, Env, Symbol, Vec};

use common_authorization::Ownable;
use common_error::CCIPError;
use common_guard::initializable::Initializable;
use common_pool::{
    calculate_local_amount, encode_local_decimals, parse_remote_decimals, BaseTokenPool,
    ChainUpdate, LockOrBurnIn, LockOrBurnOut, ReleaseOrMintIn, ReleaseOrMintOut,
};
use events::{LockedEvent, ReleasedEvent};

const INITIALIZED: Symbol = symbol_short!("INIT");
const OWNER: Symbol = symbol_short!("OWNER");
const PENDING_OWNER: Symbol = symbol_short!("PNDGOWNR");

#[contract]
pub struct LockReleaseTokenPoolContract;

#[contractimpl]
impl Initializable for LockReleaseTokenPoolContract {
    const INITIALIZED: Symbol = INITIALIZED;
}

#[contractimpl(contracttrait)]
impl Ownable for LockReleaseTokenPoolContract {
    const OWNER: Symbol = OWNER;
    const PENDING_OWNER: Symbol = PENDING_OWNER;
}

#[contractimpl]
impl BaseTokenPool for LockReleaseTokenPoolContract {}

#[contractimpl]
impl LockReleaseTokenPoolContract {
    // ------------------------------------------------------------------
    // Initialization
    // ------------------------------------------------------------------

    pub fn initialize(
        env: Env,
        owner: Address,
        token: Address,
        token_decimals: u32,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_not_initialized(&env)?;
        <Self as Initializable>::init(&env)?;
        <Self as Ownable>::init_owner(&env, &owner)?;
        <Self as BaseTokenPool>::init_pool(&env, &token, token_decimals)?;
        Ok(())
    }

    // ------------------------------------------------------------------
    // Pool Operations
    // ------------------------------------------------------------------

    /// Locks tokens in the pool. Called by the OnRamp during a cross-chain send.
    ///
    /// The caller (OnRamp/Router) must have arranged for the tokens to be
    /// transferred into this contract before calling `lock_or_burn`.
    /// On Stellar, this is done via the Soroban auth tree -- the sender
    /// authorizes a `transfer(sender, pool, amount)` as a sub-invocation.
    pub fn lock_or_burn(env: Env, input: LockOrBurnIn) -> Result<LockOrBurnOut, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;

        let pool_token = <Self as BaseTokenPool>::get_token(&env)?;
        if pool_token != input.local_token {
            return Err(CCIPError::PoolTokenMismatch);
        }

        if !<Self as BaseTokenPool>::is_supported_chain(&env, input.remote_chain_selector)? {
            return Err(CCIPError::ChainNotSupported);
        }

        let pool_address = env.current_contract_address();
        let token_client = token::Client::new(&env, &pool_token);
        token_client.transfer(&input.original_sender, &pool_address, &input.amount);

        LockedEvent {
            sender: input.original_sender.clone(),
            amount: input.amount,
        }
        .publish(&env);

        let remote_token =
            <Self as BaseTokenPool>::get_remote_token(&env, input.remote_chain_selector)?;

        let local_decimals = <Self as BaseTokenPool>::get_token_decimals(&env)?;
        let dest_pool_data = encode_local_decimals(&env, local_decimals)?;

        Ok(LockOrBurnOut {
            dest_token_address: remote_token,
            dest_pool_data,
        })
    }

    /// Releases tokens from the pool to the receiver. Called by the OffRamp
    /// on the destination chain after verifying the cross-chain message.
    pub fn release_or_mint(
        env: Env,
        input: ReleaseOrMintIn,
    ) -> Result<ReleaseOrMintOut, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;

        let pool_token = <Self as BaseTokenPool>::get_token(&env)?;
        if pool_token != input.local_token {
            return Err(CCIPError::PoolTokenMismatch);
        }

        if !<Self as BaseTokenPool>::is_supported_chain(&env, input.remote_chain_selector)? {
            return Err(CCIPError::ChainNotSupported);
        }

        let local_decimals = <Self as BaseTokenPool>::get_token_decimals(&env)?;
        let remote_decimals = parse_remote_decimals(&input.source_pool_data, local_decimals)?;
        let local_amount = calculate_local_amount(input.amount, remote_decimals, local_decimals)?;

        let pool_address = env.current_contract_address();
        let token_client = token::Client::new(&env, &pool_token);

        let pool_balance = token_client.balance(&pool_address);
        if pool_balance < local_amount {
            return Err(CCIPError::InsufficientPoolLiquidity);
        }

        token_client.transfer(&pool_address, &input.receiver, &local_amount);

        ReleasedEvent {
            sender: pool_address,
            recipient: input.receiver.clone(),
            amount: local_amount,
        }
        .publish(&env);

        Ok(ReleaseOrMintOut {
            destination_amount: local_amount,
        })
    }

    // ------------------------------------------------------------------
    // Admin (owner-gated wrappers around BaseTokenPool)
    // ------------------------------------------------------------------

    pub fn apply_chain_updates(
        env: Env,
        adds: Vec<ChainUpdate>,
        removes: Vec<u64>,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;
        <Self as BaseTokenPool>::apply_chain_updates(&env, adds, removes)
    }

    // ------------------------------------------------------------------
    // View helpers (re-export for contract ABI)
    // ------------------------------------------------------------------

    pub fn get_token(env: Env) -> Result<Address, CCIPError> {
        <Self as BaseTokenPool>::get_token(&env)
    }

    pub fn get_token_decimals(env: Env) -> Result<u32, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as BaseTokenPool>::get_token_decimals(&env)
    }

    pub fn is_supported_token(env: Env, token: Address) -> Result<bool, CCIPError> {
        <Self as BaseTokenPool>::is_supported_token(&env, &token)
    }

    pub fn is_supported_chain(env: Env, remote_chain_selector: u64) -> Result<bool, CCIPError> {
        <Self as BaseTokenPool>::is_supported_chain(&env, remote_chain_selector)
    }

    pub fn get_remote_pool(env: Env, remote_chain_selector: u64) -> Result<Bytes, CCIPError> {
        <Self as BaseTokenPool>::get_remote_pool(&env, remote_chain_selector)
    }

    pub fn get_remote_token(env: Env, remote_chain_selector: u64) -> Result<Bytes, CCIPError> {
        <Self as BaseTokenPool>::get_remote_token(&env, remote_chain_selector)
    }
}

#[cfg(test)]
mod test;
