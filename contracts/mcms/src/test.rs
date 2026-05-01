//! Integration-style contract tests (Soroban `Env`).

#![cfg(test)]

extern crate alloc;

use alloc::vec::Vec;

use k256::ecdsa::SigningKey;
use sha3::{Digest, Keccak256};
use soroban_sdk::testutils::Address as _;
use soroban_sdk::testutils::Ledger;
use soroban_sdk::xdr::ToXdr;
use soroban_sdk::{
    contract, contractimpl, Address, Bytes, BytesN, Env, IntoVal, Symbol, Val, Vec as SorobanVec,
};

use crate::abi_encoding::{
    eth_signed_message_hash_32, hash_root_metadata, hash_set_root_inner, hash_stellar_op,
};
use crate::constants::domain_meta;
use crate::crypto::efficient_hash_pair;
use crate::error::McmsError;
use crate::types::{
    MerkleProof, Signature, SignatureVec, SignerAddresses, SignerGroups, StellarOp,
    StellarRootMetadata,
};
use crate::{McmsContract, McmsContractClient};

const NUM_GROUP_BYTES: usize = 32;

/// Deterministic 32-byte “padded EVM” signer (strictly-increasing single-signer tests).
fn signer_a(env: &Env) -> BytesN<32> {
    let mut a = [0u8; 32];
    a[12..32].copy_from_slice(&[0x1u8; 20]);
    BytesN::from_array(env, &a)
}

fn zero_chain_id(env: &Env) -> BytesN<32> {
    BytesN::from_array(env, &[0u8; 32])
}

fn one_of_one_quorum(env: &Env) -> BytesN<32> {
    let mut gq = [0u8; NUM_GROUP_BYTES];
    gq[0] = 1; // group 0 quorum 1
    BytesN::from_array(env, &gq)
}

fn all_zero_parents(env: &Env) -> BytesN<32> {
    BytesN::from_array(env, &[0u8; NUM_GROUP_BYTES])
}

fn register_client(env: &Env) -> McmsContractClient<'_> {
    let id = env.register(McmsContract, ());
    McmsContractClient::new(env, &id)
}

/// Minimal callee for successful MCMS `execute` tests (no self re-entry).
#[contract]
pub struct ExecPingMock;

#[contractimpl]
impl ExecPingMock {
    pub fn ping(_env: Env) {}
}

mod test_support {
    use soroban_sdk::{address_payload::AddressPayload, Address, BytesN, Env};

    pub fn addr_to_contract_id(addr: &Address, env: &Env) -> BytesN<32> {
        match addr.to_payload() {
            Some(AddressPayload::ContractIdHash(id)) => id,
            _ => BytesN::from_array(env, &[0u8; 32]),
        }
    }
}

// --- lifecycle ---

#[test]
fn test_initialize_and_chain_id() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);

    client.initialize(&owner, &chain);
    assert_eq!(client.chain_network_id(), chain);
    assert_eq!(client.get_op_count(), 0);
    let (root, valid_until) = client.get_root();
    assert_eq!(root, BytesN::from_array(&env, &[0u8; 32]));
    assert_eq!(valid_until, 0u32);
}

/// Second `initialize` returns contract error; non-`try` entrypoint may panic in tests.
#[test]
#[should_panic]
fn test_double_initialize_panics() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);
    client.initialize(&owner, &chain);
}

#[test]
fn test_set_config_1_of_1_and_get_config() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let s = signer_a(&env);
    let addrs = SignerAddresses {
        inner: SorobanVec::from_array(&env, [s.clone()]),
    };
    let groups = SignerGroups {
        inner: SorobanVec::from_array(&env, [0u32]),
    };

    client.set_config(
        &addrs,
        &groups,
        &one_of_one_quorum(&env),
        &all_zero_parents(&env),
        &false,
    );

    let cfg = client.get_config();
    assert_eq!(cfg.signers.len(), 1);
    assert_eq!(cfg.signers.get(0).unwrap().addr, s);
    assert_eq!(cfg.signers.get(0).unwrap().index, 0);
    assert_eq!(cfg.signers.get(0).unwrap().group, 0u32);
}

/// No `set_config` yet → `get_config` fails with `MissingConfig`
#[test]
fn test_get_config_fails_before_set() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    // Generated `try_*` type: `Result<Result<Config, ConversionError>, Result<McmsError, InvokeError>>`
    // — contract `Err(_)` is `Err(Ok(McmsError::...))` on the outer `Result`.
    assert!(matches!(
        client.try_get_config(),
        Err(Ok(McmsError::MissingConfig))
    ));
}

