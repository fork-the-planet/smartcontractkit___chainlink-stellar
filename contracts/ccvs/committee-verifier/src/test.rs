#![cfg(test)]

extern crate alloc;

use super::*;
use alloc::vec::Vec as HostVec;
use common_verifier::signatures::SignatureQuorumConfig;
use ed25519_dalek::{Signer, SigningKey};
use soroban_sdk::{testutils::Address as _, vec, Address, Bytes, BytesN, Env, Vec as SorobanVec};

use crate::types::{DynamicConfig, RemoteChainConfig};
use crate::{CommitteeVerifierContract, CommitteeVerifierContractClient};
use rmn_proxy::{RmnProxyContract, RmnProxyContractClient};
use rmn_remote::{RmnRemoteContract, RmnRemoteContractClient};

/// Version tag bytes must match `VERSION_TAG_V1_7_0` in the contract.
const VERSION_TAG: [u8; 4] = [0x49, 0xff, 0x34, 0xed];

fn make_signing_key(seed: u8) -> SigningKey {
    let mut bytes = [0u8; 32];
    bytes[0] = seed;
    SigningKey::from_bytes(&bytes)
}

/// Deterministic Ed25519 keys from seeds, sorted by public key (ascending), as required by `apply_signature_configs`.
fn sorted_signers_from_seeds(seeds: &[u8]) -> HostVec<(SigningKey, [u8; 32])> {
    let mut v: HostVec<(SigningKey, [u8; 32])> = seeds
        .iter()
        .copied()
        .map(|s| {
            let sk = make_signing_key(s);
            let pk = sk.verifying_key().to_bytes();
            (sk, pk)
        })
        .collect();
    v.sort_by(|a, b| a.1.cmp(&b.1));
    v
}

fn signers_to_soroban_vec(
    env: &Env,
    pairs: &HostVec<(SigningKey, [u8; 32])>,
) -> SorobanVec<BytesN<32>> {
    let mut out = SorobanVec::new(env);
    for (_, pk) in pairs {
        out.push_back(BytesN::from_array(env, pk));
    }
    out
}

/// `keccak256(version_tag || message_hash)` — must match `verify_message` signed payload hashing.
fn keccak_signed_hash(env: &Env, message_hash: &BytesN<32>) -> BytesN<32> {
    let mut signed_payload = Bytes::new(env);
    signed_payload.append(&Bytes::from_array(env, &VERSION_TAG));
    signed_payload.append(&Bytes::from_array(env, &message_hash.to_array()));
    env.crypto().keccak256(&signed_payload).into()
}

fn build_verifier_results(env: &Env, sig_payload: &[u8]) -> Bytes {
    let len = sig_payload.len();
    assert!(len <= u16::MAX as usize);
    let b0 = ((len >> 8) & 0xff) as u8;
    let b1 = (len & 0xff) as u8;
    let mut raw: HostVec<u8> = HostVec::with_capacity(6 + len);
    raw.extend_from_slice(&VERSION_TAG);
    raw.push(b0);
    raw.push(b1);
    raw.extend_from_slice(sig_payload);
    Bytes::from_slice(env, &raw)
}

/// Build signature payload: strictly ascending pubkeys, each followed by a valid Ed25519 signature over `signed_hash`.
fn signature_payload_valid(
    pairs_in_wire_order: &[(SigningKey, [u8; 32])],
    signed_hash: &[u8; 32],
) -> HostVec<u8> {
    let mut out = HostVec::with_capacity(pairs_in_wire_order.len() * 96);
    for (sk, pk) in pairs_in_wire_order {
        let sig = sk.sign(signed_hash.as_slice());
        out.extend_from_slice(pk);
        out.extend_from_slice(&sig.to_bytes());
    }
    out
}

fn default_dynamic_config(env: &Env) -> DynamicConfig {
    DynamicConfig {
        fee_aggregator: Some(Address::generate(env)),
        allowlist_admin: None,
    }
}

