#![cfg(test)]

use soroban_sdk::{testutils::Address as _, Address, Env, Vec};

use crate::types::{OffRampEntry, OnRampEntry};
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

    client.set_onramp(&dest, &onramp);
    assert_eq!(client.get_onramp(&dest), onramp);

    client.add_offramp(&src, &offramp);
    assert!(client.is_offramp(&src, &offramp));

    client.remove_offramp(&src, &offramp);
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

    client.add_offramp(&300u64, &f);
    assert!(client.is_offramp(&300, &f));

    let on_updates = Vec::from_array(
        &env,
        [OnRampEntry {
            dest_chain_selector: 1,
            onramp: o1.clone(),
        }],
    );
    let off_rem = Vec::from_array(
        &env,
        [OffRampEntry {
            source_chain_selector: 300,
            offramp: f.clone(),
        }],
    );
    let off_add = Vec::from_array(
        &env,
        [OffRampEntry {
            source_chain_selector: 400,
            offramp: o2.clone(),
        }],
    );

    client.apply_ramp_updates(&on_updates, &off_rem, &off_add);

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