/// Without a prior `set_root`, stored root metadata is absent: `execute` must fail
#[test]
fn test_execute_fails_missing_root_metadata() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);
    let self_addr = client.address.clone();
    let multisig = test_support::addr_to_contract_id(&self_addr, &env);
    let op = StellarOp {
        chain_id: chain.clone(),
        multisig: multisig.clone(),
        nonce: 0,
        to: multisig, // self-invoke
        value: BytesN::from_array(&env, &[0u8; 32]),
        data: Bytes::new(&env),
    };
    let proof = MerkleProof {
        inner: SorobanVec::new(&env),
    };
    assert!(matches!(
        client.try_execute(&op, &proof),
        Err(Ok(McmsError::MissingRootMetadata))
    ));
}

/// `set_root` with no prior `set_config` must fail before reaching signature verification.
#[test]
fn test_set_root_fails_without_config() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let root = BytesN::from_array(&env, &[0u8; 32]);
    let metadata = StellarRootMetadata {
        chain_id: chain.clone(),
        multisig: BytesN::from_array(&env, &[0u8; 32]),
        pre_op_count: 0,
        post_op_count: 1,
        override_previous_root: false,
    };
    let metadata_proof = MerkleProof {
        inner: SorobanVec::new(&env),
    };
    let signatures = SignatureVec {
        inner: SorobanVec::new(&env),
    };

    assert!(matches!(
        client.try_set_root(&root, &0u32, &metadata, &metadata_proof, &signatures),
        Err(Ok(McmsError::MissingConfig))
    ));
}

/// `set_config(clear_root=true)` must zero the expiring root, preserve op_count, and
/// write root metadata — exercising a security path (invalidating pending ops) and
/// the otherwise-untested `get_root_metadata` getter.
#[test]
fn test_set_config_clear_root_resets_state() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let s = signer_a(&env);
    let addrs = SignerAddresses {
        inner: SorobanVec::from_array(&env, [s]),
    };
    let groups = SignerGroups {
        inner: SorobanVec::from_array(&env, [0u32]),
    };

    client.set_config(
        &addrs,
        &groups,
        &one_of_one_quorum(&env),
        &all_zero_parents(&env),
        &true,
    );

    let (root, valid_until) = client.get_root();
    assert_eq!(root, BytesN::from_array(&env, &[0u8; 32]));
    assert_eq!(valid_until, 0u32);
    assert_eq!(client.get_op_count(), 0u64);

    let meta = client.get_root_metadata();
    assert_eq!(meta.pre_op_count, 0);
    assert_eq!(meta.post_op_count, 0);
    assert!(meta.override_previous_root);
}

// --- set_config error paths (still as owner) ---

#[test]
fn test_set_config_mismatched_vec_lengths() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let s = signer_a(&env);
    let addrs = SignerAddresses {
        inner: SorobanVec::from_array(&env, [s]),
    };
    let groups = SignerGroups {
        inner: SorobanVec::new(&env), // 0 — mismatch
    };

    assert!(matches!(
        client.try_set_config(
            &addrs,
            &groups,
            &one_of_one_quorum(&env),
            &all_zero_parents(&env),
            &false,
        ),
        Err(Ok(McmsError::SignerGroupsLengthMismatch))
    ));
}

// ---------------------------------------------------------------------------
// Merkle + signing helpers (align with ccip-owner MerkleHelper + Anvil key)
// ---------------------------------------------------------------------------

/// Anvil / Foundry default account #0 secret (public; test-only).
const ANVIL_SK_0: [u8; 32] = [
    0xac, 0x09, 0x74, 0xbe, 0xc3, 0x9a, 0x17, 0xe3, 0x6b, 0xa4, 0xa6, 0xb4, 0xd2, 0x38, 0xff, 0x94,
    0x4b, 0xac, 0xb4, 0x78, 0xcb, 0xed, 0x5e, 0xfc, 0xae, 0x78, 0x4d, 0x7b, 0xf4, 0xf2, 0xff, 0x80,
];

fn padded_eth_address(env: &Env, sk: &SigningKey) -> BytesN<32> {
    let vk = sk.verifying_key();
    let encoded = vk.to_encoded_point(false);
    let mut hasher = Keccak256::new();
    hasher.update(&encoded.as_bytes()[1..]);
    let out = hasher.finalize();
    let mut padded = [0u8; 32];
    padded[12..32].copy_from_slice(&out[12..]);
    BytesN::from_array(env, &padded)
}

