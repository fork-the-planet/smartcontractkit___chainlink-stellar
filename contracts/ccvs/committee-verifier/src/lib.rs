#![no_std]

pub mod error;
mod events;
pub mod types;

use common_verifier::{
    AllowlistConfigArgs, BaseVerifier, RemoteChainConfig, RemoteChainConfigArgs,
};
use error::CommitteeVerifierError;
use soroban_sdk::{
    contract, contractimpl, symbol_short, Address, Bytes, BytesN, Env, Map, Symbol, Vec,
};
use types::{DynamicConfig, SignatureConfig, SignatureConfigState};

// ============================================================
// Storage Keys
// ============================================================

const INITIALIZED: Symbol = symbol_short!("INIT");
const OWNER: Symbol = symbol_short!("OWNER");
const DYNAMIC_CONFIG: Symbol = symbol_short!("DYNCFG");
const SIGNATURE_CONFIGS: Symbol = symbol_short!("SIGCFGS");
const STORAGE_LOC_ADMIN: Symbol = symbol_short!("STORADM");
const PENDING_STORAGE_LOC_ADMIN: Symbol = symbol_short!("PSTORADM");


// ============================================================
// Constants
// ============================================================

const VERSION_TAG_V1_7_0: [u8; 4] = [0x49, 0xff, 0x34, 0xed];
const VERIFIER_VERSION_BYTES: u32 = 4;
const SIGNATURE_LENGTH_BYTES: u32 = 2;

// ============================================================
// Contract
// ============================================================

#[contract]
pub struct CommitteeVerifierContract;

#[contractimpl]
impl CommitteeVerifierContract {
    /// Initializes CommitteeVerifier with owner/dynamic config/storage locations/RMN proxy.
    ///
    /// Mirrors EVM constructor + dynamic config initialization.
    pub fn initialize(
        env: Env,
        owner: Address,
        dynamic_config: DynamicConfig,
        storage_locations: Vec<Bytes>,
        rmn_proxy: Address,
    ) -> Result<(), CommitteeVerifierError> {
        if env.storage().instance().has(&INITIALIZED) {
            return Err(CommitteeVerifierError::AlreadyInitialized);
        }

        env.storage().instance().set(&OWNER, &owner);
        env.storage()
            .instance()
            .set(&DYNAMIC_CONFIG, &dynamic_config);
        env.storage().instance().set(&STORAGE_LOC_ADMIN, &owner);
        env.storage().instance().set(&INITIALIZED, &true);
        BaseVerifier::init(&env, &storage_locations, &rmn_proxy);

        let sig_cfgs: Map<u64, SignatureConfigState> = Map::new(&env);
        env.storage()
            .persistent()
            .set(&SIGNATURE_CONFIGS, &sig_cfgs);

        // TODO: Publish ConfigSet + ownership/bootstrap events via `events.rs`.
        Ok(())
    }

    // ========================================
    // Core verifier methods
    // ========================================

    /// Source-side hook that checks sender permissions and returns version tag.
    ///
    /// TODO: replace current args with canonical MessageV1 once shared message types are finalized.
    pub fn forward_to_verifier(
        env: Env,
        dest_chain_selector: u64,
        sender: Address,
        _message_hash: BytesN<32>,
        _fee_token: Address,
        _fee_token_amount: i128,
        _verifier_args: Bytes,
    ) -> Result<Bytes, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        BaseVerifier::assert_not_cursed_by_rmn(&env, dest_chain_selector)?;
        BaseVerifier::assert_sender_is_allowed(&env, dest_chain_selector, &sender)?;