fn default_remote_chain_config(env: &Env, remote_chain_selector: u64) -> RemoteChainConfig {
    RemoteChainConfig {
        remote_chain_selector,
        router: Some(Address::generate(env)),
        allowlist_enabled: false,
        fee_usd_cents: 10,
        gas_for_verification: 100_000,
        payload_size_bytes: 256,
    }
}

fn setup() -> (
    Env,
    CommitteeVerifierContractClient<'static>,
    Address,
    Address,
    SorobanVec<Bytes>,
) {
    let env = Env::default();
    env.mock_all_auths();

    let contract_id = env.register(CommitteeVerifierContract, ());
    let client = CommitteeVerifierContractClient::new(&env, &contract_id);

    let owner = Address::generate(&env);
    let storage_locations = vec![&env];

    // Initialize RMN Remote and Proxy (required for require_not_cursed / is_cursed)
    let rmn_remote_id = env.register(RmnRemoteContract, ());
    let rmn_remote_client = RmnRemoteContractClient::new(&env, &rmn_remote_id);
    rmn_remote_client.initialize(&owner, &1u64);

    let rmn_proxy = env.register(RmnProxyContract, ());
    let rmn_proxy_client = RmnProxyContractClient::new(&env, &rmn_proxy);
    rmn_proxy_client.initialize(&owner, &rmn_remote_id);

    let dynamic_config = default_dynamic_config(&env);
    client.initialize(&owner, &dynamic_config, &storage_locations, &rmn_proxy);

    (env, client, owner, rmn_proxy, storage_locations)
}

// ============================================================
// Initialization Tests
// ============================================================

#[test]
fn test_initialize() {
    let (_env, client, owner, _rmn_proxy, _storage_locations) = setup();
    assert_eq!(client.owner(), Some(owner));
}

#[test]
#[should_panic(expected = "Error(Contract, #2)")] // AlreadyInitialized
fn test_double_initialize_fails() {
    let (env, client, _owner, _rmn_proxy, storage_locations) = setup();
    let owner2 = Address::generate(&env);
    let dynamic_config = default_dynamic_config(&env);

    client.initialize(
        &owner2,
        &dynamic_config,
        &storage_locations,
        &Address::generate(&env),
    );
}

// ============================================================
// Version Tag Tests
// ============================================================

#[test]
fn test_version_tag() {
    let (env, client, ..) = setup();

    let expected = BytesN::from_array(&env, &[0x49, 0xff, 0x34, 0xed]);
    assert_eq!(client.version_tag(), expected);
}

// ============================================================
// Dynamic Config Tests
// ============================================================

#[test]
fn test_get_and_set_dynamic_config() {
    let (env, client, _owner, ..) = setup();

    let new_fee_aggregator = Address::generate(&env);
    let new_config = DynamicConfig {
        fee_aggregator: Some(new_fee_aggregator.clone()),
        allowlist_admin: Some(Address::generate(&env)),
    };

    client.set_dynamic_config(&new_config);

    let stored = client.get_dynamic_config();
    assert_eq!(stored.fee_aggregator, Some(new_fee_aggregator));
}

// ============================================================
// Remote Chain Config Tests
// ============================================================

#[test]
fn test_apply_remote_chain_config_and_get_fee() {
    let (env, client, _owner, ..) = setup();

    let dest_chain: u64 = 12345;
    let remote_config = default_remote_chain_config(&env, dest_chain);

    client.apply_remote_chain_cfg_updates(&vec![&env, remote_config.clone()]);

    let retrieved = client.get_remote_chain_config(&dest_chain);
    assert_eq!(retrieved.remote_chain_selector, dest_chain);
    assert_eq!(retrieved.fee_usd_cents, 10);
    assert_eq!(retrieved.gas_for_verification, 100_000);

    let FeeResponse {
        fee,
        dest_gas_limit,
        dest_bytes_overhead,
    } = client.get_fee(&dest_chain, &Bytes::new(&env), &Bytes::new(&env), &0u32);
    assert_eq!(fee, 10);
    assert_eq!(dest_gas_limit, 100_000);
    assert_eq!(dest_bytes_overhead, 256);
}

