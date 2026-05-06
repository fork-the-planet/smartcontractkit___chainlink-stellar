//! Domain separator constants — must match [`docs/mcms-stellar-plan.md`](../../../docs/mcms-stellar-plan.md).
//!
//! Verified with `cast keccak "$(printf '%s' 'MANY_CHAIN_MULTI_SIG_DOMAIN_SEPARATOR_…')"` (Foundry).

use soroban_sdk::{BytesN, Env};

/// `keccak256("MANY_CHAIN_MULTI_SIG_DOMAIN_SEPARATOR_OP_STELLAR")`
pub const DOMAIN_OP_STELLAR: [u8; 32] = [
    0x12, 0xcd, 0xc8, 0x8e, 0x33, 0xb5, 0x9a, 0x3a, 0x5a, 0x9f, 0xe3, 0x07, 0x2e, 0x0b, 0xab, 0x63,
    0xee, 0x3d, 0xb8, 0x88, 0xaf, 0x2c, 0xdb, 0x10, 0xbc, 0x93, 0x34, 0x56, 0x88, 0x05, 0x8d, 0x16,
];

/// `keccak256("MANY_CHAIN_MULTI_SIG_DOMAIN_SEPARATOR_METADATA_STELLAR")`
pub const DOMAIN_META_STELLAR: [u8; 32] = [
    0xde, 0x51, 0xf2, 0xd6, 0x7b, 0xb4, 0x89, 0x5d, 0x0d, 0xd1, 0xf3, 0x6a, 0xdb, 0x04, 0x42, 0x27,
    0xaa, 0x7b, 0x76, 0x4d, 0x4e, 0x52, 0x4d, 0x6b, 0x0d, 0x70, 0x04, 0x72, 0x27, 0x28, 0xfd, 0xa0,
];

pub fn domain_op(env: &Env) -> BytesN<32> {
    BytesN::from_array(env, &DOMAIN_OP_STELLAR)
}

pub fn domain_meta(env: &Env) -> BytesN<32> {
    BytesN::from_array(env, &DOMAIN_META_STELLAR)
}

/// If a persistent entry's remaining TTL falls below this ledger count (~1 week at 5 s/ledger),
/// proactively extend it to [`LEDGER_BUMP`].
pub const LEDGER_THRESHOLD: u32 = 120_960;

/// Target TTL (in ledgers) after a proactive extension (~1 year at 5 s/ledger).
pub const LEDGER_BUMP: u32 = 6_307_200;

/// Maximum horizon for `set_root(..., valid_until, ...)`: `valid_until` must be ≤ ledger timestamp + this value.
///
/// Limits how long a signed root remains acceptable relative to `SeenHash` replay protection and
/// Stellar persistent entry TTL. **90 days** in seconds.
///
/// This is the **static absolute upper bound**. The runtime check in `set_root` further clamps
/// `valid_until` against a **dynamic** cap derived from [`LEDGER_BUMP`] and the operator-configured
/// `min_secs_per_ledger` (see `McmsContract::set_min_secs_per_ledger`), so the effective cap is
/// `min(MAX_ROOT_VALIDITY_SECS, LEDGER_BUMP * min_secs_per_ledger - SEEN_TTL_SAFETY_MARGIN_SECS)`.
pub const MAX_ROOT_VALIDITY_SECS: u64 = 90 * 24 * 60 * 60;

/// Default pessimistic floor on seconds-per-ledger used by the dynamic `valid_until` cap when the
/// contract has not been told otherwise. Stellar mainnet has historically targeted ~5 s/ledger.
pub const MIN_SECS_PER_LEDGER_DEFAULT: u64 = 5;

/// Hard lower bound for the operator-settable `min_secs_per_ledger`. A value of 0 is invalid
/// (would make the dynamic cap collapse to a negative interval and trivially reject every root).
pub const MIN_SECS_PER_LEDGER_LOWER_BOUND: u64 = 1;

/// Hard upper bound for the operator-settable `min_secs_per_ledger`. Stellar ledgers don't
/// realistically take a minute; this prevents accidentally widening the dynamic cap by an
/// implausibly large multiplier.
pub const MIN_SECS_PER_LEDGER_UPPER_BOUND: u64 = 60;

/// Safety margin (in seconds) subtracted from the dynamic `valid_until` cap so a freshly bumped
/// `SeenHash` entry is guaranteed to outlive the longest acceptable `valid_until`. **1 week**.
pub const SEEN_TTL_SAFETY_MARGIN_SECS: u64 = 7 * 24 * 60 * 60;

// Compile-time invariant: the static cap, plus the safety margin, must stay strictly below the
// minimum possible lifetime of a freshly bumped `SeenHash` entry under the *default* assumed
// seconds-per-ledger. Any future change to `LEDGER_BUMP`, `MAX_ROOT_VALIDITY_SECS`,
// `MIN_SECS_PER_LEDGER_DEFAULT`, or `SEEN_TTL_SAFETY_MARGIN_SECS` that breaks this relation will
// fail to compile, forcing the change to be reasoned about explicitly.
const _: () = {
    assert!(
        MAX_ROOT_VALIDITY_SECS + SEEN_TTL_SAFETY_MARGIN_SECS
            < (LEDGER_BUMP as u64) * MIN_SECS_PER_LEDGER_DEFAULT,
        "MAX_ROOT_VALIDITY_SECS must stay strictly below the default-pessimistic SeenHash TTL window"
    );
};