        Ok(Bytes::from_array(&env, &VERSION_TAG_V1_7_0))
    }

    /// Destination-side hook that parses verifier result payload and validates signatures.
    ///
    /// TODO: bind to canonical inbound message struct instead of `(source_chain_selector, message_hash)`.
    pub fn verify_message(
        env: Env,
        source_chain_selector: u64,
        message_hash: BytesN<32>,
        verifier_results: Bytes,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        BaseVerifier::assert_not_cursed_by_rmn(&env, source_chain_selector)?;

        if verifier_results.len() < VERIFIER_VERSION_BYTES + SIGNATURE_LENGTH_BYTES {
            return Err(CommitteeVerifierError::InvalidVerifierResults);
        }

        let version = Self::extract_version_tag(&env, &verifier_results)?;
        if version != BytesN::from_array(&env, &VERSION_TAG_V1_7_0) {
            return Err(CommitteeVerifierError::InvalidCCVVersion);
        }

        let signature_len = Self::extract_signature_len(&verifier_results)?;
        let expected = VERIFIER_VERSION_BYTES + SIGNATURE_LENGTH_BYTES + signature_len;
        if verifier_results.len() < expected {
            return Err(CommitteeVerifierError::InvalidVerifierResults);
        }

        // TODO: finalize exact signed payload format with offchain signer pipeline.
        // EVM signs keccak256(versionTag || messageHash). We mirror that shape here.
        let mut signed_payload = Bytes::new(&env);
        signed_payload.append(&Bytes::from_array(&env, &VERSION_TAG_V1_7_0));
        signed_payload.append(&Bytes::from_array(&env, &message_hash.to_array()));
        let signed_hash: BytesN<32> = env.crypto().keccak256(&signed_payload).into();

        let signatures =
            verifier_results.slice(VERIFIER_VERSION_BYTES + SIGNATURE_LENGTH_BYTES..expected);
        Self::validate_signatures(&env, source_chain_selector, signed_hash, signatures)
    }

    /// Returns static version tag used in outbound verifier responses.
    pub fn version_tag(env: Env) -> BytesN<4> {
        BytesN::from_array(&env, &VERSION_TAG_V1_7_0)
    }

    // ========================================
    // Dynamic config
    // ========================================

    pub fn get_dynamic_config(env: Env) -> Result<DynamicConfig, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&DYNAMIC_CONFIG)
            .ok_or(CommitteeVerifierError::NotInitialized)
    }

    pub fn set_dynamic_config(
        env: Env,
        dynamic_config: DynamicConfig,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;
        env.storage()
            .instance()
            .set(&DYNAMIC_CONFIG, &dynamic_config);
        // TODO: publish ConfigSet event.
        Ok(())
    }

    // ========================================
    // Base verifier config
    // ========================================

    pub fn get_storage_locations(env: Env) -> Result<Vec<Bytes>, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        Ok(BaseVerifier::get_storage_locations(&env))
    }

    pub fn apply_remote_chain_cfg_updates(
        env: Env,
        remote_chain_config_args: Vec<RemoteChainConfigArgs>,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        BaseVerifier::apply_remote_chain_config_updates(&env, &remote_chain_config_args)?;
        Ok(())
    }

    pub fn get_remote_chain_config(
        env: Env,
        remote_chain_selector: u64,
    ) -> Result<RemoteChainConfig, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        BaseVerifier::get_remote_chain_config(&env, remote_chain_selector).map_err(Into::into)
    }

    pub fn apply_allowlist_updates(
        env: Env,
        allowlist_config_args_items: Vec<AllowlistConfigArgs>,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        Self::require_owner_or_allowlist_admin(&env)?;

        BaseVerifier::apply_allowlist_updates(&env, &allowlist_config_args_items)?;
        Ok(())
    }

    /// EVM-equivalent fee quote shape.
    pub fn get_fee(
        env: Env,
        dest_chain_selector: u64,
        _message: Bytes,
        _extra_args: Bytes,
        _block_confirmations: u32,
    ) -> Result<(u32, u32, u32), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        BaseVerifier::get_fee(&env, dest_chain_selector).map_err(Into::into)
    }

    // ========================================
    // Signature config
    // ========================================

    pub fn get_signature_config(
        env: Env,
        source_chain_selector: u64,
    ) -> Result<SignatureConfigState, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        let sig_cfgs: Map<u64, SignatureConfigState> = env
            .storage()
            .persistent()
            .get(&SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(&env));
        sig_cfgs
            .get(source_chain_selector)
            .ok_or(CommitteeVerifierError::SourceNotConfigured)
    }

    pub fn get_all_signature_configs(
        env: Env,
    ) -> Result<Vec<SignatureConfig>, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        let sig_cfgs: Map<u64, SignatureConfigState> = env
            .storage()
            .persistent()
            .get(&SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(&env));

        let mut out = Vec::new(&env);
        for (source_chain_selector, cfg) in sig_cfgs.iter() {
            out.push_back(SignatureConfig {
                source_chain_selector,
                threshold: cfg.threshold,
                signers: cfg.signers,
            });
        }
        Ok(out)
    }

    pub fn apply_signature_configs(
        env: Env,
        source_chains_to_remove: Vec<u64>,
        signature_configs: Vec<SignatureConfig>,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;

        let mut sig_cfgs: Map<u64, SignatureConfigState> = env
            .storage()
            .persistent()
            .get(&SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(&env));

        for source_chain_selector in source_chains_to_remove.iter() {
            sig_cfgs.remove(source_chain_selector);
            // TODO: publish SignatureConfigSet(source_chain_selector, [], 0).
        }

        for update in signature_configs.iter() {
            if update.threshold == 0 || update.threshold > update.signers.len() {
                return Err(CommitteeVerifierError::InvalidSignatureConfig);
            }
            sig_cfgs.set(
                update.source_chain_selector,
                SignatureConfigState {
                    threshold: update.threshold,
                    signers: update.signers,
                },
            );
            // TODO: publish SignatureConfigSet.
        }

        env.storage()
            .persistent()
            .set(&SIGNATURE_CONFIGS, &sig_cfgs);
        Ok(())
    }

    // ========================================
    // Storage locations
    // ========================================

    pub fn get_storage_locations_admin(env: Env) -> Result<Address, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&STORAGE_LOC_ADMIN)
            .ok_or(CommitteeVerifierError::NotInitialized)
    }

    pub fn get_pending_storage_loc_admin(
        env: Env,
    ) -> Result<Option<Address>, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        Ok(env.storage().instance().get(&PENDING_STORAGE_LOC_ADMIN))
    }

    pub fn transfer_storage_locations_admin(
        env: Env,
        to: Address,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        let current_admin = Self::get_storage_locations_admin(env.clone())?;
        current_admin.require_auth();

        env.storage()
            .instance()
            .set(&PENDING_STORAGE_LOC_ADMIN, &to);

        // TODO: publish StorageLocationsAdminTransferRequested.
        Ok(())
    }

    pub fn accept_storage_locations_admin(env: Env) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        let pending: Address = env
            .storage()
            .instance()
            .get(&PENDING_STORAGE_LOC_ADMIN)
            .ok_or(CommitteeVerifierError::InvalidStorageLocationsAdmin)?;
        pending.require_auth();

        let old_admin = Self::get_storage_locations_admin(env.clone())?;
        env.storage().instance().set(&STORAGE_LOC_ADMIN, &pending);
        env.storage().instance().remove(&PENDING_STORAGE_LOC_ADMIN);

        let _ = old_admin;
        // TODO: publish StorageLocationsAdminTransferred.
        Ok(())
    }

    pub fn update_storage_locations(
        env: Env,
        new_locations: Vec<Bytes>,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        let admin = Self::get_storage_locations_admin(env.clone())?;
        admin.require_auth();

        BaseVerifier::set_storage_locations(&env, &new_locations);

        // TODO: publish StorageLocationsUpdated(old_locations, new_locations).
        Ok(())
    }

    // ========================================
    // Fees
    // ========================================

    pub fn withdraw_fee_tokens(
        env: Env,
        fee_tokens: Vec<Address>,
    ) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        let dynamic = Self::get_dynamic_config(env.clone())?;

        // TODO: integrate token transfer / fee token handler logic.
        // Fee withdrawal is permissionless in EVM and transfers to fee_aggregator.
        let _ = (dynamic.fee_aggregator, fee_tokens);
        Ok(())
    }

    // ========================================
    // Ownership
    // ========================================

    pub fn owner(env: Env) -> Result<Address, CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        env.storage()
            .instance()
            .get(&OWNER)
            .ok_or(CommitteeVerifierError::NotInitialized)
    }

    pub fn transfer_ownership(env: Env, new_owner: Address) -> Result<(), CommitteeVerifierError> {
        Self::require_initialized(&env)?;
        Self::require_owner(&env)?;
        // TODO: use common-authorization two-step ownership transfer once this crate is wired.
        env.storage().instance().set(&OWNER, &new_owner);
        Ok(())
    }

    // ========================================
    // Internal helpers
    // ========================================

    fn require_initialized(env: &Env) -> Result<(), CommitteeVerifierError> {
        if !env.storage().instance().has(&INITIALIZED) {
            return Err(CommitteeVerifierError::NotInitialized);
        }
        Ok(())
    }

    fn require_owner(env: &Env) -> Result<(), CommitteeVerifierError> {
        let owner: Address = env
            .storage()
            .instance()
            .get(&OWNER)
            .ok_or(CommitteeVerifierError::NotInitialized)?;
        owner.require_auth();
        Ok(())
    }

    fn require_owner_or_allowlist_admin(env: &Env) -> Result<(), CommitteeVerifierError> {
        let dynamic: DynamicConfig = env
            .storage()
            .instance()
            .get(&DYNAMIC_CONFIG)
            .ok_or(CommitteeVerifierError::NotInitialized)?;

        // TODO: Soroban auth is capability-based; implement explicit owner-or-admin policy
        // matching EVM msg.sender checks without requiring both parties in auth entries.
        // Current scaffold requires owner auth and records allowlist admin for future wiring.
        Self::require_owner(env)?;
        if dynamic.allowlist_admin.is_some() {
            return Ok(());
        }
        Ok(())
    }

    fn extract_version_tag(
        env: &Env,
        verifier_results: &Bytes,
    ) -> Result<BytesN<4>, CommitteeVerifierError> {
        if verifier_results.len() < VERIFIER_VERSION_BYTES {
            return Err(CommitteeVerifierError::InvalidVerifierResults);
        }
        let mut out = [0u8; 4];
        let mut i = 0u32;
        while i < VERIFIER_VERSION_BYTES {
            out[i as usize] = verifier_results
                .get(i)
                .ok_or(CommitteeVerifierError::InvalidVerifierResults)?;
            i += 1;
        }
        Ok(BytesN::from_array(env, &out))
    }

    fn extract_signature_len(verifier_results: &Bytes) -> Result<u32, CommitteeVerifierError> {
        if verifier_results.len() < VERIFIER_VERSION_BYTES + SIGNATURE_LENGTH_BYTES {
            return Err(CommitteeVerifierError::InvalidVerifierResults);
        }
        let b0 = verifier_results
            .get(VERIFIER_VERSION_BYTES)
            .ok_or(CommitteeVerifierError::InvalidVerifierResults)?;
        let b1 = verifier_results
            .get(VERIFIER_VERSION_BYTES + 1)
            .ok_or(CommitteeVerifierError::InvalidVerifierResults)?;
        Ok(((b0 as u32) << 8) | (b1 as u32))
    }

    fn validate_signatures(
        env: &Env,
        source_chain_selector: u64,
        signed_hash: BytesN<32>,
        signatures: Bytes,
    ) -> Result<(), CommitteeVerifierError> {
        let sig_cfgs: Map<u64, SignatureConfigState> = env
            .storage()
            .persistent()
            .get(&SIGNATURE_CONFIGS)
            .unwrap_or(Map::new(env));
        let cfg = sig_cfgs
            .get(source_chain_selector)
            .ok_or(CommitteeVerifierError::SourceNotConfigured)?;
        if cfg.threshold == 0 {
            return Err(CommitteeVerifierError::SourceNotConfigured);
        }

        // TODO: implement native Soroban Ed25519 quorum validation:
        // 1) Define signature serialization format in verifier_results.
        // 2) Recover/parse per-signature public keys and signature bytes.
        // 3) Enforce ordering/uniqueness semantics to match EVM invariants.
        // 4) Call env.crypto().ed25519_verify(pubkey, signed_hash, signature).
        let _ = (cfg, signatures, signed_hash);
        Ok(())
    }
}

mod test;