#[test]
#[should_panic(expected = "Error(Contract, #48)")] // RemoteChainNotSupported
fn test_get_remote_chain_config_fails_when_not_configured() {
    let (_env, client, ..) = setup();

    client.get_remote_chain_config(&99999);
}

#[test]
#[should_panic(expected = "Error(Contract, #48)")] // RemoteChainNotSupported
fn test_get_fee_fails_when_chain_not_configured() {
    let (env, client, ..) = setup();

    client.get_fee(&99999, &Bytes::new(&env), &Bytes::new(&env), &0u32);
}

// ============================================================
// Storage Locations Tests
// ============================================================

#[test]
fn test_update_storage_locations() {
    let (env, client, _owner, ..) = setup();

    let new_locations = vec![
        &env,
        Bytes::from_slice(&env, b"location1"),
        Bytes::from_slice(&env, b"location2"),
    ];

    client.update_storage_locations(&new_locations);

    let stored = client.get_storage_locations();
    assert_eq!(stored.len(), 2);
}

// ============================================================
// Ownership Tests
// ============================================================

#[test]
#[ignore]
fn test_transfer_ownership() {
    let (env, client, owner, ..) = setup();

    let new_owner = Address::generate(&env);
    let _ = <CommitteeVerifierContract as common_authorization::Ownable>::transfer_ownership(
        &env, &new_owner,
    );

    env.as_contract(&client.address, || {
        assert_eq!(CommitteeVerifierContract::owner(&env).unwrap(), owner); // Pending, not yet accepted
        let _ =
            <CommitteeVerifierContract as common_authorization::Ownable>::accept_ownership(&env);
        assert_eq!(CommitteeVerifierContract::owner(&env).unwrap(), new_owner);
    });
}

// ============================================================
// Forward to Verifier Tests
// ============================================================

#[test]
fn test_forward_to_verifier_passes_when_allowlist_not_enabled_for_chain() {
    let (env, client, _owner, ..) = setup();

    let dest_chain: u64 = 12345;
    let sender = Address::generate(&env);
    let message_id = BytesN::from_array(&env, &[0u8; 32]);
    let fee_token = Address::generate(&env);
    let verifier_args = Bytes::new(&env);

    client.forward_to_verifier(
        &dest_chain,
        &sender,
        &message_id,
        &fee_token,
        &0,
        &verifier_args,
    );
}

#[test]
#[should_panic(expected = "Error(Contract, #6)")] // CallerNotAuthorized
fn test_forward_to_verifier_fails_when_sender_not_in_allowlist() {
    let (env, client, _owner, ..) = setup();

    let dest_chain: u64 = 12345;
    let sender = Address::generate(&env);
    let message_id = BytesN::from_array(&env, &[0u8; 32]);
    let fee_token = Address::generate(&env);
    let verifier_args = Bytes::new(&env);

    client.apply_allowlist_updates(&vec![
        &env,
        AllowListUpdate {
            dest_chain_selector: dest_chain,
            allowlist_enabled: true,
            added_allowlisted_senders: vec![&env],
            removed_allowlisted_senders: vec![&env],
        },
    ]);

    client.forward_to_verifier(
        &dest_chain,
        &sender,
        &message_id,
        &fee_token,
        &0,
        &verifier_args,
    );
}

// ============================================================
// Verify Message Tests
// ============================================================

#[test]
#[should_panic(expected = "Error(Contract, #20)")] // InvalidVerifierResults
fn test_verify_message_fails_when_verifier_results_too_short() {
    let (env, client, ..) = setup();

    let source_chain: u64 = 1;
    let message_hash = BytesN::from_array(&env, &[0u8; 32]);
    let short_results = Bytes::from_slice(&env, &[0x49, 0xff]); // Only 2 bytes, need at least 6

    client.verify_message(&source_chain, &message_hash, &short_results);
}