fn proof_len_ceiling(leaf_count: usize) -> usize {
    let mut power = 1usize;
    let mut exp = 0usize;
    while power < leaf_count {
        power *= 2;
        exp += 1;
    }
    exp
}

fn hash_level_native(env: &Env, data: &[BytesN<32>]) -> Vec<BytesN<32>> {
    assert_eq!(data.len() % 2, 0);
    let mut out = Vec::new();
    let mut i = 0usize;
    while i < data.len() {
        out.push(efficient_hash_pair(env, &data[i], &data[i + 1]));
        i += 2;
    }
    out
}

fn merkle_root_native(env: &Env, leaves: &[BytesN<32>]) -> BytesN<32> {
    assert_eq!(leaves.len() % 2, 0);
    let mut data: Vec<BytesN<32>> = leaves.to_vec();
    while data.len() > 1 {
        data = hash_level_native(env, &data);
    }
    data[0].clone()
}

/// Proof for leaf at `index` (same sibling walk as `MerkleHelper.computeProofForLeaf`).
fn compute_proof_for_leaf(
    env: &Env,
    mut leaves: Vec<BytesN<32>>,
    mut index: usize,
) -> SorobanVec<BytesN<32>> {
    assert_eq!(leaves.len() % 2, 0);
    let plen = proof_len_ceiling(leaves.len());
    let mut proof = SorobanVec::new(env);
    while leaves.len() > 1 {
        let sibling_idx = if index & 1 == 1 { index - 1 } else { index + 1 };
        proof.push_back(leaves[sibling_idx].clone());
        index /= 2;
        leaves = hash_level_native(env, &leaves);
    }
    assert_eq!(proof.len() as usize, plen);
    proof
}

fn encode_ping(env: &Env) -> Bytes {
    let mut v: SorobanVec<Val> = SorobanVec::new(env);
    v.push_back(Symbol::new(env, "ping").into_val(env));
    v.to_xdr(env)
}

fn encode_extend_all_ttls(env: &Env) -> Bytes {
    let mut v: SorobanVec<Val> = SorobanVec::new(env);
    v.push_back(Symbol::new(env, "extend_all_ttls").into_val(env));
    v.to_xdr(env)
}

fn signature_vec_single(env: &Env, sk: &SigningKey, signed_digest: &BytesN<32>) -> SignatureVec {
    let (sig64, recid) = sk
        .sign_prehash_recoverable(signed_digest.to_array().as_slice())
        .expect("secp256k1 sign");
    let b = sig64.to_bytes();
    let r = BytesN::from_array(env, b[..32].try_into().unwrap());
    let s = BytesN::from_array(env, b[32..].try_into().unwrap());
    let v = 27u32 + recid.to_byte() as u32;
    let sig = Signature { v, r, s };
    SignatureVec {
        inner: SorobanVec::from_array(env, [sig]),
    }
}

// --- additional coverage (EVM-aligned negative paths + happy execution) ---

#[test]
fn test_set_config_rejects_without_owner_auth() {
    let env = Env::default();
    env.mock_auths(&[]);
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let sk = SigningKey::from_slice(&ANVIL_SK_0).unwrap();
    let addr = padded_eth_address(&env, &sk);
    let addrs = SignerAddresses {
        inner: SorobanVec::from_array(&env, [addr]),
    };
    let groups = SignerGroups {
        inner: SorobanVec::from_array(&env, [0u32]),
    };

    assert!(client
        .try_set_config(
            &addrs,
            &groups,
            &one_of_one_quorum(&env),
            &all_zero_parents(&env),
            &false,
        )
        .is_err());
}

#[test]
fn test_extend_all_ttls_without_auth_succeeds() {
    let env = Env::default();
    env.mock_auths(&[]);
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let r = client.try_extend_all_ttls();
    assert!(r.is_ok());
}

#[test]
fn test_set_config_rejects_zero_signers() {
    let env = Env::default();
    env.mock_all_auths();
    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let addrs = SignerAddresses {
        inner: SorobanVec::new(&env),
    };
    let groups = SignerGroups {
        inner: SorobanVec::new(&env),
    };

    assert!(matches!(
        client.try_set_config(
            &addrs,
            &groups,
            &one_of_one_quorum(&env),
            &all_zero_parents(&env),
            &false,
        ),
        Err(Ok(McmsError::OutOfBoundsNumOfSigners))
    ));
}

