//! Storage keys and TTL management for RBACTimelock.

use crate::types::TimelockDataKey;
use soroban_sdk::{symbol_short, Address, BytesN, Env, Map, Symbol, Vec};

/// Minimum delay between scheduling and execution (seconds). Stored in instance storage.
pub const MIN_DELAY: Symbol = symbol_short!("MNDELAY");

/// Role membership map: `Symbol (role) → Vec<Address>` — persistent.
pub const ROLES_KEY: Symbol = symbol_short!("ROLES");

/// Blocked function selectors (Soroban Symbols) — persistent.
pub const BLK_SEL: Symbol = symbol_short!("BLKSEL");

/// If a persistent entry's TTL falls below this count (~1 week at 5 s/ledger),
/// proactively extend to [`LEDGER_BUMP`].
pub const LEDGER_THRESHOLD: u32 = 120_960;

/// Target TTL after proactive extension (~1 year at 5 s/ledger).
pub const LEDGER_BUMP: u32 = 6_307_200;

// --- Role storage ---

pub fn get_roles_map(env: &Env) -> Map<Symbol, Vec<Address>> {
    env.storage()
        .persistent()
        .get(&ROLES_KEY)
        .unwrap_or(Map::new(env))
}

pub fn set_roles_map(env: &Env, roles: &Map<Symbol, Vec<Address>>) {
    env.storage().persistent().set(&ROLES_KEY, roles);
}

// --- Operation timestamp storage ---
//
// Each operation id uses its own persistent entry (`TimelockDataKey::OpTime(id)`), matching
// Solidity's `mapping(bytes32 => uint256) _timestamps` (no monolithic map growth).

fn op_time_key(id: &BytesN<32>) -> TimelockDataKey {
    TimelockDataKey::OpTime(id.clone())
}

/// Refresh TTL for a single operation timestamp entry (if present).
pub fn extend_op_time_entry_ttl(env: &Env, id: &BytesN<32>) {
    let key = op_time_key(id);
    let st = env.storage().persistent();
    if st.has(&key) {
        st.extend_ttl(&key, LEDGER_THRESHOLD, LEDGER_BUMP);
    }
}

pub fn get_op_timestamp(env: &Env, id: &BytesN<32>) -> u64 {
    let key = op_time_key(id);
    let st = env.storage().persistent();
    if !st.has(&key) {
        return 0;
    }
    st.extend_ttl(&key, LEDGER_THRESHOLD, LEDGER_BUMP);
    st.get(&key).unwrap()
}

pub fn set_op_timestamp(env: &Env, id: &BytesN<32>, ts: u64) {
    let key = op_time_key(id);
    let st = env.storage().persistent();
    st.set(&key, &ts);
    st.extend_ttl(&key, LEDGER_THRESHOLD, LEDGER_BUMP);
}

pub fn delete_op_timestamp(env: &Env, id: &BytesN<32>) {
    let key = op_time_key(id);
    env.storage().persistent().remove(&key);
}

// --- Blocked selector storage ---

pub fn get_blocked_selectors(env: &Env) -> Vec<Symbol> {
    env.storage()
        .persistent()
        .get(&BLK_SEL)
        .unwrap_or(Vec::new(env))
}

pub fn set_blocked_selectors(env: &Env, selectors: &Vec<Symbol>) {
    env.storage().persistent().set(&BLK_SEL, selectors);
}

// --- Min delay ---

pub fn get_min_delay(env: &Env) -> u64 {
    env.storage().instance().get(&MIN_DELAY).unwrap_or(0)
}

pub fn set_min_delay(env: &Env, delay: u64) {
    env.storage().instance().set(&MIN_DELAY, &delay);
}

// --- TTL management ---

/// Bump TTLs for instance storage and fixed persistent keys (`ROLES`, blocked selectors).
///
/// Per-operation timestamps live in separate entries; each is extended when read or
/// written (`get_op_timestamp` / `set_op_timestamp`), or via [`extend_op_time_entry_ttl`].
/// Called at the end of every successful mutation. Also used by `extend_all_ttls`.
pub fn bump_ttls(env: &Env) {
    env.storage()
        .instance()
        .extend_ttl(LEDGER_THRESHOLD, LEDGER_BUMP);
    if env.storage().persistent().has(&ROLES_KEY) {
        env.storage()
            .persistent()
            .extend_ttl(&ROLES_KEY, LEDGER_THRESHOLD, LEDGER_BUMP);
    }
    if env.storage().persistent().has(&BLK_SEL) {
        env.storage()
            .persistent()
            .extend_ttl(&BLK_SEL, LEDGER_THRESHOLD, LEDGER_BUMP);
    }
}
