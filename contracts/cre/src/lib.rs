#![no_std]

mod events;
mod types;

use common_authorization::Ownable;
use common_error::CCIPError;
use common_guard::initializable::Initializable;
use soroban_sdk::{
    address_payload::AddressPayload, contract, contracterror, contractimpl, crypto::Hash,
    panic_with_error, symbol_short, Address, Bytes, BytesN, Env, Executable, IntoVal, InvokeError,
    Symbol, Vec,
};

use events::{ConfigSetEvent, ForwarderAddedEvent, ForwarderRemovedEvent, ReportProcessedEvent};
use types::{Config, DataKey, Transmission, TransmissionState};

// ============================================================
// Storage Keys
// ============================================================

const INITIALIZED: Symbol = symbol_short!("INIT");
const OWNER: Symbol = symbol_short!("OWNER");
const PENDING_OWNER: Symbol = symbol_short!("PNDGOWNR");

// ============================================================
// Protocol Constants
// ============================================================

const MAX_ORACLES: u32 = 31;
const METADATA_LENGTH: u32 = 109;
const FORWARDER_METADATA_LENGTH: u32 = 45;
const REPORT_CONTEXT_LENGTH: u32 = 96;
const SIGNATURE_LENGTH: usize = 65;
const SECP256K1_ORDER: [u8; 32] = [
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe,
    0xba, 0xae, 0xdc, 0xe6, 0xaf, 0x48, 0xa0, 0x3b, 0xbf, 0xd2, 0x5e, 0x8c, 0xd0, 0x36, 0x41, 0x41,
];

// Storage TTL constants (ledger counts; 1 ledger ≈ 5 s on Mainnet).
// TODO adjust
const BUMP_FOR_60_DAYS: u32 = 1_036_800; // ~60 days
const BUMP_AFTER_30_DAYS: u32 = 518_400; // ~30 days

#[contracterror]
#[derive(Copy, Clone, Eq, PartialEq)]
#[repr(u32)]
pub enum Error {
    AlreadyInitialized = 1,
    InvalidReport = 2,
    InvalidReportContext = 3,
    InvalidReportVersion = 4,
    FaultToleranceMustBePositive = 5,
    ExcessSigners = 6,
    InsufficientSigners = 7,
    InvalidConfig = 8,
    InvalidSignatureCount = 9,
    DuplicateSigner = 10,
    InvalidSignature = 11,
    InvalidRecoveryId = 12,
    AlreadyProcessed = 13,
    NotOwner = 14,
    NotProposedOwner = 15,
    Uninitialized = 18,
    UnauthorizedForwarder = 19,
    InvalidReceiver = 20,
    InvalidSigner = 22,
    CannotRemoveSelf = 23,
}

impl From<CCIPError> for Error {
    fn from(e: CCIPError) -> Self {
        match e {
            CCIPError::AlreadyInitialized => Error::AlreadyInitialized,
            CCIPError::NotInitialized => Error::Uninitialized,
            CCIPError::NotOwner => Error::NotOwner,
            CCIPError::NoPendingOwner => Error::NotProposedOwner,
            _ => unreachable!("unexpected CCIPError variant from Ownable/Initializable"),
        }
    }
}

struct ParsedReport {
    workflow_execution_id: BytesN<32>,
    config_id: u64,
    report_id: BytesN<2>,
    metadata: Bytes,
    payload: Bytes,
}

#[contract]
pub struct KeystoneForwarder;

#[contractimpl]
impl Initializable for KeystoneForwarder {
    const INITIALIZED: Symbol = INITIALIZED;
}

#[contractimpl(contracttrait)]
impl Ownable for KeystoneForwarder {
    const OWNER: Symbol = OWNER;
    const PENDING_OWNER: Symbol = PENDING_OWNER;
}

