#![no_std]

use common_error::CCIPError;
use soroban_sdk::{
    contracttype,
    xdr::{FromXdr, ToXdr},
    Address, Bytes, BytesN, Env, TryFromVal, Vec,
};

// ============================================================
// MessageIdCompute Trait
// ============================================================

/// Trait for types that can be serialized to Bytes.
/// Unlike `Into<Bytes>`, this takes `&Env` which is required by Soroban's `Bytes` type.
pub trait ToBytes {
    fn to_bytes(&self, env: &Env) -> Bytes;
}

/// Trait for computing CCIP message IDs.
/// Implementors of this trait provide the logic for generating deterministic
/// message identifiers based on message content.
pub trait MessageIdCompute: ToBytes {
    /// Computes the message ID for a CCIP message.
    fn compute_message_id(&self, env: &Env) -> BytesN<32> {
        let bytes = self.to_bytes(env);
        let hash = env.crypto().keccak256(&bytes);
        hash.into()
    }
}

// ============================================================
// TokenAmount
// ============================================================

/// Token amount struct for message token transfers.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TokenAmount {
    /// Token contract address
    pub token: Address,
    /// Amount to transfer - we use i128 to match SACs which does it to simplify arithmetic operations
    pub amount: i128,
}

impl TokenAmount {
    pub fn validate(&self) -> Result<(), CCIPError> {
        if self.amount < 0 {
            return Err(CCIPError::InvalidTokenAmount);
        }
        Ok(())
    }
}

impl ToBytes for TokenAmount {
    fn to_bytes(&self, env: &Env) -> Bytes {
        let mut bytes = Bytes::new(env);
        // Convert Address to its XDR byte representation
        bytes.append(&self.token.clone().to_xdr(env));
        // Convert i128 to big-endian bytes (16 bytes)
        bytes.append(&Bytes::from_array(env, &self.amount.to_be_bytes()));
        bytes
    }
}

// ============================================================
// GenericExtraArgsV3
// ============================================================

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GenericExtraArgsV3 {
    pub gas_limit: u32,
    pub block_confirmations: u32,
    pub ccvs: Vec<Address>,
    pub ccv_args: Vec<Bytes>,
    pub executor: Address,
    pub executor_args: Bytes,
    pub token_receiver: Bytes,
    pub token_args: Bytes,
}

impl GenericExtraArgsV3 {
    pub fn new(env: &Env, executor: Address) -> Self {
        Self {
            gas_limit: 0,
            block_confirmations: 0,
            ccvs: Vec::new(env),
            ccv_args: Vec::new(env),
            executor,
            executor_args: Bytes::new(env),
            token_receiver: Bytes::new(env),
            token_args: Bytes::new(env),
        }
    }
}

// ============================================================
// StellarToAnyMessage
// ============================================================

/// CCIP Message structure for sending cross-chain messages.
/// This represents the message from the sender's perspective.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StellarToAnyMessage {
    /// Receiver address on destination chain (raw bytes)
    pub receiver: Bytes,
    /// Arbitrary data payload to deliver
    pub data: Bytes,
    /// Tokens to transfer (max 1 in CCIP 1.7)
    pub token_amounts: Vec<TokenAmount>,
    /// Fee token address
    pub fee_token: Address,
    /// Extra arguments (encoded ExtraArgsV3 or legacy format)
    pub extra_args: Bytes,
}

impl StellarToAnyMessage {
    pub fn validate(&self) -> Result<(), CCIPError> {
        if self.token_amounts.len() > 1 {
            return Err(CCIPError::CanOnlySendOneTokenPerMessage);
        }

        for token_amount in self.token_amounts.iter() {
            token_amount.validate()?;
        }

        // TODO: add other validations
        // if self.receiver.len() != 32 {
        //     return Err(CCIPError::InvalidReceiverAddress);
        // }

        Ok(())
    }
}

impl ToBytes for StellarToAnyMessage {
    fn to_bytes(&self, env: &Env) -> Bytes {
        let mut bytes = Bytes::new(env);
        bytes.append(&self.receiver);
        bytes.append(&self.data);
        for token_amount in self.token_amounts.iter() {
            bytes.append(&token_amount.to_bytes(env));
        }
        bytes.append(&self.fee_token.clone().to_xdr(env));
        bytes.append(&self.extra_args);
        bytes
    }
}