#[test]
#[should_panic(expected = "Error(Contract, #59)")] // InvalidCCVVersion
fn test_verify_message_fails_when_wrong_version() {
    let (env, client, ..) = setup();

    let source_chain: u64 = 1;
    let message_hash = BytesN::from_array(&env, &[0u8; 32]);
    // Wrong version tag (0x00,0x00,0x00,0x00) + 2 bytes sig len (0x00, 0x00) + 0 sig bytes
    let wrong_version_results = Bytes::from_slice(&env, &[0x00, 0x00, 0x00, 0x00, 0x00, 0x00]);

    client.verify_message(&source_chain, &message_hash, &wrong_version_results);
}

#[test]
#[should_panic(expected = "Error(Contract, #19)")] // SourceNotConfigured
fn test_verify_message_fails_when_source_chain_not_configured() {
    let (env, client, ..) = setup();

    let source_chain: u64 = 99999; // Not configured
    let message_hash = BytesN::from_array(&env, &[0u8; 32]);
    // Correct version + sig len (0, 0) + no signatures - will fail at validate_signatures
    let verifier_results = Bytes::from_slice(&env, &[0x49, 0xff, 0x34, 0xed, 0x00, 0x00]);

    client.verify_message(&source_chain, &message_hash, &verifier_results);
}

// ============================================================
// SignatureQuorum — config management
// ============================================================

#[test]
fn test_apply_signature_configs_and_get() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 100;
    let pairs = sorted_signers_from_seeds(&[3, 7, 11]);
    let signers = signers_to_soroban_vec(&env, &pairs);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 2,
        signers: signers.clone(),
    };

    client.apply_signature_configs(&vec![&env], &vec![&env, cfg.clone()]);

    let got = client.get_signature_config(&source_chain);
    assert_eq!(got.source_chain_selector, source_chain);
    assert_eq!(got.threshold, 2);
    assert_eq!(got.signers.len(), 3);
    for i in 0..3 {
        assert_eq!(got.signers.get(i).unwrap(), signers.get(i).unwrap());
    }
}

#[test]
#[should_panic(expected = "Error(Contract, #19)")] // SourceSignersNotConfigured
fn test_apply_signature_configs_remove() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 101;
    let pairs = sorted_signers_from_seeds(&[5, 9, 13]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 2,
        signers: signers_to_soroban_vec(&env, &pairs),
    };

    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);
    client.apply_signature_configs(&vec![&env, source_chain], &vec![&env]);

    client.get_signature_config(&source_chain);
}

#[test]
fn test_get_all_signature_configs() {
    let (env, client, _owner, ..) = setup();

    let pairs_a = sorted_signers_from_seeds(&[2, 4, 6]);
    let pairs_b = sorted_signers_from_seeds(&[8, 10, 12]);

    let cfg_a = SignatureQuorumConfig {
        source_chain_selector: 200,
        threshold: 2,
        signers: signers_to_soroban_vec(&env, &pairs_a),
    };
    let cfg_b = SignatureQuorumConfig {
        source_chain_selector: 201,
        threshold: 1,
        signers: signers_to_soroban_vec(&env, &pairs_b),
    };

    client.apply_signature_configs(&vec![&env], &vec![&env, cfg_a, cfg_b]);

    let all = client.get_all_signature_configs();
    assert_eq!(all.len(), 2);

    let c0 = all.get(0).unwrap();
    let c1 = all.get(1).unwrap();
    assert!(
        (c0.source_chain_selector == 200 && c1.source_chain_selector == 201)
            || (c0.source_chain_selector == 201 && c1.source_chain_selector == 200)
    );

    let mut saw_200 = false;
    let mut saw_201 = false;
    for i in 0..2 {
        let c = all.get(i).unwrap();
        match c.source_chain_selector {
            200 => {
                assert_eq!(c.threshold, 2);
                saw_200 = true;
            }
            201 => {
                assert_eq!(c.threshold, 1);
                saw_201 = true;
            }
            _ => panic!("unexpected source_chain_selector"),
        }
    }
    assert!(saw_200 && saw_201);
}

