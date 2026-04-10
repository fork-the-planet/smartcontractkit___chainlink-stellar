use soroban_sdk::{Bytes, Env};

use crate::decimals::{calculate_local_amount, encode_local_decimals, parse_remote_decimals};
use common_error::CCIPError;

#[test]
fn encode_decode_roundtrip_single_byte() {
    let env = Env::default();
    let enc = encode_local_decimals(&env, 7).unwrap();
    assert_eq!(enc.len(), 32);
    assert_eq!(parse_remote_decimals(&enc, 18).unwrap(), 7);
}

#[test]
fn encode_matches_big_endian_uint256_layout() {
    let env = Env::default();
    let enc = encode_local_decimals(&env, 255).unwrap();
    let mut expected = [0u8; 32];
    expected[31] = 255;
    for i in 0..32 {
        assert_eq!(enc.get(i).unwrap(), expected[i as usize]);
    }
}

#[test]
fn empty_source_pool_data_falls_back_to_local() {
    let env = Env::default();
    let empty = Bytes::new(&env);
    assert_eq!(parse_remote_decimals(&empty, 9).unwrap(), 9);
}

#[test]
fn wrong_length_source_pool_data_errors() {
    let env = Env::default();
    let short = Bytes::from_slice(&env, &[1, 2, 3]);
    assert_eq!(
        parse_remote_decimals(&short, 7),
        Err(CCIPError::InvalidRemoteChainDecimals)
    );
}

#[test]
fn scale_down_remote_gt_local() {
    // 9 remote, 6 local: 1_000_000_000 remote minimal units → 1_000_000 local
    assert_eq!(
        calculate_local_amount(1_000_000_000, 9, 6).unwrap(),
        1_000_000
    );
}

#[test]
fn scale_up_remote_lt_local() {
    // 6 remote, 9 local: 1_000_000 → 1_000_000_000
    assert_eq!(
        calculate_local_amount(1_000_000, 6, 9).unwrap(),
        1_000_000_000
    );
}

#[test]
fn same_decimals_unchanged() {
    assert_eq!(calculate_local_amount(42, 18, 18).unwrap(), 42);
}

#[test]
fn negative_source_errors() {
    assert_eq!(
        calculate_local_amount(-1, 18, 18),
        Err(CCIPError::InvalidTokenAmount)
    );
}
