#![cfg(test)]

use soroban_sdk::{contract, contractimpl, testutils::Ledger, Address, Env};

use crate::{
    rate_limit::{self, calculate_refill},
    types::{PoolDataKey, RateLimitConfig, TokenBucket},
};
use common_error::CCIPError;

#[contract]
struct TestContract;

#[contractimpl]
impl TestContract {}

fn test_env() -> (Env, Address) {
    let env = Env::default();
    let contract_id = env.register(TestContract, ());
    (env, contract_id)
}

// ================================================================
//  calculate_refill (pure fn, no storage needed)
// ================================================================

#[test]
fn refill_zero_time_unchanged() {
    assert_eq!(calculate_refill(1000, 500, 0, 10), 500);
}

#[test]
fn refill_partial() {
    assert_eq!(calculate_refill(1000, 500, 20, 10), 700);
}

#[test]
fn refill_capped_at_capacity() {
    assert_eq!(calculate_refill(1000, 900, 20, 10), 1000);
}

#[test]
fn refill_from_zero() {
    assert_eq!(calculate_refill(1000, 0, 100, 10), 1000);
}

#[test]
fn refill_saturating_no_overflow() {
    assert_eq!(
        calculate_refill(u128::MAX, 0, u64::MAX, u128::MAX),
        u128::MAX
    );
}

// ================================================================
//  set_config validation
// ================================================================

#[test]
fn set_config_disabled_zeroes_accepted() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig::disabled();
    env.as_contract(&cid, || {
        assert!(rate_limit::set_config(&env, &key, &cfg).is_ok());
        let bucket: TokenBucket = env.storage().persistent().get(&key).unwrap();
        assert!(!bucket.is_enabled);
        assert_eq!(bucket.tokens, 0);
    });
}

#[test]
fn set_config_disabled_nonzero_capacity_rejected() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: false,
        capacity: 100,
        rate: 0,
    };
    env.as_contract(&cid, || {
        assert_eq!(
            rate_limit::set_config(&env, &key, &cfg),
            Err(CCIPError::DisabledNonZeroRateLimit)
        );
    });
}

#[test]
fn set_config_disabled_nonzero_rate_rejected() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: false,
        capacity: 0,
        rate: 5,
    };
    env.as_contract(&cid, || {
        assert_eq!(
            rate_limit::set_config(&env, &key, &cfg),
            Err(CCIPError::DisabledNonZeroRateLimit)
        );
    });
}

#[test]
fn set_config_rate_exceeds_capacity_rejected() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 100,
        rate: 101,
    };
    env.as_contract(&cid, || {
        assert_eq!(
            rate_limit::set_config(&env, &key, &cfg),
            Err(CCIPError::InvalidRateLimitRate)
        );
    });
}

#[test]
fn set_config_enabled_resets_to_full_capacity() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 1000);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 500,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        let bucket: TokenBucket = env.storage().persistent().get(&key).unwrap();
        assert!(bucket.is_enabled);
        assert_eq!(bucket.tokens, 500);
        assert_eq!(bucket.capacity, 500);
        assert_eq!(bucket.rate, 10);
        assert_eq!(bucket.last_updated, 1000);
    });
}

// ================================================================
//  consume
// ================================================================

#[test]
fn consume_zero_is_noop() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(1);
    env.as_contract(&cid, || {
        assert!(rate_limit::consume(&env, &key, 0).is_ok());
    });
}

#[test]
fn consume_disabled_bucket_is_noop() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(1);
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &RateLimitConfig::disabled()).unwrap();
        assert!(rate_limit::consume(&env, &key, 1_000_000).is_ok());
    });
}

#[test]
fn consume_no_bucket_stored_is_noop() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(42);
    env.as_contract(&cid, || {
        assert!(rate_limit::consume(&env, &key, 1_000_000).is_ok());
    });
}

#[test]
fn consume_negative_amount_rejected() {
    let (env, cid) = test_env();
    let key = PoolDataKey::OutboundRateLimit(1);
    env.as_contract(&cid, || {
        assert_eq!(
            rate_limit::consume(&env, &key, -1),
            Err(CCIPError::InvalidTokenAmount)
        );
    });
}

#[test]
fn consume_within_capacity() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 100);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 1000,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        rate_limit::consume(&env, &key, 400).unwrap();
        let bucket: TokenBucket = env.storage().persistent().get(&key).unwrap();
        assert_eq!(bucket.tokens, 600);
    });
}

