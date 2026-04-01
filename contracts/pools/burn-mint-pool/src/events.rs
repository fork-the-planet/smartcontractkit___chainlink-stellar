use soroban_sdk::{contractevent, Address};

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
