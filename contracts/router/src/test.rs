#![cfg(test)]

use super::*;
use soroban_sdk::{testutils::Address as _, Address, Env, Vec};

fn setup_env() -> (Env, Address, Address, Address) {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let rmn_proxy = Address::generate(&env);
    let contract_id = env.register(RouterContract, ());

    (env, contract_id, owner, rmn_proxy)
}

#[test]
fn test_initialize() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    // Initialize
    client.initialize(&owner, &rmn_proxy);

    // Verify owner
    assert_eq!(client.owner(), owner);

    // Verify config
    let config = client.get_config();
    assert_eq!(config.rmn_proxy, rmn_proxy);
}

#[test]
#[should_panic(expected = "Error(Contract, #1)")]
fn test_initialize_already_initialized() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    // Initialize twice should fail
    client.initialize(&owner, &rmn_proxy);
    client.initialize(&owner, &rmn_proxy);
}

#[test]
fn test_set_onramp() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    let onramp = Address::generate(&env);
    let dest_chain_selector: u64 = 123;

    // Set OnRamp
    client.set_onramp(&dest_chain_selector, &onramp);

    // Verify
    assert_eq!(client.get_onramp(&dest_chain_selector), onramp);
    assert!(client.is_chain_supported(&dest_chain_selector));
}

#[test]
fn test_add_remove_offramp() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    let offramp = Address::generate(&env);
    let source_chain_selector: u64 = 456;

    // Add OffRamp
    client.add_offramp(&source_chain_selector, &offramp);

    // Verify added
    assert!(client.is_offramp(&source_chain_selector, &offramp));

    let offramps = client.get_offramps();
    assert_eq!(offramps.len(), 1);
    assert_eq!(offramps.get(0).unwrap().source_chain_selector, source_chain_selector);
    assert_eq!(offramps.get(0).unwrap().offramp, offramp);

    // Remove OffRamp
    client.remove_offramp(&source_chain_selector, &offramp);

    // Verify removed
    assert!(!client.is_offramp(&source_chain_selector, &offramp));
    assert_eq!(client.get_offramps().len(), 0);
}

#[test]
#[should_panic(expected = "Error(Contract, #12)")]
fn test_add_duplicate_offramp() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    let offramp = Address::generate(&env);
    let source_chain_selector: u64 = 456;

    // Add same OffRamp twice should fail
    client.add_offramp(&source_chain_selector, &offramp);
    client.add_offramp(&source_chain_selector, &offramp);
}

#[test]
#[should_panic(expected = "Error(Contract, #8)")]
fn test_remove_nonexistent_offramp() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    let offramp = Address::generate(&env);
    let source_chain_selector: u64 = 456;

    // Remove non-existent OffRamp should fail
    client.remove_offramp(&source_chain_selector, &offramp);
}

#[test]
fn test_apply_ramp_updates() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    let onramp1 = Address::generate(&env);
    let onramp2 = Address::generate(&env);
    let offramp1 = Address::generate(&env);
    let offramp2 = Address::generate(&env);

    // Create update vectors
    let mut onramp_updates: Vec<OnRampEntry> = Vec::new(&env);
    onramp_updates.push_back(OnRampEntry {
        dest_chain_selector: 100,
        onramp: onramp1.clone(),
    });
    onramp_updates.push_back(OnRampEntry {
        dest_chain_selector: 200,
        onramp: onramp2.clone(),
    });

    let offramp_removes: Vec<OffRampEntry> = Vec::new(&env);

    let mut offramp_adds: Vec<OffRampEntry> = Vec::new(&env);
    offramp_adds.push_back(OffRampEntry {
        source_chain_selector: 300,
        offramp: offramp1.clone(),
    });
    offramp_adds.push_back(OffRampEntry {
        source_chain_selector: 400,
        offramp: offramp2.clone(),
    });

    // Apply updates
    client.apply_ramp_updates(&onramp_updates, &offramp_removes, &offramp_adds);

    // Verify OnRamps
    assert_eq!(client.get_onramp(&100), onramp1);
    assert_eq!(client.get_onramp(&200), onramp2);

    // Verify OffRamps
    assert!(client.is_offramp(&300, &offramp1));
    assert!(client.is_offramp(&400, &offramp2));
}

#[test]
fn test_transfer_ownership() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    let new_owner = Address::generate(&env);

    // Transfer ownership
    client.transfer_ownership(&new_owner);

    // Verify new owner
    assert_eq!(client.owner(), new_owner);
}

#[test]
fn test_get_onramps() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    let onramp1 = Address::generate(&env);
    let onramp2 = Address::generate(&env);

    client.set_onramp(&100, &onramp1);
    client.set_onramp(&200, &onramp2);

    let onramps = client.get_onramps();
    assert_eq!(onramps.len(), 2);
}

#[test]
fn test_is_chain_supported_false() {
    let (env, contract_id, owner, rmn_proxy) = setup_env();
    let client = RouterContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn_proxy);

    // Non-configured chain should not be supported
    assert!(!client.is_chain_supported(&999));
}
