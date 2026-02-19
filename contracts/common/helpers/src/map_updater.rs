use soroban_sdk::{Env, IntoVal, Map, Symbol, TryFromVal, Val, Vec};
use common_error::CCIPError;

pub trait MapUpdate {
    type Key: TryFromVal<Env, Val> + IntoVal<Env, Val>;
    type Value: TryFromVal<Env, Val> + IntoVal<Env, Val>;

    fn key(&self) -> Self::Key;
    fn value(&self) -> Option<Self::Value>;
}

/// A trait to define abstract behavior for applying updates to a state map using
/// a key-value pair.
///
/// # Generic Parameters
/// * `T` - The type of the update, must implement `MapUpdate<Key = K, Value = V>`
/// * `K` - The type of the key
/// * `V` - The type of the value
///
/// # Errors
/// * `Error` - An error type that can be converted to an `Error`
///
/// # Returns
/// * `Ok(())` - If the updates are applied successfully
/// * `Err(Error)` - If the updates are not applied successfully
pub trait MapUpdater<T, K, V>
where
    T: MapUpdate<Key = K, Value = V> + TryFromVal<Env, Val> + IntoVal<Env, Val> + Clone,
    K: TryFromVal<Env, Val> + IntoVal<Env, Val> + Clone,
    V: TryFromVal<Env, Val> + IntoVal<Env, Val>,
{
    const MAP_NAME: Symbol;
    const KEY_SET_NAME: Symbol;
    type Error: From<CCIPError>;

    fn get_map(&self, env: &Env) -> Option<Map<K, V>> {
        env.storage().instance().get(&Self::MAP_NAME)
    }

    fn get_key_set(&self, env: &Env) -> Option<Vec<K>> {
        env.storage().instance().get(&Self::KEY_SET_NAME)
    }

    fn validate_update(&self, _update: &T) -> Result<(), Self::Error> {
        Ok(())
    }

    fn emit_set_event(&self, _env: &Env, _update: &T) {}
    fn emit_remove_event(&self, _env: &Env, _update: &T) {}

    fn apply_updates(&self, env: &Env, updates: &Vec<T>) -> Result<(), Self::Error> {
        // TODO: should this emit an error instead of using defaults?
        let mut map = self.get_map(env).unwrap_or(Map::new(env));
        let mut key_set = self.get_key_set(env).unwrap_or(Vec::new(env));

        updates.iter().for_each(|update| {
            let _ = match (update.key(), update.value()) {
                (_, None) => {
                    map.remove(update.key());
                    match key_set.first_index_of(&update.key()) {
                        Some(idx) => key_set.remove(idx),
                        None => None,
                    };
                    self.emit_remove_event(env, &update);
                }
                (key, Some(value)) => {
                    if !key_set.contains(key.clone()) {
                        key_set.push_back(key.clone());
                    }

                    map.set(key, value);
                    self.emit_set_event(env, &update);
                }
            };
        });

        self.save_changes(env, &key_set, &map)?;

        Ok(())
    }

    fn save_changes(&self, env: &Env, key_set: &Vec<K>, map: &Map<K, V>)
        -> Result<(), Self::Error>;
}
