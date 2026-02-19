//! Versioned Verifier Resolver cross-contract client interface.
//!
//! Defines the subset of Versioned Verifier Resolver functions callable by other contracts
//! (primarily the Router). The generated `VersionedVerifierResolverClient` makes typed
//! cross-contract calls without importing the full Versioned Verifier Resolver implementation.

use soroban_sdk::{contractclient, Address, Bytes, Env, Vec};

// Re-export types and error from the resolver crate so consumers only need this interface.
pub use ccvs_versioned_verifier_resolver::{
    InboundImplementationArgs, InboundImplementationUpdate, OutboundImplementationArgs,
    OutboundImplementationUpdate,
};
pub use common_error::CCIPError as VerifierResolverError;

#[contractclient(name = "VersionedVerifierResolverClient")]
pub trait VersionedVerifierResolverInterface {
    fn initialize(
        env: Env,
        owner: Address,
        fee_aggregator: Address,
    ) -> Result<(), VerifierResolverError>;
    fn get_inbound_implementation(
        env: Env,
        verifier_results: Bytes,
    ) -> Result<Address, VerifierResolverError>;
    fn get_all_inbound_implementations(env: Env) -> Vec<InboundImplementationArgs>;
    fn get_outbound_implementation(
        env: Env,
        dest_chain_selector: u64,
        extra_args: Bytes,
    ) -> Result<Address, VerifierResolverError>;
    fn get_all_outbound_implementations(env: Env) -> Vec<OutboundImplementationArgs>;
    fn get_fee_aggregator(env: Env) -> Result<Address, VerifierResolverError>;
    fn owner(env: Env) -> Result<Address, VerifierResolverError>;
    fn apply_inbound_impl_updates(
        env: Env,
        implementations: Vec<InboundImplementationUpdate>,
    ) -> Result<(), VerifierResolverError>;
    fn apply_outbound_impl_updates(
        env: Env,
        implementations: Vec<OutboundImplementationUpdate>,
    ) -> Result<(), VerifierResolverError>;
    fn set_fee_aggregator(env: Env, fee_aggregator: Address) -> Result<(), VerifierResolverError>;
    fn transfer_ownership(env: Env, new_owner: Address) -> Result<(), VerifierResolverError>;
    fn accept_ownership(env: Env) -> Result<(), VerifierResolverError>;
}