/// `set_root` → `execute(ping)` on an external mock (single op, 1-of-1), then second `execute` hits post-op count.
#[test]
fn test_set_root_execute_and_post_op_count_reached() {
    let env = Env::default();
    env.mock_all_auths();
    env.ledger().with_mut(|li| {
        li.timestamp = 1_000;
    });

    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let sk = SigningKey::from_slice(&ANVIL_SK_0).unwrap();
    let signer_addr = padded_eth_address(&env, &sk);
    let addrs = SignerAddresses {
        inner: SorobanVec::from_array(&env, [signer_addr.clone()]),
    };
    let groups = SignerGroups {
        inner: SorobanVec::from_array(&env, [0u32]),
    };
    client.set_config(
        &addrs,
        &groups,
        &one_of_one_quorum(&env),
        &all_zero_parents(&env),
        &false,
    );

    let self_cid = test_support::addr_to_contract_id(&client.address, &env);
    let ping_addr = env.register(ExecPingMock, ());
    let ping_cid = test_support::addr_to_contract_id(&ping_addr, &env);
    let valid_until: u32 = 2_000_000;
    let metadata = StellarRootMetadata {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        pre_op_count: 0,
        post_op_count: 1,
        override_previous_root: false,
    };
    let meta_leaf = hash_root_metadata(&env, &domain_meta(&env), &metadata).unwrap();
    let op = StellarOp {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        nonce: 0,
        to: ping_cid,
        value: BytesN::from_array(&env, &[0u8; 32]),
        data: encode_ping(&env),
    };
    let op_leaf = hash_stellar_op(&env, &crate::constants::domain_op(&env), &op).unwrap();
    let leaves = Vec::from([meta_leaf.clone(), op_leaf.clone()]);
    let root = merkle_root_native(&env, &leaves);
    let metadata_proof = MerkleProof {
        inner: compute_proof_for_leaf(&env, leaves.clone(), 0),
    };
    let inner = hash_set_root_inner(&env, &root, valid_until);
    let signed = eth_signed_message_hash_32(&env, &inner);
    let sigs = signature_vec_single(&env, &sk, &signed);

    client.set_root(&root, &valid_until, &metadata, &metadata_proof, &sigs);

    let op_proof = MerkleProof {
        inner: compute_proof_for_leaf(&env, leaves, 1),
    };
    assert_eq!(client.get_op_count(), 0);
    client.execute(&op, &op_proof);
    assert_eq!(client.get_op_count(), 1);

    assert!(matches!(
        client.try_execute(&op, &op_proof),
        Err(Ok(McmsError::PostOpCountReached))
    ));
}

#[test]
fn test_execute_reverts_bad_proof() {
    let env = Env::default();
    env.mock_all_auths();
    env.ledger().with_mut(|li| {
        li.timestamp = 1_000;
    });

    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let sk = SigningKey::from_slice(&ANVIL_SK_0).unwrap();
    let signer_addr = padded_eth_address(&env, &sk);
    client.set_config(
        &SignerAddresses {
            inner: SorobanVec::from_array(&env, [signer_addr]),
        },
        &SignerGroups {
            inner: SorobanVec::from_array(&env, [0u32]),
        },
        &one_of_one_quorum(&env),
        &all_zero_parents(&env),
        &false,
    );

    let self_cid = test_support::addr_to_contract_id(&client.address, &env);
    let valid_until: u32 = 2_000_000;
    let metadata = StellarRootMetadata {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        pre_op_count: 0,
        post_op_count: 1,
        override_previous_root: false,
    };
    let meta_leaf = hash_root_metadata(&env, &domain_meta(&env), &metadata).unwrap();
    let op = StellarOp {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        nonce: 0,
        to: self_cid.clone(),
        value: BytesN::from_array(&env, &[0u8; 32]),
        data: encode_extend_all_ttls(&env),
    };
    let op_leaf = hash_stellar_op(&env, &crate::constants::domain_op(&env), &op).unwrap();
    let leaves = Vec::from([meta_leaf, op_leaf]);
    let root = merkle_root_native(&env, &leaves);
    let metadata_proof = MerkleProof {
        inner: compute_proof_for_leaf(&env, leaves.clone(), 0),
    };
    let inner = hash_set_root_inner(&env, &root, valid_until);
    let signed = eth_signed_message_hash_32(&env, &inner);
    let sigs = signature_vec_single(&env, &sk, &signed);
    client.set_root(&root, &valid_until, &metadata, &metadata_proof, &sigs);

    let empty_proof = MerkleProof {
        inner: SorobanVec::new(&env),
    };
    assert!(matches!(
        client.try_execute(&op, &empty_proof),
        Err(Ok(McmsError::ProofCannotBeVerified))
    ));
}

