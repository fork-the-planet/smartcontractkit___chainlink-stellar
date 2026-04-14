#![cfg(test)]

extern crate std;

use soroban_sdk::{testutils::Address as _, vec, Address, Bytes, BytesN, Env};

use crate::{ExampleCcipReceiver, ExampleCcipReceiverClient};

use common_interfaces::ccip_receiver::{CcvChainConfig, CcvConfigUpdate};
use common_message::AnyToStellarMessage;

#[test]
fn ccip_receive_accepts_router_auth() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let router = Address::generate(&env);
    let receiver_id = env.register(ExampleCcipReceiver, ());

    let client = ExampleCcipReceiverClient::new(&env, &receiver_id);
    client.initialize(&owner, &router);

    let msg = AnyToStellarMessage {
        message_id: BytesN::from_array(&env, &[7u8; 32]),
        source_chain_selector: 1,
        sender: Bytes::from_array(&env, &[0u8; 32]),
        data: Bytes::from_slice(&env, &[1, 2, 3]),
        dest_token_amounts: vec![&env],
    };

    // With `mock_all_auths`, `router.require_auth_for_args` is satisfied without `as_contract`.
    client.ccip_receive(&msg);

    let stored = client.last_message_id();
    assert_eq!(stored, msg.message_id);
}

#[test]
fn enable_remote_chain_stores_extra_args() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let router = Address::generate(&env);
    let receiver_id = env.register(ExampleCcipReceiver, ());
    let client = ExampleCcipReceiverClient::new(&env, &receiver_id);
    client.initialize(&owner, &router);

    let dest: u64 = 9_876;
    let extra = Bytes::from_slice(&env, &[0xde, 0xad, 0xbe, 0xef]);
    client.enable_remote_chain(&owner, &dest, &extra);

    let got = client.get_remote_chain_extra_args(&dest);
    assert_eq!(got, extra);
}

#[test]
fn apply_ccv_config_updates_stores_per_source_chain() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let router = Address::generate(&env);
    let receiver_id = env.register(ExampleCcipReceiver, ());
    let client = ExampleCcipReceiverClient::new(&env, &receiver_id);
    client.initialize(&owner, &router);

    let a1 = Address::generate(&env);
    let a2 = Address::generate(&env);
    let o1 = Address::generate(&env);
    let o2 = Address::generate(&env);
    let src: u64 = 5_001;
    let upd = CcvConfigUpdate {
        source_chain_selector: src,
        required_ccvs: vec![&env, a1.clone(), a2.clone()],
        optional_ccvs: vec![&env, o1.clone(), o2.clone()],
        optional_threshold: 1,
    };
    client.apply_ccv_config_updates(&owner, &vec![&env, upd]);

    let cfg = client.get_ccv_config(&src);
    assert_eq!(cfg.required_ccvs.len(), 2);
    assert_eq!(cfg.optional_ccvs.len(), 2);
    assert_eq!(cfg.optional_threshold, 1);
}

#[test]
fn apply_ccv_config_rejects_invalid_optional_threshold() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let router = Address::generate(&env);
    let receiver_id = env.register(ExampleCcipReceiver, ());
    let client = ExampleCcipReceiverClient::new(&env, &receiver_id);
    client.initialize(&owner, &router);

    let o1 = Address::generate(&env);
    let o2 = Address::generate(&env);
    // optional_threshold >= optional.len() is invalid (EVM parity)
    let upd = CcvConfigUpdate {
        source_chain_selector: 42,
        required_ccvs: vec![&env],
        optional_ccvs: vec![&env, o1, o2],
        optional_threshold: 2,
    };
    let r = client.try_apply_ccv_config_updates(&owner, &vec![&env, upd]);
    assert!(r.is_err());
}

#[test]
fn apply_ccv_config_rejects_duplicate_ccv() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let router = Address::generate(&env);
    let receiver_id = env.register(ExampleCcipReceiver, ());
    let client = ExampleCcipReceiverClient::new(&env, &receiver_id);
    client.initialize(&owner, &router);

    let dup = Address::generate(&env);
    let upd = CcvConfigUpdate {
        source_chain_selector: 99,
        required_ccvs: vec![&env, dup.clone()],
        optional_ccvs: vec![&env, dup.clone()],
        optional_threshold: 0,
    };
    let r = client.try_apply_ccv_config_updates(&owner, &vec![&env, upd]);
    assert!(r.is_err());
}

#[test]
fn get_ccv_config_returns_empty_when_unset() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let router = Address::generate(&env);
    let receiver_id = env.register(ExampleCcipReceiver, ());
    let client = ExampleCcipReceiverClient::new(&env, &receiver_id);
    client.initialize(&owner, &router);

    let cfg = client.get_ccv_config(&12345u64);
    let empty = CcvChainConfig {
        required_ccvs: vec![&env],
        optional_ccvs: vec![&env],
        optional_threshold: 0,
    };
    assert_eq!(cfg, empty);
}
