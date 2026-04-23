//! Merkle verification (sorted pairwise keccak256) + ECDSA recover helpers.

use soroban_sdk::{crypto::Hash, BytesN, Env, Vec};

use crate::error::McmsError;

/// Keccak(pair) with **lexicographic sort** — matches `mcms/internal/core/merkle` & OpenZeppelin.
pub fn efficient_hash_pair(env: &Env, a: &BytesN<32>, b: &BytesN<32>) -> BytesN<32> {
    let mut combined = soroban_sdk::Bytes::new(env);
    if cmp_bytes32(a, b) <= 0 {
        combined.extend_from_slice(&a.to_array());
        combined.extend_from_slice(&b.to_array());
    } else {
        combined.extend_from_slice(&b.to_array());
        combined.extend_from_slice(&a.to_array());
    }
    env.crypto().keccak256(&combined).into()
}

pub fn cmp_bytes32(a: &BytesN<32>, b: &BytesN<32>) -> i32 {
    let aa = a.to_array();
    let bb = b.to_array();
    let mut i = 0usize;
    while i < 32 {
        let ac = aa[i];
        let bc = bb[i];
        if ac < bc {
            return -1;
        }
        if ac > bc {
            return 1;
        }
        i += 1;
    }
    0
}

pub fn verify_merkle_proof(
    env: &Env,
    root: &BytesN<32>,
    leaf: &BytesN<32>,
    proof: Vec<BytesN<32>>,
) -> bool {
    let mut computed = leaf.clone();
    let mut i = 0u32;
    while i < proof.len() {
        let sib = proof.get(i).unwrap();
        computed = efficient_hash_pair(env, &computed, &sib);
        i += 1;
    }
    computed == *root
}

/// Match OpenZeppelin `ECDSA.recover` with `(v, r, s)` signature bytes.
pub fn recover_eth_address_vrs(
    env: &Env,
    digest: &BytesN<32>,
    v: u32,
    r: &BytesN<32>,
    s: &BytesN<32>,
) -> Result<BytesN<32>, McmsError> {
    let mut sig = [0u8; 64];
    sig[..32].copy_from_slice(&r.to_array());
    sig[32..].copy_from_slice(&s.to_array());
    let mut recovery_id = v;
    if recovery_id >= 27 {
        recovery_id -= 27;
    }
    if recovery_id > 1 {
        return Err(McmsError::InvalidSignature);
    }
    let sig64 = BytesN::<64>::from_array(env, &sig);
    let hash_ref: &Hash<32> = unsafe { &*(digest as *const BytesN<32> as *const Hash<32>) };
    let pubkey: BytesN<65> = env
        .crypto()
        .secp256k1_recover(hash_ref, &sig64, recovery_id);

    let body = soroban_sdk::Bytes::from_slice(env, &pubkey.to_array()[1..]);
    let kh: BytesN<32> = env.crypto().keccak256(&body).into();
    let arr = kh.to_array();

    let mut padded = [0u8; 32];
    padded[12..32].copy_from_slice(&arr[12..32]);
    Ok(BytesN::from_array(env, &padded))
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_sdk::Env;

    #[test]
    fn cmp_bytes32_orders_lexicographically() {
        let env = Env::default();
        let low = BytesN::from_array(&env, &[1u8; 32]);
        let high = BytesN::from_array(&env, &[2u8; 32]);
        assert!(cmp_bytes32(&low, &high) < 0);
        assert!(cmp_bytes32(&high, &low) > 0);
        assert_eq!(cmp_bytes32(&low, &low), 0);
    }

    #[test]
    fn efficient_hash_pair_sorts_operands() {
        let env = Env::default();
        let a = BytesN::from_array(&env, &[2u8; 32]);
        let b = BytesN::from_array(&env, &[1u8; 32]);
        let h_ab = efficient_hash_pair(&env, &a, &b);
        let h_ba = efficient_hash_pair(&env, &b, &a);
        assert_eq!(h_ab, h_ba);
    }

    #[test]
    fn verify_merkle_proof_empty_proof_requires_leaf_equals_root() {
        let env = Env::default();
        let leaf = BytesN::from_array(&env, &[7u8; 32]);
        let root_match = leaf.clone();
        assert!(verify_merkle_proof(
            &env,
            &root_match,
            &leaf,
            Vec::<BytesN<32>>::new(&env)
        ));

        let root_other = BytesN::from_array(&env, &[8u8; 32]);
        assert!(!verify_merkle_proof(
            &env,
            &root_other,
            &leaf,
            Vec::<BytesN<32>>::new(&env)
        ));
    }
}
