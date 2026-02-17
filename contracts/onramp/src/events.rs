use soroban_sdk::{contractevent, Address, Bytes, BytesN, Vec};

use crate::{DestChainConfig, DynamicConfig, Receipt, StaticConfig};

/// Event data for CCIPMessageSent
#[contractevent(topics = ["onramp_1_7_CCIPMessageSent"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CCIPMessageSentEvent {
    /// Destination chain selector
    pub dest_chain_selector: u64,
    /// Sequence number for this message to the destination chain
    pub sequence_number: u64,
    /// Original sender address
    pub sender: Address,
    /// Unique message ID (hash of encoded message)
    pub message_id: BytesN<32>,
    /// Fee token used for payment
    pub fee_token: Address,
    /// Token amount before pool fees (0 if no tokens)
    pub token_amount_before_fees: i128,
    /// Full encoded message (MessageV1 format)
    pub encoded_message: Bytes,
    /// Receipts for all components
    pub receipts: Vec<Receipt>,
    /// Blobs from each verifier
    pub verifier_blobs: Vec<Bytes>,
}

#[contractevent(topics = ["onramp_1_7_ConfigSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ConfigSetEvent {
    pub static_config: StaticConfig,
    pub dynamic_config: DynamicConfig,
}

#[contractevent(topics = ["onramp_1_7_DestChainConfigSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DestChainConfigSetEvent {
    pub dest_chain_selector: u64,
    pub message_number: u64,
    pub config: DestChainConfig,
}

#[contractevent(topics = ["onramp_1_7_OwnershipTransferred"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OwnershipTransferredEvent {
    pub new_owner: Address,
}
