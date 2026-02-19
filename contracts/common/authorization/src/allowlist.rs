use soroban_sdk::{Address, Env, IntoVal, Map, Symbol, TryFromVal, Val, Vec};

use common_error::CCIPError;
use common_helpers::validation::Validatable;

pub trait AllowListUpdateInterface: Validatable {
    fn key(&self) -> u64;
    fn get_allowlist_addresses_to_remove(&self) -> Vec<Address>;
    fn get_allowlist_addresses_to_add(&self) -> Vec<Address>;
}

// Ownable is now a trait; use DefaultOwnable for generic owner checks in apply_allowlist_updates.

/// A trait to maintain a set of allowed addresses for a any purpose.
/// It can be used for authorization as well as guarding access to certain functions.
///
/// For example: The Committee Verifier contract can use this trait to maintain a set of allowed addresses for a given destination chain.
///
/// The allow list is a map of u64 to vector of addresses.
pub trait AllowListable {
    const ALLOW_LIST: Symbol; // Storage key for the allow list data

    type AllowListUpdate: AllowListUpdateInterface
        + TryFromVal<Env, Val>
        + IntoVal<Env, Val>
        + Clone;

    fn emit_allowlist_updated_event(
        env: &Env,
        key: u64,
        added_addresses: &Vec<Address>,
        removed_addresses: &Vec<Address>,
    );

    /// Initialize the authorized callers list.
    /// This enables the feature and sets the initial list of authorized callers.
    ///
    /// # Arguments
    /// * `env` - The environment
    /// * `initial_callers` - Initial list of authorized addresses
    fn init_allowlist(env: &Env, initial_allowlist: Map<u64, Vec<Address>>) {
        env.storage()
            .instance()
            .set(&Self::ALLOW_LIST, &initial_allowlist);
    }

    /// Check if the allow list is enabled for a given key.
    fn is_allowlist_enabled(env: &Env, key: u64) -> bool {
        env.storage()
            .instance()
            .get(&Self::ALLOW_LIST)
            .map(|map: Option<Map<u64, Vec<Address>>>| match map {
                Some(map) => map.contains_key(key),
                None => false,
            })
            .unwrap_or(false)
    }

    /// Add addresses to the allow list.
    /// Requires owner authorization.
    ///
    /// # Arguments
    /// * `env` - The environment
    /// * `callers` - Addresses to add
    ///
    /// # Errors
    /// * `FeatureNotEnabled` - Allow list not initialized
    /// * `NotInitialized` - Owner not set
    fn apply_allowlist_updates(
        env: &Env,
        updates: &Vec<Self::AllowListUpdate>,
    ) -> Result<(), CCIPError> {
        for update in updates.iter() {
            update.validate()?;

            let key = update.key();
            let to_add = update.get_allowlist_addresses_to_add();
            let to_remove = update.get_allowlist_addresses_to_remove();

            if !Self::is_allowlist_enabled(env, key) {
                return Err(CCIPError::FeatureNotEnabled);
            }

            let mut data: Map<u64, Vec<Address>> = env
                .storage()
                .instance()
                .get(&Self::ALLOW_LIST)
                .unwrap_or(Map::new(env));

            let mut allowlist = data.get(key).unwrap_or(Vec::new(env));

            for address in to_add.iter() {
                if !allowlist.contains(address.clone()) {
                    allowlist.push_back(address.clone());
                }
            }

            for address in to_remove.iter() {
                if allowlist.contains(address.clone()) {
                    allowlist.remove(allowlist.first_index_of(address.clone()).unwrap());
                }
            }

            data.set(key, allowlist);
            env.storage().instance().set(&Self::ALLOW_LIST, &data);

            Self::emit_allowlist_updated_event(env, key, &to_add, &to_remove);
        }

        Ok(())
    }

    /// Get the allowlist for a specific key.
    fn get_allowlist_entry(env: &Env, key: u64) -> Vec<Address> {
        env.storage()
            .instance()
            .get(&Self::ALLOW_LIST)
            .unwrap_or(Map::new(env))
            .get(key)
            .unwrap_or(Vec::new(env))
    }

    /// Check if an address is in the allow list.
    fn is_in_allowlist(env: &Env, key: u64, addr: &Address) -> bool {
        let allowlist = Self::get_allowlist_entry(env, key);
        allowlist.contains(addr)
    }

    /// Require that a given address is in the allow list.
    ///
    /// # Errors
    /// * `FeatureNotEnabled` - AuthorizedCallers not initialized
    /// * `CallerNotAuthorized` - No authorized caller provided auth
    fn require_in_allowlist(env: &Env, key: u64, address: &Address) -> Result<(), CCIPError> {
        if !Self::is_allowlist_enabled(env, key) {
            return Err(CCIPError::FeatureNotEnabled);
        }

        let allowlist = Self::get_allowlist_entry(env, key);
        if !allowlist.contains(address) {
            return Err(CCIPError::CallerNotAuthorized);
        }

        Ok(())
    }
}
