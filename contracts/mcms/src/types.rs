//! On-chain structs for MCMS-Stellar (see `docs/mcms-stellar-plan.md`).

use soroban_sdk::{contracttype, Bytes, BytesN};

pub const NUM_GROUPS: u32 = 32;
pub const MAX_NUM_SIGNERS: u32 = 200;

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Signer {
    /// Ethereum address, left-padded to 32 bytes (Solidity `address` ABI layout).
    pub addr: BytesN<32>,
    pub index: u32,
    pub group: u32,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Config {
    pub signers: soroban_sdk::Vec<Signer>,
    pub group_quorums: BytesN<32>,
    pub group_parents: BytesN<32>,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExpiringRootAndOpCount {
    pub root: BytesN<32>,
    pub valid_until: u32,
    pub op_count: u64,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StellarRootMetadata {
    pub chain_id: BytesN<32>,
    pub multisig: BytesN<32>,
    pub pre_op_count: u64,
    pub post_op_count: u64,
    pub override_previous_root: bool,
}

/// Operation leaf (must match ABI hashing in `abi_encoding` and off-chain `mcms` encoder).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StellarOp {
    pub chain_id: BytesN<32>,
    pub multisig: BytesN<32>,
    pub nonce: u64,
    pub to: BytesN<32>,
    /// ABI `uint256` value; MUST be zero (32 zero bytes) in v1.
    pub value: BytesN<32>,
    pub data: Bytes,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Signature {
    pub v: u32,
    pub r: BytesN<32>,
    pub s: BytesN<32>,
}

/// Wrapper so exported contract methods avoid `Vec<BytesN<32>>` (restricted by Soroban ABI).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignerAddresses {
    pub inner: soroban_sdk::Vec<BytesN<32>>,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignerGroups {
    pub inner: soroban_sdk::Vec<u32>,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MerkleProof {
    pub inner: soroban_sdk::Vec<BytesN<32>>,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignatureVec {
    pub inner: soroban_sdk::Vec<Signature>,
}
