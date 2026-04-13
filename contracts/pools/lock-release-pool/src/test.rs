#![cfg(test)]

use soroban_sdk::{testutils::Address as _, token, Address, Bytes, Env, Vec};

use crate::{LockReleaseTokenPoolContract, LockReleaseTokenPoolContractClient};
use common_error::CCIPError;
use common_pool::{encode_local_decimals, ChainUpdate, LockOrBurnIn, ReleaseOrMintIn};

fn setup_env() -> (
    Env,
    LockReleaseTokenPoolContractClient<'static>,
    Address,
    Address,
    token::Client<'static>,
    token::StellarAssetClient<'static>,
) {
    let env = Env::default();
    env.mock_all_auths_allowing_non_root_auth();

    let pool_id = env.register(LockReleaseTokenPoolContract, ());
    let pool_client = LockReleaseTokenPoolContractClient::new(&env, &pool_id);

    let owner = Address::generate(&env);
    let token_admin = Address::generate(&env);
    let token_contract = env.register_stellar_asset_contract_v2(token_admin.clone());
    let token_address = token_contract.address();
    let token_client = token::Client::new(&env, &token_address);
    let token_admin_client = token::StellarAssetClient::new(&env, &token_address);

    pool_client.initialize(&owner, &token_address, &7u32);

    (
        env,
        pool_client,
        owner,
        token_address,
        token_client,
        token_admin_client,
    )
}

#[test]
fn test_initialize() {
    let (env, pool_client, _owner, token_address, _token_client, _token_admin_client) = setup_env();

    let pool_token = pool_client.get_token();
    assert_eq!(pool_token, token_address);
    assert_eq!(pool_client.get_token_decimals(), 7);

    assert!(pool_client.is_supported_token(&token_address));
    let other_token = Address::generate(&env);
    assert!(!pool_client.is_supported_token(&other_token));
}

#[test]
fn test_lock_and_release() {
    let (env, pool_client, _owner, token_address, token_client, token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    let remote_pool = Bytes::from_slice(&env, &[1u8; 20]);
    let remote_token = Bytes::from_slice(&env, &[2u8; 20]);

    let chain_update = ChainUpdate {
        remote_chain_selector: remote_chain,
        remote_pool_addresses: remote_pool,
        remote_token_address: remote_token.clone(),
    };
    pool_client.apply_chain_updates(&Vec::from_array(&env, [chain_update]), &Vec::new(&env));

    let sender = Address::generate(&env);
    let lock_amount: i128 = 1_000_000_000;
    token_admin_client.mint(&sender, &lock_amount);

    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender.clone(),
        amount: lock_amount,
        local_token: token_address.clone(),
    };

    let lock_result = pool_client.lock_or_burn(&lock_input);
    assert_eq!(lock_result.dest_token_address, remote_token);

    let pool_address = pool_client.address.clone();
    assert_eq!(token_client.balance(&pool_address), lock_amount);
    assert_eq!(token_client.balance(&sender), 0);

    let receiver = Address::generate(&env);
    let release_input = ReleaseOrMintIn {
        original_sender: Bytes::from_slice(&env, &[4u8; 20]),
        remote_chain_selector: remote_chain,
        receiver: receiver.clone(),
        amount: lock_amount,
        local_token: token_address.clone(),
        source_pool_address: Bytes::from_slice(&env, &[5u8; 20]),
        source_pool_data: Bytes::new(&env),
    };

    let release_result = pool_client.release_or_mint(&release_input);
    assert_eq!(release_result.destination_amount, lock_amount);
    assert_eq!(token_client.balance(&receiver), lock_amount);
    assert_eq!(token_client.balance(&pool_address), 0);
}

#[test]
fn test_unsupported_chain_rejected() {
    let (env, pool_client, _owner, token_address, _token_client, token_admin_client) = setup_env();

    let sender = Address::generate(&env);
    token_admin_client.mint(&sender, &1_000_000_000);

    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[1u8; 20]),
        remote_chain_selector: 999,
        original_sender: sender,
        amount: 100,
        local_token: token_address,
    };

    let result = pool_client.try_lock_or_burn(&lock_input);
    assert!(result.is_err());
}

#[test]
fn test_wrong_token_rejected() {
    let (env, pool_client, _owner, _token_address, _token_client, _token_admin_client) =
        setup_env();

    let wrong_token = Address::generate(&env);
    let sender = Address::generate(&env);

    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[1u8; 20]),
        remote_chain_selector: 1,
        original_sender: sender,
        amount: 100,
        local_token: wrong_token,
    };

    let result = pool_client.try_lock_or_burn(&lock_input);
    assert!(result.is_err());
}

