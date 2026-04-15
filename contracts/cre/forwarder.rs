#![no_std]

use core::convert::Infallible;

use soroban_sdk::{
    address_payload::AddressPayload,
    crypto::Hash,
    contract, contracterror, contractimpl, contracttype,
    panic_with_error, symbol_short,
    Address, Bytes, BytesN, ConversionError, Env, Executable, IntoVal, InvokeError,
    String, Symbol, Val, Vec,
};

const MAX_ORACLES: u32 = 31;
const METADATA_LENGTH: u32 = 109;
const FORWARDER_METADATA_LENGTH: u32 = 45;
const REPORT_CONTEXT_LENGTH: u32 = 96;
const SIGNATURE_LENGTH: usize = 65;
const RECEIVER_INTERFACE_VERSION: u32 = 1;

const EXECUTION_ID_OFFSET: u32 = 1;
const DON_ID_OFFSET: u32 = 37;
const CONFIG_VERSION_OFFSET: u32 = 41;
const REPORT_ID_OFFSET: u32 = 107;

type TryCall<T> = Result<Result<T, ConversionError>, Result<Infallible, InvokeError>>;

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
    CannotTransferToSelf = 16,
    DispatchSystemFailure = 17,
    Uninitialized = 18,
    UnauthorizedForwarder = 19,
}

#[contracttype]
#[derive(Copy, Clone, Eq, PartialEq)]
#[repr(u32)]
pub enum TransmissionState {
    NotAttempted = 0,
    Succeeded = 1,
    InvalidReceiver = 2,
    Failed = 3,
}

#[contracttype]
#[derive(Clone)]
pub struct Transmission {
    pub gas_limit: u64,
    pub state: TransmissionState,
    pub transmitter: Address,
}

#[contracttype]
#[derive(Clone)]
pub struct TransmissionInfo {
    pub gas_limit: u64,
    pub invalid_receiver: bool,
    pub state: u32,
    pub success: bool,
    pub transmission_id: BytesN<32>,
    pub transmitter: Option<Address>,
}

#[contracttype]
#[derive(Clone)]
pub struct Config {
    pub f: u32,
    pub signers: Vec<BytesN<65>>,
}

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Owner,
    PendingOwner,
    Forwarder(Address),
    Config(u64),
    Transmission(BytesN<32>),
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
impl KeystoneForwarder {
    pub fn __constructor(env: Env, owner: Address) {
        if env.storage().instance().has(&DataKey::Owner) {
            panic_with_error!(&env, Error::AlreadyInitialized);
        }
        owner.require_auth();
        env.storage().instance().set(&DataKey::Owner, &owner);
        env.storage()
            .persistent()
            .set(&DataKey::Forwarder(env.current_contract_address()), &true);
    }

    pub fn type_and_version(env: Env) -> String {
        String::from_str(&env, "KeystoneForwarder 1.0.0")
    }

    pub fn get_owner(env: Env) -> Address {
        load_owner(&env)
    }

    pub fn add_forwarder(env: Env, owner: Address, forwarder: Address) {
        assert_owner(&env, &owner);
        env.storage()
            .persistent()
            .set(&DataKey::Forwarder(forwarder.clone()), &true);
        env.events()
            .publish((Symbol::new(&env, "forwarder_add"),), forwarder);
    }

    pub fn remove_forwarder(env: Env, owner: Address, forwarder: Address) {
        assert_owner(&env, &owner);
        env.storage()
            .persistent()
            .remove(&DataKey::Forwarder(forwarder.clone()));
        env.events()
            .publish((Symbol::new(&env, "forwarder_rm"),), forwarder);
    }

    pub fn is_forwarder(env: Env, forwarder: Address) -> bool {
        is_forwarder(&env, &forwarder)
    }

    pub fn transfer_ownership(env: Env, to: Address) {
        let owner = load_owner(&env);
        owner.require_auth();
        if owner == to {
            panic_with_error!(&env, Error::CannotTransferToSelf);
        }
        env.storage().instance().set(&DataKey::PendingOwner, &to);
        env.events()
            .publish((Symbol::new(&env, "ownership_transfer"),), (owner, to));
    }

