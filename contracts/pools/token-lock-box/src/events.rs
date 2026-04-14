use soroban_sdk::{contractevent, Address};

#[contractevent(topics = ["lockbox_Deposit"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DepositEvent {
    pub token: Address,
    pub depositor: Address,
    pub amount: i128,
}

#[contractevent(topics = ["lockbox_Withdrawal"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WithdrawalEvent {
    pub token: Address,
    pub recipient: Address,
    pub amount: i128,
}
