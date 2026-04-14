//! Token bucket rate limiting aligned with EVM [`RateLimiter.sol`].
//!
//! Each remote chain has separate outbound and inbound buckets stored in
//! persistent storage. Buckets refill at a constant `rate` (tokens per second)
//! up to `capacity`. Disabled buckets (or zero-amount requests) are no-ops.

use soroban_sdk::Env;

use common_error::CCIPError;

use crate::types::{PoolDataKey, RateLimitConfig, TokenBucket};

// ================================================================
//  Core bucket operations
// ================================================================

/// Consume `amount` tokens from the bucket at `bucket_key`.
///
/// Mirrors EVM `RateLimiter._consume`:
/// 1. Skip if bucket disabled or amount is 0.
/// 2. Refill bucket based on elapsed time.
/// 3. Revert if amount > capacity or amount > available tokens.
/// 4. Deduct and persist.
pub fn consume(env: &Env, bucket_key: &PoolDataKey, amount: i128) -> Result<(), CCIPError> {
    if amount < 0 {
        return Err(CCIPError::InvalidTokenAmount);
    }
    let request = amount as u128;
    if request == 0 {
        return Ok(());
    }

    let mut bucket: TokenBucket = env
        .storage()
        .persistent()
        .get(bucket_key)
        .unwrap_or(TokenBucket::disabled());

    if !bucket.is_enabled {
        return Ok(());
    }

    let now = env.ledger().timestamp();
    let time_diff = now.saturating_sub(bucket.last_updated);

    if time_diff > 0 {
        if bucket.tokens > bucket.capacity {
            return Err(CCIPError::BucketOverfilled);
        }
        bucket.tokens = calculate_refill(bucket.capacity, bucket.tokens, time_diff, bucket.rate);
        bucket.last_updated = now;
    }

    if bucket.capacity < request {
        return Err(CCIPError::TokenMaxCapacityExceeded);
    }
    if bucket.tokens < request {
        return Err(CCIPError::TokenRateLimitReached);
    }

    bucket.tokens -= request;
    env.storage().persistent().set(bucket_key, &bucket);

    Ok(())
}

/// Apply a [`RateLimitConfig`] to the bucket at `bucket_key`, resetting it to
/// full capacity. Mirrors EVM `RateLimiter._setTokenBucketConfig`.
pub fn set_config(
    env: &Env,
    bucket_key: &PoolDataKey,
    config: &RateLimitConfig,
) -> Result<(), CCIPError> {
    if config.is_enabled {
        if config.rate > config.capacity {
            return Err(CCIPError::InvalidRateLimitRate);
        }
    } else if config.rate != 0 || config.capacity != 0 {
        return Err(CCIPError::DisabledNonZeroRateLimit);
    }

    let bucket = TokenBucket {
        tokens: config.capacity,
        last_updated: env.ledger().timestamp(),
        is_enabled: config.is_enabled,
        capacity: config.capacity,
        rate: config.rate,
    };
    env.storage().persistent().set(bucket_key, &bucket);
    Ok(())
}

/// Return the bucket's current state with time-based refill applied.
/// Mirrors EVM `RateLimiter._currentTokenBucketState`.
pub fn current_state(env: &Env, bucket_key: &PoolDataKey) -> TokenBucket {
    let mut bucket: TokenBucket = env
        .storage()
        .persistent()
        .get(bucket_key)
        .unwrap_or(TokenBucket::disabled());

    if bucket.is_enabled {
        let now = env.ledger().timestamp();
        let time_diff = now.saturating_sub(bucket.last_updated);
        bucket.tokens = calculate_refill(bucket.capacity, bucket.tokens, time_diff, bucket.rate);
        bucket.last_updated = now;
    }

    bucket
}

/// Remove both outbound and inbound rate limit buckets for a chain
/// (default + fast-finality).
pub fn remove_buckets(env: &Env, chain_selector: u64) {
    env.storage()
        .persistent()
        .remove(&PoolDataKey::OutboundRateLimit(chain_selector));
    env.storage()
        .persistent()
        .remove(&PoolDataKey::InboundRateLimit(chain_selector));
    env.storage()
        .persistent()
        .remove(&PoolDataKey::FtfOutboundRateLimit(chain_selector));
    env.storage()
        .persistent()
        .remove(&PoolDataKey::FtfInboundRateLimit(chain_selector));
}

// ================================================================
//  Convenience wrappers keyed by chain + direction
// ================================================================

/// Consume outbound rate limit for a remote chain.
/// Pool contracts should emit [`OutboundRateLimitConsumedEvent`] after calling.
pub fn consume_outbound(
    env: &Env,
    remote_chain_selector: u64,
    amount: i128,
) -> Result<(), CCIPError> {
    consume(
        env,
        &PoolDataKey::OutboundRateLimit(remote_chain_selector),
        amount,
    )
}

/// Consume inbound rate limit for a remote chain.
/// Pool contracts should emit [`InboundRateLimitConsumedEvent`] after calling.
pub fn consume_inbound(
    env: &Env,
    remote_chain_selector: u64,
    amount: i128,
) -> Result<(), CCIPError> {
    consume(
        env,
        &PoolDataKey::InboundRateLimit(remote_chain_selector),
        amount,
    )
}

/// Consume fast-finality outbound rate limit. Falls back to default outbound
/// bucket if the FTF bucket is not enabled. Returns `true` if FTF bucket was
/// consumed, `false` if fallback to default.
///
/// TODO: This function is likely unnecessary for Stellar. Stellar has
/// deterministic ~5s finality, so outbound sends will never carry a non-default
/// finality tag. Consider removing once the FTF-outbound simplification in
/// `lock_or_burn` is applied. Only `consume_ftf_inbound` is needed for
/// Stellar as a destination chain.
pub fn consume_ftf_outbound(
    env: &Env,
    remote_chain_selector: u64,
    amount: i128,
) -> Result<bool, CCIPError> {
    let ftf_key = PoolDataKey::FtfOutboundRateLimit(remote_chain_selector);
    let ftf_bucket: TokenBucket = env
        .storage()
        .persistent()
        .get(&ftf_key)
        .unwrap_or(TokenBucket::disabled());

    if !ftf_bucket.is_enabled {
        consume_outbound(env, remote_chain_selector, amount)?;
        return Ok(false);
    }
    consume(env, &ftf_key, amount)?;
    Ok(true)
}

/// Consume fast-finality inbound rate limit. Falls back to default inbound
/// bucket if the FTF bucket is not enabled. Returns `true` if FTF bucket was
/// consumed, `false` if fallback to default.
pub fn consume_ftf_inbound(
    env: &Env,
    remote_chain_selector: u64,
    amount: i128,
) -> Result<bool, CCIPError> {
    let ftf_key = PoolDataKey::FtfInboundRateLimit(remote_chain_selector);
    let ftf_bucket: TokenBucket = env
        .storage()
        .persistent()
        .get(&ftf_key)
        .unwrap_or(TokenBucket::disabled());

    if !ftf_bucket.is_enabled {
        consume_inbound(env, remote_chain_selector, amount)?;
        return Ok(false);
    }
    consume(env, &ftf_key, amount)?;
    Ok(true)
}

// ================================================================
//  Internal helpers
// ================================================================

/// `min(capacity, tokens + time_diff * rate)` with saturating arithmetic.
pub(crate) fn calculate_refill(capacity: u128, tokens: u128, time_diff: u64, rate: u128) -> u128 {
    let refill = (time_diff as u128).saturating_mul(rate);
    core::cmp::min(capacity, tokens.saturating_add(refill))
}
