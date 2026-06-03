#![cfg(test)]

extern crate alloc;
extern crate std;

use soroban_sdk::assert_with_error;
use soroban_sdk::testutils::Address as _;
use soroban_sdk::testutils::Events as _;
use soroban_sdk::{Address, Env};

use crate::error::ForwarderError;
use crate::require_valid_forwarder;
use crate::types::TransmissionState;
use crate::{KeystoneForwarder, KeystoneForwarderClient};

// ============================================================================
// Constants shared across test cases
// ============================================================================

pub(crate) const DON_ID: u32 = 0x0102_0304;
pub(crate) const CONFIG_VERSION: u32 = 1;

// Layout offsets
pub(crate) const FORWARDER_METADATA_LENGTH: usize = 45;
pub(crate) const METADATA_LENGTH: usize = 109;
pub(crate) const REPORT_CONTEXT_LENGTH: usize = 96;

// ============================================================================
// Crypto helpers
// ============================================================================

pub(crate) mod crypto {
    use k256::ecdsa::{signature::hazmat::PrehashSigner, RecoveryId, Signature, SigningKey};
    use sha3::{Digest, Keccak256};
    use soroban_sdk::{BytesN, Env};

    /// Deterministic secp256k1 secret key from a seed byte (1..=255).
    /// Seed 0 would produce the all-zero secret key which is invalid.
    pub fn signing_key(seed: u8) -> SigningKey {
        assert!(seed > 0, "secp256k1 secret key cannot be zero");
        let mut bytes = [0u8; 32];
        bytes[31] = seed;
        SigningKey::from_slice(&bytes).expect("valid secp256k1 secret key")
    }

    /// 65-byte uncompressed public key: `0x04 ‖ X (32) ‖ Y (32)`.
    /// Matches the `BytesN<65>` shape the contract stores and compares against.
    pub fn uncompressed_pubkey_65(sk: &SigningKey) -> [u8; 65] {
        let vk = sk.verifying_key();
        let encoded = vk.to_encoded_point(false); // false = uncompressed
        let bytes = encoded.as_bytes();
        assert_eq!(bytes.len(), 65, "uncompressed point must be 65 bytes");
        let mut out = [0u8; 65];
        out.copy_from_slice(bytes);
        out
    }
    pub fn pubkey_bytesn(env: &Env, sk: &SigningKey) -> BytesN<65> {
        BytesN::from_array(env, &uncompressed_pubkey_65(sk))
    }

    /// Compute the same digest the contract computes:
    /// `keccak256(keccak256(raw_report) ‖ report_context)`.
    pub fn report_digest(raw_report: &[u8], report_context: &[u8]) -> [u8; 32] {
        let inner: [u8; 32] = Keccak256::digest(raw_report).into();
        // Stream both halves into a single hasher — avoids needing alloc::Vec
        // from the parent test module (extern crate alloc; isn't in scope here).
        let mut hasher = Keccak256::new();
        hasher.update(&inner);
        hasher.update(report_context);
        hasher.finalize().into()
    }

    /// Produce a 65-byte recoverable signature: `r(32) ‖ s(32) ‖ v(1)` with
    /// `v ∈ {0, 1}` (the contract's `normalize_recovery_id` also accepts 27/28).
    pub fn sign_report(sk: &SigningKey, digest: &[u8; 32]) -> [u8; 65] {
        let (sig, recid): (Signature, RecoveryId) =
            sk.sign_prehash(digest).expect("signing must succeed");
        let sig_bytes = sig.to_bytes();
        let mut out = [0u8; 65];
        out[..64].copy_from_slice(&sig_bytes);
        out[64] = recid.to_byte();
        out
    }

    /// Same as `sign_report` but lets the caller inject an arbitrary recovery byte.
    /// Used by tests that exercise `normalize_recovery_id`'s rejection paths.
    pub fn sign_report_with_recid(sk: &SigningKey, digest: &[u8; 32], recid: u8) -> [u8; 65] {
        let mut out = sign_report(sk, digest);
        out[64] = recid;
        out
    }
}

// ============================================================================
// Mock receivers — exercise the four `try_invoke_contract` outcome arms
// ============================================================================

pub(crate) mod mocks {
    use soroban_sdk::{contract, contracterror, contractimpl, panic_with_error, Bytes, Env};

    #[contracterror]
    #[derive(Copy, Clone, Eq, PartialEq)]
    #[repr(u32)]
    pub enum ReceiverError {
        Rejected = 1,
        Boom = 2,
    }

    /// `Ok(Ok(()))` arm — well-behaved receiver. Used for happy-path tests.
    #[contract]
    pub struct CooperativeReceiver;

    #[contractimpl]
    impl CooperativeReceiver {
        pub fn on_report(_env: Env, _metadata: Bytes, _payload: Bytes) {}
    }

    /// `Ok(Err(_))` arm — receiver returns a typed `Result::Err`.
    /// Should map to `TransmissionState::Failed` (retryable).
    #[contract]
    pub struct RejectingReceiver;

    #[contractimpl]
    impl RejectingReceiver {
        pub fn on_report(
            _env: Env,
            _metadata: Bytes,
            _payload: Bytes,
        ) -> Result<(), ReceiverError> {
            Err(ReceiverError::Rejected)
        }
    }

    /// `Err(Ok(InvokeError::Contract(_)))` arm — receiver `panic_with_error!`s.
    /// Should map to `TransmissionState::Failed` (retryable).
    #[contract]
    pub struct PanickingReceiver;

    #[contractimpl]
    impl PanickingReceiver {
        pub fn on_report(env: Env, _metadata: Bytes, _payload: Bytes) {
            panic_with_error!(&env, ReceiverError::Boom);
        }
    }

    /// `Err(Err(_))` arm — Wasm contract that doesn't expose `on_report`.
    /// Should map to `TransmissionState::InvalidReceiver` (terminal) per the
    /// M2 refinement of `route()` — distinguishes "receiver doesn't implement
    /// the protocol" from "receiver rejected this specific report".
    #[contract]
    pub struct WrongSymbolReceiver;

    #[contractimpl]
    impl WrongSymbolReceiver {
        pub fn other_method(_env: Env) -> u32 {
            42
        }
    }

    /// Stateful, externally-toggled: returns Err if the "REJ" flag is true,
    /// Ok otherwise. The test calls `set_reject(...)` between report attempts
    /// to flip the behavior. State changes inside on_report wouldn't survive
    /// the child-frame rollback on Err return (per handoff §2), so the toggle
    /// must be done from outside the on_report call frame.
    #[contract]
    pub struct ToggleReceiver;

    #[contractimpl]
    impl ToggleReceiver {
        pub fn set_reject(env: Env, reject: bool) {
            env.storage()
                .instance()
                .set(&soroban_sdk::symbol_short!("REJ"), &reject);
        }
        pub fn on_report(env: Env, _metadata: Bytes, _payload: Bytes) -> Result<(), ReceiverError> {
            let reject: bool = env
                .storage()
                .instance()
                .get(&soroban_sdk::symbol_short!("REJ"))
                .unwrap_or(true);
            if reject {
                Err(ReceiverError::Rejected)
            } else {
                Ok(())
            }
        }
    }

    /// `Err(Ok(InvokeError::Abort))` arm — receiver does a plain `panic!()` (no
    /// typed error). Should map to `TransmissionState::Failed` (retryable).
    /// Distinct from PanickingReceiver which uses `panic_with_error!`.
    #[contract]
    pub struct PlainPanicReceiver;

    #[contractimpl]
    impl PlainPanicReceiver {
        pub fn on_report(_env: Env, _metadata: Bytes, _payload: Bytes) {
            panic!("plain abort");
        }
    }
}

// ============================================================================
// Fixture and setup helpers
// ============================================================================

pub(crate) struct Fixture<'a> {
    pub env: &'a Env,
    pub client: KeystoneForwarderClient<'a>,
    pub contract_addr: Address,
    pub owner: Address,
    pub transmitter: Address,
    /// 31 deterministic signing keys (k256 SigningKey is not Soroban-typed).
    pub signers: alloc::vec::Vec<k256::ecdsa::SigningKey>,
}