fn chain_update(env: &Env, selector: u64, pool_byte: u8, token_byte: u8) -> ChainUpdate {
    ChainUpdate {
        remote_chain_selector: selector,
        remote_pool_addresses: Bytes::from_slice(env, &[pool_byte; 20]),
        remote_token_address: Bytes::from_slice(env, &[token_byte; 20]),
    }
}

#[test]
#[should_panic(expected = "Error(Contract, #2)")] // AlreadyInitialized
fn test_initialize_twice_rejected() {
    let (_env, pool_client, owner, token_address, _token_client, _token_admin_client) = setup_env();
    pool_client.initialize(&owner, &token_address, &7u32);
}

#[test]
fn test_lock_or_burn_zero_amount_succeeds_when_chain_configured() {
    let (env, pool_client, _owner, token_address, token_client, _token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );

    let sender = Address::generate(&env);
    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender.clone(),
        amount: 0,
        local_token: token_address.clone(),
    };

    let out = pool_client.lock_or_burn(&lock_input);
    assert_eq!(out.dest_token_address, Bytes::from_slice(&env, &[2u8; 20]));
    assert_eq!(token_client.balance(&pool_client.address), 0);
    assert_eq!(token_client.balance(&sender), 0);
}

#[test]
fn test_release_or_mint_zero_amount_succeeds_without_pool_balance() {
    let (env, pool_client, _owner, token_address, token_client, _token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );

    let receiver = Address::generate(&env);
    let release_input = ReleaseOrMintIn {
        original_sender: Bytes::from_slice(&env, &[4u8; 20]),
        remote_chain_selector: remote_chain,
        receiver: receiver.clone(),
        amount: 0,
        local_token: token_address,
        source_pool_address: Bytes::from_slice(&env, &[5u8; 20]),
        source_pool_data: Bytes::new(&env),
    };

    let out = pool_client.release_or_mint(&release_input);
    assert_eq!(out.destination_amount, 0);
    assert_eq!(token_client.balance(&receiver), 0);
}

#[test]
fn test_lock_or_burn_amount_exceeds_sender_balance_fails() {
    let (env, pool_client, _owner, token_address, _token_client, token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );

    let sender = Address::generate(&env);
    token_admin_client.mint(&sender, &100);

    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender,
        amount: 101,
        local_token: token_address,
    };

    let result = pool_client.try_lock_or_burn(&lock_input);
    assert!(result.is_err());
}

#[test]
fn test_lock_or_burn_negative_amount_fails() {
    let (env, pool_client, _owner, token_address, _token_client, token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );

    let sender = Address::generate(&env);
    token_admin_client.mint(&sender, &1_000);

    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender,
        amount: -1,
        local_token: token_address,
    };

    let result = pool_client.try_lock_or_burn(&lock_input);
    assert!(result.is_err());
}

#[test]
fn test_release_or_mint_insufficient_pool_liquidity() {
    let (env, pool_client, _owner, token_address, _token_client, token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );

    let sender = Address::generate(&env);
    let locked: i128 = 50;
    token_admin_client.mint(&sender, &locked);

    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender,
        amount: locked,
        local_token: token_address.clone(),
    };
    pool_client.lock_or_burn(&lock_input);

    let receiver = Address::generate(&env);
    let release_input = ReleaseOrMintIn {
        original_sender: Bytes::from_slice(&env, &[4u8; 20]),
        remote_chain_selector: remote_chain,
        receiver,
        amount: locked + 1,
        local_token: token_address,
        source_pool_address: Bytes::from_slice(&env, &[5u8; 20]),
        source_pool_data: Bytes::new(&env),
    };

    let result = pool_client.try_release_or_mint(&release_input);
    assert_eq!(result, Err(Ok(CCIPError::InsufficientPoolLiquidity)));
}

#[test]
fn test_apply_chain_updates_remove_unlists_chain() {
    let (env, pool_client, _owner, token_address, _token_client, _token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );
    assert!(pool_client.is_supported_chain(&remote_chain));

    pool_client.apply_chain_updates(&Vec::new(&env), &Vec::from_array(&env, [remote_chain]));
    assert!(!pool_client.is_supported_chain(&remote_chain));

    let sender = Address::generate(&env);
    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender,
        amount: 1,
        local_token: token_address.clone(),
    };
    let result = pool_client.try_lock_or_burn(&lock_input);
    assert_eq!(result, Err(Ok(CCIPError::ChainNotSupported)));

    // Owner can re-add the same selector with fresh config
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 9, 8)]),
        &Vec::new(&env),
    );
    assert!(pool_client.is_supported_chain(&remote_chain));
    assert_eq!(
        pool_client.get_remote_token(&remote_chain),
        Bytes::from_slice(&env, &[8u8; 20])
    );
}

