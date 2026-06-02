#![cfg(test)]

use soroban_sdk::testutils::Address as _;
use soroban_sdk::{Address, Env};

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
        let mut combined = std::vec::Vec::with_capacity(32 + report_context.len());
        combined.extend_from_slice(&inner);
        combined.extend_from_slice(report_context);
        Keccak256::digest(&combined).into()
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
    pub signers: std::vec::Vec<k256::ecdsa::SigningKey>,
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
    let signers: std::vec::Vec<_> = (1u8..=31).map(crypto::signing_key).collect();

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
pub(crate) fn setup_with_config<'a>(env: &'a Env, f: u8, n_signers: usize) -> Fixture<'a> {
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
    pub payload: std::vec::Vec<u8>,
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
    pub fn with_payload(mut self, payload: std::vec::Vec<u8>) -> Self {
        self.payload = payload;
        self
    }

    /// Build the raw byte sequence in the on-chain layout.
    pub fn build_bytes(&self) -> std::vec::Vec<u8> {
        let mut out = std::vec::Vec::with_capacity(METADATA_LENGTH + self.payload.len());
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
    // is_forwarder should return true for the contract itself (auto-registered in initialize).
    assert!(fx.client.is_forwarder(&fx.contract_addr));
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
    let mut combined = std::vec::Vec::with_capacity(32 + ctx.len());
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
    assert_eq!(u32::from_be_bytes([don_be[0], don_be[1], don_be[2], don_be[3]]), DON_ID);
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