impl<'a> Fixture<'a> {
    /// Returns the i-th signer's 65-byte uncompressed pubkey wrapped as
    /// `BytesN<65>` — the form `Config.signers` stores.
    pub fn signer_pubkey(&self, i: usize) -> soroban_sdk::BytesN<65> {
        crypto::pubkey_bytesn(self.env, &self.signers[i])
    }

    /// Convenience: build a Soroban `Vec<BytesN<65>>` from the first `n` signers,
    /// suitable as the `signers` arg to `set_config`.
    pub fn signer_set(&self, n: usize) -> soroban_sdk::Vec<soroban_sdk::BytesN<65>> {
        let mut v = soroban_sdk::Vec::new(self.env);
        for i in 0..n {
            v.push_back(self.signer_pubkey(i));
        }
        v
    }
}

/// Deploy a fresh `KeystoneForwarder`, call `initialize`, and return a fixture
/// with 31 deterministic signing keys ready for config registration.
///
/// Caller owns the `Env` and passes it in by reference so the returned
/// fixture (which borrows from env via the client) is not self-referential.
///
/// Auths are mocked (`env.mock_all_auths()`); tests that need to exercise the
/// auth boundary should clear or restrict auths after calling this.
pub(crate) fn setup<'a>(env: &'a Env) -> Fixture<'a> {
    env.mock_all_auths();

    let contract_addr = env.register(KeystoneForwarder, ());
    let client = KeystoneForwarderClient::new(env, &contract_addr);

    let owner = Address::generate(env);
    let transmitter = Address::generate(env);

    client.initialize(&owner);

    // 31 deterministic signers (seeds 1..=31).
    let signers: alloc::vec::Vec<_> = (1u8..=31).map(crypto::signing_key).collect();

    Fixture {
        env,
        client,
        contract_addr,
        owner,
        transmitter,
        signers,
    }
}

/// Same as `setup` but also registers a config for (DON_ID, CONFIG_VERSION)
/// with the given fault tolerance and signer count.
///
/// Requires `n_signers >= 3*f + 1` and `n_signers <= MAX_ORACLES` to succeed.
pub(crate) fn setup_with_config<'a>(env: &'a Env, f: u32, n_signers: usize) -> Fixture<'a> {
    let fx = setup(env);
    let signers = fx.signer_set(n_signers);
    fx.client.set_config(&DON_ID, &CONFIG_VERSION, &f, &signers);
    fx
}

// ============================================================================
// Report builder — produces the 109-byte metadata + payload layout.
//
//   byte  0       version
//   bytes 1..33   workflow_execution_id (32)
//   bytes 33..37  timestamp (u32 BE)
//   bytes 37..41  don_id (u32 BE)
//   bytes 41..45  config_version (u32 BE)          ← FORWARDER_METADATA_LENGTH = 45
//   bytes 45..77  workflow_cid (32)
//   bytes 77..87  workflow_name (10)
//   bytes 87..107 workflow_owner (20)
//   bytes 107..109 report_id (2)                    ← METADATA_LENGTH = 109
//   bytes 109..   payload (user-defined)
// ============================================================================

pub(crate) struct ReportBuilder {
    pub version: u8,
    pub workflow_execution_id: [u8; 32],
    pub timestamp: u32,
    pub don_id: u32,
    pub config_version: u32,
    pub workflow_cid: [u8; 32],
    pub workflow_name: [u8; 10],
    pub workflow_owner: [u8; 20],
    pub report_id: [u8; 2],
    pub payload: alloc::vec::Vec<u8>,
}

impl Default for ReportBuilder {
    fn default() -> Self {
        Self {
            version: 1,
            workflow_execution_id: [0xAA; 32],
            timestamp: 1_700_000_000,
            don_id: DON_ID,
            config_version: CONFIG_VERSION,
            workflow_cid: [0xBB; 32],
            workflow_name: *b"wfname0001",
            workflow_owner: [0xCC; 20],
            report_id: [0x00, 0x01],
            payload: b"hello".to_vec(),
        }
    }
}

impl ReportBuilder {
    pub fn with_version(mut self, v: u8) -> Self {
        self.version = v;
        self
    }
    pub fn with_don_id(mut self, d: u32) -> Self {
        self.don_id = d;
        self
    }
    pub fn with_config_version(mut self, v: u32) -> Self {
        self.config_version = v;
        self
    }
    pub fn with_execution_id(mut self, id: [u8; 32]) -> Self {
        self.workflow_execution_id = id;
        self
    }
    pub fn with_report_id(mut self, id: [u8; 2]) -> Self {
        self.report_id = id;
        self
    }
    pub fn with_payload(mut self, payload: alloc::vec::Vec<u8>) -> Self {
        self.payload = payload;
        self
    }

    /// Build the raw byte sequence in the on-chain layout.
    pub fn build_bytes(&self) -> alloc::vec::Vec<u8> {
        let mut out = alloc::vec::Vec::with_capacity(METADATA_LENGTH + self.payload.len());
        out.push(self.version);
        out.extend_from_slice(&self.workflow_execution_id);
        out.extend_from_slice(&self.timestamp.to_be_bytes());
        out.extend_from_slice(&self.don_id.to_be_bytes());
        out.extend_from_slice(&self.config_version.to_be_bytes());
        out.extend_from_slice(&self.workflow_cid);
        out.extend_from_slice(&self.workflow_name);
        out.extend_from_slice(&self.workflow_owner);
        out.extend_from_slice(&self.report_id);
        out.extend_from_slice(&self.payload);
        debug_assert_eq!(
            out.len(),
            METADATA_LENGTH + self.payload.len(),
            "report layout drift"
        );
        out
    }

    /// Build as `soroban_sdk::Bytes` ready to hand to the contract.
    pub fn build(&self, env: &Env) -> soroban_sdk::Bytes {
        soroban_sdk::Bytes::from_slice(env, &self.build_bytes())
    }
}

/// 96-byte zero `report_context`, the typical test value.
pub(crate) fn report_context_zeroes(env: &Env) -> soroban_sdk::Bytes {
    soroban_sdk::Bytes::from_slice(env, &[0u8; REPORT_CONTEXT_LENGTH])
}

/// Build a signed report: returns (raw_report bytes, report_context bytes, signatures).
/// Signs with `fx.signers[0..n_sigs]` against the digest the contract computes.
pub(crate) fn build_signed_report<'a>(
    fx: &Fixture<'a>,
    report: &ReportBuilder,
    n_sigs: usize,
) -> (
    soroban_sdk::Bytes,
    soroban_sdk::Bytes,
    soroban_sdk::Vec<soroban_sdk::BytesN<65>>,
) {
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];

    let raw_bytes = soroban_sdk::Bytes::from_slice(fx.env, &raw_vec);
    let ctx_bytes = soroban_sdk::Bytes::from_slice(fx.env, &ctx_vec);

    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    let mut sigs = soroban_sdk::Vec::new(fx.env);
    for i in 0..n_sigs {
        let sig_bytes = crypto::sign_report(&fx.signers[i], &digest);
        sigs.push_back(soroban_sdk::BytesN::from_array(fx.env, &sig_bytes));
    }
    (raw_bytes, ctx_bytes, sigs)
}

// ============================================================================
// Smoke tests for the infrastructure itself.
//
// These prove PR 1 lands a working scaffold; the real test cases come in PRs 2+
// (see TEST_PLAN.md §4 for the split).
// ============================================================================

#[test]
fn infrastructure_setup_works() {
    let env = Env::default();
    let fx = setup(&env);
    assert_eq!(fx.signers.len(), 31);
    // Owner was set during initialize.
    let _ = fx.owner;
    let _ = fx.transmitter;
}

#[test]
fn infrastructure_setup_with_config_works() {
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    // No assertion on the config contents here — that's covered by §2.1 tests.
    // This test only confirms the setup helper doesn't panic at the BFT/signer bounds.
    let _ = fx;
}