#[contractimpl]
impl KeystoneForwarder {
    pub fn initialize(env: Env, owner: Address) -> Result<(), Error> {
        <Self as Initializable>::require_not_initialized(&env)?;
        <Self as Initializable>::init(&env)?;
        <Self as Ownable>::init_owner(&env, &owner)?;

        let self_addr = env.current_contract_address();
        let key = DataKey::Forwarder(self_addr.clone());
        env.storage().instance().set(&key, &true);

        ForwarderAddedEvent {
            forwarder: self_addr,
        }
        .publish(&env);

        // Mark as initialized
        env.storage().instance().set(&INITIALIZED, &true);
        env.storage()
            .instance()
            .extend_ttl(BUMP_AFTER_30_DAYS, BUMP_FOR_60_DAYS);
        Ok(())
    }

    pub fn add_forwarder(env: Env, forwarder: Address) -> Result<(), Error> {
        assert_owner(&env)?;

        let key = DataKey::Forwarder(forwarder.clone());
        env.storage().instance().set(&key, &true);

        ForwarderAddedEvent { forwarder }.publish(&env);
        Ok(())
    }

    pub fn remove_forwarder(env: Env, forwarder: Address) -> Result<(), Error> {
        assert_owner(&env)?;

        if forwarder == env.current_contract_address() {
            panic_with_error!(&env, Error::CannotRemoveSelf);
        }

        env.storage()
            .instance()
            .remove(&DataKey::Forwarder(forwarder.clone()));

        ForwarderRemovedEvent { forwarder }.publish(&env);
        Ok(())
    }

    pub fn set_config(
        env: Env,
        don_id: u32,
        config_version: u32,
        f: u32,
        signers: Vec<BytesN<65>>,
    ) -> Result<(), Error> {
        assert_owner(&env)?;

        if f == 0 {
            panic_with_error!(&env, Error::FaultToleranceMustBePositive);
        }
        if signers.len() > MAX_ORACLES {
            panic_with_error!(&env, Error::ExcessSigners);
        }
        if signers.len() < min_signers(&env, f) {
            panic_with_error!(&env, Error::InsufficientSigners);
        }
        ensure_unique_pubkeys(&env, &signers);

        let cfg = Config { f, signers };
        let key = DataKey::Config(config_id(don_id, config_version));
        env.storage().instance().set(&key, &cfg);

        ConfigSetEvent {
            don_id,
            config_version,
            f: cfg.f,
            signers: cfg.signers,
        }
        .publish(&env);
        Ok(())
    }

    pub fn clear_config(env: Env, don_id: u32, config_version: u32) -> Result<(), Error> {
        assert_owner(&env)?;

        let key = DataKey::Config(config_id(don_id, config_version));
        if !env.storage().instance().has(&key) {
            panic_with_error!(&env, Error::InvalidConfig);
        }

        env.storage().instance().remove(&key);

        ConfigSetEvent {
            don_id,
            config_version,
            f: 0,
            signers: Vec::new(&env),
        }
        .publish(&env);
        Ok(())
    }

    pub fn report(
        env: Env,
        transmitter: Address,
        receiver: Address,
        raw_report: Bytes,
        report_context: Bytes,
        signatures: Vec<BytesN<65>>,
    ) {
        if raw_report.len() < METADATA_LENGTH {
            panic_with_error!(&env, Error::InvalidReport);
        }
        if report_context.len() != REPORT_CONTEXT_LENGTH {
            panic_with_error!(&env, Error::InvalidReportContext);
        }
        if signatures.is_empty() {
            panic_with_error!(&env, Error::InvalidSignatureCount);
        }

        let parsed = parse_report(&env, &raw_report);
        let transmission_id = get_transmission_id(
            &env,
            &receiver,
            &parsed.workflow_execution_id,
            &parsed.report_id,
        );

        // Auth before any storage reads.
        transmitter.require_auth();
        ensure_initialized(&env);

        match env
            .storage()
            .persistent()
            .get::<_, Transmission>(&DataKey::Transmission(transmission_id.clone()))
        {
            Some(t)
                if t.state == TransmissionState::Succeeded
                    || t.state == TransmissionState::InvalidReceiver =>
            {
                panic_with_error!(&env, Error::AlreadyProcessed);
            }
            _ => {}
        }

        let cfg = load_config(&env, parsed.config_id);
        if signatures.len() != required_signatures(&env, cfg.f) {
            panic_with_error!(&env, Error::InvalidSignatureCount);
        }

        verify_signatures(
            &env,
            &cfg.signers,
            &raw_report,
            &report_context,
            &signatures,
        );
        env.storage()
            .instance()
            .extend_ttl(BUMP_AFTER_30_DAYS, BUMP_FOR_60_DAYS);

        let self_addr = env.current_contract_address();
        let ok = KeystoneForwarderClient::new(&env, &self_addr).route(
            &self_addr,
            &transmission_id,
            &transmitter,
            &receiver,
            &parsed.metadata,
            &parsed.payload,
        );

        ReportProcessedEvent {
            receiver,
            workflow_execution_id: parsed.workflow_execution_id,
            report_id: parsed.report_id,
            success: ok,
        }
        .publish(&env);
    }

