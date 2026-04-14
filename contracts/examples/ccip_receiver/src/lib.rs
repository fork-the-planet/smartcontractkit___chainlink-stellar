#![no_std]

//! Stellar CCIP **application** receiver example — analogue of Solidity
//! [`CCIPReceiver`](https://github.com/smartcontractkit/chainlink-ccip/blob/develop/chains/evm/contracts/applications/CCIPReceiver.sol)
//! plus pieces of [`CCIPClientExample`](https://github.com/smartcontractkit/chainlink-ccip/blob/develop/chains/evm/contracts/applications/CCIPClientExample.sol)
//! / [`CCIPClientExampleWithCCVs`](https://github.com/smartcontractkit/chainlink-ccip/blob/develop/chains/evm/contracts/applications/CCIPClientExampleWithCCVs.sol):
//!
//! - **Router-only `ccip_receive`**: [`Address::require_auth_for_args`](soroban_sdk::Address::require_auth_for_args)
//!   (same role as `onlyRouter` on EVM).
//! - **Per-destination outbound `extra_args`**: stored when owner calls [`ExampleCcipReceiver::enable_remote_chain`],
//!   used by [`ExampleCcipReceiver::send_data_pay_fee_token`] (EVM `enableChain` / `sendDataPayFeeToken` pattern).
//! - **Per-source CCV lists**: owner calls [`ExampleCcipReceiver::apply_ccv_config_updates`] (EVM `applyCCVConfigUpdates` pattern).
//!   Stellar OffRamp does not static-call the receiver for CCV policy today; this storage is for app-level policy,
//!   monitoring, or future integration.

mod events;

use soroban_sdk::{
    contract, contractimpl, symbol_short, Address, Bytes, BytesN, Env, IntoVal, Symbol, Vec,
};

use common_authorization::Ownable;
use common_error::CCIPError;
use common_guard::initializable::Initializable;
use common_interfaces::ccip_receiver::{CcvChainConfig, CcvConfigUpdate};
use common_interfaces::router::{RouterClient, StellarToAnyMessage};
use common_message::AnyToStellarMessage;
use events::{CcipCcvConfigSetEvent, CcipMessageReceivedEvent, CcipRemoteChainConfiguredEvent};

const ROUTER: Symbol = symbol_short!("ROUTER");
const LAST_MID: Symbol = symbol_short!("LASTMID");
const OWNER: Symbol = symbol_short!("OWNER");
const PENDING_OWNER: Symbol = symbol_short!("PNDGOWNR");
const REM_EXTRA: Symbol = symbol_short!("RMEXT");
const CCV_KEY: Symbol = symbol_short!("CCVCG");

#[contract]
pub struct ExampleCcipReceiver;

#[contractimpl]
impl Initializable for ExampleCcipReceiver {
    const INITIALIZED: Symbol = symbol_short!("INIT");
}

#[contractimpl]
impl Ownable for ExampleCcipReceiver {
    const OWNER: Symbol = OWNER;
    const PENDING_OWNER: Symbol = PENDING_OWNER;
}

#[contractimpl]
impl ExampleCcipReceiver {
    /// One-time setup: `owner` governs outbound/CCV config; `router` is the only caller for `ccip_receive`.
    pub fn initialize(env: Env, owner: Address, router: Address) -> Result<(), CCIPError> {
        <Self as Initializable>::require_not_initialized(&env)?;
        <Self as Ownable>::init_owner(&env, &owner)?;
        <Self as Initializable>::init(&env)?;
        env.storage().instance().set(&ROUTER, &router);
        Ok(())
    }

