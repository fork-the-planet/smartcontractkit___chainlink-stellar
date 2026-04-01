//! Test-only Soroban contract whose `ccip_receive` always traps. Used by `route_message` tests.

use soroban_sdk::{contract, contractimpl, Env};

use common_error::CCIPError;
use common_message::AnyToStellarMessage;

#[contract]
pub struct PanicCcipReceiver;

#[contractimpl]
impl PanicCcipReceiver {
    pub fn ccip_receive(_env: Env, _message: AnyToStellarMessage) -> Result<(), CCIPError> {
        panic!("intentional trap for route_message tests");
    }
}

/// Returns `Err(CCIPError)` without panicking (contract error return). The host surfaces this as an
/// invoke error; the Router maps that to `CCIPError::ReceiverError` (see `route_message` match).
#[contract]
pub struct ErrReturningCcipReceiver;

#[contractimpl]
impl ErrReturningCcipReceiver {
    pub fn ccip_receive(_env: Env, _message: AnyToStellarMessage) -> Result<(), CCIPError> {
        Err(CCIPError::InvalidConfig)
    }
}
