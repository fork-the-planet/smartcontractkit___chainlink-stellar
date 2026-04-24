#![no_std]

mod abi_encoding;
mod constants;
mod crypto;
mod error;
mod types;

pub use error::McmsError;
pub use types::{
    Config, ExpiringRootAndOpCount, MerkleProof, Signature, SignatureVec, Signer, SignerAddresses,
    SignerGroups, StellarOp, StellarRootMetadata, MAX_NUM_SIGNERS, NUM_GROUPS,
};

use abi_encoding::{
    eth_signed_message_hash_32, hash_root_metadata, hash_set_root_inner, hash_stellar_op,
};
use common_authorization::Ownable;
use common_error::CCIPError;
use common_guard::initializable::Initializable;
use constants::{domain_meta, domain_op};
use crypto::{cmp_bytes32, recover_eth_address_vrs, verify_merkle_proof};
use soroban_sdk::{
    address_payload::AddressPayload, contract, contractimpl, symbol_short, Address, Bytes, BytesN,
    Env, Map, Symbol, Vec,
};
use stellar_strkey::Contract as StrkeyContract;

// --- Storage ---

const INITIALIZED: Symbol = symbol_short!("INIT");
const OWNER: Symbol = symbol_short!("OWNER");
const PENDING_OWNER: Symbol = symbol_short!("PNDGOWNR");
/// Network passphrase hash bytes32 (`chain-selectors` Stellar `ChainID`).
const CHAIN_NETWORK_ID: Symbol = symbol_short!("CHNET");
const CONFIG: Symbol = symbol_short!("MCSCFG");
/// Map padded signer addr -> Signer
const SIGNER_MAP: Symbol = symbol_short!("SIGMAP");
/// replay protection for set_root
const SEEN_HASHES: Symbol = symbol_short!("SEEN");
const EXPIRING_ROOT: Symbol = symbol_short!("EXPROOT");
const ROOT_META_STORE: Symbol = symbol_short!("RTMETA");

#[contract]
pub struct McmsContract;

#[contractimpl]
impl Initializable for McmsContract {
    const INITIALIZED: Symbol = INITIALIZED;
}

#[contractimpl(contracttrait)]
impl Ownable for McmsContract {
    const OWNER: Symbol = OWNER;
    const PENDING_OWNER: Symbol = PENDING_OWNER;
}

#[contractimpl]
impl McmsContract {
    /// Initialize MCMS with owner and Stellar network id (32-byte passphrase hash per `chain-selectors`).
    pub fn initialize(
        env: Env,
        owner: Address,
        chain_network_id: BytesN<32>,
    ) -> Result<(), McmsError> {
        <Self as Initializable>::require_not_initialized(&env)?;
        <Self as Ownable>::init_owner(&env, &owner).map_err(McmsError::from)?;
        <Self as Initializable>::init(&env)?;
        env.storage()
            .instance()
            .set(&CHAIN_NETWORK_ID, &chain_network_id);
        Ok(())
    }

    /// Governance smoke test target — callable via `execute` with empty `data`.
    pub fn mcms_ping(env: Env) {
        let _ = env.current_contract_address();
    }

