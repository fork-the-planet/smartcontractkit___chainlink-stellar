use soroban_sdk::{contracttype, Address, Bytes};

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LockOrBurnIn {
    pub receiver: Bytes,
    pub remote_chain_selector: u64,
    pub original_sender: Address,
    pub amount: i128,
    pub local_token: Address,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LockOrBurnOut {
    pub dest_token_address: Bytes,
    pub dest_pool_data: Bytes,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReleaseOrMintIn {
    pub original_sender: Bytes,
    pub remote_chain_selector: u64,
    pub receiver: Address,
    pub amount: i128,
    pub local_token: Address,
    pub source_pool_address: Bytes,
    pub source_pool_data: Bytes,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReleaseOrMintOut {
    pub destination_amount: i128,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RemoteChainConfig {
    pub remote_pool_address: Bytes,
    pub remote_token_address: Bytes,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainUpdate {
    pub remote_chain_selector: u64,
    pub remote_pool_addresses: Bytes,
    pub remote_token_address: Bytes,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PoolDataKey {
    Token,
    RemoteChainConfig(u64),
    SupportedChains,
}
