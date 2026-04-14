#![cfg(test)]

extern crate std;

use soroban_sdk::{testutils::Address as _, token, vec, Address, Bytes, Env, Vec};

use crate::{LockBoxEntry, SiloedLockReleaseTokenPoolContract, SiloedLockReleaseTokenPoolContractClient};
use common_pool::{ChainUpdate, LockOrBurnIn, RateLimitConfig, ReleaseOrMintIn};
use pools_token_lock_box::{TokenLockBox, TokenLockBoxClient};

const REMOTE_CHAIN: u64 = 99_999;

fn disabled_rl() -> RateLimitConfig {
    RateLimitConfig {
        is_enabled: false,
        capacity: 0,
        rate: 0,
    }
}

struct TestEnv<'a> {
    env: Env,
    _owner: Address,
    token_addr: Address,
    pool_client: SiloedLockReleaseTokenPoolContractClient<'a>,
    lockbox_client: TokenLockBoxClient<'a>,
    sac: token::StellarAssetClient<'a>,
    tc: token::Client<'a>,
}

fn setup() -> TestEnv<'static> {
    let env = Env::default();
    env.mock_all_auths_allowing_non_root_auth();

    let owner = Address::generate(&env);
    let token_admin = Address::generate(&env);
    let token_contract = env.register_stellar_asset_contract_v2(token_admin.clone());
    let token_addr = token_contract.address();
    let sac = token::StellarAssetClient::new(&env, &token_addr);
    let tc = token::Client::new(&env, &token_addr);

    let lockbox_id = env.register(TokenLockBox, ());
    let lockbox_client = TokenLockBoxClient::new(&env, &lockbox_id);
    lockbox_client.initialize(&owner, &token_addr);

    let router = Address::generate(&env);

    let pool_id = env.register(SiloedLockReleaseTokenPoolContract, ());
    let pool_client = SiloedLockReleaseTokenPoolContractClient::new(&env, &pool_id);
    pool_client.initialize(&owner, &token_addr, &7, &router);

    lockbox_client.add_allowed_callers(&vec![&env, pool_client.address.clone()]);

    let remote_pool = Bytes::from_slice(&env, &[0xaa; 32]);
    let remote_token = Bytes::from_slice(&env, &[0xbb; 32]);
    pool_client.apply_chain_updates(
        &vec![
            &env,
            ChainUpdate {
                remote_chain_selector: REMOTE_CHAIN,
                remote_pool_addresses: remote_pool,
                remote_token_address: remote_token,
                outbound_rate_limiter_config: disabled_rl(),
                inbound_rate_limiter_config: disabled_rl(),
            },
        ],
        &Vec::new(&env),
    );

    pool_client.configure_lock_boxes(&vec![
        &env,
        LockBoxEntry {
            remote_chain_selector: REMOTE_CHAIN,
            lock_box: lockbox_client.address.clone(),
        },
    ]);

    TestEnv {
        env,
        _owner: owner,
        token_addr,
        pool_client,
        lockbox_client,
        sac,
        tc,
    }
}

#[test]
fn lock_deposits_into_lockbox() {
    let t = setup();
    let sender = Address::generate(&t.env);
    t.sac.mint(&sender, &1_000);

    let input = LockOrBurnIn {
        receiver: Bytes::from_slice(&t.env, &[0x01; 20]),
        remote_chain_selector: REMOTE_CHAIN,
        original_sender: sender.clone(),
        amount: 500,
        local_token: t.token_addr.clone(),
    };
    let out = t.pool_client.lock_or_burn(&input, &0);

    assert_eq!(t.tc.balance(&sender), 500);
    assert_eq!(t.tc.balance(&t.lockbox_client.address), 500);
    assert!(!out.dest_token_address.is_empty());
}

#[test]
fn release_withdraws_from_lockbox() {
    let t = setup();

    let liquidity_provider = Address::generate(&t.env);
    t.sac.mint(&liquidity_provider, &2_000);
    t.lockbox_client.add_allowed_callers(&vec![&t.env, liquidity_provider.clone()]);
    t.lockbox_client.deposit(&liquidity_provider, &2_000);

    let receiver = Address::generate(&t.env);
    let input = ReleaseOrMintIn {
        original_sender: Bytes::from_slice(&t.env, &[0xcd; 20]),
        remote_chain_selector: REMOTE_CHAIN,
        receiver: receiver.clone(),
        amount: 800,
        local_token: t.token_addr.clone(),
        source_pool_address: Bytes::from_slice(&t.env, &[0xaa; 32]),
        source_pool_data: Bytes::new(&t.env),
    };
    let out = t.pool_client.release_or_mint(&input, &0);

    assert_eq!(out.destination_amount, 800);
    assert_eq!(t.tc.balance(&receiver), 800);
    assert_eq!(t.tc.balance(&t.lockbox_client.address), 1_200);
}