impl MessageIdCompute for StellarToAnyMessage {}

// ============================================================
// AnyToStellarMessage
// ============================================================

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AnyToStellarMessage {
    // TODO: add fields
    pub placeholder: u64,
}

impl AnyToStellarMessage {
    pub fn validate(&self) -> Result<(), CCIPError> {
        // TODO: add validations
        Ok(())
    }
}

impl ToBytes for AnyToStellarMessage {
    fn to_bytes(&self, env: &Env) -> Bytes {
        unimplemented!()
    }
}

impl MessageIdCompute for AnyToStellarMessage {}

// ============================================================
// CCIP MessageV1 Canonical Format
// ============================================================
//
// These types implement the chain-agnostic CCIP v1.7 canonical message
// encoding, matching the protocol.Message.Encode() and
// protocol.TokenTransfer.Encode() formats used by offchain services.
//
// The OnRamp constructs a CcipMessageV1 from user message fields plus
// routing metadata, then calls compute_message_id() to derive the
// deterministic message ID that matches the offchain computation.

/// Current message format version for CCIP v1.7.
pub const MESSAGE_V1_VERSION: u8 = 1;

/// Canonical token transfer encoding for CCIP v1.7.
///
/// Matches protocol.TokenTransfer.Encode() byte layout:
///   version(1) | amount(32) | src_pool(1+N) | src_token(1+N) |
///   dest_token(1+N) | token_receiver(1+N) | extra_data(2+N)
#[derive(Clone)]
pub struct CcipTokenTransferV1 {
    pub version: u8,
    /// Transfer amount as 32 big-endian bytes (uint256).
    pub amount: BytesN<32>,
    pub source_pool_address: Bytes,
    pub source_token_address: Bytes,
    pub dest_token_address: Bytes,
    pub token_receiver: Bytes,
    pub extra_data: Bytes,
}

impl ToBytes for CcipTokenTransferV1 {
    fn to_bytes(&self, env: &Env) -> Bytes {
        let mut buf = Bytes::new(env);

        buf.append(&Bytes::from_array(env, &[self.version]));
        buf.append(&Bytes::from_slice(env, &self.amount.to_array()));

        buf.append(&Bytes::from_array(env, &[self.source_pool_address.len() as u8]));
        buf.append(&self.source_pool_address);

        buf.append(&Bytes::from_array(env, &[self.source_token_address.len() as u8]));
        buf.append(&self.source_token_address);

        buf.append(&Bytes::from_array(env, &[self.dest_token_address.len() as u8]));
        buf.append(&self.dest_token_address);

        buf.append(&Bytes::from_array(env, &[self.token_receiver.len() as u8]));
        buf.append(&self.token_receiver);

        buf.append(&Bytes::from_array(env, &(self.extra_data.len() as u16).to_be_bytes()));
        buf.append(&self.extra_data);

        buf
    }
}

/// CCIP MessageV1 canonical encoding, matching protocol.Message.Encode().
///
/// This is the chain-agnostic message format used for computing message IDs
/// and for the `encoded_message` field in CCIPMessageSent events.
///
/// Byte layout:
///   version(1) | src_chain(8) | dst_chain(8) | seq_num(8) |
///   exec_gas(4) | recv_gas(4) | finality(2) | ccv_exec_hash(32) |
///   onramp(1+N) | offramp(1+N) | sender(1+N) | receiver(1+N) |
///   dest_blob(2+N) | token_transfer(2+N) | data(2+N)
///
/// All multi-byte integers are big-endian. Address fields use a 1-byte
/// length prefix (max 255 bytes). Data/blob fields use a 2-byte length
/// prefix (max 65535 bytes).
#[derive(Clone)]
pub struct CcipMessageV1 {
    pub source_chain_selector: u64,
    pub dest_chain_selector: u64,
    pub sequence_number: u64,
    pub execution_gas_limit: u32,
    pub ccip_receive_gas_limit: u32,
    pub finality: u16,
    pub ccv_and_executor_hash: BytesN<32>,
    pub onramp_address: Bytes,
    pub offramp_address: Bytes,
    pub sender: Bytes,
    pub receiver: Bytes,
    pub dest_blob: Bytes,
    /// Pre-encoded token transfer bytes (from CcipTokenTransferV1::to_bytes).
    pub token_transfer: Bytes,
    pub data: Bytes,
}