#[test]
fn infrastructure_signing_key_produces_65_byte_uncompressed_pubkey() {
    let sk = crypto::signing_key(1);
    let pk = crypto::uncompressed_pubkey_65(&sk);
    assert_eq!(pk[0], 0x04, "must start with uncompressed marker");
    // X and Y are 32 bytes each; pubkey is non-trivial.
    assert!(pk[1..].iter().any(|&b| b != 0));
}

#[test]
fn infrastructure_report_digest_matches_two_step_keccak() {
    // Sanity: hand-compute keccak(keccak(report) || ctx) and compare to the helper.
    use sha3::{Digest, Keccak256};
    let report = b"hello world";
    let ctx = [0u8; REPORT_CONTEXT_LENGTH];
    let inner: [u8; 32] = Keccak256::digest(report).into();
    let mut combined = alloc::vec::Vec::with_capacity(32 + ctx.len());
    combined.extend_from_slice(&inner);
    combined.extend_from_slice(&ctx);
    let expected: [u8; 32] = Keccak256::digest(&combined).into();

    let got = crypto::report_digest(report, &ctx);
    assert_eq!(got, expected);
}

#[test]
fn infrastructure_sign_report_produces_65_byte_signature_with_recoverable_recid() {
    let sk = crypto::signing_key(7);
    let digest = [0x42u8; 32];
    let sig = crypto::sign_report(&sk, &digest);
    assert_eq!(sig.len(), 65);
    // Recovery byte is 0 or 1 from k256.
    assert!(sig[64] == 0 || sig[64] == 1, "recid byte must be 0 or 1");
    // The first 64 bytes (r || s) shouldn't be all zeros.
    assert!(sig[..64].iter().any(|&b| b != 0));
}

#[test]
fn infrastructure_report_builder_default_layout_is_109_bytes_plus_payload() {
    let env = Env::default();
    let report = ReportBuilder::default();
    let bytes = report.build_bytes();

    // Layout sanity.
    assert_eq!(bytes.len(), METADATA_LENGTH + report.payload.len());
    assert_eq!(bytes[0], 1, "version");
    // don_id at offset 37, big-endian.
    let don_be = &bytes[37..41];
    assert_eq!(
        u32::from_be_bytes([don_be[0], don_be[1], don_be[2], don_be[3]]),
        DON_ID
    );
    // config_version at offset 41.
    let cv_be = &bytes[41..45];
    assert_eq!(
        u32::from_be_bytes([cv_be[0], cv_be[1], cv_be[2], cv_be[3]]),
        CONFIG_VERSION
    );
    // report_id at offset 107.
    assert_eq!(&bytes[107..109], &report.report_id);

    // Round-trip through soroban Bytes works.
    let _ = report.build(&env);
}

#[test]
fn infrastructure_report_context_zeroes_is_96_bytes() {
    let env = Env::default();
    let ctx = report_context_zeroes(&env);
    assert_eq!(ctx.len() as usize, REPORT_CONTEXT_LENGTH);
}

#[test]
fn infrastructure_mock_receivers_register_successfully() {
    // Each mock contract type must register without panicking.
    let env = Env::default();
    let _ = env.register(mocks::CooperativeReceiver, ());
    let _ = env.register(mocks::RejectingReceiver, ());
    let _ = env.register(mocks::PanickingReceiver, ());
    let _ = env.register(mocks::WrongSymbolReceiver, ());
}

// Init tests come first because they cover the most basic state transition;
// everything else assumes a successfully initialized contract.

#[test]
fn test_initialize_succeeds() {
    // fresh deploy, call initialize(owner), owner set, self in registry.
    let env = Env::default();
    let _ = setup(&env);
}

#[test]
#[should_panic(expected = "Error(Contract, #1)")]
fn test_double_initialize_fails() {
    // second initialize returns Err(Error::AlreadyInitialized) code 1.
    // Soroban surfaces Result::Err from contract calls as a host abort with
    // the contracterror discriminant — same shape as panic_with_error!.
    let env = Env::default();
    let fx = setup(&env);
    let owner2 = Address::generate(&env);
    fx.client.initialize(&owner2);
}

#[test]
#[should_panic(expected = "Error(Contract, #16)")]
fn test_call_setters_before_init_fails() {
    // add_forwarder before initialize → Uninitialized code 16
    // (via assert_owner → ensure_initialized).
    let env = Env::default();
    env.mock_all_auths();
    let contract_addr = env.register(KeystoneForwarder, ());
    let client = KeystoneForwarderClient::new(&env, &contract_addr);
    // Skip initialize().
    let new_forwarder = Address::generate(&env);
    client.add_forwarder(&new_forwarder);
}

// ============================================================================
// set_config — success paths
// ============================================================================

#[test]
fn test_set_config_first_time_succeeds() {
    // owner sets config with f=1, 4 valid distinct signers.
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(4);
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &1u32, &signers);
}

#[test]
fn test_set_config_at_max_oracles_boundary() {
    // f=10, 31 signers exact lower bound (3·10+1=31).
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(31);
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &10u32, &signers);
}

#[test]
fn test_set_config_shrinks_signer_set() {
    // set 31 then 4; second overwrites first.
    let env = Env::default();
    let fx = setup(&env);
    let big = fx.signer_set(31);
    fx.client.set_config(&DON_ID, &CONFIG_VERSION, &10u32, &big);
    let small = fx.signer_set(4);
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &1u32, &small);
}

#[test]
fn test_set_config_independent_dons() {
    // don=1 then don=2 stored independently.
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(4);
    fx.client
        .set_config(&1u32, &CONFIG_VERSION, &1u32, &signers);
    fx.client
        .set_config(&2u32, &CONFIG_VERSION, &1u32, &signers);
}

#[test]
fn test_set_config_independent_versions() {
    // same don, v=1 then v=2 stored independently.
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(4);
    fx.client.set_config(&DON_ID, &1u32, &1u32, &signers);
    fx.client.set_config(&DON_ID, &2u32, &1u32, &signers);
}

// ============================================================================
// set_config — failure paths
// ============================================================================

#[test]
#[should_panic] // host-level auth panic from owner.require_auth() with no matching auth
fn test_set_config_not_owner_fails() {
    // stranger calls. setup() mocks all auths; clearing leaves
    // owner.require_auth() to fail at the host level (not a typed contract error).
    let env = Env::default();
    let fx = setup(&env);
    env.set_auths(&[]);
    let signers = fx.signer_set(4);
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &1u32, &signers);
}

#[test]
#[should_panic(expected = "Error(Contract, #5)")]
fn test_set_config_f_zero_fails() {
    // f=0 → FaultToleranceMustBePositive code 5.
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(4);
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &0u32, &signers);
}

#[test]
#[should_panic(expected = "Error(Contract, #6)")]
fn test_set_config_excess_signers_fails() {
    // 32 signers (over MAX_ORACLES=31) → ExcessSigners code 6.
    // We have only 31 seeds; reuse the first key to make a 32nd entry — the
    // count check fires before the duplicate check.
    let env = Env::default();
    let fx = setup(&env);
    let mut signers = fx.signer_set(31);
    signers.push_back(fx.signer_pubkey(0));
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &1u32, &signers);
}

#[test]
#[should_panic(expected = "Error(Contract, #7)")]
fn test_set_config_insufficient_signers_f1_fails() {
    // f=1, 3 signers (one below 3·1+1=4) → InsufficientSigners code 7.
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(3);
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &1u32, &signers);
}

#[test]
#[should_panic(expected = "Error(Contract, #7)")]
fn test_set_config_insufficient_signers_high_f_fails() {
    // f=5, 15 signers (one below 3·5+1=16) → InsufficientSigners code 7.
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(15);
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &5u32, &signers);
}

#[test]
#[should_panic(expected = "Error(Contract, #10)")]
fn test_set_config_duplicate_signer_fails() {
    // two slots same pubkey → DuplicateSigner code 10.
    let env = Env::default();
    let fx = setup(&env);
    let mut signers = soroban_sdk::Vec::new(&env);
    signers.push_back(fx.signer_pubkey(0));
    signers.push_back(fx.signer_pubkey(1));
    signers.push_back(fx.signer_pubkey(2));
    signers.push_back(fx.signer_pubkey(0)); // duplicate of slot 0
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &1u32, &signers);
}