    pub fn accept_ownership(env: Env) {
        let pending = load_pending_owner(&env)
            .unwrap_or_else(|| panic_with_error!(&env, Error::NotProposedOwner));
        pending.require_auth();

        let old_owner = load_owner(&env);
        env.storage().instance().set(&DataKey::Owner, &pending);
        env.storage().instance().remove(&DataKey::PendingOwner);

        env.events()
            .publish((Symbol::new(&env, "ownership_accept"),), (old_owner, pending));
    }

    pub fn set_config(
        env: Env,
        owner: Address,
        don_id: u32,
        config_version: u32,
        f: u32,
        signers: Vec<BytesN<65>>,
    ) {
        assert_owner(&env, &owner);

        if f == 0 {
            panic_with_error!(&env, Error::FaultToleranceMustBePositive);
        }
        if signers.len() > MAX_ORACLES {
            panic_with_error!(&env, Error::ExcessSigners);
        }
        if signers.len() < min_signers(f) {
            panic_with_error!(&env, Error::InsufficientSigners);
        }
        ensure_unique_pubkeys(&env, &signers);

        let cfg = Config { f, signers };
        env.storage()
            .persistent()
            .set(&DataKey::Config(config_id(don_id, config_version)), &cfg);

        env.events().publish(
            (Symbol::new(&env, "config_set"), don_id, config_version),
            cfg,
        );
    }

    pub fn clear_config(env: Env, owner: Address, don_id: u32, config_version: u32) {
        assert_owner(&env, &owner);
        env.storage()
            .persistent()
            .remove(&DataKey::Config(config_id(don_id, config_version)));

        env.events().publish(
            (Symbol::new(&env, "config_set"), don_id, config_version),
            Config {
                f: 0,
                signers: Vec::new(&env),
            },
        );
    }

    pub fn report(
        env: Env,
        transmitter: Address,
        receiver: Address,
        gas_limit: u64,
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

        let parsed = parse_report(&env, &raw_report);
        let transmission_id = get_transmission_id_impl(
            &env,
            &receiver,
            &parsed.workflow_execution_id,
            &parsed.report_id,
        );

        if env
            .storage()
            .persistent()
            .has(&DataKey::Transmission(transmission_id.clone()))
        {
            panic_with_error!(&env, Error::AlreadyProcessed);
        }

        let cfg = load_config(&env, parsed.config_id);
        if signatures.len() != required_signatures(&env, cfg.f) {
            panic_with_error!(&env, Error::InvalidSignatureCount);
        }

        transmitter.require_auth();
        verify_signatures(&env, &cfg.signers, &raw_report, &report_context, &signatures);

        let self_addr = env.current_contract_address();
        let ok = KeystoneForwarderClient::new(&env, &self_addr).route(
            &self_addr,
            &transmission_id,
            &transmitter,
            &receiver,
            &gas_limit,
            &parsed.metadata,
            &parsed.payload,
        );

        env.events().publish(
            (
                Symbol::new(&env, "report_processed"),
                receiver,
                parsed.workflow_execution_id,
                parsed.report_id,
            ),
            ok,
        );
    }

    pub fn route(
        env: Env,
        forwarder: Address,
        transmission_id: BytesN<32>,
        transmitter: Address,
        receiver: Address,
        gas_limit: u64,
        metadata: Bytes,
        validated_report: Bytes,
    ) -> bool {
        assert_forwarder(&env, &forwarder);

        let key = DataKey::Transmission(transmission_id);
        if env.storage().persistent().has(&key) {
            panic_with_error!(&env, Error::AlreadyProcessed);
        }

        // Pre-store FAILED so a typed receiver failure is already terminal
        // without needing a post-call write.
        let mut tx = Transmission {
            gas_limit,
            state: TransmissionState::Failed,
            transmitter,
        };
        env.storage().persistent().set(&key, &tx);

        match dispatch(&env, &receiver, &metadata, &validated_report) {
            Ok(state) => {
                if state != TransmissionState::Failed {
                    tx.state = state;
                    env.storage().persistent().set(&key, &tx);
                }
                state == TransmissionState::Succeeded
            }
            Err(()) => panic_with_error!(&env, Error::DispatchSystemFailure),
        }
    }