#[test]
fn get_lock_box_returns_configured_address() {
    let t = setup();
    let addr = t.pool_client.get_lock_box(&REMOTE_CHAIN);
    assert_eq!(addr, t.lockbox_client.address);
}

#[test]
fn get_all_lock_box_configs() {
    let t = setup();
    let cfgs = t.pool_client.get_all_lock_box_configs();
    assert_eq!(cfgs.len(), 1);
    let entry = cfgs.get(0).unwrap();
    assert_eq!(entry.remote_chain_selector, REMOTE_CHAIN);
    assert_eq!(entry.lock_box, t.lockbox_client.address);
}

#[test]
fn unconfigured_lockbox_rejects_lock() {
    let env = Env::default();
    env.mock_all_auths_allowing_non_root_auth();

    let owner = Address::generate(&env);
    let token_admin = Address::generate(&env);
    let token_contract = env.register_stellar_asset_contract_v2(token_admin.clone());
    let token_addr = token_contract.address();
    let router = Address::generate(&env);

    let pool_id = env.register(SiloedLockReleaseTokenPoolContract, ());
    let pool_client = SiloedLockReleaseTokenPoolContractClient::new(&env, &pool_id);
    pool_client.initialize(&owner, &token_addr, &7, &router);

    let remote_pool = Bytes::from_slice(&env, &[0xaa; 32]);
    let remote_token = Bytes::from_slice(&env, &[0xbb; 32]);
    pool_client.apply_chain_updates(
        &vec![
            &env,
            ChainUpdate {
                remote_chain_selector: REMOTE_CHAIN,
                remote_pool_addresses: remote_pool,
                remote_token_address: remote_token,
                outbound_rate_limiter_config: disabled_rl(),
                inbound_rate_limiter_config: disabled_rl(),
            },
        ],
        &Vec::new(&env),
    );

    let sender = Address::generate(&env);
    let sac = token::StellarAssetClient::new(&env, &token_addr);
    sac.mint(&sender, &500);

    let input = LockOrBurnIn {
        receiver: Bytes::from_slice(&env, &[0x01; 20]),
        remote_chain_selector: REMOTE_CHAIN,
        original_sender: sender,
        amount: 100,
        local_token: token_addr,
    };
    let r = pool_client.try_lock_or_burn(&input, &0);
    assert!(r.is_err());
}

#[test]
fn many_to_one_lockbox_shared_liquidity() {
    let t = setup();

    let chain_b: u64 = 88_888;
    let remote_pool = Bytes::from_slice(&t.env, &[0xcc; 32]);
    let remote_token = Bytes::from_slice(&t.env, &[0xdd; 32]);
    t.pool_client.apply_chain_updates(
        &vec![
            &t.env,
            ChainUpdate {
                remote_chain_selector: chain_b,
                remote_pool_addresses: remote_pool,
                remote_token_address: remote_token,
                outbound_rate_limiter_config: disabled_rl(),
                inbound_rate_limiter_config: disabled_rl(),
            },
        ],
        &Vec::new(&t.env),
    );
    t.pool_client.configure_lock_boxes(&vec![
        &t.env,
        LockBoxEntry {
            remote_chain_selector: chain_b,
            lock_box: t.lockbox_client.address.clone(),
        },
    ]);

    let sender = Address::generate(&t.env);
    t.sac.mint(&sender, &1_000);

    let input_a = LockOrBurnIn {
        receiver: Bytes::from_slice(&t.env, &[0x01; 20]),
        remote_chain_selector: REMOTE_CHAIN,
        original_sender: sender.clone(),
        amount: 300,
        local_token: t.token_addr.clone(),
    };
    t.pool_client.lock_or_burn(&input_a, &0);

    let input_b = LockOrBurnIn {
        receiver: Bytes::from_slice(&t.env, &[0x02; 20]),
        remote_chain_selector: chain_b,
        original_sender: sender.clone(),
        amount: 200,
        local_token: t.token_addr.clone(),
    };
    t.pool_client.lock_or_burn(&input_b, &0);

    assert_eq!(t.tc.balance(&t.lockbox_client.address), 500);
}