    /// Owner-only signer configuration (mirrors Solidity `setConfig`).
    pub fn set_config(
        env: Env,
        signer_addresses: SignerAddresses,
        signer_groups: SignerGroups,
        group_quorums: BytesN<32>,
        group_parents: BytesN<32>,
        clear_root: bool,
    ) -> Result<(), McmsError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env).map_err(McmsError::from)?;

        let signer_addresses = signer_addresses.inner;
        let signer_groups = signer_groups.inner;

        let len = signer_addresses.len();
        if len == 0 || len > MAX_NUM_SIGNERS {
            return Err(McmsError::OutOfBoundsNumOfSigners);
        }
        if len != signer_groups.len() {
            return Err(McmsError::SignerGroupsLengthMismatch);
        }

        validate_group_tree(&env, &signer_groups, &group_quorums, &group_parents)?;

        let mut sig_map: Map<BytesN<32>, Signer> = Map::new(&env);
        let mut prev: Option<BytesN<32>> = None;
        let mut i = 0u32;
        while i < len {
            let addr = signer_addresses.get(i).unwrap();
            if let Some(ref p) = prev {
                if cmp_bytes32(p, &addr) >= 0 {
                    return Err(McmsError::SignersAddressesMustBeStrictlyIncreasing);
                }
            }
            let grp = signer_groups.get(i).unwrap();
            if grp >= NUM_GROUPS {
                return Err(McmsError::OutOfBoundsGroup);
            }
            let signer = Signer {
                addr: addr.clone(),
                index: i,
                group: grp,
            };
            sig_map.set(addr.clone(), signer);
            prev = Some(addr.clone());
            i += 1;
        }

        let cfg = Config {
            signers: collect_signers_vec(&env, &signer_addresses, &signer_groups)?,
            group_quorums,
            group_parents,
        };

        env.storage().persistent().set(&CONFIG, &cfg);
        env.storage().persistent().set(&SIGNER_MAP, &sig_map);

        if clear_root {
            let exp: ExpiringRootAndOpCount = env
                .storage()
                .persistent()
                .get(&EXPIRING_ROOT)
                .unwrap_or(ExpiringRootAndOpCount {
                    root: BytesN::from_array(&env, &[0u8; 32]),
                    valid_until: 0,
                    op_count: 0,
                });
            let oc = exp.op_count;
            let self_id = contract_id_of_address(&env.current_contract_address());
            let meta = StellarRootMetadata {
                chain_id: env.storage().instance().get(&CHAIN_NETWORK_ID).unwrap(),
                multisig: self_id,
                pre_op_count: oc,
                post_op_count: oc,
                override_previous_root: true,
            };
            env.storage().persistent().set(
                &EXPIRING_ROOT,
                &ExpiringRootAndOpCount {
                    root: BytesN::from_array(&env, &[0u8; 32]),
                    valid_until: 0,
                    op_count: oc,
                },
            );
            env.storage().persistent().set(&ROOT_META_STORE, &meta);
        }

        Ok(())
    }

    pub fn set_root(
        env: Env,
        root: BytesN<32>,
        valid_until: u32,
        metadata: StellarRootMetadata,
        metadata_proof: MerkleProof,
        signatures: SignatureVec,
    ) -> Result<(), McmsError> {
        <Self as Initializable>::require_initialized(&env)?;

        let inner = hash_set_root_inner(&env, &root, valid_until);
        let signed_hash = eth_signed_message_hash_32(&env, &inner);

        if !env.storage().persistent().has(&SEEN_HASHES) {
            let empty: Map<BytesN<32>, bool> = Map::new(&env);
            env.storage().persistent().set(&SEEN_HASHES, &empty);
        }

        let mut seen: Map<BytesN<32>, bool> = env.storage().persistent().get(&SEEN_HASHES).unwrap();
        if seen.get(signed_hash.clone()).unwrap_or(false) {
            return Err(McmsError::SignedHashAlreadySeen);
        }

        let cfg: Config = env
            .storage()
            .persistent()
            .get(&CONFIG)
            .ok_or(McmsError::MissingConfig)?;
        let sig_map: Map<BytesN<32>, Signer> = env.storage().persistent().get(&SIGNER_MAP).unwrap();

        verify_signatures(&env, &cfg, &sig_map, &signed_hash, &signatures.inner)?;

        let now = env.ledger().timestamp();
        if u64::from(valid_until) < now {
            return Err(McmsError::ValidUntilHasAlreadyPassed);
        }

        let dm = domain_meta(&env);
        let hashed_leaf = hash_root_metadata(&env, &dm, &metadata)?;
        if !verify_merkle_proof(&env, &root, &hashed_leaf, metadata_proof.inner) {
            return Err(McmsError::ProofCannotBeVerified);
        }

        let chain_net: BytesN<32> = env.storage().instance().get(&CHAIN_NETWORK_ID).unwrap();
        if metadata.chain_id != chain_net {
            return Err(McmsError::WrongChainIdMeta);
        }

        let self_id = contract_id_of_address(&env.current_contract_address());
        if metadata.multisig != self_id {
            return Err(McmsError::WrongMultiSigMeta);
        }

        let exp: ExpiringRootAndOpCount =
            env.storage()
                .persistent()
                .get(&EXPIRING_ROOT)
                .unwrap_or(ExpiringRootAndOpCount {
                    root: BytesN::from_array(&env, &[0u8; 32]),
                    valid_until: 0,
                    op_count: 0,
                });

        let stored_meta: StellarRootMetadata = env
            .storage()
            .persistent()
            .get(&ROOT_META_STORE)
            .unwrap_or(StellarRootMetadata {
                chain_id: chain_net.clone(),
                multisig: self_id.clone(),
                pre_op_count: 0,
                post_op_count: 0,
                override_previous_root: false,
            });

        let op_count = exp.op_count;
        if op_count != stored_meta.post_op_count && !metadata.override_previous_root {
            return Err(McmsError::PendingOps);
        }
        if op_count != metadata.pre_op_count {
            return Err(McmsError::WrongPreOpCount);
        }
        if metadata.pre_op_count > metadata.post_op_count {
            return Err(McmsError::WrongPostOpCount);
        }

        seen.set(signed_hash.clone(), true);
        env.storage().persistent().set(&SEEN_HASHES, &seen);

        env.storage().persistent().set(
            &EXPIRING_ROOT,
            &ExpiringRootAndOpCount {
                root: root.clone(),
                valid_until,
                op_count: metadata.pre_op_count,
            },
        );
        env.storage().persistent().set(&ROOT_META_STORE, &metadata);

        Ok(())
    }

    pub fn execute(env: Env, op: StellarOp, proof: MerkleProof) -> Result<(), McmsError> {
        <Self as Initializable>::require_initialized(&env)?;

        let meta: StellarRootMetadata = env
            .storage()
            .persistent()
            .get(&ROOT_META_STORE)
            .ok_or(McmsError::MissingConfig)?;
        let mut exp: ExpiringRootAndOpCount = env
            .storage()
            .persistent()
            .get(&EXPIRING_ROOT)
            .unwrap_or(ExpiringRootAndOpCount {
                root: BytesN::from_array(&env, &[0u8; 32]),
                valid_until: 0,
                op_count: 0,
            });

        if meta.post_op_count <= exp.op_count {
            return Err(McmsError::PostOpCountReached);
        }

        let chain_net: BytesN<32> = env.storage().instance().get(&CHAIN_NETWORK_ID).unwrap();
        if op.chain_id != chain_net {
            return Err(McmsError::WrongChainIdOp);
        }

        let self_id = contract_id_of_address(&env.current_contract_address());
        if op.multisig != self_id {
            return Err(McmsError::WrongMultiSigOp);
        }

        let now = env.ledger().timestamp();
        if now > u64::from(exp.valid_until) {
            return Err(McmsError::RootExpired);
        }

        if op.nonce != exp.op_count {
            return Err(McmsError::WrongNonce);
        }

        let d = domain_op(&env);
        let leaf = hash_stellar_op(&env, &d, &op)?;

        if !verify_merkle_proof(&env, &exp.root, &leaf, proof.inner) {
            return Err(McmsError::ProofCannotBeVerified);
        }

        exp.op_count = exp
            .op_count
            .checked_add(1)
            .ok_or(McmsError::NonceOverflow)?;
        env.storage().persistent().set(&EXPIRING_ROOT, &exp);

        let target = contract_address_from_contract_id(&env, &op.to);
        let fn_sym = decode_fn(&op.data)?;
        let args = Vec::new(&env);

        let _ = env.invoke_contract::<()>(&target, &fn_sym, args);

        Ok(())
    }

    // --- getters ---

    pub fn get_config(env: Env) -> Result<Config, McmsError> {
        <Self as Initializable>::require_initialized(&env)?;
        env.storage()
            .persistent()
            .get(&CONFIG)
            .ok_or(McmsError::MissingConfig)
    }

    pub fn get_op_count(env: Env) -> Result<u64, McmsError> {
        <Self as Initializable>::require_initialized(&env)?;
        let exp: ExpiringRootAndOpCount =
            env.storage()
                .persistent()
                .get(&EXPIRING_ROOT)
                .unwrap_or(ExpiringRootAndOpCount {
                    root: BytesN::from_array(&env, &[0u8; 32]),
                    valid_until: 0,
                    op_count: 0,
                });
        Ok(exp.op_count)
    }

    pub fn get_root(env: Env) -> Result<(BytesN<32>, u32), McmsError> {
        <Self as Initializable>::require_initialized(&env)?;
        let exp: ExpiringRootAndOpCount =
            env.storage()
                .persistent()
                .get(&EXPIRING_ROOT)
                .unwrap_or(ExpiringRootAndOpCount {
                    root: BytesN::from_array(&env, &[0u8; 32]),
                    valid_until: 0,
                    op_count: 0,
                });
        Ok((exp.root, exp.valid_until))
    }

    pub fn get_root_metadata(env: Env) -> Result<StellarRootMetadata, McmsError> {
        <Self as Initializable>::require_initialized(&env)?;
        env.storage()
            .persistent()
            .get(&ROOT_META_STORE)
            .ok_or(McmsError::MissingConfig)
    }

    pub fn chain_network_id(env: Env) -> Result<BytesN<32>, McmsError> {
        <Self as Initializable>::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&CHAIN_NETWORK_ID)
            .ok_or(McmsError::NotInitialized)
    }
}