    pub fn get_transmission_id(
        env: Env,
        receiver: Address,
        workflow_execution_id: BytesN<32>,
        report_id: BytesN<2>,
    ) -> BytesN<32> {
        get_transmission_id_impl(&env, &receiver, &workflow_execution_id, &report_id)
    }

    pub fn get_transmission_info(
        env: Env,
        receiver: Address,
        workflow_execution_id: BytesN<32>,
        report_id: BytesN<2>,
    ) -> TransmissionInfo {
        let transmission_id =
            get_transmission_id_impl(&env, &receiver, &workflow_execution_id, &report_id);

        match env
            .storage()
            .persistent()
            .get::<_, Transmission>(&DataKey::Transmission(transmission_id.clone()))
        {
            Some(t) => TransmissionInfo {
                gas_limit: t.gas_limit,
                invalid_receiver: t.state == TransmissionState::InvalidReceiver,
                state: t.state as u32,
                success: t.state == TransmissionState::Succeeded,
                transmission_id,
                transmitter: Some(t.transmitter),
            },
            None => TransmissionInfo {
                gas_limit: 0,
                invalid_receiver: false,
                state: TransmissionState::NotAttempted as u32,
                success: false,
                transmission_id,
                transmitter: None,
            },
        }
    }

    pub fn get_transmitter(
        env: Env,
        receiver: Address,
        workflow_execution_id: BytesN<32>,
        report_id: BytesN<2>,
    ) -> Option<Address> {
        let transmission_id =
            get_transmission_id_impl(&env, &receiver, &workflow_execution_id, &report_id);

        env.storage()
            .persistent()
            .get::<_, Transmission>(&DataKey::Transmission(transmission_id))
            .map(|t| t.transmitter)
    }
}

fn load_owner(env: &Env) -> Address {
    env.storage()
        .instance()
        .get::<_, Address>(&DataKey::Owner)
        .unwrap_or_else(|| panic_with_error!(env, Error::Uninitialized))
}

fn load_pending_owner(env: &Env) -> Option<Address> {
    env.storage().instance().get::<_, Address>(&DataKey::PendingOwner)
}

fn assert_owner(env: &Env, owner: &Address) {
    if load_owner(env) != owner.clone() {
        panic_with_error!(env, Error::NotOwner);
    }
    owner.require_auth();
}

fn is_forwarder(env: &Env, forwarder: &Address) -> bool {
    env.storage()
        .persistent()
        .has(&DataKey::Forwarder(forwarder.clone()))
}

fn assert_forwarder(env: &Env, forwarder: &Address) {
    if !is_forwarder(env, forwarder) {
        panic_with_error!(env, Error::UnauthorizedForwarder);
    }
    forwarder.require_auth();
}

fn config_id(don_id: u32, config_version: u32) -> u64 {
    ((don_id as u64) << 32) | config_version as u64
}

fn min_signers(f: u32) -> u32 {
    f.checked_mul(3)
        .and_then(|n| n.checked_add(1))
        .unwrap_or(u32::MAX)
}

fn required_signatures(env: &Env, f: u32) -> u32 {
    f.checked_add(1)
        .unwrap_or_else(|| panic_with_error!(env, Error::InvalidConfig))
}

fn load_config(env: &Env, id: u64) -> Config {
    let cfg = env
        .storage()
        .persistent()
        .get::<_, Config>(&DataKey::Config(id))
        .unwrap_or_else(|| panic_with_error!(env, Error::InvalidConfig));

    if cfg.f == 0 || cfg.signers.len() > MAX_ORACLES || cfg.signers.len() < min_signers(cfg.f) {
        panic_with_error!(env, Error::InvalidConfig);
    }
    cfg
}

