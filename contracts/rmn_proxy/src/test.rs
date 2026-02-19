#![cfg(test)]

use super::*;
use soroban_sdk::{testutils::Address as _, Address, Env};

fn setup_env() -> (Env, Address, Address, Address) {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let rmn = Address::generate(&env);
    let contract_id = env.register(RmnProxyContract, ());

    (env, contract_id, owner, rmn)
}

#[test]
fn test_initialize() {
    let (env, contract_id, owner, rmn) = setup_env();
    let client = RmnProxyContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn);

    assert_eq!(client.owner(), owner);
    assert_eq!(client.get_rmn(), rmn);
}

#[test]
#[should_panic(expected = "Error(Contract, #2)")] // AlreadyInitialized
fn test_double_initialize_fails() {
    let (env, contract_id, owner, rmn) = setup_env();
    let client = RmnProxyContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn);
    client.initialize(&owner, &rmn);
}

#[test]
fn test_set_rmn() {
    let (env, contract_id, owner, rmn) = setup_env();
    let client = RmnProxyContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn);

    let new_rmn = Address::generate(&env);
    client.set_rmn(&new_rmn);

    assert_eq!(client.get_rmn(), new_rmn);
}

#[test]
fn test_is_cursed_returns_false() {
    let (env, contract_id, owner, rmn) = setup_env();
    let client = RmnProxyContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn);

    // Should return false (not cursed) since no RMN Remote exists yet
    assert!(!client.is_cursed());
}

#[test]
fn test_transfer_ownership_two_step() {
    let (env, contract_id, owner, rmn) = setup_env();
    let client = RmnProxyContractClient::new(&env, &contract_id);

    client.initialize(&owner, &rmn);

    let new_owner = Address::generate(&env);

    // Step 1: Initiate transfer
    client.transfer_ownership(&new_owner);

    // Owner should still be the original owner until accepted
    assert_eq!(client.owner(), owner);

    // Step 2: Accept transfer
    client.accept_ownership();

    // Now the new owner should be set
    assert_eq!(client.owner(), new_owner);
}