fn collect_signers_vec(
    env: &Env,
    addrs: &Vec<BytesN<32>>,
    groups: &Vec<u32>,
) -> Result<Vec<Signer>, McmsError> {
    let mut out = Vec::new(env);
    let mut i = 0u32;
    while i < addrs.len() {
        out.push_back(Signer {
            addr: addrs.get(i).unwrap(),
            index: i,
            group: groups.get(i).unwrap(),
        });
        i += 1;
    }
    Ok(out)
}

fn validate_group_tree(
    env: &Env,
    signer_groups: &Vec<u32>,
    group_quorums: &BytesN<32>,
    group_parents: &BytesN<32>,
) -> Result<(), McmsError> {
    let mut group_children_counts = [0u32; 32];

    let mut i = 0u32;
    while i < signer_groups.len() {
        let g = signer_groups.get(i).unwrap() as usize;
        if g >= 32 {
            return Err(McmsError::OutOfBoundsGroup);
        }
        group_children_counts[g] += 1;
        i += 1;
    }

    let gq = group_quorums.to_array();
    let gp = group_parents.to_array();

    let mut j = 0usize;
    while j < NUM_GROUPS as usize {
        let idx = NUM_GROUPS as usize - 1 - j;
        if (idx != 0 && gp[idx] as usize >= idx) || (idx == 0 && gp[idx] != 0) {
            return Err(McmsError::GroupTreeNotWellFormed);
        }
        let disabled = gq[idx] == 0;
        if disabled {
            if group_children_counts[idx] > 0 {
                return Err(McmsError::SignerInDisabledGroup);
            }
        } else {
            if group_children_counts[idx] < gq[idx] as u32 {
                return Err(McmsError::OutOfBoundsGroupQuorum);
            }
            let parent = gp[idx] as usize;
            group_children_counts[parent] += 1;
        }
        j += 1;
    }

    let _ = env;
    Ok(())
}