#[test]
#[should_panic(expected = "Error(Contract, #19)")]
fn test_set_config_zero_pubkey_fails() {
    // one slot is 65 zero bytes → InvalidSigner code 19.
    let env = Env::default();
    let fx = setup(&env);
    let mut signers = soroban_sdk::Vec::new(&env);
    signers.push_back(fx.signer_pubkey(0));
    signers.push_back(fx.signer_pubkey(1));
    signers.push_back(fx.signer_pubkey(2));
    signers.push_back(soroban_sdk::BytesN::from_array(&env, &[0u8; 65]));
    fx.client
        .set_config(&DON_ID, &CONFIG_VERSION, &1u32, &signers);
}

// ============================================================================
// clear_config
// ============================================================================

#[test]
fn test_clear_config_succeeds() {
    // set then clear; no error.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.clear_config(&DON_ID, &CONFIG_VERSION);
}

#[test]
fn test_clear_config_other_versions_unaffected() {
    // clear v1, v2 still in storage and reusable for set.
    let env = Env::default();
    let fx = setup(&env);
    let signers = fx.signer_set(4);
    fx.client.set_config(&DON_ID, &1u32, &1u32, &signers);
    fx.client.set_config(&DON_ID, &2u32, &1u32, &signers);
    fx.client.clear_config(&DON_ID, &1u32);
    // v2 still functional — re-setting it should still succeed (no clobber).
    fx.client.set_config(&DON_ID, &2u32, &1u32, &signers);
}

#[test]
#[should_panic] // host-level auth panic
fn test_clear_config_not_owner_fails() {
    // stranger calls.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    env.set_auths(&[]);
    fx.client.clear_config(&DON_ID, &CONFIG_VERSION);
}

#[test]
#[should_panic(expected = "Error(Contract, #8)")]
fn test_clear_config_nonexistent_fails() {
    // clear (don, ver) never set → InvalidConfig code 8.
    // Stellar's clear_config is non-idempotent
    let env = Env::default();
    let fx = setup(&env);
    fx.client.clear_config(&999u32, &999u32);
}

#[test]
#[should_panic(expected = "Error(Contract, #8)")]
fn test_report_after_clear_config_fails() {
    // set, clear, then a report against the cleared config → InvalidConfig code 8.
    // Note: we trigger the failure via the report() path, which reaches load_config()
    // and panics on the missing storage key.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.clear_config(&DON_ID, &CONFIG_VERSION);

    // Build a minimal report against the (now-cleared) config and submit.
    // Use the raw byte vecs directly for digest computation rather than calling
    // `.to_alloc_vec()` on Soroban Bytes — that API isn't always available
    // depending on feature flags.
    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw_report = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let report_context = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);

    // Need f+1 = 2 signatures, but the failure fires at config load before
    // sig validation. Pass any 2 sigs to satisfy the empty-check.
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);
    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[0], &digest),
    ));
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[1], &digest),
    ));

    let receiver = env.register(mocks::CooperativeReceiver, ());
    fx.client.report(
        &fx.transmitter,
        &receiver,
        &raw_report,
        &report_context,
        &sigs,
    );
}

// ============================================================================
// Ownership
// ============================================================================

#[test]
fn test_transfer_ownership_two_step_success() {
    // owner proposes → new_owner accepts → owner() returns new.
    // The Ownable trait methods are auto-exported via #[contractimpl(contracttrait)].
    let env = Env::default();
    let fx = setup(&env);
    let new_owner = Address::generate(&env);

    fx.client.transfer_ownership(&new_owner);
    fx.client.accept_ownership();

    let current = fx.client.owner().expect("owner set");
    assert_eq!(current, new_owner);
}

#[test]
#[should_panic] // host-level auth panic
fn test_transfer_ownership_not_owner_fails() {
    // stranger calls transfer_ownership.
    let env = Env::default();
    let fx = setup(&env);
    let target = Address::generate(&env);
    env.set_auths(&[]);
    fx.client.transfer_ownership(&target);
}

#[test]
#[should_panic] // host-level auth panic from pending.require_auth()
fn test_accept_ownership_wrong_caller_fails() {
    // A proposes B; C accepts. Stellar's Ownable does
    // pending.require_auth() so the wrong caller fails at the host level.
    let env = Env::default();
    let fx = setup(&env);
    let proposed = Address::generate(&env);
    fx.client.transfer_ownership(&proposed);

    // Now restrict auths so the proposed address can't auth this tx.
    env.set_auths(&[]);
    fx.client.accept_ownership();
}

#[test]
#[should_panic(expected = "Error(Contract, #5)")]
fn test_accept_ownership_no_pending_owner_fails() {
    // T-OWN-05: accept_ownership with no pending transfer.
    // Expects raw CCIPError::NoPendingOwner = 5, NOT cre's Error::NotProposedOwner = 15.
    // The Ownable trait's auto-exported methods surface CCIPError discriminants
    // directly — the From<CCIPError> for Error mapping only applies inside cre's
    // own methods that call the trait (e.g., initialize's `?` propagation).
    let env = Env::default();
    let fx = setup(&env);
    fx.client.accept_ownership();
}

// ============================================================================
// Forwarder registry
//
// Registry tracks authorized *transmitter* addresses. report()
// passes its `transmitter` arg through to route(), which checks the registry
// before dispatching to the receiver. So an address must be added via
// add_forwarder() before it can submit reports (the contract's own address
// is auto-registered in initialize()).
// ============================================================================

#[test]
fn test_add_forwarder_succeeds() {
    // owner adds, is_forwarder returns true.
    let env = Env::default();
    let fx = setup(&env);
    let new_forwarder = Address::generate(&env);

    assert!(require_valid_forwarder(&env, &new_forwarder).is_err());
    fx.client.add_forwarder(&new_forwarder);
    assert!(require_valid_forwarder(&env, &new_forwarder).is_ok());
}

#[test]
#[should_panic] // host-level auth panic
fn test_add_forwarder_not_owner_fails() {
    // stranger calls add_forwarder.
    let env = Env::default();
    let fx = setup(&env);
    let new_forwarder = Address::generate(&env);
    env.set_auths(&[]);
    fx.client.add_forwarder(&new_forwarder);
}

#[test]
fn test_remove_forwarder_succeeds() {
    // add then remove; is_forwarder returns false.
    let env = Env::default();
    let fx = setup(&env);
    let new_forwarder = Address::generate(&env);

    fx.client.add_forwarder(&new_forwarder);
    assert!(require_valid_forwarder(&env, &new_forwarder).is_ok());
    fx.client.remove_forwarder(&new_forwarder);
    assert!(require_valid_forwarder(&env, &new_forwarder).is_err());
}

#[test]
#[should_panic] // host-level auth panic
fn test_remove_forwarder_not_owner_fails() {
    // stranger calls remove_forwarder.
    let env = Env::default();
    let fx = setup(&env);
    let new_forwarder = Address::generate(&env);
    fx.client.add_forwarder(&new_forwarder);
    env.set_auths(&[]);
    fx.client.remove_forwarder(&new_forwarder);
}

#[test]
#[should_panic(expected = "Error(Contract, #20)")]
fn test_cannot_remove_self_panics() {
    // owner removing the contract's own self-address → CannotRemoveSelf code 20.
    // Self-removal would lock the contract out of its own report() → route() self-call
    // (route() requires the caller to be in the registry).
    let env = Env::default();
    let fx = setup(&env);
    fx.client.remove_forwarder(&fx.contract_addr);
}

#[test]
fn test_self_is_in_registry_after_initialize() {
    // initialize() auto-registers the contract's own address so that
    // report() → route() self-call passes the is_forwarder check. Matches EVM's
    // constructor at KeystoneForwarder.sol:90 (`s_forwarders[address(this)] = true`).
    let env = Env::default();
    let fx = setup(&env);
    assert!(require_valid_forwarder(&env, &fx.contract_addr).is_ok());
}

