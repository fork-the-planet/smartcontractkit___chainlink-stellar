//! RMN Proxy cross-contract client interface.
//!
//! Defines the subset of RMN Proxy functions callable by other contracts
//! (primarily the Router). The generated `RmnProxyClient` makes typed
//! cross-contract calls without importing the full RMN Proxy implementation.

use soroban_sdk::{contractclient, Env};

#[contractclient(name = "RmnProxyClient")]
pub trait RmnProxyInterface {
    /// Check if the network is globally cursed by RMN.
    fn is_cursed(env: Env) -> bool;
}