#[test]
fn consume_exceeds_capacity_rejected() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 100);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 1000,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        assert_eq!(
            rate_limit::consume(&env, &key, 1001),
            Err(CCIPError::TokenMaxCapacityExceeded)
        );
    });
}

#[test]
fn consume_exceeds_available_tokens_rejected() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 100);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 1000,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        rate_limit::consume(&env, &key, 800).unwrap();
        assert_eq!(
            rate_limit::consume(&env, &key, 201),
            Err(CCIPError::TokenRateLimitReached)
        );
    });
}

#[test]
fn consume_refills_over_time() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 100);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 1000,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        rate_limit::consume(&env, &key, 1000).unwrap();

        env.ledger().with_mut(|li| li.timestamp = 150);
        rate_limit::consume(&env, &key, 500).unwrap();

        let bucket: TokenBucket = env.storage().persistent().get(&key).unwrap();
        assert_eq!(bucket.tokens, 0);
        assert_eq!(bucket.last_updated, 150);
    });
}

#[test]
fn consume_partial_refill_then_drain() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 0);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 100,
        rate: 5,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        rate_limit::consume(&env, &key, 100).unwrap();

        env.ledger().with_mut(|li| li.timestamp = 10);
        rate_limit::consume(&env, &key, 50).unwrap();

        env.ledger().with_mut(|li| li.timestamp = 14);
        assert_eq!(
            rate_limit::consume(&env, &key, 21),
            Err(CCIPError::TokenRateLimitReached)
        );
        rate_limit::consume(&env, &key, 20).unwrap();
    });
}

#[test]
fn consume_refill_caps_at_capacity() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 0);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 100,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        rate_limit::consume(&env, &key, 50).unwrap();

        env.ledger().with_mut(|li| li.timestamp = 1000);
        let state = rate_limit::current_state(&env, &key);
        assert_eq!(state.tokens, 100);
    });
}

// ================================================================
//  current_state
// ================================================================

#[test]
fn current_state_disabled_bucket() {
    let (env, cid) = test_env();
    let key = PoolDataKey::InboundRateLimit(1);
    env.as_contract(&cid, || {
        let state = rate_limit::current_state(&env, &key);
        assert!(!state.is_enabled);
        assert_eq!(state.tokens, 0);
    });
}

#[test]
fn current_state_reflects_refill() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 100);
    let key = PoolDataKey::InboundRateLimit(1);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 1000,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg).unwrap();
        rate_limit::consume(&env, &key, 600).unwrap();

        env.ledger().with_mut(|li| li.timestamp = 120);
        let state = rate_limit::current_state(&env, &key);
        assert_eq!(state.tokens, 600);
        assert_eq!(state.last_updated, 120);
    });
}

// ================================================================
//  remove_buckets
// ================================================================

#[test]
fn remove_buckets_clears_both() {
    let (env, cid) = test_env();
    let chain = 42u64;
    let out_key = PoolDataKey::OutboundRateLimit(chain);
    let in_key = PoolDataKey::InboundRateLimit(chain);
    let cfg = RateLimitConfig {
        is_enabled: true,
        capacity: 100,
        rate: 1,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &out_key, &cfg).unwrap();
        rate_limit::set_config(&env, &in_key, &cfg).unwrap();
        assert!(env.storage().persistent().has(&out_key));
        assert!(env.storage().persistent().has(&in_key));

        rate_limit::remove_buckets(&env, chain);
        assert!(!env.storage().persistent().has(&out_key));
        assert!(!env.storage().persistent().has(&in_key));
    });
}

// ================================================================
//  set_config overwrites previous config
// ================================================================

#[test]
fn set_config_overwrites_and_resets_tokens() {
    let (env, cid) = test_env();
    env.ledger().with_mut(|li| li.timestamp = 100);
    let key = PoolDataKey::OutboundRateLimit(1);
    let cfg1 = RateLimitConfig {
        is_enabled: true,
        capacity: 1000,
        rate: 10,
    };
    env.as_contract(&cid, || {
        rate_limit::set_config(&env, &key, &cfg1).unwrap();
        rate_limit::consume(&env, &key, 800).unwrap();

        let cfg2 = RateLimitConfig {
            is_enabled: true,
            capacity: 5000,
            rate: 50,
        };
        rate_limit::set_config(&env, &key, &cfg2).unwrap();

        let bucket: TokenBucket = env.storage().persistent().get(&key).unwrap();
        assert_eq!(bucket.tokens, 5000);
        assert_eq!(bucket.capacity, 5000);
        assert_eq!(bucket.rate, 50);
    });
}
