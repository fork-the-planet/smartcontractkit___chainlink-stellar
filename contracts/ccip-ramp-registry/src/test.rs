#![cfg(test)]

use soroban_sdk::{testutils::Address as _, Address, Env, Vec};

use crate::types::{OffRampUpdate, OnRampUpdate};
use crate::{RampRegistryContract, RampRegistryContractClient};
use common_error::CCIPError;

#[test]
fn test_ramp_lifecycle() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let onramp = Address::generate(&env);
    let offramp = Address::generate(&env);

    let id = env.register(RampRegistryContract, ());
    let client = RampRegistryContractClient::new(&env, &id);
    client.initialize(&owner);

    let dest: u64 = 100;
    let src: u64 = 200;

    client.apply_onramp_updates(&Vec::from_array(
        &env,
        [OnRampUpdate {
            dest_chain_selector: dest,
            onramp: Some(onramp.clone()),
        }],
    ));
    assert_eq!(client.get_onramp(&dest), onramp);

    client.apply_offramp_updates(&Vec::from_array(
        &env,
        [OffRampUpdate {
            source_chain_selector: src,
            offramp: offramp.clone(),
            enabled: true,
        }],
    ));
    assert!(client.is_offramp(&src, &offramp));

    client.apply_offramp_updates(&Vec::from_array(
        &env,
        [OffRampUpdate {
            source_chain_selector: src,
            offramp: offramp.clone(),
            enabled: false,
        }],
    ));
    assert!(!client.is_offramp(&src, &offramp));
}

#[test]
fn test_apply_ramp_updates() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let o1 = Address::generate(&env);
    let o2 = Address::generate(&env);
    let f = Address::generate(&env);

    let id = env.register(RampRegistryContract, ());
    let client = RampRegistryContractClient::new(&env, &id);
    client.initialize(&owner);

    client.apply_offramp_updates(&Vec::from_array(
        &env,
        [OffRampUpdate {
            source_chain_selector: 300,
            offramp: f.clone(),
            enabled: true,
        }],
    ));
    assert!(client.is_offramp(&300, &f));

    client.apply_onramp_updates(&Vec::from_array(
        &env,
        [OnRampUpdate {
            dest_chain_selector: 1,
            onramp: Some(o1.clone()),
        }],
    ));

    client.apply_offramp_updates(&Vec::from_array(
        &env,
        [
            OffRampUpdate {
                source_chain_selector: 300,
                offramp: f.clone(),
                enabled: false,
            },
            OffRampUpdate {
                source_chain_selector: 400,
                offramp: o2.clone(),
                enabled: true,
            },
        ],
    ));

    assert_eq!(client.get_onramp(&1), o1);
    assert!(!client.is_offramp(&300, &f));
    assert!(client.is_offramp(&400, &o2));
}

#[test]
fn test_get_onramp_missing_chain() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let id = env.register(RampRegistryContract, ());
    let client = RampRegistryContractClient::new(&env, &id);
    client.initialize(&owner);

    let r = client.try_get_onramp(&999u64);
    assert_eq!(r, Err(Ok(CCIPError::UnsupportedDestinationChain)));
}

#[test]
fn test_apply_onramp_rejects_zero_selector() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let onramp = Address::generate(&env);

    let id = env.register(RampRegistryContract, ());
    let client = RampRegistryContractClient::new(&env, &id);
    client.initialize(&owner);

    let r = client.try_apply_onramp_updates(&Vec::from_array(
        &env,
        [OnRampUpdate {
            dest_chain_selector: 0,
            onramp: Some(onramp),
        }],
    ));
    assert_eq!(r, Err(Ok(CCIPError::InvalidChainSelector)));
}

#[test]
fn test_apply_offramp_rejects_zero_selector() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let offramp = Address::generate(&env);

    let id = env.register(RampRegistryContract, ());
    let client = RampRegistryContractClient::new(&env, &id);
    client.initialize(&owner);

    let r = client.try_apply_offramp_updates(&Vec::from_array(
        &env,
        [OffRampUpdate {
            source_chain_selector: 0,
            offramp,
            enabled: true,
        }],
    ));
    assert_eq!(r, Err(Ok(CCIPError::InvalidChainSelector)));
}
