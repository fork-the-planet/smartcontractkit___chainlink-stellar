#![cfg(test)]

use soroban_sdk::{testutils::Address as _, Address, Bytes, BytesN, Env, Vec};

use crate::types::{DataKey, MessageExecutionState, SourceChainConfigArgs, StaticConfig};
use crate::{OffRampContract, OffRampContractClient};

fn setup_env() -> (Env, Address, OffRampContractClient<'static>) {
    let env = Env::default();
    env.mock_all_auths();

    let contract_id = env.register(OffRampContract, ());
    let client = OffRampContractClient::new(&env, &contract_id);

    let owner = Address::generate(&env);
    (env, owner, client)
}

fn default_static_config(env: &Env) -> StaticConfig {
    StaticConfig {
        chain_selector: 1234,
        rmn_proxy: Address::generate(env),
        token_admin_registry: Address::generate(env),
    }
}

#[test]
fn test_initialize() {
    let (env, owner, client) = setup_env();
    let static_config = default_static_config(&env);

    client.initialize(&owner, &static_config);

    let stored_config = client.get_static_config();
    assert_eq!(stored_config.chain_selector, 1234);
}

#[test]
#[should_panic(expected = "Error(Contract, #2)")]
fn test_double_initialize_fails() {
    let (env, owner, client) = setup_env();
    let static_config = default_static_config(&env);

    client.initialize(&owner, &static_config);
    client.initialize(&owner, &static_config);
}

#[test]
fn test_get_execution_state_untouched() {
    let (env, owner, client) = setup_env();
    let static_config = default_static_config(&env);
    client.initialize(&owner, &static_config);

    let message_id = BytesN::from_array(&env, &[0u8; 32]);
    let state = client.get_execution_state(&message_id);
    assert_eq!(state, MessageExecutionState::Untouched);
}

#[test]
#[should_panic(expected = "Error(Contract, #106)")]
fn test_extend_execution_state_ttl_requires_storage_entry() {
    let (env, owner, client) = setup_env();
    let static_config = default_static_config(&env);
    client.initialize(&owner, &static_config);

    let message_id = BytesN::from_array(&env, &[9u8; 32]);
    let _ = client.extend_execution_state_ttl(&message_id);
}

#[test]
fn test_extend_execution_state_ttl_ok() {
    let env = Env::default();
    env.mock_all_auths();
    let contract_id = env.register(OffRampContract, ());
    let client = OffRampContractClient::new(&env, &contract_id);
    let owner = Address::generate(&env);
    let static_config = default_static_config(&env);
    client.initialize(&owner, &static_config);

    let message_id = BytesN::from_array(&env, &[7u8; 32]);
    let state_key = DataKey::ExecState(message_id.clone());

    env.as_contract(&contract_id, || {
        env.storage()
            .persistent()
            .set(&state_key, &MessageExecutionState::Failure);
        env.storage()
            .persistent()
            .extend_ttl(&state_key, 518_400, 3_110_400);
    });

    client.extend_execution_state_ttl(&message_id);
}

#[test]
fn test_apply_source_chain_config() {
    let (env, owner, client) = setup_env();
    let static_config = default_static_config(&env);
    client.initialize(&owner, &static_config);

    let router = Address::generate(&env);
    let default_ccv = Address::generate(&env);
    let onramp_bytes = Bytes::from_array(&env, &[1u8; 32]);

    let mut on_ramps = Vec::new(&env);
    on_ramps.push_back(onramp_bytes);

    let mut default_ccvs = Vec::new(&env);
    default_ccvs.push_back(default_ccv);

    let args = SourceChainConfigArgs {
        source_chain_selector: 5678,
        router: router.clone(),
        is_enabled: true,
        on_ramps,
        default_ccvs,
        lane_mandated_ccvs: Vec::new(&env),
    };

    let mut updates = Vec::new(&env);
    updates.push_back(args);

    client.apply_source_chain_cfg_updates(&updates);

    let config = client.get_source_chain_config(&5678);
    assert_eq!(config.is_enabled, true);
    assert_eq!(config.router, router);
}

#[test]
fn test_get_all_source_chain_configs() {
    let (env, owner, client) = setup_env();
    let static_config = default_static_config(&env);
    client.initialize(&owner, &static_config);

    let onramp_bytes = Bytes::from_array(&env, &[1u8; 32]);

    let mut on_ramps = Vec::new(&env);
    on_ramps.push_back(onramp_bytes.clone());

    let mut default_ccvs = Vec::new(&env);
    default_ccvs.push_back(Address::generate(&env));

    let args1 = SourceChainConfigArgs {
        source_chain_selector: 100,
        router: Address::generate(&env),
        is_enabled: true,
        on_ramps: on_ramps.clone(),
        default_ccvs: default_ccvs.clone(),
        lane_mandated_ccvs: Vec::new(&env),
    };

    let args2 = SourceChainConfigArgs {
        source_chain_selector: 200,
        router: Address::generate(&env),
        is_enabled: false,
        on_ramps,
        default_ccvs,
        lane_mandated_ccvs: Vec::new(&env),
    };

    let mut updates = Vec::new(&env);
    updates.push_back(args1);
    updates.push_back(args2);

    client.apply_source_chain_cfg_updates(&updates);

    let (selectors, configs) = client.get_all_source_chain_configs();
    assert_eq!(selectors.len(), 2);
    assert_eq!(configs.len(), 2);
}

#[test]
#[should_panic(expected = "Error(Contract, #101)")]
fn test_source_chain_config_zero_selector_fails() {
    let (env, owner, client) = setup_env();
    let static_config = default_static_config(&env);
    client.initialize(&owner, &static_config);

    let args = SourceChainConfigArgs {
        source_chain_selector: 0,
        router: Address::generate(&env),
        is_enabled: true,
        on_ramps: Vec::new(&env),
        default_ccvs: Vec::new(&env),
        lane_mandated_ccvs: Vec::new(&env),
    };

    let mut updates = Vec::new(&env);
    updates.push_back(args);
    client.apply_source_chain_cfg_updates(&updates);
}
