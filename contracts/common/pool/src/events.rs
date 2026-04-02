use soroban_sdk::{contractevent, Address, Bytes};

#[contractevent(topics = ["pool_Locked"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LockedEvent {
    pub sender: Address,
    pub amount: i128,
}

#[contractevent(topics = ["pool_Released"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReleasedEvent {
    pub sender: Address,
    pub recipient: Address,
    pub amount: i128,
}

#[contractevent(topics = ["pool_Burned"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BurnedEvent {
    pub sender: Address,
    pub amount: i128,
}

#[contractevent(topics = ["pool_Minted"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MintedEvent {
    pub sender: Address,
    pub recipient: Address,
    pub amount: i128,
}

#[contractevent(topics = ["pool_ChainConfigured"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainConfiguredEvent {
    pub remote_chain_selector: u64,
    pub remote_pool_address: Bytes,
    pub remote_token_address: Bytes,
}

#[contractevent(topics = ["pool_ChainRemoved"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainRemovedEvent {
    pub remote_chain_selector: u64,
}
