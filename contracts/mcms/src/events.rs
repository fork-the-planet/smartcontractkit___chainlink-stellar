use soroban_sdk::{contractevent, Bytes, BytesN};

use crate::types::{Config, StellarRootMetadata};

/// Emitted when signer configuration is updated (mirrors Solidity `ConfigSet`).
#[contractevent(topics = ["mcms_ConfigSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ConfigSetEvent {
    pub config: Config,
    pub is_root_cleared: bool,
}

/// Emitted when a new Merkle root is accepted (mirrors Solidity `NewRoot`).
#[contractevent(topics = ["mcms_NewRoot"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NewRootEvent {
    pub root: BytesN<32>,
    pub valid_until: u32,
    pub metadata: StellarRootMetadata,
}

/// Emitted when a governance op is successfully executed (mirrors Solidity `OpExecuted`).
#[contractevent(topics = ["mcms_OpExecuted"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OpExecutedEvent {
    pub nonce: u64,
    pub to: BytesN<32>,
    pub data: Bytes,
    pub value: BytesN<32>,
}

/// Emitted when the owner-configured `min_secs_per_ledger` (used to derive the dynamic
/// `valid_until` cap) is updated. No Solidity counterpart — Stellar-specific.
#[contractevent(topics = ["mcms_MinSecsPerLedgerSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MinSecsPerLedgerSetEvent {
    pub min_secs_per_ledger: u64,
}
