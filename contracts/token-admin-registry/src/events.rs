use soroban_sdk::{contractevent, Address};

#[contractevent(topics = ["tar_PoolSet"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PoolSetEvent {
    pub token: Address,
    pub previous_pool: Option<Address>,
    pub new_pool: Option<Address>,
}

#[contractevent(topics = ["tar_AdminTransferReq"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdminTransferRequestedEvent {
    pub token: Address,
    pub current_admin: Option<Address>,
    pub new_admin: Option<Address>,
}

#[contractevent(topics = ["tar_AdminTransferred"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdministratorTransferredEvent {
    pub token: Address,
    pub new_admin: Address,
}

#[contractevent(topics = ["tar_ModuleAdded"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RegistryModuleAddedEvent {
    pub module: Address,
}

#[contractevent(topics = ["tar_ModuleRemoved"])]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RegistryModuleRemovedEvent {
    pub module: Address,
}
