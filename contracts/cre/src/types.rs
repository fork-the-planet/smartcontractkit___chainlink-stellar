use soroban_sdk::{contracttype, Address, BytesN, Vec};

// ============================================================
// Types & Structs
// ============================================================

#[contracttype]
#[derive(Copy, Clone, Eq, PartialEq)]
#[repr(u32)]
pub enum TransmissionState {
    NotAttempted = 0,
    Succeeded = 1,
    InvalidReceiver = 2,
    Failed = 3,
}

#[contracttype]
#[derive(Clone)]
pub struct Transmission {
    pub state: TransmissionState,
    pub transmitter: Address,
}

#[contracttype]
#[derive(Clone)]
pub struct TransmissionInfo {
    pub state: TransmissionState,
    pub transmitter: Option<Address>,
}

#[contracttype]
#[derive(Clone)]
pub struct Config {
    pub f: u32,
    pub signers: Vec<BytesN<65>>,
}

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Forwarder(Address),
    Config(u64),
    Transmission(BytesN<32>),
}
