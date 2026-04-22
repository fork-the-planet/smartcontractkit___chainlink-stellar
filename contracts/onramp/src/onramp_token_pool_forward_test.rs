//! Router → OnRamp outbound flow with a lock-release token pool, asserting CCIPMessageSent
//! receipt ordering: `[CCV…, TokenPool, Executor, NetworkFee]` (EVM / chainlink-ccv parity).

use crate::types::Receipt;
use crate::{OnRampContract, OnRampContractClient};
use ccip_ramp_registry::{RampRegistryContract, RampRegistryContractClient};
use ccvs_versioned_verifier_resolver::{
    OutboundImplementationUpdate, VersionedVerifierResolverContract,
    VersionedVerifierResolverContractClient,
};
use common_error::CCIPError;
use common_interfaces::committee_verifier::FeeResponse;
use common_message::{StellarToAnyMessage, TokenAmount};
use common_pool::{ChainUpdate, RateLimitConfig};
use fee_quoter::{
    types::{
        DestChainConfig, DestChainConfigArgs as FqDestChainConfigArgs, GasPriceUpdate,
        PriceUpdates, StaticConfig as FqStaticConfig, TokenFeeConfigArgs, TokenPriceUpdate,
        TokenTransferFeeConfig,
    },
    FeeQuoterContract, FeeQuoterContractClient,
};
use pools_lock_release_pool::{LockReleaseTokenPoolContract, LockReleaseTokenPoolContractClient};
use rmn_proxy::{RmnProxyContract, RmnProxyContractClient};
use rmn_remote::{RmnRemoteContract, RmnRemoteContractClient};
use router::{RouterContract, RouterContractClient};
use soroban_sdk::{
    contract, contractimpl, symbol_short,
    testutils::{Address as _, Events as _, Ledger},
    token, vec, Address, Bytes, BytesN, Env, Map, Symbol, TryFromVal, TryIntoVal, Val, Vec,
};
use token_admin_registry::{TokenAdminRegistryContract, TokenAdminRegistryContractClient};

use crate::types::{DestChainConfigArgs as OnrampDestChainConfigArgs, DynamicConfig, StaticConfig};

#[contract]
pub struct MockOutboundCcvVerifier;

#[contractimpl]
impl MockOutboundCcvVerifier {
    pub fn get_fee(
        _env: Env,
        _dest_chain_selector: u64,
        _message: Bytes,
        _extra_args: Bytes,
        _block_confirmations: u32,
    ) -> Result<FeeResponse, CCIPError> {
        Ok(FeeResponse {
            dest_bytes_overhead: 0,
            dest_gas_limit: 0,
            fee: 0,
        })
    }

    pub fn forward_to_verifier(
        env: Env,
        _dest_chain_selector: u64,
        _sender: Address,
        _message_id: BytesN<32>,
        _fee_token: Address,
        _fee_token_amount: i128,
        _verifier_args: Bytes,
    ) -> Result<Bytes, CCIPError> {
        Ok(Bytes::new(&env))
    }
}

fn deploy_default_ccv_resolver(env: &Env, owner: &Address, dest_chain_selector: u64) -> Address {
    let verifier_id = env.register(MockOutboundCcvVerifier, ());
    let vvr_id = env.register(VersionedVerifierResolverContract, ());
    let vvr = VersionedVerifierResolverContractClient::new(env, &vvr_id);
    vvr.initialize(owner, &Address::generate(env));
    vvr.apply_outbound_impl_updates(&vec![
        env,
        OutboundImplementationUpdate {
            dest_chain_selector,
            verifier: Some(verifier_id),
        },
    ]);
    vvr_id
}

