//! Solidity ABI v1 encoding matching `abi.encode` for MCMS-Stellar leaf hashes (see `docs/mcms-stellar-plan.md`).

use soroban_sdk::{Bytes, BytesN, Env};

use crate::error::McmsError;
use crate::types::{StellarOp, StellarRootMetadata};

fn extend_word32(_env: &Env, buf: &mut Bytes, word: &[u8; 32]) {
    buf.extend_from_slice(word);
}

fn append_uint40(env: &Env, buf: &mut Bytes, v: u64) -> Result<(), McmsError> {
    if v >= (1u64 << 40) {
        return Err(McmsError::InvalidUint40);
    }
    let mut w = [0u8; 32];
    let be = v.to_be_bytes();
    w[27..32].copy_from_slice(&be[3..8]);
    extend_word32(env, buf, &w);
    Ok(())
}

fn append_uint256_be(env: &Env, buf: &mut Bytes, word: &BytesN<32>) {
    extend_word32(env, buf, &word.to_array());
}

/// ABI-encodes `bytes`: length (32) + data + padding to 32-byte boundary.
fn append_abi_bytes(env: &Env, buf: &mut Bytes, data: &soroban_sdk::Bytes) {
    let len = data.len() as u64;
    let mut len_word = [0u8; 32];
    let lb = len.to_be_bytes();
    len_word[24..32].copy_from_slice(&lb);
    extend_word32(env, buf, &len_word);
    // raw bytes
    let mut i = 0u32;
    while i < data.len() {
        buf.push_back(data.get(i).unwrap());
        i += 1;
    }
    let pad = (32 - (data.len() as u32 % 32)) % 32;
    let mut z = 0u32;
    while z < pad {
        buf.push_back(0);
        z += 1;
    }
}

// TODO(M3): Audit byte-for-byte parity between hash_root_metadata / hash_stellar_op and the
// off-chain Go SDK encoder (mcms/internal/core/merkle or equivalent). Run both against identical
// inputs and compare the resulting keccak256 digests. Any field-order, padding, or offset
// mismatch here will silently invalidate every Merkle proof.

/// `keccak256(abi.encode(D_META, StellarRootMetadata))` — all-static tuple tail.
pub fn hash_root_metadata(
    env: &Env,
    domain: &BytesN<32>,
    meta: &StellarRootMetadata,
) -> Result<BytesN<32>, McmsError> {
    let mut buf = Bytes::new(env);
    extend_word32(env, &mut buf, &domain.to_array());
    append_uint256_be(env, &mut buf, &meta.chain_id);
    append_uint256_be(env, &mut buf, &meta.multisig);
    append_uint40(env, &mut buf, meta.pre_op_count)?;
    append_uint40(env, &mut buf, meta.post_op_count)?;
    let mut bool_word = [0u8; 32];
    if meta.override_previous_root {
        bool_word[31] = 1;
    }
    extend_word32(env, &mut buf, &bool_word);
    Ok(env.crypto().keccak256(&buf).into())
}

/// `keccak256(abi.encode(D_OP, StellarOp))` — tuple with trailing dynamic `bytes`.
pub fn hash_stellar_op(
    env: &Env,
    domain: &BytesN<32>,
    op: &StellarOp,
) -> Result<BytesN<32>, McmsError> {
    let mut buf = Bytes::new(env);
    extend_word32(env, &mut buf, &domain.to_array());
    // Inner tuple starts here; offset of dynamic `data` is 6 * 32 = 192 from tuple start.
    append_uint256_be(env, &mut buf, &op.chain_id);
    append_uint256_be(env, &mut buf, &op.multisig);
    append_uint40(env, &mut buf, op.nonce)?;
    append_uint256_be(env, &mut buf, &op.to);
    append_uint256_be(env, &mut buf, &op.value);
    // offset pointer (192 = 0xc0)
    let mut off = [0u8; 32];
    off[30] = 0;
    off[31] = 192u8;
    extend_word32(env, &mut buf, &off);
    append_abi_bytes(env, &mut buf, &op.data);
    Ok(env.crypto().keccak256(&buf).into())
}

/// Inner hash for ECDSA: `keccak256(abi.encode(bytes32 root, uint32 validUntil))`.
pub fn hash_set_root_inner(env: &Env, root: &BytesN<32>, valid_until: u32) -> BytesN<32> {
    let mut buf = Bytes::new(env);
    extend_word32(env, &mut buf, &root.to_array());
    let mut vu = [0u8; 32];
    let vb = valid_until.to_be_bytes();
    vu[28..32].copy_from_slice(&vb);
    extend_word32(env, &mut buf, &vu);
    env.crypto().keccak256(&buf).into()
}

/// EIP-191 Ethereum Signed Message prefix for a **32-byte** digest payload.
pub fn eth_signed_message_hash_32(env: &Env, digest: &BytesN<32>) -> BytesN<32> {
    const PREFIX: &[u8] = b"\x19Ethereum Signed Message:\n32";
    let mut buf = Bytes::new(env);
    buf.extend_from_slice(PREFIX);
    buf.extend_from_slice(&digest.to_array());
    env.crypto().keccak256(&buf).into()
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_sdk::Env;

    /// `cast keccak $(cast abi-encode "f(bytes32,uint32)" 0x00..00 0)` — ABI tuple hash preimage.
    const INNER_ROOT0_VU0: [u8; 32] = [
        0xad, 0x32, 0x28, 0xb6, 0x76, 0xf7, 0xd3, 0xcd, 0x42, 0x84, 0xa5, 0x44, 0x3f, 0x17, 0xf1,
        0x96, 0x2b, 0x36, 0xe4, 0x91, 0xb3, 0x0a, 0x40, 0xb2, 0x40, 0x58, 0x49, 0xe5, 0x97, 0xba,
        0x5f, 0xb5,
    ];

    /// `keccak256(concat(EIP191_prefix_32byte, INNER_ROOT0_VU0))` via `cast keccak`.
    const ETH_SIGNED_MESSAGE_INNER0: [u8; 32] = [
        0xf0, 0xde, 0x48, 0x43, 0x5c, 0xed, 0x1d, 0x7a, 0xe2, 0x58, 0x43, 0x89, 0x20, 0x66, 0x83,
        0xb4, 0x61, 0xf4, 0x99, 0xee, 0x5f, 0x2a, 0xf6, 0xbe, 0x74, 0x20, 0x2b, 0x7f, 0xc3, 0xca,
        0xf5, 0x9d,
    ];

    #[test]
    fn hash_set_root_inner_matches_abi_encode_bytes32_uint32() {
        let env = Env::default();
        let root = BytesN::from_array(&env, &[0u8; 32]);
        let got = hash_set_root_inner(&env, &root, 0);
        assert_eq!(got.to_array(), INNER_ROOT0_VU0);
    }

    #[test]
    fn eth_signed_message_hash_32_matches_eip191() {
        let env = Env::default();
        let inner = BytesN::from_array(&env, &INNER_ROOT0_VU0);
        let got = eth_signed_message_hash_32(&env, &inner);
        assert_eq!(got.to_array(), ETH_SIGNED_MESSAGE_INNER0);
    }
}
