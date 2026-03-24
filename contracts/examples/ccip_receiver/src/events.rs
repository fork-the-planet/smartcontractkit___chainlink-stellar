use soroban_sdk::{contractevent, BytesN};

/// Emitted when this example receiver handles an inbound CCIP message via `ccip_receive`.
///
/// Includes scalars only (no unbounded `Bytes` in the event) so payloads stay small on-chain.
#[contractevent(topics = ["example_CcipMessageReceived"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CcipMessageReceivedEvent {
    pub message_id: BytesN<32>,
    pub source_chain_selector: u64,
    pub data_len: u32,
    pub sender_len: u32,
    pub dest_token_transfers: u32,
}