    pub fn route(
        env: Env,
        forwarder: Address,
        transmission_id: BytesN<32>,
        transmitter: Address,
        receiver: Address,
        metadata: Bytes,
        validated_report: Bytes,
    ) -> bool {
        ensure_initialized(&env);
        assert_forwarder(&env, &forwarder);
        env.storage()
            .instance()
            .extend_ttl(BUMP_AFTER_30_DAYS, BUMP_FOR_60_DAYS);

        let key = DataKey::Transmission(transmission_id);

        match env.storage().persistent().get::<_, Transmission>(&key) {
            Some(t)
                if t.state == TransmissionState::Succeeded
                    || t.state == TransmissionState::InvalidReceiver =>
            {
                panic_with_error!(&env, Error::AlreadyProcessed);
            }
            _ => {}
        }

        let state = if !matches!(receiver.executable(), Some(Executable::Wasm(_))) {
            // Not a Wasm contract — terminal; AlreadyProcessed above blocks retries.
            TransmissionState::InvalidReceiver
        } else {
            let args = (metadata.clone(), validated_report.clone()).into_val(&env);
            let call = env.try_invoke_contract::<(), InvokeError>(
                &receiver,
                &symbol_short!("on_report"),
                args,
            );

            // try_invoke_contract -> Result<Result<R, E>, InvokeError>:
            //   Ok(Ok(())) = receiver returned cleanly, everything else is a retryable Failed
            match call {
                Ok(Ok(())) => TransmissionState::Succeeded,
                _ => TransmissionState::Failed,
            }
        };

        let tx = Transmission { state, transmitter };
        env.storage().persistent().set(&key, &tx);
        env.storage()
            .persistent()
            .extend_ttl(&key, BUMP_AFTER_30_DAYS, BUMP_FOR_60_DAYS);

        state == TransmissionState::Succeeded
    }

    // ========================================
    // Query Functions
    // ========================================

    pub fn is_forwarder(env: Env, forwarder: Address) -> bool {
        ensure_initialized(&env);
        is_forwarder_impl(&env, &forwarder)
    }

