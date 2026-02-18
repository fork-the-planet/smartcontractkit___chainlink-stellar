use soroban_sdk::{contracttype, xdr::Error, Address, Bytes, Env, Map, Symbol, Vec};

// const REMOTE_CHAINS: Symbol = symbol_short!("RCHAINS");
// const ALLOWLIST: Symbol = symbol_short!("ALLOWLST");
// const STORAGE_LOCATIONS: Symbol = symbol_short!("STORLOC");
// const RMN_PROXY: Symbol = symbol_short!("RMNPROXY");

/// Remote chain config mirrored from EVM BaseVerifier.RemoteChainConfig.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RemoteChainConfig {
    pub remote_chain_selector: u64,
    pub router: Option<Address>,
    pub allowlist_enabled: bool,
    pub fee_usd_cents: u32,
    pub gas_for_verification: u32,
    pub payload_size_bytes: u32,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RemoteChainConfigArgs {
    pub remote_chain_selector: u64,
    pub router: Option<Address>,
    pub allowlist_enabled: bool,
    pub fee_usd_cents: u32,
    pub gas_for_verification: u32,
    pub payload_size_bytes: u32,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AllowlistConfigArgs {
    pub dest_chain_selector: u64,
    pub allowlist_enabled: bool,
    pub added_allowlisted_senders: Vec<Address>,
    pub removed_allowlisted_senders: Vec<Address>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BaseVerifierError {
    InvalidConfig,
    RemoteChainNotSupported,
    SenderNotAllowed,
    NotInitialized,
}

/// Shared base-verifier logic used by concrete verifier contracts.
// pub struct BaseVerifier;

pub trait BaseVerifier {
    const STORAGE_LOCATIONS: Symbol;
    const RMN_PROXY: Symbol;
    const REMOTE_CHAINS: Symbol;
    const ALLOWLIST: Symbol;

    type Error: Into<Error>;

    fn init(
        env: &Env,
        storage_locations: &Vec<Bytes>,
        rmn_proxy: &Address,
    ) -> Result<(), Self::Error>;
    fn get_storage_locations(env: &Env) -> Option<Vec<Bytes>>;
    fn set_storage_locations(env: &Env, new_locations: &Vec<Bytes>) -> Result<(), Self::Error>;
    fn apply_remote_chain_config_updates(
        env: &Env,
        remote_chain_config_args: &Vec<RemoteChainConfigArgs>,
    ) -> Result<(), Self::Error>;
    fn get_remote_chain_config(
        env: &Env,
        remote_chain_selector: u64,
    ) -> Result<RemoteChainConfig, Self::Error>;
    fn apply_allowlist_updates(
        env: &Env,
        allowlist_config_args_items: &Vec<AllowlistConfigArgs>,
    ) -> Result<(), Self::Error>;
    fn get_fee(env: &Env, dest_chain_selector: u64) -> Result<(u32, u32, u32), Self::Error>;
}

// impl BaseVerifier {
//     /// Initializes base verifier storage fields.
//     pub fn init(env: &Env, storage_locations: &Vec<Bytes>, rmn_proxy: &Address) {
//         env.storage().instance().set(&STORAGE_LOCATIONS, storage_locations);
//         env.storage().instance().set(&RMN_PROXY, rmn_proxy);

//         let remote_chains: Map<u64, RemoteChainConfig> = Map::new(env);
//         env.storage().persistent().set(&REMOTE_CHAINS, &remote_chains);

//         let allowlist: Map<u64, Vec<Address>> = Map::new(env);
//         env.storage().persistent().set(&ALLOWLIST, &allowlist);
//     }

//     pub fn get_storage_locations(env: &Env) -> Vec<Bytes> {
//         env.storage()
//             .instance()
//             .get(&STORAGE_LOCATIONS)
//             .unwrap_or(Vec::new(env))
//     }

//     pub fn set_storage_locations(env: &Env, new_locations: &Vec<Bytes>) {
//         env.storage().instance().set(&STORAGE_LOCATIONS, new_locations);
//     }

//     pub fn apply_remote_chain_config_updates(
//         env: &Env,
//         remote_chain_config_args: &Vec<RemoteChainConfigArgs>,
//     ) -> Result<(), BaseVerifierError> {
//         let mut remote_chains: Map<u64, RemoteChainConfig> = env
//             .storage()
//             .persistent()
//             .get(&REMOTE_CHAINS)
//             .unwrap_or(Map::new(env));

//         for update in remote_chain_config_args.iter() {
//             if update.remote_chain_selector == 0 || update.gas_for_verification == 0 {
//                 return Err(BaseVerifierError::InvalidConfig);
//             }

//             remote_chains.set(
//                 update.remote_chain_selector,
//                 RemoteChainConfig {
//                     remote_chain_selector: update.remote_chain_selector,
//                     router: update.router.clone(),
//                     allowlist_enabled: update.allowlist_enabled,
//                     fee_usd_cents: update.fee_usd_cents,
//                     gas_for_verification: update.gas_for_verification,
//                     payload_size_bytes: update.payload_size_bytes,
//                 },
//             );

//             // TODO: publish RemoteChainConfigSet event from caller contract.
//         }

//         env.storage().persistent().set(&REMOTE_CHAINS, &remote_chains);
//         Ok(())
//     }

//     pub fn get_remote_chain_config(
//         env: &Env,
//         remote_chain_selector: u64,
//     ) -> Result<RemoteChainConfig, BaseVerifierError> {
//         let remote_chains: Map<u64, RemoteChainConfig> = env
//             .storage()
//             .persistent()
//             .get(&REMOTE_CHAINS)
//             .unwrap_or(Map::new(env));

//         remote_chains
//             .get(remote_chain_selector)
//             .ok_or(BaseVerifierError::RemoteChainNotSupported)
//     }

//     pub fn apply_allowlist_updates(
//         env: &Env,
//         allowlist_config_args_items: &Vec<AllowlistConfigArgs>,
//     ) -> Result<(), BaseVerifierError> {
//         let mut remote_chains: Map<u64, RemoteChainConfig> = env
//             .storage()
//             .persistent()
//             .get(&REMOTE_CHAINS)
//             .unwrap_or(Map::new(env));
//         let mut allowlist: Map<u64, Vec<Address>> = env
//             .storage()
//             .persistent()
//             .get(&ALLOWLIST)
//             .unwrap_or(Map::new(env));

//         for update in allowlist_config_args_items.iter() {
//             let mut cfg = remote_chains
//                 .get(update.dest_chain_selector)
//                 .ok_or(BaseVerifierError::RemoteChainNotSupported)?;
//             cfg.allowlist_enabled = update.allowlist_enabled;
//             remote_chains.set(update.dest_chain_selector, cfg);

//             let mut chain_allowlist = allowlist
//                 .get(update.dest_chain_selector)
//                 .unwrap_or(Vec::new(env));

//             for to_remove in update.removed_allowlisted_senders.iter() {
//                 let mut filtered = Vec::new(env);
//                 for existing in chain_allowlist.iter() {
//                     if existing != to_remove {
//                         filtered.push_back(existing);
//                     } else {
//                         // TODO: publish AllowListSendersRemoved event from caller contract.
//                     }
//                 }
//                 chain_allowlist = filtered;
//             }

//             for to_add in update.added_allowlisted_senders.iter() {
//                 if !update.allowlist_enabled {
//                     return Err(BaseVerifierError::InvalidConfig);
//                 }
//                 let mut exists = false;
//                 for existing in chain_allowlist.iter() {
//                     if existing == to_add {
//                         exists = true;
//                         break;
//                     }
//                 }
//                 if !exists {
//                     chain_allowlist.push_back(to_add);
//                     // TODO: publish AllowListSendersAdded event from caller contract.
//                 }
//             }

//             allowlist.set(update.dest_chain_selector, chain_allowlist);
//             // TODO: publish AllowListStateChanged event from caller contract.
//         }

//         env.storage().persistent().set(&REMOTE_CHAINS, &remote_chains);
//         env.storage().persistent().set(&ALLOWLIST, &allowlist);
//         Ok(())
//     }

//     pub fn get_fee(
//         env: &Env,
//         dest_chain_selector: u64,
//     ) -> Result<(u32, u32, u32), BaseVerifierError> {
//         let cfg = Self::get_remote_chain_config(env, dest_chain_selector)?;
//         Ok((
//             cfg.fee_usd_cents,
//             cfg.gas_for_verification,
//             cfg.payload_size_bytes,
//         ))
//     }

//     pub fn assert_not_cursed_by_rmn(
//         env: &Env,
//         _chain_selector: u64,
//     ) -> Result<(), BaseVerifierError> {
//         // TODO: call RMN proxy contract once interface is added.
//         let exists = env.storage().instance().has(&RMN_PROXY);
//         if !exists {
//             return Err(BaseVerifierError::NotInitialized);
//         }
//         Ok(())
//     }

//     pub fn assert_sender_is_allowed(
//         env: &Env,
//         dest_chain_selector: u64,
//         sender: &Address,
//     ) -> Result<(), BaseVerifierError> {
//         let cfg = Self::get_remote_chain_config(env, dest_chain_selector)?;
//         if cfg.router.is_none() {
//             return Err(BaseVerifierError::RemoteChainNotSupported);
//         }

//         if cfg.allowlist_enabled {
//             let allowlist: Map<u64, Vec<Address>> = env
//                 .storage()
//                 .persistent()
//                 .get(&ALLOWLIST)
//                 .unwrap_or(Map::new(env));
//             let chain_allowlist = allowlist.get(dest_chain_selector).unwrap_or(Vec::new(env));
//             let mut allowed = false;
//             for listed in chain_allowlist.iter() {
//                 if listed == *sender {
//                     allowed = true;
//                     break;
//                 }
//             }
//             if !allowed {
//                 return Err(BaseVerifierError::SenderNotAllowed);
//             }
//         }

//         // TODO: enforce caller is router's OnRamp equivalent (BaseVerifier parity).
//         Ok(())
//     }
// }
