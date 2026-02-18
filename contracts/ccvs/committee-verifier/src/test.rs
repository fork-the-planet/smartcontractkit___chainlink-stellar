#![cfg(test)]

use super::*;
use soroban_sdk::{testutils::Address as _, Address, Bytes, Vec, Env};

#[test]
fn initialize_scaffold() {
    let env = Env::default();
    let contract_id = env.register(CommitteeVerifierContract, ());
    let client = CommitteeVerifierContractClient::new(&env, &contract_id);

    let owner = Address::generate(&env);
    let rmn_proxy = Address::generate(&env);
    let dynamic_config = DynamicConfig {
        fee_aggregator: None,
        allowlist_admin: None,
    };
    let storage_locations: Vec<Bytes> = Vec::new(&env);

    let res = client.initialize(&owner, &dynamic_config, &storage_locations, &rmn_proxy);
    assert_eq!(res, Ok(()));
}
