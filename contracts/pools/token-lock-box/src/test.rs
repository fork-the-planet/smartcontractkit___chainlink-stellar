#![cfg(test)]

extern crate std;

use soroban_sdk::{testutils::Address as _, token, vec, Address, Env};

use crate::{TokenLockBox, TokenLockBoxClient};

fn setup() -> (Env, Address, Address, Address, TokenLockBoxClient<'static>) {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let token_admin = Address::generate(&env);
    let token_contract = env.register_stellar_asset_contract_v2(token_admin.clone());
    let token_addr = token_contract.address();

    let lockbox_id = env.register(TokenLockBox, ());
    let client = TokenLockBoxClient::new(&env, &lockbox_id);
    client.initialize(&owner, &token_addr);

    (env, owner, token_addr, token_admin, client)
}

#[test]
fn initialize_and_get_token() {
    let (_env, _owner, token_addr, _admin, client) = setup();
    assert_eq!(client.get_token(), token_addr);
    assert!(client.is_token_supported(&token_addr));
}

#[test]
fn deposit_and_withdraw() {
    let (env, _owner, token_addr, _token_admin, client) = setup();

    let pool = Address::generate(&env);
    client.add_allowed_callers(&vec![&env, pool.clone()]);

    let sac = token::StellarAssetClient::new(&env, &token_addr);
    sac.mint(&pool, &1_000);

    let tc = token::Client::new(&env, &token_addr);
    let exp = env.ledger().sequence().saturating_add(10_000);
    tc.approve(&pool, &client.address, &500, &exp);
    client.deposit(&pool, &500);
    assert_eq!(tc.balance(&pool), 500);
    assert_eq!(tc.balance(&client.address), 500);

    let receiver = Address::generate(&env);
    client.withdraw(&pool, &200, &receiver);
    assert_eq!(tc.balance(&receiver), 200);
    assert_eq!(tc.balance(&client.address), 300);
}

#[test]
fn withdraw_insufficient_balance() {
    let (env, _owner, token_addr, _token_admin, client) = setup();

    let pool = Address::generate(&env);
    client.add_allowed_callers(&vec![&env, pool.clone()]);

    let sac = token::StellarAssetClient::new(&env, &token_addr);
    sac.mint(&pool, &100);
    let tc = token::Client::new(&env, &token_addr);
    let exp = env.ledger().sequence().saturating_add(10_000);
    tc.approve(&pool, &client.address, &100, &exp);
    client.deposit(&pool, &100);

    let receiver = Address::generate(&env);
    let r = client.try_withdraw(&pool, &200, &receiver);
    assert!(r.is_err());
}

#[test]
fn unauthorized_caller_rejected() {
    let (env, _owner, _token_addr, _admin, client) = setup();

    let stranger = Address::generate(&env);
    let r = client.try_deposit(&stranger, &100);
    assert!(r.is_err());
}

#[test]
fn add_and_remove_callers() {
    let (env, _owner, _token_addr, _admin, client) = setup();

    let a = Address::generate(&env);
    let b = Address::generate(&env);
    client.add_allowed_callers(&vec![&env, a.clone(), b.clone()]);

    let callers = client.get_allowed_callers();
    assert_eq!(callers.len(), 2);

    client.remove_allowed_callers(&vec![&env, a.clone()]);
    let callers2 = client.get_allowed_callers();
    assert_eq!(callers2.len(), 1);
    assert_eq!(callers2.get(0).unwrap(), b);
}

#[test]
fn deposit_zero_rejected() {
    let (env, _owner, _token_addr, _admin, client) = setup();

    let pool = Address::generate(&env);
    client.add_allowed_callers(&vec![&env, pool.clone()]);

    let r = client.try_deposit(&pool, &0);
    assert!(r.is_err());
}
