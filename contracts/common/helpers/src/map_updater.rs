use soroban_sdk::{Env, IntoVal, Map, TryFromVal, Val, Vec, xdr::Error};

pub trait MapUpdate {
    type Key: TryFromVal<Env, Val> + IntoVal<Env, Val>;
    type Value: TryFromVal<Env, Val> + IntoVal<Env, Val>;

    fn key(&self) -> Self::Key;
    fn value(&self) -> Option<Self::Value>;
}

pub trait MapUpdater<T, K, V>
 where
    T: MapUpdate<Key = K, Value = V> + TryFromVal<Env, Val> + IntoVal<Env, Val> + Clone,
    K: TryFromVal<Env, Val> + IntoVal<Env, Val> + Clone,
    V: TryFromVal<Env, Val> + IntoVal<Env, Val>,
{
    type Error: Into<Error>;

    fn get_map(&self, env: &Env) -> Result<Map<K, V>, Self::Error>;
    fn get_key_set(&self, env: &Env) -> Result<Vec<K>, Self::Error>;

    fn apply_updates(&self, env: &Env, updates: &Vec<T>) -> Result<(), Self::Error> {
        let mut map = self.get_map(env)?;
        let mut key_set = self.get_key_set(env)?;

        updates.iter().for_each(|update| {
            let _ = match (update.key(), update.value()) {
                (_, None) => {
                    map.remove(update.key());
                    match key_set.first_index_of(&update.key()) {
                        Some(idx) => key_set.remove(idx),
                        None => None,
                    };
                },
                (key, Some(value)) => {
                    if !key_set.contains(key.clone()) {
                        key_set.push_back(key.clone());
                    }

                    map.set(key, value);
                },
            };
        });

        Ok(())
    }
}
