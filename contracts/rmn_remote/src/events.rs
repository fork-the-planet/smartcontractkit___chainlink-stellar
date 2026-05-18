use soroban_sdk::{contractevent, BytesN, Vec};

/// Emitted when one or more subjects are cursed.
#[contractevent(topics = ["rmn_Cursed"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CursedEvent {
    pub subjects: Vec<BytesN<16>>,
}

/// Emitted when one or more subjects are uncursed.
#[contractevent(topics = ["rmn_Uncursed"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct UncursedEvent {
    pub subjects: Vec<BytesN<16>>,
}