    pub fn get_router(env: Env) -> Result<Address, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&ROUTER)
            .ok_or(CCIPError::NotInitialized)
    }

    /// Entry point invoked by the Router after OffRamp verification (EVM `ccipReceive` analogue).
    pub fn ccip_receive(env: Env, message: AnyToStellarMessage) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;

        let router: Address = env
            .storage()
            .instance()
            .get(&ROUTER)
            .ok_or(CCIPError::NotInitialized)?;

        router.require_auth_for_args(soroban_sdk::vec![&env, message.clone().into_val(&env)]);

        CcipMessageReceivedEvent {
            message_id: message.message_id.clone(),
            source_chain_selector: message.source_chain_selector,
            data_len: message.data.len(),
            sender_len: message.sender.len(),
            dest_token_transfers: message.dest_token_amounts.len(),
        }
        .publish(&env);

        env.storage().instance().set(&LAST_MID, &message.message_id);

        Ok(())
    }

    pub fn last_message_id(env: Env) -> Result<BytesN<32>, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&LAST_MID)
            .ok_or(CCIPError::NotInitialized)
    }

    /// Store outbound `extra_args` for `dest_chain_selector` (must be non-empty). Owner-only.
    pub fn enable_remote_chain(
        env: Env,
        caller: Address,
        dest_chain_selector: u64,
        extra_args: Bytes,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        require_owner_auth(&env, &caller)?;
        if extra_args.is_empty() {
            return Err(CCIPError::ZeroValueNotAllowed);
        }
        let key = (REM_EXTRA, dest_chain_selector);
        env.storage().persistent().set(&key, &extra_args);
        CcipRemoteChainConfiguredEvent {
            dest_chain_selector,
            extra_args_len: extra_args.len(),
        }
        .publish(&env);
        Ok(())
    }

    pub fn disable_remote_chain(
        env: Env,
        caller: Address,
        dest_chain_selector: u64,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        require_owner_auth(&env, &caller)?;
        let key = (REM_EXTRA, dest_chain_selector);
        env.storage().persistent().remove(&key);
        CcipRemoteChainConfiguredEvent {
            dest_chain_selector,
            extra_args_len: 0,
        }
        .publish(&env);
        Ok(())
    }

    pub fn get_remote_chain_extra_args(
        env: Env,
        dest_chain_selector: u64,
    ) -> Result<Bytes, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        let key = (REM_EXTRA, dest_chain_selector);
        Ok(env
            .storage()
            .persistent()
            .get(&key)
            .unwrap_or_else(|| Bytes::new(&env)))
    }

    /// Batch-set CCV lists per source chain (mirrors EVM validation rules from `applyCCVConfigUpdates`).
    pub fn apply_ccv_config_updates(
        env: Env,
        caller: Address,
        updates: Vec<CcvConfigUpdate>,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        require_owner_auth(&env, &caller)?;
        for i in 0..updates.len() {
            let u = updates.get(i).ok_or(CCIPError::InvalidConfig)?;
            validate_ccv_config_update(&u)?;
            let cfg = CcvChainConfig {
                required_ccvs: u.required_ccvs.clone(),
                optional_ccvs: u.optional_ccvs.clone(),
                optional_threshold: u.optional_threshold,
            };
            let key = (CCV_KEY, u.source_chain_selector);
            env.storage().persistent().set(&key, &cfg);
            CcipCcvConfigSetEvent {
                source_chain_selector: u.source_chain_selector,
                required_len: u.required_ccvs.len(),
                optional_len: u.optional_ccvs.len(),
                optional_threshold: u.optional_threshold,
            }
            .publish(&env);
        }
        Ok(())
    }

    pub fn get_ccv_config(env: Env, source_chain_selector: u64) -> Result<CcvChainConfig, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        let key = (CCV_KEY, source_chain_selector);
        Ok(env.storage().persistent().get(&key).unwrap_or(CcvChainConfig {
            required_ccvs: Vec::new(&env),
            optional_ccvs: Vec::new(&env),
            optional_threshold: 0,
        }))
    }

    /// Data-only CCIP send using stored per-destination `extra_args`. `caller` is the Router `sender` and pays fees.
    pub fn send_data_pay_fee_token(
        env: Env,
        caller: Address,
        dest_chain_selector: u64,
        receiver: Bytes,
        data: Bytes,
        fee_token: Address,
        fee_token_amount: i128,
    ) -> Result<BytesN<32>, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        caller.require_auth();

        let extra_args = {
            let key = (REM_EXTRA, dest_chain_selector);
            match env.storage().persistent().get::<(Symbol, u64), Bytes>(&key) {
                Some(b) if !b.is_empty() => b,
                _ => return Err(CCIPError::DestinationChainNotEnabled),
            }
        };

        let router: Address = env
            .storage()
            .instance()
            .get(&ROUTER)
            .ok_or(CCIPError::NotInitialized)?;

        let message = StellarToAnyMessage {
            receiver,
            data,
            token_amounts: Vec::new(&env),
            fee_token,
            extra_args,
        };

        let router_client = RouterClient::new(&env, &router);
        Ok(router_client.ccip_send(
            &caller,
            &dest_chain_selector,
            &message,
            &fee_token_amount,
        ))
    }
}

fn require_owner_auth(env: &Env, caller: &Address) -> Result<(), CCIPError> {
    let owner = <ExampleCcipReceiver as Ownable>::owner(env).ok_or(CCIPError::NotOwner)?;
    if caller != &owner {
        return Err(CCIPError::Unauthorized);
    }
    caller.require_auth();
    Ok(())
}

fn validate_ccv_config_update(u: &CcvConfigUpdate) -> Result<(), CCIPError> {
    let olen = u.optional_ccvs.len();
    if olen > 0 {
        if u.optional_threshold >= olen {
            return Err(CCIPError::InvalidConfig);
        }
    } else if u.optional_threshold > 0 {
        return Err(CCIPError::InvalidConfig);
    }

    let rlen = u.required_ccvs.len();
    let total = rlen + olen;
    for i in 0..total {
        let ai = addr_at(u, i)?;
        for j in (i + 1)..total {
            let aj = addr_at(u, j)?;
            if ai == aj {
                return Err(CCIPError::InvalidConfig);
            }
        }
    }
    Ok(())
}

fn addr_at(u: &CcvConfigUpdate, idx: u32) -> Result<Address, CCIPError> {
    let rlen = u.required_ccvs.len();
    if idx < rlen {
        u.required_ccvs.get(idx).ok_or(CCIPError::InvalidConfig)
    } else {
        u.optional_ccvs
            .get(idx - rlen)
            .ok_or(CCIPError::InvalidConfig)
    }
}

mod test;