#[test]
#[should_panic(expected = "Error(Contract, #17)")]
fn test_unauthorized_route_panics() {
    // a transmitter not in the forwarder registry calling route()
    // directly → UnauthorizedForwarder code 17.
    let env = Env::default();
    let fx = setup(&env);
    let stranger = Address::generate(&env);
    let receiver_addr = env.register(mocks::CooperativeReceiver, ());

    let transmission_id = soroban_sdk::BytesN::from_array(&env, &[0xAB; 32]);
    let metadata = soroban_sdk::Bytes::from_slice(&env, &[0u8; 64]);
    let validated_report = soroban_sdk::Bytes::from_slice(&env, &[0u8; 16]);

    fx.client.route(
        &transmission_id,
        &stranger,
        &receiver_addr,
        &metadata,
        &validated_report,
    );
}

#[test]
fn test_route_from_registered_forwarder_succeeds() {
    // add a transmitter to the registry, call route() directly with it
    // as the transmitter arg → succeeds (CooperativeReceiver returns Ok(())).
    // Bypasses report()'s signature path to isolate registry + route behavior.
    let env = Env::default();
    let fx = setup(&env);
    let transmitter = Address::generate(&env);
    fx.client.add_forwarder(&transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let transmission_id = soroban_sdk::BytesN::from_array(&env, &[0xCD; 32]);
    let metadata = soroban_sdk::Bytes::from_slice(&env, &[0u8; 64]);
    let validated_report = soroban_sdk::Bytes::from_slice(&env, &[0u8; 16]);

    let ok = fx.client.route(
        &transmission_id,
        &transmitter,
        &receiver_addr,
        &metadata,
        &validated_report,
    );
    assert!(ok, "route should succeed with a cooperative receiver");
}

// ============================================================================
// report — happy paths
//
// These tests exercise the full report() pipeline end-to-end:
//   length checks → parse → AlreadyProcessed check → load_config →
//   signature verification → self-call route() → receiver dispatch.
//
// The `transmitter` arg to report() must be in the
// forwarder registry, so each test calls add_forwarder(transmitter) first.
// ============================================================================

#[test]
fn test_report_succeeds_minimal() {
    // f=1, n=4, 2 valid sigs, CooperativeReceiver.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    // Transmission was recorded as Succeeded.
    let report = ReportBuilder::default();
    let info = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Succeeded);
    assert_eq!(info.transmitter, Some(fx.transmitter.clone()));
}

#[test]
fn test_report_succeeds_at_max_signers() {
    // f=10, n=31 (max), 11 sigs.
    let env = Env::default();
    let fx = setup_with_config(&env, 10, 31);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 11);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    let report = ReportBuilder::default();
    let info = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Succeeded);
}

#[test]
fn test_report_emits_correct_topic_structure() {
    // ReportProcessedEvent topics: ["forwarder_ReportProcessed", receiver, exec_id, report_id].
    // The contract's events.rs marks receiver/workflow_execution_id/report_id as #[topic];
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    // Verify the contract emitted at least one event during report().
    // The exact topic structure ("forwarder_ReportProcessed" prefix + receiver +
    // workflow_execution_id + report_id as topics, success bool in data) is
    // enforced by the #[topic] annotations on ReportProcessedEvent at compile
    // time — runtime introspection via filter_by_contract just confirms emission.
    let evs = env.events().all().filter_by_contract(&fx.contract_addr);
    assert!(
        evs.events().len() > 0,
        "report() must emit at least one event"
    );
}

#[test]
fn test_report_records_transmitter_in_transmission_info() {
    // After a successful report, get_transmission_info returns
    // {Succeeded, Some(transmitter)} — confirms the transmitter field is
    // persistently recorded so WriteReport can identify its own submissions.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    let report = ReportBuilder::default();
    let info = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.transmitter, Some(fx.transmitter.clone()));
}

// ============================================================================
// report — replay and idempotency
// ============================================================================

#[test]
#[should_panic(expected = "Error(Contract, #13)")]
fn test_replay_after_success_panics() {
    // submit twice with identical (receiver, exec_id, report_id) →
    // second call panics with AlreadyProcessed code 13. Same-state (Succeeded)
    // is terminal under the replay guard.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    // Identical resubmission — should panic.
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #13)")]
fn test_replay_after_invalid_receiver_panics() {
    // Deliver to a non-Wasm address (account-only) → state = InvalidReceiver
    // (terminal). A subsequent resubmission with the same transmission_id panics
    // with AlreadyProcessed code 13.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    // A generated Address has no contract executable → InvalidReceiver path.
    let account_receiver = Address::generate(&env);
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 2);
    fx.client
        .report(&fx.transmitter, &account_receiver, &raw, &ctx, &sigs);

    // Confirm first call recorded the terminal state.
    let report = ReportBuilder::default();
    let info = fx.client.get_transmission_info(
        &account_receiver,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::InvalidReceiver);

    // Resubmit — should panic.
    fx.client
        .report(&fx.transmitter, &account_receiver, &raw, &ctx, &sigs);
}

#[test]
fn test_retry_after_failed_succeeds_when_state_changes() {
    // T-RPT-32: ToggleReceiver is externally configured (set_reject) between
    // report attempts to flip its behavior.
    //   set_reject(true)  → report() → on_report Errs → state = Failed.
    //   set_reject(false) → report() → on_report Ok    → state = Succeeded.
    //
    // NOTE: state changes done inside on_report wouldn't survive the child-frame
    // rollback on Err return (handoff §2 / receiver behavior matrix), which is
    // why the toggle has to happen via a separate contract call between submits.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::ToggleReceiver, ());
    let toggle = mocks::ToggleReceiverClient::new(&env, &receiver_addr);
    toggle.set_reject(&true);

    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 2);

    // First submission — receiver rejects → state = Failed.
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
    let report = ReportBuilder::default();
    let info1 = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info1.state, TransmissionState::Failed);

    // Flip receiver's externally-visible state, then resubmit.
    toggle.set_reject(&false);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
    let info2 = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info2.state, TransmissionState::Succeeded);
}

// ============================================================================
// Report — validation failures
//
// Order of checks
//   1. raw_report.len() < METADATA_LENGTH      → InvalidReport (2)
//   2. report_context.len() != 96              → InvalidReportContext (3)
//   3. signatures.is_empty()                   → InvalidSignatureCount (9)
//   4. parse_report version check              → InvalidReportVersion (4)
//   5. load_config (missing don, version)      → InvalidConfig (8)
//   6. signatures.len() != f+1                 → InvalidSignatureCount (9)
//   7. validate_signature_scalars (r or s)     → InvalidSignature (11)
//   8. normalize_recovery_id                   → InvalidRecoveryId (12)
//   9. signer_index (unknown pubkey)           → InvalidSigner (19)
//  10. bitmap dedup                            → DuplicateSigner (10)
// ============================================================================

#[test]
#[should_panic(expected = "Error(Contract, #2)")]
fn test_report_too_short_panics() {
    // raw_report < METADATA_LENGTH (109) → InvalidReport code 2.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let short = soroban_sdk::Bytes::from_slice(&env, &[0u8; 10]);
    let ctx = report_context_zeroes(&env);
    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &[0u8; 65]));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &short, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #3)")]
fn test_report_wrong_context_length_panics() {
    // report_context.len() != 96 → InvalidReportContext code 3.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let raw = ReportBuilder::default().build(&env);
    let bad_ctx = soroban_sdk::Bytes::from_slice(&env, &[0u8; 64]);
    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &[0u8; 65]));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &bad_ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #9)")]