fn setup_fee_quoter(
    env: &Env,
    owner: &Address,
    dest_chain_selector: u64,
    fee_token: &Address,
    transfer_token: &Address,
) -> Address {
    env.ledger().with_mut(|li| {
        li.timestamp = 1000;
    });

    let fee_quoter_id = env.register(FeeQuoterContract, ());
    let fee_quoter_client = FeeQuoterContractClient::new(env, &fee_quoter_id);

    let link_token = Address::generate(env);
    let static_config = FqStaticConfig {
        max_fee_juels_per_msg: 1_000_000_000_000_000_000,
        link_token: link_token.clone(),
    };

    let mut authorized_callers: Vec<Address> = Vec::new(env);
    authorized_callers.push_back(owner.clone());

    fee_quoter_client.initialize(owner, &static_config, &authorized_callers);

    let dest_config = DestChainConfig {
        is_enabled: true,
        max_data_bytes: 50000,
        max_per_msg_gas_limit: 4_000_000,
        dest_gas_overhead: 350_000,
        dest_gas_per_payload_byte: 16,
        default_token_fee_usd: 50,
        default_token_dest_gas: 50_000,
        default_tx_gas_limit: 200_000,
        network_fee_usd_cents: 100,
        link_premium_percent: 90,
    };

    let mut config_args: Vec<FqDestChainConfigArgs> = Vec::new(env);
    config_args.push_back(FqDestChainConfigArgs {
        dest_chain_selector,
        config: dest_config,
    });
    fee_quoter_client.apply_dest_chain_configs(&config_args);

    let mut token_updates: Vec<TokenPriceUpdate> = Vec::new(env);
    token_updates.push_back(TokenPriceUpdate {
        token: link_token.clone(),
        usd_per_token: 15_000_000_000_000_000_000,
    });
    token_updates.push_back(TokenPriceUpdate {
        token: fee_token.clone(),
        usd_per_token: 15_000_000_000_000_000_000,
    });
    token_updates.push_back(TokenPriceUpdate {
        token: transfer_token.clone(),
        usd_per_token: 1_000_000_000_000_000_000,
    });

    let mut gas_updates: Vec<GasPriceUpdate> = Vec::new(env);
    gas_updates.push_back(GasPriceUpdate {
        dest_chain_selector,
        usd_per_unit_gas: 100_000_000_000_000,
    });

    fee_quoter_client.update_prices(
        owner,
        &PriceUpdates {
            token_price_updates: token_updates,
            gas_price_updates: gas_updates,
        },
    );

    let token_fee_args = vec![
        env,
        TokenFeeConfigArgs {
            dest_chain_selector,
            token: transfer_token.clone(),
            config: TokenTransferFeeConfig {
                fee_usd_cents: 5000,
                dest_gas_overhead: 75_000,
                dest_bytes_overhead: 64,
                is_enabled: true,
            },
        },
    ];
    fee_quoter_client.apply_token_fee_configs(&token_fee_args, &Vec::new(env));

    fee_quoter_id
}

fn receipts_from_last_onramp_ccip_event(env: &Env, onramp: &Address) -> Vec<Receipt> {
    let evs = env.events().all().filter_by_contract(onramp);
    for e in evs.events().iter().rev() {
        let soroban_sdk::xdr::ContractEventBody::V0(ref v0) = e.body;
        let val: Val = v0
            .data
            .clone()
            .try_into_val(env)
            .expect("event data ScVal to Val");
        let Ok(map) = Map::<Symbol, Val>::try_from_val(env, &val) else {
            continue;
        };
        let Some(rval) = map.get(symbol_short!("receipts")) else {
            continue;
        };
        if let Ok(rvec) = Vec::<Receipt>::try_from_val(env, &rval) {
            return rvec;
        }
    }
    panic!("expected CCIPMessageSent event with receipts from onramp");
}