impl ToBytes for CcipMessageV1 {
    fn to_bytes(&self, env: &Env) -> Bytes {
        let mut buf = Bytes::new(env);

        // Version (1 byte)
        buf.append(&Bytes::from_array(env, &[MESSAGE_V1_VERSION]));

        // Chain selectors and sequence number (8 bytes each, big-endian)
        buf.append(&Bytes::from_array(env, &self.source_chain_selector.to_be_bytes()));
        buf.append(&Bytes::from_array(env, &self.dest_chain_selector.to_be_bytes()));
        buf.append(&Bytes::from_array(env, &self.sequence_number.to_be_bytes()));

        // Gas limits (4 bytes each, big-endian)
        buf.append(&Bytes::from_array(env, &self.execution_gas_limit.to_be_bytes()));
        buf.append(&Bytes::from_array(env, &self.ccip_receive_gas_limit.to_be_bytes()));

        // Finality (2 bytes, big-endian)
        buf.append(&Bytes::from_array(env, &self.finality.to_be_bytes()));

        // CCV and executor hash (32 bytes)
        buf.append(&Bytes::from_slice(env, &self.ccv_and_executor_hash.to_array()));

        // On-ramp address (1 byte length + bytes)
        buf.append(&Bytes::from_array(env, &[self.onramp_address.len() as u8]));
        buf.append(&self.onramp_address);

        // Off-ramp address (1 byte length + bytes)
        buf.append(&Bytes::from_array(env, &[self.offramp_address.len() as u8]));
        buf.append(&self.offramp_address);

        // Sender (1 byte length + bytes)
        buf.append(&Bytes::from_array(env, &[self.sender.len() as u8]));
        buf.append(&self.sender);

        // Receiver (1 byte length + bytes)
        buf.append(&Bytes::from_array(env, &[self.receiver.len() as u8]));
        buf.append(&self.receiver);

        // Dest blob (2 bytes length, big-endian + bytes)
        buf.append(&Bytes::from_array(env, &(self.dest_blob.len() as u16).to_be_bytes()));
        buf.append(&self.dest_blob);

        // Token transfer (2 bytes length, big-endian + pre-encoded bytes)
        buf.append(&Bytes::from_array(env, &(self.token_transfer.len() as u16).to_be_bytes()));
        buf.append(&self.token_transfer);

        // Data (2 bytes length, big-endian + bytes)
        buf.append(&Bytes::from_array(env, &(self.data.len() as u16).to_be_bytes()));
        buf.append(&self.data);

        buf
    }
}

impl MessageIdCompute for CcipMessageV1 {}

impl CcipMessageV1 {
    /// Compute the CCV-and-executor hash from CCV addresses and executor address.
    /// Matches protocol.ComputeCCVAndExecutorHash() in Go.
    /// Format: keccak256(addressLength(1) || ccv1 || ccv2 || ... || executor)
    ///
    /// All addresses must have the same byte length (derived from the executor).
    pub fn compute_ccv_and_executor_hash(
        env: &Env,
        ccv_addresses: &Vec<Address>,
        executor: &Address,
    ) -> BytesN<32> {
        let executor_bytes = Self::address_raw_bytes(env, executor.clone());
        let addr_len = executor_bytes.len() as u8;

        let mut encoded = Bytes::new(env);
        encoded.append(&Bytes::from_array(env, &[addr_len]));

        for ccv in ccv_addresses.iter() {
            encoded.append(&Self::address_raw_bytes(env, ccv));
        }

        encoded.append(&executor_bytes);

        env.crypto().keccak256(&encoded).into()
    }

    /// Extract the raw 32-byte key from a Soroban Address.
    /// For contract addresses this is the contract ID; for account addresses
    /// it is the ed25519 public key. The raw bytes are the final 32 bytes of
    /// the ScVal XDR encoding, which holds the key for both address types.
    fn address_raw_bytes(env: &Env, addr: Address) -> Bytes {
        let xdr = addr.to_xdr(env);
        let len = xdr.len();
        xdr.slice((len - 32)..len)
    }
}
