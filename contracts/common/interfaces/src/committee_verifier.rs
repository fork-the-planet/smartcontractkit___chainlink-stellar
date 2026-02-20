use common_error::CCIPError;

#[soroban_sdk::contractclient(name = "CommitteeVerifierClient")]
pub trait CommitteeVerifierInterface {
    fn owner(env: soroban_sdk::Env) -> Option<soroban_sdk::Address>;
    fn get_fee(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
        message: soroban_sdk::Bytes,
        extra_args: soroban_sdk::Bytes,
        block_confirmations: u32,
    ) -> Result<(u32, u32, u32), CCIPError>;
    fn is_owner(env: soroban_sdk::Env, addr: soroban_sdk::Address) -> bool;
    fn init_owner(env: soroban_sdk::Env, owner: soroban_sdk::Address) -> Result<(), CCIPError>;
    fn initialize(
        env: soroban_sdk::Env,
        owner: soroban_sdk::Address,
        dynamic_config: DynamicConfig,
        storage_locations: soroban_sdk::Vec<soroban_sdk::Bytes>,
        rmn_proxy: soroban_sdk::Address,
    ) -> Result<(), CCIPError>;
    fn version_tag(env: soroban_sdk::Env) -> soroban_sdk::BytesN<4>;
    fn require_owner(env: soroban_sdk::Env) -> Result<soroban_sdk::Address, CCIPError>;
    fn set_new_owner(
        env: soroban_sdk::Env,
        new_owner: soroban_sdk::Address,
    ) -> Result<(), CCIPError>;
    fn verify_message(
        env: soroban_sdk::Env,
        source_chain_selector: u64,
        message_hash: soroban_sdk::BytesN<32>,
        verifier_results: soroban_sdk::Bytes,
    ) -> Result<(), CCIPError>;
    fn accept_ownership(env: soroban_sdk::Env) -> Result<(), CCIPError>;
    fn get_pending_owner(env: soroban_sdk::Env) -> Option<soroban_sdk::Address>;
    fn get_dynamic_config(env: soroban_sdk::Env) -> Result<DynamicConfig, CCIPError>;
    fn set_dynamic_config(
        env: soroban_sdk::Env,
        dynamic_config: DynamicConfig,
    ) -> Result<(), CCIPError>;
    fn transfer_ownership(
        env: soroban_sdk::Env,
        new_owner: soroban_sdk::Address,
    ) -> Result<(), CCIPError>;
    fn forward_to_resolver(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
        sender: soroban_sdk::Address,
        message_id: soroban_sdk::BytesN<32>,
        fee_token: soroban_sdk::Address,
        fee_token_amount: i128,
        verifier_args: soroban_sdk::Bytes,
    ) -> Result<soroban_sdk::Bytes, CCIPError>;
    fn withdraw_fee_tokens(
        env: soroban_sdk::Env,
        fee_tokens: soroban_sdk::Vec<soroban_sdk::Address>,
    ) -> Result<(), CCIPError>;
    fn get_storage_locations(
        env: soroban_sdk::Env,
    ) -> Result<soroban_sdk::Vec<soroban_sdk::Bytes>, CCIPError>;
    fn get_remote_chain_config(
        env: soroban_sdk::Env,
        remote_chain_selector: u64,
    ) -> Result<RemoteChainConfig, CCIPError>;
    fn update_storage_locations(
        env: soroban_sdk::Env,
        new_locations: soroban_sdk::Vec<soroban_sdk::Bytes>,
    ) -> Result<(), CCIPError>;
    fn cancel_ownership_transfer(env: soroban_sdk::Env) -> Result<(), CCIPError>;
    fn get_storage_locations_admin(
        env: soroban_sdk::Env,
    ) -> Result<soroban_sdk::Address, CCIPError>;
    fn emit_allowlist_updated_event(
        env: soroban_sdk::Env,
        key: u64,
        added_addresses: soroban_sdk::Vec<soroban_sdk::Address>,
        removed_addresses: soroban_sdk::Vec<soroban_sdk::Address>,
    );
    fn get_pending_storage_loc_admin(
        env: soroban_sdk::Env,
    ) -> Result<Option<soroban_sdk::Address>, CCIPError>;
    fn accept_storage_locations_admin(env: soroban_sdk::Env) -> Result<(), CCIPError>;
    fn apply_remote_chain_cfg_updates(
        env: soroban_sdk::Env,
        remote_chain_config_args: soroban_sdk::Vec<RemoteChainConfig>,
    ) -> Result<(), CCIPError>;
    fn transfer_storage_locations_admin(
        env: soroban_sdk::Env,
        to: soroban_sdk::Address,
    ) -> Result<(), CCIPError>;
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct DynamicConfig {
    pub allowlist_admin: Option<soroban_sdk::Address>,
    pub fee_aggregator: Option<soroban_sdk::Address>,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct AllowListUpdate {
    pub added_allowlisted_senders: soroban_sdk::Vec<soroban_sdk::Address>,
    pub dest_chain_selector: u64,
    pub removed_allowlisted_senders: soroban_sdk::Vec<soroban_sdk::Address>,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct RemoteChainConfig {
    pub allowlist_enabled: bool,
    pub fee_usd_cents: u32,
    pub gas_for_verification: u32,
    pub payload_size_bytes: u32,
    pub remote_chain_selector: u64,
    pub router: Option<soroban_sdk::Address>,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct SignatureQuorumConfig {
    pub signers: soroban_sdk::Vec<soroban_sdk::BytesN<32>>,
    pub source_chain_selector: u64,
    pub threshold: u32,
}
#[soroban_sdk::contractevent(topics = ["ccv_ConfigSet"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct ConfigSetEvent {
    pub dynamic_config: DynamicConfig,
}
#[soroban_sdk::contractevent(topics = ["ccv_RemoteChainConfigSet"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct RemoteChainConfigSetEvent {
    pub remote_chain_selector: u64,
    pub router: Option<soroban_sdk::Address>,
    pub allowlist_enabled: bool,
}
#[soroban_sdk::contractevent(topics = ["ccv_AllowListSendersAdded"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct AllowListSendersAddedEvent {
    pub dest_chain_selector: u64,
    pub sender: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["ccv_AllowListStateChanged"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct AllowListStateChangedEvent {
    pub dest_chain_selector: u64,
    pub allowlist_enabled: bool,
}
#[soroban_sdk::contractevent(topics = ["ccv_AllowListSendersRemoved"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct AllowListSendersRemovedEvent {
    pub dest_chain_selector: u64,
    pub sender: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["ccv_StorageAdminTransferred"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct StorageAdminTransferredEvent {
    pub from: soroban_sdk::Address,
    pub to: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["ccv_StorageAdminTransferReq"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct StorageAdminTransferReqEvent {
    pub from: soroban_sdk::Address,
    pub to: soroban_sdk::Address,
}
#[soroban_sdk::contractevent(topics = ["ccv_StorageLocationsUpdated"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct StorageLocationsUpdatedEvent {
    pub old_locations: soroban_sdk::Vec<soroban_sdk::Bytes>,
    pub new_locations: soroban_sdk::Vec<soroban_sdk::Bytes>,
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
#[soroban_sdk::contractevent(topics = ["auth_OwnerTransferStart"], export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct OwnershipTransferStartedEvent {
    pub previous_owner: soroban_sdk::Address,
    pub new_owner: soroban_sdk::Address,
}
