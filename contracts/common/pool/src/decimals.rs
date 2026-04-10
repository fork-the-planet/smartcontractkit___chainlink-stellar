//! Cross-chain token decimal handling aligned with EVM [`TokenPool`](https://github.com/smartcontractkit/chainlink-ccip/blob/develop/chains/evm/contracts/pools/TokenPool.sol):
//! - `encode_local_decimals` matches `abi.encode(uint256(localDecimals))` (32-byte big-endian).
//! - `parse_remote_decimals` matches `_parseRemoteDecimals` (empty → local decimals).
//! - `calculate_local_amount` matches `_calculateLocalAmount` (including the 77-decimal guard).

use soroban_sdk::{Bytes, Env};

use common_error::CCIPError;

/// ABI width for `abi.encode(uint256)` decimals payload on EVM.
pub const ENCODED_DECIMALS_LEN: u32 = 32;

/// Same guard as EVM `TokenPool._calculateLocalAmount` to avoid unbounded `10**diff`.
const MAX_DECIMALS_DIFF: u32 = 77;

/// Encode local token decimals for `LockOrBurnOut.dest_pool_data` (EVM `_encodeLocalDecimals`).
pub fn encode_local_decimals(env: &Env, local_decimals: u32) -> Result<Bytes, CCIPError> {
    if local_decimals > u8::MAX as u32 {
        return Err(CCIPError::InvalidPoolTokenDecimals);
    }
    // `abi.encode(uint256(decimals))` for decimals ≤ 255 is 31 zero bytes + one least-significant byte.
    let mut buf = [0u8; ENCODED_DECIMALS_LEN as usize];
    buf[31] = local_decimals as u8;
    Ok(Bytes::from_array(env, &buf))
}

/// Parse remote decimals from `ReleaseOrMintIn.source_pool_data` (EVM `_parseRemoteDecimals`).
pub fn parse_remote_decimals(
    source_pool_data: &Bytes,
    local_decimals: u32,
) -> Result<u32, CCIPError> {
    if source_pool_data.len() == 0 {
        return Ok(local_decimals);
    }
    if source_pool_data.len() != ENCODED_DECIMALS_LEN {
        return Err(CCIPError::InvalidRemoteChainDecimals);
    }
    let v = read_u256_be32(source_pool_data)?;
    if v > u8::MAX as u128 {
        return Err(CCIPError::InvalidRemoteChainDecimals);
    }
    Ok(v as u32)
}

fn read_u256_be32(data: &Bytes) -> Result<u128, CCIPError> {
    let mut v: u128 = 0;
    for i in 0u32..ENCODED_DECIMALS_LEN {
        let b = data.get(i).ok_or(CCIPError::InvalidRemoteChainDecimals)? as u128;
        v = v
            .checked_mul(256)
            .and_then(|x| x.checked_add(b))
            .ok_or(CCIPError::InvalidRemoteChainDecimals)?;
    }
    Ok(v)
}

fn pow10_u128(exp: u32) -> Result<u128, CCIPError> {
    let mut p: u128 = 1;
    for _ in 0..exp {
        p = p.checked_mul(10).ok_or(CCIPError::DecimalAmountOverflow)?;
    }
    Ok(p)
}

/// Convert `source_denominated_amount` (remote decimals) into local minimal units (EVM `_calculateLocalAmount`).
///
/// Requires a non-negative `source_denominated_amount` (bridge amounts are unsigned).
pub fn calculate_local_amount(
    source_denominated_amount: i128,
    remote_decimals: u32,
    local_decimals: u32,
) -> Result<i128, CCIPError> {
    if source_denominated_amount < 0 {
        return Err(CCIPError::InvalidTokenAmount);
    }
    let s = source_denominated_amount as u128;

    if remote_decimals == local_decimals {
        return Ok(source_denominated_amount);
    }

    if remote_decimals > local_decimals {
        let diff = remote_decimals - local_decimals;
        if diff > MAX_DECIMALS_DIFF {
            return Err(CCIPError::DecimalAmountOverflow);
        }
        let pow = pow10_u128(diff)?;
        let q = s / pow;
        if q > i128::MAX as u128 {
            return Err(CCIPError::DecimalAmountOverflow);
        }
        return Ok(q as i128);
    }

    let diff = local_decimals - remote_decimals;
    if diff > MAX_DECIMALS_DIFF {
        return Err(CCIPError::DecimalAmountOverflow);
    }
    let pow = pow10_u128(diff)?;
    let out = s.checked_mul(pow).ok_or(CCIPError::DecimalAmountOverflow)?;
    if out > i128::MAX as u128 {
        return Err(CCIPError::DecimalAmountOverflow);
    }
    Ok(out as i128)
}