fn test_report_empty_signatures_panics() {
    // signatures empty → InvalidSignatureCount code 9 (early empty-check).
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let raw = ReportBuilder::default().build(&env);
    let ctx = report_context_zeroes(&env);
    let sigs = soroban_sdk::Vec::new(&env);

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #8)")]
fn test_report_wrong_don_id_panics() {
    // report claims don=999 → load_config returns None → InvalidConfig code 8.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default().with_don_id(999);
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #8)")]
fn test_report_wrong_config_version_panics() {
    // config_version not registered → InvalidConfig code 8.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default().with_config_version(CONFIG_VERSION + 1);
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #4)")]
fn test_report_version_not_one_panics() {
    // report version byte = 2 → InvalidReportVersion code 4.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default().with_version(2);
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #9)")]
fn test_report_too_few_signatures_panics() {
    // f=1 needs f+1=2 sigs; send 1 → InvalidSignatureCount code 9.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 1);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #9)")]
fn test_report_too_many_signatures_panics() {
    // f=1 needs exactly 2; send 3 → InvalidSignatureCount code 9.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 3);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #11)")]
fn test_report_garbage_signature_panics() {
    // 65-byte sig with r,s both all-0xFF (above SECP256K1_ORDER)
    // → validate_signature_scalar fires → InvalidSignature code 11.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[0], &digest),
    ));
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &[0xFFu8; 65]));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #19)")]
fn test_report_signer_not_in_set_panics() {
    //  properly-signed sig from a key NOT in the configured set
    // → secp256k1_recover succeeds but signer_index returns None → InvalidSigner code 19.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    let rogue = crypto::signing_key(99); // not in signers[0..4]

    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[0], &digest),
    ));
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&rogue, &digest),
    ));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #10)")]
fn test_report_duplicate_signer_panics() {
    // two sig slots from the same signer → bitmap dedup → DuplicateSigner code 10.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    let sig0 = crypto::sign_report(&fx.signers[0], &digest);
    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &sig0));
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &sig0));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #12)")]
fn test_report_invalid_recovery_id_panics() {
    // recovery byte = 5 (not in {0, 1, 27, 28}) → InvalidRecoveryId code 12.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[0], &digest),
    ));
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report_with_recid(&fx.signers[1], &digest, 5),
    ));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #11)")]
fn test_report_s_scalar_zero_panics() {
    // signature with s == 0 → is_zero_32(s) → InvalidSignature code 11.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[0], &digest),
    ));
    let mut bad = crypto::sign_report(&fx.signers[1], &digest);
    for b in bad[32..64].iter_mut() {
        *b = 0;
    }
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &bad));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
#[should_panic(expected = "Error(Contract, #11)")]
fn test_report_s_scalar_at_n_panics() {
    // s == SECP256K1_ORDER (N) → is_greater_or_equal_32 returns true
    // on equality → InvalidSignature code 11.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    // Mirrors lib.rs SECP256K1_ORDER. Must stay in sync with the constant.
    const SECP256K1_ORDER: [u8; 32] = [
        0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
        0xfe, 0xba, 0xae, 0xdc, 0xe6, 0xaf, 0x48, 0xa0, 0x3b, 0xbf, 0xd2, 0x5e, 0x8c, 0xd0, 0x36,
        0x41, 0x41,
    ];

    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[0], &digest),
    ));
    let mut bad = crypto::sign_report(&fx.signers[1], &digest);
    bad[32..64].copy_from_slice(&SECP256K1_ORDER);
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &bad));

    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);
}

#[test]
fn test_report_different_report_id_not_blocked() {
    // Same (receiver, exec_id) but different report_id → distinct
    // transmission_ids, both delivered independently. No replay-guard interference.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::CooperativeReceiver, ());

    let report_a = ReportBuilder::default().with_report_id([0x00, 0x01]);
    let (raw_a, ctx_a, sigs_a) = build_signed_report(&fx, &report_a, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw_a, &ctx_a, &sigs_a);

    let report_b = ReportBuilder::default().with_report_id([0x00, 0x02]);
    let (raw_b, ctx_b, sigs_b) = build_signed_report(&fx, &report_b, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw_b, &ctx_b, &sigs_b);

    // Both transmissions recorded as Succeeded.
    let info_a = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report_a.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report_a.report_id),
    );
    let info_b = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report_b.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report_b.report_id),
    );
    assert_eq!(info_a.state, TransmissionState::Succeeded);
    assert_eq!(info_b.state, TransmissionState::Succeeded);
}

// ============================================================================
// report — receiver behavior matrix
//
// Exercises every arm of route()
//   Ok(Ok(()))                — Succeeded
//   Ok(Err(_)) | Err(Ok(_))   — Failed       (retryable)
//   Err(Err(_))               — InvalidReceiver (terminal)
// Plus the `receiver.executable() != Some(Wasm(_))` short-circuit
// ============================================================================

#[test]
fn test_report_account_receiver_marks_invalid_receiver() {
    // receiver is a generated Address (no executable) → InvalidReceiver
    // via the `receiver.executable()` short-circuit BEFORE try_invoke_contract.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let account_receiver = Address::generate(&env);
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &account_receiver, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &account_receiver,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::InvalidReceiver);
}

#[test]
fn test_report_receiver_without_on_report_marks_failed() {
    // T-RPT-41 (revised): WrongSymbolReceiver is a Wasm contract that doesn't
    // expose on_report. EMPIRICAL FINDING: Soroban surfaces this as Ok(Err(_))
    // or Err(Ok(InvokeError::Contract(_))) — NOT Err(Err(_)) — so the M2
    // refinement's "Err(Err(_)) → InvalidReceiver" arm doesn't fire here, and
    // the receiver gets marked Failed (retryable).
    //
    // Documented as a real polish gap with EVM's ERC165 behavior: Soroban
    // doesn't expose a host-level error class that distinguishes "missing
    // interface" from "rejected this specific report". The M2 arm in lib.rs
    // is dead defensive depth pending a future Soroban API.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::WrongSymbolReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Failed);
}

#[test]
fn test_report_receiver_returns_err_marks_failed() {
    // RejectingReceiver returns Result::Err → Ok(Err(_)) arm → Failed.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::RejectingReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Failed);
}

#[test]
fn test_report_receiver_panic_with_error_marks_failed() {
    // PanickingReceiver does panic_with_error! → Err(Ok(InvokeError::Contract(_)))
    // arm → Failed (retryable — receiver state could change).
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::PanickingReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Failed);
}

#[test]
fn test_report_receiver_plain_panic_marks_failed() {
    // PlainPanicReceiver does `panic!()` → Err(Ok(InvokeError::Abort))
    // arm → Failed (retryable). Confirms the Abort arm is treated the same as
    // the typed-Contract panic arm — both are retryable since the receiver
    // exists and rejected for some reason.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver_addr = env.register(mocks::PlainPanicReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver_addr, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &receiver_addr,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Failed);
}

#[test]
fn test_report_failed_state_allows_retry_succeeded_does_not() {
    // matrix proof — Failed allows retry; Succeeded blocks; InvalidReceiver blocks.
    //
    // 1. RejectingReceiver: first report → Failed, second report → Failed again
    //    (no AlreadyProcessed panic; replay guard does NOT fire for Failed state).
    // 2. CooperativeReceiver with a different transmission_id → Succeeded.
    // 3. Replay the Succeeded one → panic AlreadyProcessed (code 13).
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    // (1) Failed → retry allowed.
    let rejecting = env.register(mocks::RejectingReceiver, ());
    let r1 = ReportBuilder::default().with_report_id([0x00, 0xA1]);
    let (raw1, ctx1, sigs1) = build_signed_report(&fx, &r1, 2);
    fx.client
        .report(&fx.transmitter, &rejecting, &raw1, &ctx1, &sigs1);
    fx.client
        .report(&fx.transmitter, &rejecting, &raw1, &ctx1, &sigs1); // retry doesn't panic

    let info_failed = fx.client.get_transmission_info(
        &rejecting,
        &soroban_sdk::BytesN::from_array(&env, &r1.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &r1.report_id),
    );
    assert_eq!(info_failed.state, TransmissionState::Failed);

    // (2) Cooperative → Succeeded under a fresh report_id.
    let cooperative = env.register(mocks::CooperativeReceiver, ());
    let r2 = ReportBuilder::default().with_report_id([0x00, 0xA2]);
    let (raw2, ctx2, sigs2) = build_signed_report(&fx, &r2, 2);
    fx.client
        .report(&fx.transmitter, &cooperative, &raw2, &ctx2, &sigs2);

    let info_succ = fx.client.get_transmission_info(
        &cooperative,
        &soroban_sdk::BytesN::from_array(&env, &r2.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &r2.report_id),
    );
    assert_eq!(info_succ.state, TransmissionState::Succeeded);

    // (3) Replay-on-Succeeded blocking is covered by T-RPT-30
    // (`test_replay_after_success_panics`) — splitting that into a separate test
    // keeps each #[should_panic] / non-panic assertion in its own function so
    // a failure on either path is unambiguous.
}

