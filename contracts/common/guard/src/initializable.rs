use common_error::CCIPError;
use soroban_sdk::{symbol_short, Env, Symbol};

pub trait Initializable {
    const INITIALIZED: Symbol = symbol_short!("INIT");

    fn init(env: &Env) -> Result<(), CCIPError> {
        if env.storage().instance().has(&Self::INITIALIZED) {
            return Err(CCIPError::AlreadyInitialized);
        }
        env.storage().instance().set(&Self::INITIALIZED, &true);
        Ok(())
    }

    fn is_initialized(env: &Env) -> bool {
        env.storage().instance().has(&Self::INITIALIZED)
    }

    fn require_initialized(env: &Env) -> Result<(), CCIPError> {
        if !Self::is_initialized(env) {
            return Err(CCIPError::NotInitialized);
        }
        Ok(())
    }

    fn require_not_initialized(env: &Env) -> Result<(), CCIPError> {
        if Self::is_initialized(env) {
            return Err(CCIPError::AlreadyInitialized);
        }
        Ok(())
    }
}
