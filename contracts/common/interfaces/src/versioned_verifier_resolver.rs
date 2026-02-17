pub const WASM: &[u8] = soroban_sdk::contractfile!(
    file = "./target/wasm32v1-none/release/ccvs_versioned_verifier_resolver.wasm", sha256
    = "e102e057d770add39875aede6389f4fd56117854ea9638cd22b67614eadb1191"
);
#[soroban_sdk::contractargs(name = "Args")]
#[soroban_sdk::contractclient(name = "Client")]
pub trait Contract {
    fn initialize(
        env: soroban_sdk::Env,
        owner: soroban_sdk::Address,
        fee_aggregator: soroban_sdk::Address,
    ) -> Result<(), VerifierResolverError>;
    fn get_inbound_implementation(
        env: soroban_sdk::Env,
        verifier_results: soroban_sdk::Bytes,
    ) -> Result<soroban_sdk::Address, VerifierResolverError>;
    fn get_all_inbound_implementations(
        env: soroban_sdk::Env,
    ) -> soroban_sdk::Vec<InboundImplementationArgs>;
    fn get_outbound_implementation(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
        extra_args: soroban_sdk::Bytes,
    ) -> Result<soroban_sdk::Address, VerifierResolverError>;
    fn get_all_outbound_implementations(
        env: soroban_sdk::Env,
    ) -> soroban_sdk::Vec<OutboundImplementationArgs>;
    fn get_fee_aggregator(
        env: soroban_sdk::Env,
    ) -> Result<soroban_sdk::Address, VerifierResolverError>;
    fn owner(
        env: soroban_sdk::Env,
    ) -> Result<soroban_sdk::Address, VerifierResolverError>;
    fn apply_inbound_impl_updates(
        env: soroban_sdk::Env,
        implementations: soroban_sdk::Vec<InboundImplementationUpdate>,
    ) -> Result<(), VerifierResolverError>;
    fn apply_outbound_impl_updates(
        env: soroban_sdk::Env,
        implementations: soroban_sdk::Vec<OutboundImplementationUpdate>,
    ) -> Result<(), VerifierResolverError>;
    fn set_fee_aggregator(
        env: soroban_sdk::Env,
        fee_aggregator: soroban_sdk::Address,
    ) -> Result<(), VerifierResolverError>;
    fn transfer_ownership(
        env: soroban_sdk::Env,
        new_owner: soroban_sdk::Address,
    ) -> Result<(), VerifierResolverError>;
    fn accept_ownership(env: soroban_sdk::Env) -> Result<(), VerifierResolverError>;
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct InboundImplementationUpdate {
    pub verifier: Option<soroban_sdk::Address>,
    pub version: soroban_sdk::BytesN<4>,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct InboundImplementationArgs {
    pub verifier: soroban_sdk::Address,
    pub version: soroban_sdk::BytesN<4>,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OutboundImplementationUpdate {
    pub dest_chain_selector: u64,
    pub verifier: Option<soroban_sdk::Address>,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OutboundImplementationArgs {
    pub dest_chain_selector: u64,
    pub verifier: soroban_sdk::Address,
}
#[soroban_sdk::contracterror(export = false)]
#[derive(Debug, Copy, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub enum VerifierResolverError {
    AlreadyInitialized = 1,
    NotInitialized = 2,
    Unauthorized = 3,
    InvalidVerifierResultsLength = 4,
    InboundImplementationNotFound = 5,
    OutboundImplementationNotFound = 6,
    InvalidAddress = 7,
    InvalidChainSelector = 8,
    InvalidVersion = 9,
}
#[soroban_sdk::contracterror(export = false)]
#[derive(Debug, Copy, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub enum AuthError {
    NotInitialized = 1,
    Unauthorized = 2,
    NotOwner = 3,
    NoPendingOwner = 4,
    CallerNotAuthorized = 5,
    CallerAlreadyAuthorized = 6,
    CallerNotFound = 7,
    RoleNotGranted = 8,
    FeatureNotEnabled = 9,
    RoleAlreadyGranted = 10,
    CannotRenounceRole = 11,
}
#[soroban_sdk::contractevent(topics = ["vvr_InboundImplSet"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct InboundImplSetEvent {
    pub version: soroban_sdk::BytesN<4>,
    pub verifier: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["vvr_InboundImplRemoved"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct InboundImplRemovedEvent {
    pub version: soroban_sdk::BytesN<4>,
}
#[soroban_sdk::contractevent(topics = ["vvr_OutboundImplSet"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OutboundImplSetEvent {
    pub dest_chain_selector: u64,
    pub verifier: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["vvr_OutboundImplRemoved"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OutboundImplRemovedEvent {
    pub dest_chain_selector: u64,
}
#[soroban_sdk::contractevent(topics = ["vvr_FeeAggregatorSet"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct FeeAggregatorSetEvent {
    pub fee_aggregator: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["vvr_OwnerTransferred"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OwnershipTransferredEvent {
    pub new_owner: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["auth_OwnerTransferStart"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OwnershipTransferStartedEvent {
    pub previous_owner: soroban_sdk::Address,
    pub new_owner: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["auth_OwnerTransferred"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OwnershipTransferredEvent {
    pub previous_owner: soroban_sdk::Address,
    pub new_owner: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["auth_CallerAdded"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct AuthorizedCallerAddedEvent {
    pub caller: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["auth_CallerRemoved"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct AuthorizedCallerRemovedEvent {
    pub caller: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["auth_RoleGranted"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct RoleGrantedEvent {
    pub role: soroban_sdk::Symbol,
    pub account: soroban_sdk::Address,
    pub sender: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["auth_RoleRevoked"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct RoleRevokedEvent {
    pub role: soroban_sdk::Symbol,
    pub account: soroban_sdk::Address,
    pub sender: soroban_sdk::Address,
}