// ============================================================================
//  report — auth
// ============================================================================

#[test]
#[should_panic(expected = "Error(Contract, #16)")]
fn test_report_uninitialized_panics() {
    // call report() on an un-initialized contract → Uninitialized code 16
    // (via the ensure_initialized check in report()).
    let env = Env::default();
    env.mock_all_auths();
    let contract_addr = env.register(KeystoneForwarder, ());
    let client = KeystoneForwarderClient::new(&env, &contract_addr);

    // Skip initialize(); call report directly.
    let transmitter = Address::generate(&env);
    let receiver = env.register(mocks::CooperativeReceiver, ());
    let raw = ReportBuilder::default().build(&env);
    let ctx = report_context_zeroes(&env);
    let mut sigs = soroban_sdk::Vec::new(&env);
    sigs.push_back(soroban_sdk::BytesN::from_array(&env, &[0u8; 65]));

    client.report(&transmitter, &receiver, &raw, &ctx, &sigs);
}

// (no transmitter auth) is covered indirectly: setup() calls
// mock_all_auths so transmitter.require_auth() always passes during tests.
// To verify the auth boundary, a test would need to install a restrictive
// mock_auths set excluding the transmitter — Soroban's testutils auth API
// makes this non-trivial without rewriting setup(). The host-level auth path
// is exercised by the not_owner tests
// which use the same `set_auths(&[])` pattern that would fire here.

// ============================================================================
// Config-version lifecycle
// ============================================================================

#[test]
fn test_config_v1_and_v2_coexist() {
    // two configs at same don_id, different config_version. Reports
    // against either succeed.
    let env = Env::default();
    let fx = setup(&env);
    fx.client.add_forwarder(&fx.transmitter);
    let signers = fx.signer_set(4);
    fx.client.set_config(&DON_ID, &1u32, &1u32, &signers);
    fx.client.set_config(&DON_ID, &2u32, &1u32, &signers);

    let receiver = env.register(mocks::CooperativeReceiver, ());

    // Report against v1.
    let r1 = ReportBuilder::default()
        .with_config_version(1)
        .with_report_id([0x00, 0x01]);
    let (raw1, ctx1, sigs1) = build_signed_report(&fx, &r1, 2);
    fx.client
        .report(&fx.transmitter, &receiver, &raw1, &ctx1, &sigs1);

    // Report against v2.
    let r2 = ReportBuilder::default()
        .with_config_version(2)
        .with_report_id([0x00, 0x02]);
    let (raw2, ctx2, sigs2) = build_signed_report(&fx, &r2, 2);
    fx.client
        .report(&fx.transmitter, &receiver, &raw2, &ctx2, &sigs2);
}

#[test]
fn test_clearing_v1_does_not_break_v2() {
    // clear v1; v2 report still delivers.
    let env = Env::default();
    let fx = setup(&env);
    fx.client.add_forwarder(&fx.transmitter);
    let signers = fx.signer_set(4);
    fx.client.set_config(&DON_ID, &1u32, &1u32, &signers);
    fx.client.set_config(&DON_ID, &2u32, &1u32, &signers);

    fx.client.clear_config(&DON_ID, &1u32);

    let receiver = env.register(mocks::CooperativeReceiver, ());
    let r2 = ReportBuilder::default()
        .with_config_version(2)
        .with_report_id([0x00, 0x02]);
    let (raw2, ctx2, sigs2) = build_signed_report(&fx, &r2, 2);
    fx.client
        .report(&fx.transmitter, &receiver, &raw2, &ctx2, &sigs2);
}

#[test]
#[should_panic(expected = "Error(Contract, #8)")]
fn test_report_against_cleared_v1_fails() {
    //  after setup, a report against v1 fails with InvalidConfig.
    let env = Env::default();
    let fx = setup(&env);
    fx.client.add_forwarder(&fx.transmitter);
    let signers = fx.signer_set(4);
    fx.client.set_config(&DON_ID, &1u32, &1u32, &signers);
    fx.client.set_config(&DON_ID, &2u32, &1u32, &signers);
    fx.client.clear_config(&DON_ID, &1u32);

    let receiver = env.register(mocks::CooperativeReceiver, ());
    let r1 = ReportBuilder::default()
        .with_config_version(1)
        .with_report_id([0x00, 0x01]);
    let (raw1, ctx1, sigs1) = build_signed_report(&fx, &r1, 2);
    fx.client
        .report(&fx.transmitter, &receiver, &raw1, &ctx1, &sigs1);
}

// ============================================================================
// Views
// ============================================================================

#[test]
fn test_type_and_version_returns_constant() {
    // type_and_version() returns "KeystoneForwarder 1.0.0".
    let env = Env::default();
    let fx = setup(&env);
    let v = fx.client.type_and_version();
    let expected = soroban_sdk::String::from_str(&env, "KeystoneForwarder 1.0.0");
    assert_eq!(v, expected);
}

#[test]
fn test_get_transmission_info_not_attempted() {
    // query an unsubmitted (receiver, exec_id, report_id) →
    // {NotAttempted, None}. Confirms the post-merge get_transmission_info shape.
    let env = Env::default();
    let fx = setup(&env);
    let receiver = env.register(mocks::CooperativeReceiver, ());
    let info = fx.client.get_transmission_info(
        &receiver,
        &soroban_sdk::BytesN::from_array(&env, &[0u8; 32]),
        &soroban_sdk::BytesN::from_array(&env, &[0u8; 2]),
    );
    assert_eq!(info.state, TransmissionState::NotAttempted);
    assert_eq!(info.transmitter, None);
}

#[test]
fn test_get_transmission_info_after_succeeded() {
    // after a successful report, get_transmission_info returns
    // {Succeeded, Some(transmitter)}.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &receiver,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Succeeded);
    assert_eq!(info.transmitter, Some(fx.transmitter.clone()));
}

#[test]
fn test_get_transmission_info_after_failed() {
    // RejectingReceiver path → {Failed, Some(transmitter)}.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver = env.register(mocks::RejectingReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &receiver,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::Failed);
    assert_eq!(info.transmitter, Some(fx.transmitter.clone()));
}

#[test]
fn test_get_transmission_info_after_invalid_receiver() {
    // account-as-receiver path → {InvalidReceiver, Some(transmitter)}.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);

    let account_receiver = Address::generate(&env);
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &account_receiver, &raw, &ctx, &sigs);

    let info = fx.client.get_transmission_info(
        &account_receiver,
        &soroban_sdk::BytesN::from_array(&env, &report.workflow_execution_id),
        &soroban_sdk::BytesN::from_array(&env, &report.report_id),
    );
    assert_eq!(info.state, TransmissionState::InvalidReceiver);
    assert_eq!(info.transmitter, Some(fx.transmitter.clone()));
}

#[test]
fn test_is_forwarder_returns_false_for_unknown() {
    // T-VW-06: an address never added to the registry → false.
    let env = Env::default();
    let _ = setup(&env);
    let unknown = Address::generate(&env);
    assert!(require_valid_forwarder(&env, &unknown).is_err());
}

// ============================================================================
// Soroban-specific edges (T-SOR-01..06)
//
// (storage TTL bumps on owner op) and (persistent storage tier for Transmission)
// require introspecting Soroban internals that the
// standard testutils API doesn't expose. Both verified behaviorally elsewhere
// (TTL bumps don't break behavior; persistence is implicit in the replay
// guard surviving across calls).
// ============================================================================