#[test]
fn test_ccip_send_emits_token_pool_receipt_before_executor_and_network_fee() {
    let env = Env::default();
    env.mock_all_auths();

    let owner = Address::generate(&env);
    let sender = Address::generate(&env);

    let stellar_chain_selector: u64 = 12345;
    let evm_chain_selector: u64 = 67890;

    let rmn_remote_id = env.register(RmnRemoteContract, ());
    let rmn_remote_client = RmnRemoteContractClient::new(&env, &rmn_remote_id);
    rmn_remote_client.initialize(&owner, &1u64);

    let rmn_proxy_id = env.register(RmnProxyContract, ());
    let rmn_proxy_client = RmnProxyContractClient::new(&env, &rmn_proxy_id);
    rmn_proxy_client.initialize(&owner, &rmn_remote_id);

    let router_id = env.register(RouterContract, ());
    let router_client = RouterContractClient::new(&env, &router_id);
    router_client.initialize(&owner, &rmn_proxy_id);

    let onramp_id = env.register(OnRampContract, ());
    let onramp_client = OnRampContractClient::new(&env, &onramp_id);

    let fee_token_admin = Address::generate(&env);
    let fee_token_contract = env.register_stellar_asset_contract_v2(fee_token_admin.clone());
    let fee_token = fee_token_contract.address();
    let fee_token_sac = token::StellarAssetClient::new(&env, &fee_token);

    let transfer_token_admin = Address::generate(&env);
    let transfer_token_contract =
        env.register_stellar_asset_contract_v2(transfer_token_admin.clone());
    let transfer_token = transfer_token_contract.address();
    let transfer_token_sac = token::StellarAssetClient::new(&env, &transfer_token);

    let ramp_registry_id = env.register(RampRegistryContract, ());
    let ramp_registry_client = RampRegistryContractClient::new(&env, &ramp_registry_id);
    ramp_registry_client.initialize(&owner);

    let pool_id = env.register(LockReleaseTokenPoolContract, ());
    let pool_client = LockReleaseTokenPoolContractClient::new(&env, &pool_id);
    pool_client.initialize(
        &owner,
        &transfer_token,
        &7u32,
        &router_id,
        &ramp_registry_client.address,
    );

    let remote_pool = Bytes::from_slice(&env, &[0x11u8; 20]);
    let remote_token = Bytes::from_slice(&env, &[0x22u8; 20]);
    pool_client.apply_chain_updates(
        &vec![
            &env,
            ChainUpdate {
                remote_chain_selector: evm_chain_selector,
                remote_pool_addresses: remote_pool,
                remote_token_address: remote_token,
                outbound_rate_limiter_config: RateLimitConfig::disabled(),
                inbound_rate_limiter_config: RateLimitConfig::disabled(),
            },
        ],
        &Vec::new(&env),
    );

    ramp_registry_client.set_onramp(&evm_chain_selector, &onramp_id);

    let tar_id = env.register(TokenAdminRegistryContract, ());
    let tar_client = TokenAdminRegistryContractClient::new(&env, &tar_id);
    tar_client.initialize(&owner);

    let token_registry_admin = Address::generate(&env);
    tar_client.propose_administrator(&owner, &transfer_token, &token_registry_admin);
    tar_client.accept_admin_role(&transfer_token);
    tar_client.set_pool(&transfer_token, &Some(pool_id.clone()));

    let fee_quoter_id = setup_fee_quoter(
        &env,
        &owner,
        evm_chain_selector,
        &fee_token,
        &transfer_token,
    );

    let static_config = StaticConfig {
        chain_selector: stellar_chain_selector,
        token_admin_registry: tar_id.clone(),
        rmn_proxy: rmn_proxy_id.clone(),
        max_usd_cents_per_message: 100_000,
    };

    let fee_aggregator = Address::generate(&env);
    let dynamic_config = DynamicConfig {
        fee_quoter: fee_quoter_id,
        fee_aggregator: fee_aggregator.clone(),
    };

    onramp_client.initialize(&owner, &static_config, &dynamic_config);

    let default_ccv = deploy_default_ccv_resolver(&env, &owner, evm_chain_selector);
    let default_executor = Address::generate(&env);

    let dest_chain_config = OnrampDestChainConfigArgs {
        dest_chain_selector: evm_chain_selector,
        router: router_id.clone(),
        address_bytes_length: 20,
        token_receiver_allowed: true,
        message_network_fee_usd_cents: 50,
        token_network_fee_usd_cents: 100,
        base_execution_gas_cost: 200_000,
        execution_fee_usd_cents: 25,
        default_executor: default_executor.clone(),
        lane_mandated_ccvs: Vec::new(&env),
        default_ccvs: vec![&env, default_ccv.clone()],
        off_ramp: Bytes::from_array(&env, &[0u8; 20]),
    };

    onramp_client.apply_dest_chain_config_updates(&vec![&env, dest_chain_config]);

    router_client.set_onramp(&evm_chain_selector, &onramp_id);

    let mut token_amounts: Vec<TokenAmount> = Vec::new(&env);
    token_amounts.push_back(TokenAmount {
        token: transfer_token.clone(),
        amount: 1_000_000,
    });

    let message = StellarToAnyMessage {
        receiver: Bytes::from_array(&env, &[0x33u8; 20]),
        data: Bytes::from_slice(&env, b"token send with data"),
        token_amounts,
        fee_token: fee_token.clone(),
        extra_args: Bytes::new(&env),
    };

    let required_fee = router_client.get_fee(&evm_chain_selector, &message);
    assert!(required_fee > 0, "quoted fee must be positive");

    fee_token_sac.mint(&sender, &(required_fee * 2));
    transfer_token_sac.mint(&sender, &1_000_000);

    let message_id = router_client.ccip_send(&sender, &evm_chain_selector, &message, &required_fee);
    assert_ne!(
        message_id,
        BytesN::from_array(&env, &[0u8; 32]),
        "message id must be non-zero"
    );

    let receipts = receipts_from_last_onramp_ccip_event(&env, &onramp_id);
    assert_eq!(
        receipts.len(),
        4,
        "expected 1 CCV + pool + executor + network"
    );

    assert_eq!(receipts.get(0).unwrap().issuer, default_ccv);
    assert_eq!(receipts.get(1).unwrap().issuer, pool_id);
    assert_eq!(receipts.get(1).unwrap().dest_gas_limit, 0);
    assert_eq!(receipts.get(1).unwrap().dest_bytes_overhead, 0);

    assert_eq!(receipts.get(2).unwrap().issuer, default_executor);
    assert_eq!(receipts.get(3).unwrap().issuer, router_id);
}