fn ensure_unique_pubkeys(env: &Env, signers: &Vec<BytesN<65>>) {
    let mut i = 0;
    while i < signers.len() {
        let a = signers.get(i).unwrap();
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

fn report_digest(env: &Env, raw_report: &Bytes, report_context: &Bytes) -> Hash<32> {
    let raw_hash = env.crypto().keccak256(raw_report).to_bytes();
    let mut data = Bytes::from_slice(env, raw_hash.as_ref());
    data.append(report_context);
    env.crypto().keccak256(&data)
}

fn normalize_recovery_id(env: &Env, v: u8) -> u32 {
    match v {
        0 | 1 | 2 | 3 => v as u32,
        27 | 28 => (v - 27) as u32,
        _ => panic_with_error!(env, Error::InvalidRecoveryId),
    }
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
    let mut seen: u32 = 0;

    let mut i = 0;
    while i < signatures.len() {
        let sig = signatures.get(i).unwrap();

        let mut full = [0u8; SIGNATURE_LENGTH];
        sig.copy_into_slice(&mut full);

        let rec_id = normalize_recovery_id(env, full[64]);

        let mut rs = [0u8; 64];
        rs.copy_from_slice(&full[..64]);
        let sig64 = BytesN::<64>::from_array(env, &rs);

        let pubkey = env.crypto().secp256k1_recover(&digest, &sig64, rec_id);
        let idx = signer_index(signers, &pubkey)
            .unwrap_or_else(|| panic_with_error!(env, Error::InvalidSignature));

        let bit = 1u32 << idx;
        if seen & bit != 0 {
            panic_with_error!(env, Error::DuplicateSigner);
        }
        seen |= bit;
        i += 1;
    }
}

fn dispatch(
    env: &Env,
    receiver: &Address,
    metadata: &Bytes,
    validated_report: &Bytes,
) -> Result<TransmissionState, ()> {
    if !matches!(receiver.executable(), Some(Executable::Wasm(_))) {
        return Ok(TransmissionState::InvalidReceiver);
    }

    let probe: TryCall<u32> = env.try_invoke_contract(
        receiver,
        &Symbol::new(env, "receiver_interface_version"),
        Vec::<Val>::new(env),
    );

    match probe {
        Ok(Ok(RECEIVER_INTERFACE_VERSION)) => {}
        Ok(Ok(_)) | Ok(Err(_)) | Err(Err(InvokeError::Contract(_))) => {
            return Ok(TransmissionState::InvalidReceiver)
        }
        Err(Ok(never)) => match never {},
        Err(Err(InvokeError::Abort)) => return Err(()),
    }

    let args = (metadata.clone(), validated_report.clone()).into_val(env);
    let call: TryCall<()> = env.try_invoke_contract(receiver, &symbol_short!("on_report"), args);

    match call {
        Ok(Ok(())) => Ok(TransmissionState::Succeeded),
        Ok(Err(_)) => Ok(TransmissionState::InvalidReceiver),
        Err(Ok(never)) => match never {},
        Err(Err(InvokeError::Contract(_))) => Ok(TransmissionState::Failed),
        Err(Err(InvokeError::Abort)) => Err(()),
    }
}

fn tagged_address_bytes(env: &Env, address: &Address) -> Bytes {
    match address.to_payload() {
        Some(AddressPayload::AccountIdPublicKeyEd25519(pk)) => {
            let mut out = Bytes::from_slice(env, &[0]);
            out.append(&Bytes::from_slice(env, pk.as_ref()));
            out
        }
        Some(AddressPayload::ContractIdHash(hash)) => {
            let mut out = Bytes::from_slice(env, &[1]);
            out.append(&Bytes::from_slice(env, hash.as_ref()));
            out
        }
        None => panic_with_error!(env, Error::InvalidReceiver),
    }
}

fn get_transmission_id_impl(
    env: &Env,
    receiver: &Address,
    workflow_execution_id: &BytesN<32>,
    report_id: &BytesN<2>,
) -> BytesN<32> {
    let mut data = tagged_address_bytes(env, receiver);
    data.append(&Bytes::from_slice(env, workflow_execution_id.as_ref()));
    data.append(&Bytes::from_slice(env, report_id.as_ref()));
    env.crypto().sha256(&data).into()
}