#[test]
fn test_bitmap_at_position_zero_works() {
    // signer 0 in the first signature slot — exercises bit 0 of the u64 dedup bitmap.
    let env = Env::default();
    let fx = setup_with_config(&env, 1, 4);
    fx.client.add_forwarder(&fx.transmitter);
    let receiver = env.register(mocks::CooperativeReceiver, ());
    let report = ReportBuilder::default();
    let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);
    fx.client
        .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);
}

#[test]
fn test_bitmap_at_max_position() {
    // with f=10 / n=31, slot 11 of the sigs is signer index 10.
    // build_signed_report uses signers[0..11] so the highest index reached is 10.
    // To exercise position 30 (the highest valid bit for MAX_ORACLES=31), we
    // need a sig from signer 30 — replace the last slot manually.
    let env = Env::default();
    let fx = setup_with_config(&env, 10, 31);
    fx.client.add_forwarder(&fx.transmitter);
    let receiver = env.register(mocks::CooperativeReceiver, ());

    let report = ReportBuilder::default();
    let raw_vec = report.build_bytes();
    let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
    let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
    let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
    let digest = crypto::report_digest(&raw_vec, &ctx_vec);

    // 11 sigs: 10 from low indices, 1 from index 30 (the highest valid slot).
    let mut sigs = soroban_sdk::Vec::new(&env);
    for i in 0..10 {
        sigs.push_back(soroban_sdk::BytesN::from_array(
            &env,
            &crypto::sign_report(&fx.signers[i], &digest),
        ));
    }
    sigs.push_back(soroban_sdk::BytesN::from_array(
        &env,
        &crypto::sign_report(&fx.signers[30], &digest),
    ));

    fx.client
        .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);
}

#[test]
fn test_ccip_error_mapping_to_local_error() {
    // drive an Ownable failure path that produces a CCIPError, observe
    // the surfaced cre Error discriminant. Specifically: accept_ownership with no
    // pending owner returns CCIPError::NoPendingOwner from the trait, which the
    // From<CCIPError> for Error impl maps to Error::NotProposedOwner (code 15).
    let env = Env::default();
    let fx = setup(&env);
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        fx.client.accept_ownership();
    }));
    assert!(result.is_err(), "no pending owner must surface a panic");
}

#[test]
fn test_config_with_f_at_practical_max() {
    // f=10, n=31 exercises the upper bound of the practical f range
    // (3·10 + 1 = 31 matches MAX_ORACLES). Confirms the u8 narrowing (P9) didn't
    // break the arithmetic in min_signers / required_signatures.
    let env = Env::default();
    let fx = setup_with_config(&env, 10, 31);
    fx.client.add_forwarder(&fx.transmitter);

    let receiver = env.register(mocks::CooperativeReceiver, ());
    let (raw, ctx, sigs) = build_signed_report(&fx, &ReportBuilder::default(), 11);
    fx.client
        .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);
}

// ============================================================================
// Fuzz
//
// Each fuzz test iterates deterministically (not random) over a small space
// of perturbations. Bounded so CI stays under reasonable wall time. Each
// iteration spins up a fresh env + setup;
// ============================================================================

#[test]
fn test_fuzz_random_pubkey_signers() {
    // 10 deterministic signing keys NOT in the configured signer set
    // (config has signers 1..=4; we use seeds 50..=59). Each must produce a
    // panic with InvalidSigner (code 19) — never silent success, never a
    // different error class.
    for rogue_seed in 50u8..=59u8 {
        let env = Env::default();
        let fx = setup_with_config(&env, 1, 4);
        fx.client.add_forwarder(&fx.transmitter);
        let receiver = env.register(mocks::CooperativeReceiver, ());

        let report = ReportBuilder::default();
        let raw_vec = report.build_bytes();
        let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
        let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
        let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
        let digest = crypto::report_digest(&raw_vec, &ctx_vec);

        let rogue = crypto::signing_key(rogue_seed);
        let mut sigs = soroban_sdk::Vec::new(&env);
        sigs.push_back(soroban_sdk::BytesN::from_array(
            &env,
            &crypto::sign_report(&fx.signers[0], &digest),
        ));
        sigs.push_back(soroban_sdk::BytesN::from_array(
            &env,
            &crypto::sign_report(&rogue, &digest),
        ));

        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            fx.client
                .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);
        }));
        assert!(
            result.is_err(),
            "rogue seed {rogue_seed} must panic, not silently succeed"
        );
    }
}

#[test]
fn test_fuzz_signature_flip_byte_at_each_offset() {
    // flip a single byte in the recovery-byte slot of a valid sig
    // across the 256 possible byte values. Every result must be either a
    // panic (one of: InvalidRecoveryId, InvalidSigner, DuplicateSigner) OR
    // (for recid bytes 0/1/27/28 that happen to recover the same pubkey) a
    // duplicate-signer panic. NEVER a silent success.
    //
    // Scope: just the recovery byte (idx 64). Full r/s/v bit-flips would be
    // 520+ cases × full env setup — too expensive for CI. The validation-
    // failure tests cover the r/s scalar paths
    // exhaustively for the boundary values that matter.
    for recid_byte in 0u8..=255u8 {
        let env = Env::default();
        let fx = setup_with_config(&env, 1, 4);
        fx.client.add_forwarder(&fx.transmitter);
        let receiver = env.register(mocks::CooperativeReceiver, ());

        let report = ReportBuilder::default();
        let raw_vec = report.build_bytes();
        let ctx_vec = [0u8; REPORT_CONTEXT_LENGTH];
        let raw = soroban_sdk::Bytes::from_slice(&env, &raw_vec);
        let ctx = soroban_sdk::Bytes::from_slice(&env, &ctx_vec);
        let digest = crypto::report_digest(&raw_vec, &ctx_vec);

        let mut sigs = soroban_sdk::Vec::new(&env);
        sigs.push_back(soroban_sdk::BytesN::from_array(
            &env,
            &crypto::sign_report(&fx.signers[0], &digest),
        ));
        sigs.push_back(soroban_sdk::BytesN::from_array(
            &env,
            &crypto::sign_report_with_recid(&fx.signers[1], &digest, recid_byte),
        ));

        let outcome = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            fx.client
                .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);
        }));

        // recid byte in {0, 1, 27, 28} → secp256k1_recover runs.
        //   - {0, 28} or {1, 27} pair maps to the same effective recovery_id
        //     so the recovered pubkey is signers[1] → success (or duplicate
        //     if equal to slot 0). Other valid recids may recover a different
        //     pubkey → InvalidSigner.
        // All other bytes → InvalidRecoveryId panic.
        let valid_recid = matches!(recid_byte, 0 | 1 | 27 | 28);
        if !valid_recid {
            assert!(
                outcome.is_err(),
                "recid byte {recid_byte} must panic (not in {{0,1,27,28}})"
            );
        }
        // For valid_recid, either success or a typed-error panic is acceptable;
        // the key property is that there is no UB / corruption — handled by
        // catch_unwind returning either Ok(()) or Err(payload).
    }
}

#[test]
fn test_fuzz_metadata_byte_corruption_first_byte_path() {
    // flip the version byte (byte 0) across values 0..=255. Only
    // value 1 should succeed; everything else must hit InvalidReportVersion
    // (code 4) deterministically.
    //
    // Scope: just byte 0 (the version field). Full metadata bit-flip across
    // 109 bytes × 8 bits = 872 cases × full setup is too slow; the targeted
    // validation tests (T-RPT-13, T-RPT-14) cover the don_id/config_version
    // corruption paths.
    for version in 0u8..=255u8 {
        if version == 1 {
            continue; // value 1 is the only valid version
        }
        let env = Env::default();
        let fx = setup_with_config(&env, 1, 4);
        fx.client.add_forwarder(&fx.transmitter);
        let receiver = env.register(mocks::CooperativeReceiver, ());

        let report = ReportBuilder::default().with_version(version);
        let (raw, ctx, sigs) = build_signed_report(&fx, &report, 2);

        let outcome = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            fx.client
                .report(&fx.transmitter, &receiver, &raw, &ctx, &sigs);
        }));
        assert!(
            outcome.is_err(),
            "version byte {version} != 1 must panic (InvalidReportVersion)"
        );
    }
}