fn verify_signatures(
    env: &Env,
    cfg: &Config,
    sig_map: &Map<BytesN<32>, Signer>,
    signed_hash: &BytesN<32>,
    signatures: &Vec<Signature>,
) -> Result<(), McmsError> {
    let gq = cfg.group_quorums.to_array();
    let gp = cfg.group_parents.to_array();

    if gq[0] == 0 {
        return Err(McmsError::MissingConfig);
    }

    let mut group_vote_counts: Map<u32, u32> = Map::new(env);
    let mut prev: Option<BytesN<32>> = None;

    let mut i = 0u32;
    while i < signatures.len() {
        let sig = signatures.get(i).unwrap();
        let recovered = recover_eth_address_vrs(env, signed_hash, sig.v, &sig.r, &sig.s)?;

        if let Some(ref p) = prev {
            if cmp_bytes32(p, &recovered) >= 0 {
                return Err(McmsError::SignersAddressesMustBeStrictlyIncreasingSigs);
            }
        }
        prev = Some(recovered.clone());

        let signer = sig_map
            .get(recovered.clone())
            .ok_or(McmsError::InvalidSigner)?;

        let mut group = signer.group;
        loop {
            let cv = group_vote_counts.get(group).unwrap_or(0);
            group_vote_counts.set(group, cv + 1);
            let cur = group_vote_counts.get(group).unwrap();

            if cur != gq[group as usize] as u32 {
                break;
            }
            if group == 0 {
                break;
            }
            group = gp[group as usize] as u32;
        }

        i += 1;
    }

    let root_votes = group_vote_counts.get(0).unwrap_or(0);
    if root_votes < gq[0] as u32 {
        return Err(McmsError::InsufficientSigners);
    }

    Ok(())
}

fn decode_fn(data: &Bytes) -> Result<Symbol, McmsError> {
    if data.len() == 0 {
        Ok(symbol_short!("mcms_ping"))
    } else {
        Err(McmsError::InvalidInvokeData)
    }
}

fn contract_address_from_contract_id(env: &Env, id: &BytesN<32>) -> Address {
    let sk = StrkeyContract(id.to_array());
    let encoded = sk.to_string();
    Address::from_str(env, encoded.as_str())
}

fn contract_id_of_address(addr: &Address) -> BytesN<32> {
    match addr.to_payload() {
        Some(AddressPayload::ContractIdHash(id)) => id,
        _ => BytesN::from_array(addr.env(), &[0u8; 32]),
    }
}

#[cfg(test)]
mod test;