#[test]
fn test_execute_reverts_wrong_chain_id_op() {
    let env = Env::default();
    env.mock_all_auths();
    env.ledger().with_mut(|li| {
        li.timestamp = 1_000;
    });

    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let sk = SigningKey::from_slice(&ANVIL_SK_0).unwrap();
    let signer_addr = padded_eth_address(&env, &sk);
    client.set_config(
        &SignerAddresses {
            inner: SorobanVec::from_array(&env, [signer_addr]),
        },
        &SignerGroups {
            inner: SorobanVec::from_array(&env, [0u32]),
        },
        &one_of_one_quorum(&env),
        &all_zero_parents(&env),
        &false,
    );

    let self_cid = test_support::addr_to_contract_id(&client.address, &env);
    let valid_until: u32 = 2_000_000;
    let metadata = StellarRootMetadata {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        pre_op_count: 0,
        post_op_count: 1,
        override_previous_root: false,
    };
    let meta_leaf = hash_root_metadata(&env, &domain_meta(&env), &metadata).unwrap();
    let op = StellarOp {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        nonce: 0,
        to: self_cid.clone(),
        value: BytesN::from_array(&env, &[0u8; 32]),
        data: encode_extend_all_ttls(&env),
    };
    let op_leaf = hash_stellar_op(&env, &crate::constants::domain_op(&env), &op).unwrap();
    let leaves = Vec::from([meta_leaf, op_leaf]);
    let root = merkle_root_native(&env, &leaves);
    let metadata_proof = MerkleProof {
        inner: compute_proof_for_leaf(&env, leaves.clone(), 0),
    };
    let inner = hash_set_root_inner(&env, &root, valid_until);
    let signed = eth_signed_message_hash_32(&env, &inner);
    let sigs = signature_vec_single(&env, &sk, &signed);
    client.set_root(&root, &valid_until, &metadata, &metadata_proof, &sigs);

    let mut bad_op = op.clone();
    bad_op.chain_id = BytesN::from_array(&env, &[0x01; 32]);
    let op_proof = MerkleProof {
        inner: compute_proof_for_leaf(&env, leaves, 1),
    };
    assert!(matches!(
        client.try_execute(&bad_op, &op_proof),
        Err(Ok(McmsError::WrongChainIdOp))
    ));
}

#[test]
fn test_set_root_reverts_valid_until_expired() {
    let env = Env::default();
    env.mock_all_auths();
    env.ledger().with_mut(|li| {
        li.timestamp = 10_000;
    });

    let owner = Address::generate(&env);
    let chain = zero_chain_id(&env);
    let client = register_client(&env);
    client.initialize(&owner, &chain);

    let sk = SigningKey::from_slice(&ANVIL_SK_0).unwrap();
    let signer_addr = padded_eth_address(&env, &sk);
    client.set_config(
        &SignerAddresses {
            inner: SorobanVec::from_array(&env, [signer_addr]),
        },
        &SignerGroups {
            inner: SorobanVec::from_array(&env, [0u32]),
        },
        &one_of_one_quorum(&env),
        &all_zero_parents(&env),
        &false,
    );

    let self_cid = test_support::addr_to_contract_id(&client.address, &env);
    let valid_until: u32 = 100;
    let metadata = StellarRootMetadata {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        pre_op_count: 0,
        post_op_count: 1,
        override_previous_root: false,
    };
    let meta_leaf = hash_root_metadata(&env, &domain_meta(&env), &metadata).unwrap();
    let op = StellarOp {
        chain_id: chain.clone(),
        multisig: self_cid.clone(),
        nonce: 0,
        to: self_cid.clone(),
        value: BytesN::from_array(&env, &[0u8; 32]),
        data: encode_extend_all_ttls(&env),
    };
    let op_leaf = hash_stellar_op(&env, &crate::constants::domain_op(&env), &op).unwrap();
    let leaves = Vec::from([meta_leaf, op_leaf]);
    let root = merkle_root_native(&env, &leaves);
    let metadata_proof = MerkleProof {
        inner: compute_proof_for_leaf(&env, leaves, 0),
    };
    let inner = hash_set_root_inner(&env, &root, valid_until);
    let signed = eth_signed_message_hash_32(&env, &inner);
    let sigs = signature_vec_single(&env, &sk, &signed);

    assert!(matches!(
        client.try_set_root(&root, &valid_until, &metadata, &metadata_proof, &sigs,),
        Err(Ok(McmsError::ValidUntilHasAlreadyPassed))
    ));
}