    pub fn get_transmission_info(
        env: Env,
        receiver: Address,
        workflow_execution_id: BytesN<32>,
        report_id: BytesN<2>,
    ) -> TransmissionInfo {
        ensure_initialized(&env);

        let transmission_id =
            get_transmission_id(&env, &receiver, &workflow_execution_id, &report_id);

        let key = DataKey::Transmission(transmission_id);

        match env
            .storage()
            .persistent()
            .get::<_, Transmission>(&key)
        {
            Some(t) => TransmissionInfo {
                state: t.state,
                transmitter: Some(t.transmitter),
            },
            None => TransmissionInfo {
                state: TransmissionState::NotAttempted,
                transmitter: None,
            },
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared guard helpers
// ─────────────────────────────────────────────────────────────────────────────

fn ensure_initialized(env: &Env) {
    <KeystoneForwarder as Initializable>::require_initialized(env)
        .unwrap_or_else(|_| panic_with_error!(env, Error::Uninitialized));
}

fn assert_owner(env: &Env) -> Result<(), Error> {
    ensure_initialized(env);
    <KeystoneForwarder as Ownable>::require_owner(env)?;
    env.storage()
        .instance()
        .extend_ttl(BUMP_AFTER_30_DAYS, BUMP_FOR_60_DAYS);
    Ok(())
}

// ─────────────────────────────────────────────────────────────────────────────
// Forwarder registry helpers
// ─────────────────────────────────────────────────────────────────────────────

fn is_forwarder_impl(env: &Env, forwarder: &Address) -> bool {
    let key = DataKey::Forwarder(forwarder.clone());
    env.storage().instance().has(&key)
}

fn assert_forwarder(env: &Env, forwarder: &Address) {
    forwarder.require_auth();
    if !is_forwarder_impl(env, forwarder) {
        panic_with_error!(env, Error::UnauthorizedForwarder);
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Config helpers
// ─────────────────────────────────────────────────────────────────────────────

fn config_id(don_id: u32, config_version: u32) -> u64 {
    ((don_id as u64) << 32) | config_version as u64
}

fn min_signers(env: &Env, f: u32) -> u32 {
    f.checked_mul(3)
        .and_then(|n| n.checked_add(1))
        .unwrap_or_else(|| panic_with_error!(env, Error::InvalidConfig))
}

fn required_signatures(env: &Env, f: u32) -> u32 {
    f.checked_add(1)
        .unwrap_or_else(|| panic_with_error!(env, Error::InvalidConfig))
}

fn load_config(env: &Env, id: u64) -> Config {
    let key = DataKey::Config(id);
    let cfg = env
        .storage()
        .instance()
        .get::<_, Config>(&key)
        .unwrap_or_else(|| panic_with_error!(env, Error::InvalidConfig));

    if cfg.f == 0 || cfg.signers.len() > MAX_ORACLES || cfg.signers.len() < min_signers(env, cfg.f)
    {
        panic_with_error!(env, Error::InvalidConfig);
    }

    cfg
}

fn ensure_unique_pubkeys(env: &Env, signers: &Vec<BytesN<65>>) {
    let zero = BytesN::<65>::from_array(env, &[0u8; 65]);

    let mut i = 0;
    while i < signers.len() {
        let a = signers.get(i).unwrap();
        if a == zero {
            panic_with_error!(env, Error::InvalidSigner);
        }

        let mut j = i + 1;
        while j < signers.len() {
            if a == signers.get(j).unwrap() {
                panic_with_error!(env, Error::DuplicateSigner);
            }
            j += 1;
        }

        i += 1;
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Report parsing
// ─────────────────────────────────────────────────────────────────────────────

const EXECUTION_ID_OFFSET: u32 = 1;
const DON_ID_OFFSET: u32 = 37;
const CONFIG_VERSION_OFFSET: u32 = 41;
const REPORT_ID_OFFSET: u32 = 107;

fn parse_report(env: &Env, raw_report: &Bytes) -> ParsedReport {
    if raw_report.get(0).unwrap() != 1 {
        panic_with_error!(env, Error::InvalidReportVersion);
    }

    let don_id = read_u32_be(raw_report, DON_ID_OFFSET);
    let config_version = read_u32_be(raw_report, CONFIG_VERSION_OFFSET);

    ParsedReport {
        workflow_execution_id: read_bytesn::<32>(env, raw_report, EXECUTION_ID_OFFSET),
        config_id: config_id(don_id, config_version),
        report_id: read_bytesn::<2>(env, raw_report, REPORT_ID_OFFSET),
        metadata: raw_report.slice(FORWARDER_METADATA_LENGTH..METADATA_LENGTH),
        payload: raw_report.slice(METADATA_LENGTH..raw_report.len()),
    }
}

fn read_bytesn<const N: usize>(env: &Env, bytes: &Bytes, start: u32) -> BytesN<N> {
    let mut buf = [0u8; N];
    bytes
        .slice(start..start + N as u32)
        .copy_into_slice(&mut buf);
    BytesN::from_array(env, &buf)
}

fn read_u32_be(bytes: &Bytes, start: u32) -> u32 {
    let mut buf = [0u8; 4];
    bytes.slice(start..start + 4).copy_into_slice(&mut buf);
    u32::from_be_bytes(buf)
}

// ─────────────────────────────────────────────────────────────────────────────
// Signature verification
// ─────────────────────────────────────────────────────────────────────────────

fn report_digest(env: &Env, raw_report: &Bytes, report_context: &Bytes) -> Hash<32> {
    let raw_hash = env.crypto().keccak256(raw_report);
    let mut data = Bytes::from(raw_hash);
    data.append(report_context);
    env.crypto().keccak256(&data)
}

fn normalize_recovery_id(env: &Env, v: u8) -> u32 {
    match v {
        0 | 1 => v as u32,
        27 | 28 => (v - 27) as u32,
        _ => panic_with_error!(env, Error::InvalidRecoveryId),
    }
}

fn is_zero_32(bytes: &[u8]) -> bool {
    let mut i = 0;
    while i < 32 {
        if bytes[i] != 0 {
            return false;
        }
        i += 1;
    }
    true
}

fn is_greater_or_equal_32(lhs: &[u8], rhs: &[u8; 32]) -> bool {
    let mut i = 0;
    while i < 32 {
        if lhs[i] > rhs[i] {
            return true;
        }
        if lhs[i] < rhs[i] {
            return false;
        }
        i += 1;
    }
    true
}

fn validate_signature_scalar(env: &Env, scalar: &[u8]) {
    if is_zero_32(scalar) || is_greater_or_equal_32(scalar, &SECP256K1_ORDER) {
        panic_with_error!(env, Error::InvalidSignature);
    }
}

fn validate_signature_scalars(env: &Env, signature: &[u8; SIGNATURE_LENGTH]) {
    validate_signature_scalar(env, &signature[..32]);
    validate_signature_scalar(env, &signature[32..64]);
}

fn signer_index(signers: &Vec<BytesN<65>>, signer: &BytesN<65>) -> Option<u32> {
    let mut i = 0;
    while i < signers.len() {
        if signers.get(i).unwrap() == signer.clone() {
            return Some(i);
        }
        i += 1;
    }
    None
}

fn verify_signatures(
    env: &Env,
    signers: &Vec<BytesN<65>>,
    raw_report: &Bytes,
    report_context: &Bytes,
    signatures: &Vec<BytesN<65>>,
) {
    let digest = report_digest(env, raw_report, report_context);
    let mut seen: u64 = 0;

    let mut i = 0;
    while i < signatures.len() {
        let sig = signatures.get(i).unwrap();

        let mut full = [0u8; SIGNATURE_LENGTH];
        sig.copy_into_slice(&mut full);

        validate_signature_scalars(env, &full);
        let rec_id = normalize_recovery_id(env, full[64]);

        let mut rs = [0u8; 64];
        rs.copy_from_slice(&full[..64]);
        let sig64 = BytesN::<64>::from_array(env, &rs);

        let pubkey = env.crypto().secp256k1_recover(&digest, &sig64, rec_id);
        let idx = signer_index(signers, &pubkey)
            .unwrap_or_else(|| panic_with_error!(env, Error::InvalidSigner));

        let bit = 1u64 << idx;
        if seen & bit != 0 {
            panic_with_error!(env, Error::DuplicateSigner);
        }
        seen |= bit;

        i += 1;
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Receiver dispatch
// ─────────────────────────────────────────────────────────────────────────────

fn receiver_contract_id_bytes(env: &Env, receiver: &Address) -> Bytes {
    match receiver.to_payload() {
        Some(AddressPayload::ContractIdHash(hash)) => Bytes::from(hash),
        _ => panic_with_error!(env, Error::InvalidReceiver),
    }
}

fn get_transmission_id(
    env: &Env,
    receiver: &Address,
    workflow_execution_id: &BytesN<32>,
    report_id: &BytesN<2>,
) -> BytesN<32> {
    let mut data = receiver_contract_id_bytes(env, receiver);
    let workflow_execution_id = workflow_execution_id.to_array();
    data.extend_from_array(&workflow_execution_id);
    let report_id = report_id.to_array();
    data.extend_from_array(&report_id);
    env.crypto().sha256(&data).into()
}