#[test]
#[should_panic(expected = "Error(Contract, #17)")] // InvalidSignatureThreshold
fn test_apply_signature_configs_rejects_zero_threshold() {
    let (env, client, _owner, ..) = setup();

    let pairs = sorted_signers_from_seeds(&[17, 19]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: 300,
        threshold: 0,
        signers: signers_to_soroban_vec(&env, &pairs),
    };

    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);
}

#[test]
#[should_panic(expected = "Error(Contract, #17)")] // InvalidSignatureThreshold
fn test_apply_signature_configs_rejects_threshold_exceeding_signers() {
    let (env, client, _owner, ..) = setup();

    let pairs = sorted_signers_from_seeds(&[21, 23]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: 301,
        threshold: 3,
        signers: signers_to_soroban_vec(&env, &pairs),
    };

    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);
}

#[test]
#[should_panic(expected = "Error(Contract, #66)")] // DuplicateOnchainPublicKey
fn test_apply_signature_configs_rejects_duplicate_signers() {
    let (env, client, _owner, ..) = setup();

    let pk = BytesN::from_array(&env, &[7u8; 32]);
    let mut signers = SorobanVec::new(&env);
    signers.push_back(pk.clone());
    signers.push_back(pk);

    let cfg = SignatureQuorumConfig {
        source_chain_selector: 302,
        threshold: 1,
        signers,
    };

    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);
}

#[test]
#[should_panic(expected = "Error(Contract, #67)")] // InvalidSignerOrder
fn test_apply_signature_configs_rejects_unordered_signers() {
    let (env, client, _owner, ..) = setup();

    let hi = BytesN::from_array(&env, &[0xFFu8; 32]);
    let lo = BytesN::from_array(&env, &[0x00u8; 32]);
    let mut signers = SorobanVec::new(&env);
    signers.push_back(hi);
    signers.push_back(lo);

    let cfg = SignatureQuorumConfig {
        source_chain_selector: 303,
        threshold: 1,
        signers,
    };

    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);
}

// ============================================================
// SignatureQuorum — verify_message (Ed25519 quorum)
// ============================================================

#[test]
fn test_verify_message_with_valid_signatures() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 400;
    let pairs = sorted_signers_from_seeds(&[29, 31, 37]);
    let signers = signers_to_soroban_vec(&env, &pairs);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 2,
        signers,
    };
    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);

    let message_hash = BytesN::from_array(&env, &[0xabu8; 32]);
    let signed_hash = keccak_signed_hash(&env, &message_hash);
    let signed_bytes: [u8; 32] = signed_hash.to_array();

    let wire_subset = [pairs[0].clone(), pairs[1].clone()];
    let sig_payload = signature_payload_valid(&wire_subset, &signed_bytes);
    let verifier_results = build_verifier_results(&env, &sig_payload);

    client.verify_message(&source_chain, &message_hash, &verifier_results);
}

#[test]
#[should_panic(expected = "Error(Contract, #71)")] // ThresholdNotMet
fn test_verify_message_fails_below_threshold() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 401;
    let pairs = sorted_signers_from_seeds(&[41, 43, 47]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 2,
        signers: signers_to_soroban_vec(&env, &pairs),
    };
    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);

    let message_hash = BytesN::from_array(&env, &[0xcd_u8; 32]);
    let signed_hash = keccak_signed_hash(&env, &message_hash);
    let signed_bytes: [u8; 32] = signed_hash.to_array();

    let wire_subset = [pairs[0].clone()];
    let sig_payload = signature_payload_valid(&wire_subset, &signed_bytes);
    let verifier_results = build_verifier_results(&env, &sig_payload);

    client.verify_message(&source_chain, &message_hash, &verifier_results);
}

#[test]
#[should_panic(expected = "Error(Contract, #70)")] // OutOfOrderSignatures
fn test_verify_message_fails_with_out_of_order_signatures() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 402;
    let pairs = sorted_signers_from_seeds(&[53, 59, 61]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 2,
        signers: signers_to_soroban_vec(&env, &pairs),
    };
    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);

    let message_hash = BytesN::from_array(&env, &[0x11u8; 32]);
    let signed_hash = keccak_signed_hash(&env, &message_hash);
    let signed_bytes: [u8; 32] = signed_hash.to_array();

    // Wire order: larger pubkey first (descending) — violates strictly ascending requirement.
    let wire_desc = [pairs[2].clone(), pairs[0].clone()];
    let sig_payload = signature_payload_valid(&wire_desc, &signed_bytes);
    let verifier_results = build_verifier_results(&env, &sig_payload);

    client.verify_message(&source_chain, &message_hash, &verifier_results);
}