#[test]
fn test_apply_chain_updates_duplicate_selector_overwrites_remote_token() {
    let (env, pool_client, _owner, token_address, _token_client, token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );
    assert_eq!(
        pool_client.get_remote_token(&remote_chain),
        Bytes::from_slice(&env, &[2u8; 20])
    );

    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 3, 4)]),
        &Vec::new(&env),
    );
    assert_eq!(
        pool_client.get_remote_token(&remote_chain),
        Bytes::from_slice(&env, &[4u8; 20])
    );
    assert_eq!(
        pool_client.get_remote_pool(&remote_chain),
        Bytes::from_slice(&env, &[3u8; 20])
    );

    let sender = Address::generate(&env);
    token_admin_client.mint(&sender, &1);
    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender,
        amount: 1,
        local_token: token_address,
    };
    let out = pool_client.try_lock_or_burn(&lock_input);
    assert!(out.is_ok());
}

#[test]
fn test_lock_or_burn_dest_pool_data_encodes_local_decimals() {
    let (env, pool_client, _owner, token_address, token_client, token_admin_client) = setup_env();

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );

    let sender = Address::generate(&env);
    token_admin_client.mint(&sender, &100);
    let lock_input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[3u8; 20]),
        remote_chain_selector: remote_chain,
        original_sender: sender,
        amount: 100,
        local_token: token_address,
    };
    let out = pool_client.lock_or_burn(&lock_input);
    assert_eq!(out.dest_pool_data, encode_local_decimals(&env, 7).unwrap());
    assert_eq!(token_client.balance(&pool_client.address), 100);
}

#[test]
fn test_release_or_mint_scales_down_remote_more_decimals() {
    let env = Env::default();
    env.mock_all_auths_allowing_non_root_auth();

    let pool_id = env.register(LockReleaseTokenPoolContract, ());
    let pool_client = LockReleaseTokenPoolContractClient::new(&env, &pool_id);
    let owner = Address::generate(&env);
    let token_admin = Address::generate(&env);
    let token_contract = env.register_stellar_asset_contract_v2(token_admin.clone());
    let token_address = token_contract.address();
    let token_client = token::Client::new(&env, &token_address);
    let token_admin_client = token::StellarAssetClient::new(&env, &token_address);

    let local_decimals: u32 = 6;
    pool_client.initialize(&owner, &token_address, &local_decimals);

    let remote_chain: u64 = 5009297550715157269;
    pool_client.apply_chain_updates(
        &Vec::from_array(&env, [chain_update(&env, remote_chain, 1, 2)]),
        &Vec::new(&env),
    );

    let expected_local: i128 = 1_000_000;
    token_admin_client.mint(&pool_id, &expected_local);

    let remote_decimals: u32 = 9;
    let receiver = Address::generate(&env);
    let release_input = ReleaseOrMintIn {
        original_sender: Bytes::from_slice(&env, &[4u8; 20]),
        remote_chain_selector: remote_chain,
        receiver: receiver.clone(),
        amount: 1_000_000_000,
        local_token: token_address.clone(),
        source_pool_address: Bytes::from_slice(&env, &[5u8; 20]),
        source_pool_data: encode_local_decimals(&env, remote_decimals).unwrap(),
    };

    let out = pool_client.release_or_mint(&release_input);
    assert_eq!(out.destination_amount, expected_local);
    assert_eq!(token_client.balance(&receiver), expected_local);
    assert_eq!(token_client.balance(&pool_id), 0);
}

#[test]
fn test_initialize_rejects_decimals_above_uint8() {
    let env = Env::default();
    env.mock_all_auths_allowing_non_root_auth();

    let pool_id = env.register(LockReleaseTokenPoolContract, ());
    let pool_client = LockReleaseTokenPoolContractClient::new(&env, &pool_id);
    let owner = Address::generate(&env);
    let token_admin = Address::generate(&env);
    let token_contract = env.register_stellar_asset_contract_v2(token_admin);
    let token_address = token_contract.address();

    let r = pool_client.try_initialize(&owner, &token_address, &256u32);
    assert_eq!(r, Err(Ok(CCIPError::InvalidPoolTokenDecimals)));
}
