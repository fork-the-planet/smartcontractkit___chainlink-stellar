use common_authorization::Ownable;
use common_error::CCIPError;
use common_guard::initializable::Initializable;
use soroban_sdk::{
    contracttrait, contracttype, symbol_short, Bytes, BytesN, Env, Map, Symbol, Vec,
};

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignatureQuorumConfig {
    pub source_chain_selector: u64,
    pub threshold: u32,
    /// TODO: confirm signer encoding from offchain verifier signer set format.
    /// Using 32-byte Ed25519 pubkeys as scaffold.
    pub signers: Vec<BytesN<32>>,
}

// TODO: re-enable this when the config types needed are beyond `SignatureConfig`
// pub trait SignatureConfigStateGetters: TryFromVal<Env, Val> + IntoVal<Env, Val> + PartialEq {
//     fn new(source_chain_selector: u64, threshold: u32, signers: Vec<BytesN<32>>) -> Self;
//     fn threshold(&self) -> u32;
//     fn signers(&self) -> Vec<BytesN<32>>;
// }

#[contracttrait]
pub trait SignatureQuorum: Initializable + Ownable {
    const SIGNATURE_CONFIGS: Symbol = symbol_short!("SIGCFGS");

    const VERIFIER_VERSION_BYTES: u32 = 4;
    const SIGNATURE_LENGTH_BYTES: u32 = 2;
    const SIGNATURE_THRESHOLD_BYTES: u32 = 2;

    fn extract_signature_length(signatures: &Bytes) -> Result<u32, CCIPError> {
        unimplemented!()
    }

    fn extract_signatures(signatures: &Bytes) -> Result<Vec<BytesN<32>>, CCIPError> {
        unimplemented!()
    }

    fn extract_signature_threshold(signatures: &Bytes) -> Result<u32, CCIPError> {
        unimplemented!()
    }

    fn extract_signature_pubkey(signatures: &Bytes) -> Result<BytesN<32>, CCIPError> {
        unimplemented!()
    }

    fn extract_version_tag(env: &Env, verifier_results: &Bytes) -> Result<BytesN<4>, CCIPError> {
        if verifier_results.len() < Self::VERIFIER_VERSION_BYTES {
            return Err(CCIPError::InvalidVerifierResults);
        }
        let mut out = [0u8; 4];
        let mut i = 0u32;
        while i < Self::VERIFIER_VERSION_BYTES {
            out[i as usize] = verifier_results
                .get(i)
                .ok_or(CCIPError::InvalidVerifierResults)?;
            i += 1;
        }
        Ok(BytesN::from_array(env, &out))
    }

    fn extract_signature_len(verifier_results: &Bytes) -> Result<u32, CCIPError> {
        if verifier_results.len() < Self::VERIFIER_VERSION_BYTES + Self::SIGNATURE_LENGTH_BYTES {
            return Err(CCIPError::InvalidVerifierResults);
        }
        let b0 = verifier_results
            .get(Self::VERIFIER_VERSION_BYTES)
            .ok_or(CCIPError::InvalidVerifierResults)?;
        let b1 = verifier_results
            .get(Self::VERIFIER_VERSION_BYTES + 1)
            .ok_or(CCIPError::InvalidVerifierResults)?;
        Ok(((b0 as u32) << 8) | (b1 as u32))
    }

    fn validate_signatures(
        env: &Env,
        source_chain_selector: u64,
        signed_hash: BytesN<32>,
        signatures: Bytes,
    ) -> Result<(), CCIPError> {
        let sig_cfgs: Map<u64, SignatureQuorumConfig> = env
            .storage()
            .persistent()
            .get(&Self::SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(env));

        let cfg = sig_cfgs
            .get(source_chain_selector)
            .ok_or(CCIPError::SourceSignersNotConfigured)?;

        if cfg.threshold == 0 {
            return Err(CCIPError::SourceSignersNotConfigured);
        }

        // TODO: implement native Soroban Ed25519 quorum validation:
        // 1) Define signature serialization format in verifier_results.
        // 2) Recover/parse per-signature public keys and signature bytes.
        // 3) Enforce ordering/uniqueness semantics to match EVM invariants.
        // 4) Call env.crypto().ed25519_verify(pubkey, signed_hash, signature).
        let _ = (cfg, signatures, signed_hash);
        Ok(())
    }

    fn get_signature_config(
        env: Env,
        source_chain_selector: u64,
    ) -> Result<SignatureQuorumConfig, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        // TODO: auth guard?

        let sig_cfgs: Map<u64, SignatureQuorumConfig> = env
            .storage()
            .persistent()
            .get(&Self::SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(&env));
        sig_cfgs
            .get(source_chain_selector)
            .ok_or(CCIPError::SourceSignersNotConfigured)
    }

    fn get_all_signature_configs(env: Env) -> Result<Vec<SignatureQuorumConfig>, CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;

        let sig_cfgs: Map<u64, SignatureQuorumConfig> = env
            .storage()
            .persistent()
            .get(&Self::SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(&env));

        let mut out = Vec::new(&env);
        for (source_chain_selector, cfg) in sig_cfgs.iter() {
            out.push_back(SignatureQuorumConfig {
                source_chain_selector,
                threshold: cfg.threshold,
                signers: cfg.signers,
            });
        }
        Ok(out)
    }

    fn apply_signature_configs(
        env: Env,
        source_chains_to_remove: Vec<u64>,
        signature_configs: Vec<SignatureQuorumConfig>,
    ) -> Result<(), CCIPError> {
        <Self as Initializable>::require_initialized(&env)?;
        <Self as Ownable>::require_owner(&env)?;

        let mut sig_cfgs: Map<u64, SignatureQuorumConfig> = env
            .storage()
            .persistent()
            .get(&Self::SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(&env));

        for source_chain_selector in source_chains_to_remove.iter() {
            sig_cfgs.remove(source_chain_selector);
            // TODO: publish SignatureConfigSet(source_chain_selector, [], 0).
        }

        for update in signature_configs.iter() {
            if update.threshold == 0 || update.threshold > update.signers.len() {
                return Err(CCIPError::InvalidSignatureThreshold);
            }

            sig_cfgs.set(
                update.source_chain_selector,
                SignatureQuorumConfig {
                    source_chain_selector: update.source_chain_selector,
                    threshold: update.threshold,
                    signers: update.signers,
                },
            );

            // TODO: publish SignatureConfigSet.
        }

        env.storage()
            .persistent()
            .set(&Self::SIGNATURE_CONFIGS, &sig_cfgs);
        Ok(())
    }
}
