//! OnRamp cross-contract client interface.
//!
//! Defines the subset of OnRamp functions callable by other contracts
//! (primarily the Router). The generated `OnRampClient` makes typed
//! cross-contract calls without importing the full OnRamp implementation.

use common_message::StellarToAnyMessage;
use soroban_sdk::{contractclient, Address, BytesN, Env};

#[contractclient(name = "OnRampClient")]
pub trait OnRampInterface {
    /// Get the fee for sending a message to a destination chain.
    fn get_fee(env: Env, dest_chain_selector: u64, message: StellarToAnyMessage) -> i128;

    /// Forward a message from the Router to the OnRamp for processing.
    fn forward_from_router(
        env: Env,
        dest_chain_selector: u64,
        message: StellarToAnyMessage,
        fee_token_amount: i128,
        original_sender: Address,
    ) -> BytesN<32>;
}