#[test]
#[should_panic(expected = "Error(Contract, #70)")] // OutOfOrderSignatures
fn test_verify_message_fails_with_duplicate_signatures() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 403;
    let pairs = sorted_signers_from_seeds(&[67, 71, 73]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 2,
        signers: signers_to_soroban_vec(&env, &pairs),
    };
    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);

    let message_hash = BytesN::from_array(&env, &[0x22u8; 32]);
    let signed_hash = keccak_signed_hash(&env, &message_hash);
    let signed_bytes: [u8; 32] = signed_hash.to_array();

    let p0 = pairs[0].clone();
    let wire_dup = [p0.clone(), p0];
    let sig_payload = signature_payload_valid(&wire_dup, &signed_bytes);
    let verifier_results = build_verifier_results(&env, &sig_payload);

    client.verify_message(&source_chain, &message_hash, &verifier_results);
}

#[test]
#[should_panic(expected = "Error(Contract, #72)")] // UnexpectedSigner
fn test_verify_message_fails_with_unknown_signer() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 404;
    let pairs = sorted_signers_from_seeds(&[79, 83, 89]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 1,
        signers: signers_to_soroban_vec(&env, &pairs),
    };
    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);

    let outsider = sorted_signers_from_seeds(&[97]);
    let message_hash = BytesN::from_array(&env, &[0x33u8; 32]);
    let signed_hash = keccak_signed_hash(&env, &message_hash);
    let signed_bytes: [u8; 32] = signed_hash.to_array();

    let wire = [outsider[0].clone()];
    let sig_payload = signature_payload_valid(&wire, &signed_bytes);
    let verifier_results = build_verifier_results(&env, &sig_payload);

    client.verify_message(&source_chain, &message_hash, &verifier_results);
}

#[test]
#[should_panic]
fn test_verify_message_fails_with_invalid_signature() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 405;
    let pairs = sorted_signers_from_seeds(&[101, 103, 107]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 1,
        signers: signers_to_soroban_vec(&env, &pairs),
    };
    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);

    let message_hash = BytesN::from_array(&env, &[0x44u8; 32]);
    let pk = pairs[0].1;
    let mut sig_payload = HostVec::with_capacity(96);
    sig_payload.extend_from_slice(&pk);
    sig_payload.extend_from_slice(&[0xEEu8; 64]);

    let verifier_results = build_verifier_results(&env, &sig_payload);
    client.verify_message(&source_chain, &message_hash, &verifier_results);
}

#[test]
#[should_panic(expected = "Error(Contract, #14)")] // InvalidSignatureLength
fn test_verify_message_fails_with_malformed_payload() {
    let (env, client, _owner, ..) = setup();

    let source_chain: u64 = 406;
    let pairs = sorted_signers_from_seeds(&[109, 113]);
    let cfg = SignatureQuorumConfig {
        source_chain_selector: source_chain,
        threshold: 1,
        signers: signers_to_soroban_vec(&env, &pairs),
    };
    client.apply_signature_configs(&vec![&env], &vec![&env, cfg]);

    let message_hash = BytesN::from_array(&env, &[0x55u8; 32]);
    let signed_hash = keccak_signed_hash(&env, &message_hash);
    let signed_bytes: [u8; 32] = signed_hash.to_array();

    let mut sig_payload = signature_payload_valid(&[pairs[0].clone()], &signed_bytes);
    sig_payload.truncate(95); // not a multiple of 96

    let verifier_results = build_verifier_results(&env, &sig_payload);
    client.verify_message(&source_chain, &message_hash, &verifier_results);
}
