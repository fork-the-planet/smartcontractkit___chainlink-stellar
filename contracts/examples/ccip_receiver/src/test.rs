#![cfg(test)]

extern crate std;

use soroban_sdk::{testutils::Address as _, vec, Address, Bytes, BytesN, Env};

use crate::{ExampleCcipReceiver, ExampleCcipReceiverClient};

use common_message::AnyToStellarMessage;

#[test]
fn ccip_receive_accepts_router_auth() {
    let env = Env::default();
    env.mock_all_auths();

    let router = Address::generate(&env);
    let receiver_id = env.register(ExampleCcipReceiver, ());

    let client = ExampleCcipReceiverClient::new(&env, &receiver_id);
    client.initialize(&router);

    let msg = AnyToStellarMessage {
        message_id: BytesN::from_array(&env, &[7u8; 32]),
        source_chain_selector: 1,
        sender: Bytes::from_array(&env, &[0u8; 32]),
        data: Bytes::from_slice(&env, &[1, 2, 3]),
        dest_token_amounts: vec![&env],
    };

    // With `mock_all_auths`, `router.require_auth_for_args` is satisfied without `as_contract`.
    client.ccip_receive(&msg);

    let stored = client.last_message_id();
    assert_eq!(stored, msg.message_id);
}
