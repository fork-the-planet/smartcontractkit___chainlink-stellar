//! Integration-style contract tests (Soroban `Env`).

#![cfg(test)]

use soroban_sdk::testutils::Address as _;
use soroban_sdk::{Address, Bytes, BytesN, Env, Vec as SorobanVec};

use crate::error::McmsError;
use crate::types::{MerkleProof, SignatureVec, SignerAddresses, SignerGroups, StellarOp, StellarRootMetadata};
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
    let metadata_proof = MerkleProof { inner: SorobanVec::new(&env) };
    let signatures = SignatureVec { inner: SorobanVec::new(&env) };

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
    let addrs = SignerAddresses { inner: SorobanVec::from_array(&env, [s]) };
    let groups = SignerGroups { inner: SorobanVec::from_array(&env, [0u32]) };

    client.set_config(&addrs, &groups, &one_of_one_quorum(&env), &all_zero_parents(&env), &true);

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
