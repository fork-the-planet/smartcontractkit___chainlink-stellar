#![no_std]
//! CRE ReadContract fixture — static getters and pure echoes for Local-CRE E2E.
//!
//! No storage. No auth. No initialize. Values are compile-time constants or
//! identity transforms so consensus can assert exact base64 XDR ScVal results.

use soroban_sdk::{
    contract, contracterror, contractimpl, contracttype, map, panic_with_error, symbol_short,
    Address, Bytes, BytesN, Env, Map, String, Symbol, Vec,
};

#[contract]
pub struct ReadFixture;

/// Compact composite return for exact XDR asserts.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Sample {
    pub flag: bool,
    pub n: u32,
    pub label: Symbol,
}

/// Multi-arg echo shape (Address + BytesN<32> + BytesN<2>).
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Triple {
    pub a: Address,
    pub id: BytesN<32>,
    pub rid: BytesN<2>,
}

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq, PartialOrd, Ord)]
#[repr(u32)]
pub enum FixtureError {
    IntentionalTrap = 1,
}

#[contractimpl]
impl ReadFixture {
    // ─── Primitive constants ───────────────────────────────────────────

    pub fn get_bool(_env: Env) -> bool {
        true
    }

    pub fn get_u32(_env: Env) -> u32 {
        42
    }

    pub fn get_i32(_env: Env) -> i32 {
        -7
    }

    pub fn get_u64(_env: Env) -> u64 {
        1_000_000_000_000
    }

    pub fn get_i64(_env: Env) -> i64 {
        -1_000_000_000_000
    }

    pub fn get_u128(_env: Env) -> u128 {
        42
    }

    pub fn get_i128(_env: Env) -> i128 {
        -42
    }

    pub fn get_string(env: Env) -> String {
        String::from_str(&env, "read-fixture")
    }

    pub fn get_symbol(_env: Env) -> Symbol {
        symbol_short!("fixture")
    }

    pub fn get_bytes(env: Env) -> Bytes {
        Bytes::from_array(&env, &[0xde, 0xad, 0xbe, 0xef])
    }

    pub fn get_void(_env: Env) {}

    pub fn get_self(env: Env) -> Address {
        env.current_contract_address()
    }

    // ─── Collections / composite ───────────────────────────────────────

    pub fn get_vec_u32(env: Env) -> Vec<u32> {
        let mut out = Vec::new(&env);
        out.push_back(1u32);
        out.push_back(2u32);
        out.push_back(3u32);
        out
    }

    pub fn get_map(env: Env) -> Map<Symbol, u32> {
        let mut m = map![&env];
        m.set(symbol_short!("a"), 1u32);
        m.set(symbol_short!("b"), 2u32);
        m
    }

    pub fn get_sample(_env: Env) -> Sample {
        Sample {
            flag: true,
            n: 7,
            label: symbol_short!("ok"),
        }
    }

    pub fn get_bytes_n32(env: Env) -> BytesN<32> {
        BytesN::from_array(&env, &[0u8; 32])
    }

    pub fn get_bytes_n2(env: Env) -> BytesN<2> {
        BytesN::from_array(&env, &[0xab, 0xcd])
    }

    // ─── Echo / round-trip (still static: no storage) ──────────────────

    pub fn echo_bool(_env: Env, v: bool) -> bool {
        v
    }

    pub fn echo_u32(_env: Env, v: u32) -> u32 {
        v
    }

    pub fn echo_i64(_env: Env, v: i64) -> i64 {
        v
    }

    pub fn echo_string(_env: Env, v: String) -> String {
        v
    }

    pub fn echo_symbol(_env: Env, v: Symbol) -> Symbol {
        v
    }

    pub fn echo_bytes(_env: Env, v: Bytes) -> Bytes {
        v
    }

    pub fn echo_address(_env: Env, v: Address) -> Address {
        v
    }

    pub fn echo_bytes_n32(_env: Env, v: BytesN<32>) -> BytesN<32> {
        v
    }

    pub fn echo_bytes_n2(_env: Env, v: BytesN<2>) -> BytesN<2> {
        v
    }

    pub fn echo_vec_u32(_env: Env, v: Vec<u32>) -> Vec<u32> {
        v
    }

    pub fn echo_sample(_env: Env, v: Sample) -> Sample {
        v
    }

    pub fn echo_triple(_env: Env, a: Address, id: BytesN<32>, rid: BytesN<2>) -> Triple {
        Triple { a, id, rid }
    }

    pub fn sum_u32(_env: Env, a: u32, b: u32) -> u32 {
        a.saturating_add(b)
    }

    // ─── Intentional failure ────────────────

    pub fn trap(env: Env) {
        panic_with_error!(&env, FixtureError::IntentionalTrap);
    }
}
