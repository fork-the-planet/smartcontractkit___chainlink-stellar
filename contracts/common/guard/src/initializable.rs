use soroban_sdk::{Env, Symbol, symbol_short};
use common_error::CCIPError as InitializableError;

pub trait Initializable {
    const INITIALIZED: Symbol = symbol_short!("INIT");

    fn init(env: &Env) -> Result<(), InitializableError> {
        if env.storage().instance().has(&Self::INITIALIZED) {
            return Err(InitializableError::AlreadyInitialized);
        }
        env.storage().instance().set(&Self::INITIALIZED, &true);
        Ok(())
    }

    fn is_initialized(env: &Env) -> bool {
        env.storage().instance().has(&Self::INITIALIZED)
    }

    fn require_initialized(env: &Env) -> Result<(), InitializableError> {
        if !Self::is_initialized(env) {
            return Err(InitializableError::NotInitialized);
        }
        Ok(())
    }

    fn require_not_initialized(env: &Env) -> Result<(), InitializableError> {
        if Self::is_initialized(env) {
            return Err(InitializableError::AlreadyInitialized);
        }
        Ok(())
    }
}
