use soroban_sdk::contracterror;

// ============================================================
// Errors
// ============================================================

#[contracterror]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum RmnProxyError {
    /// Contract already initialized
    AlreadyInitialized = 1,
    /// Contract not initialized
    NotInitialized = 2,
    /// Caller is not authorized
    Unauthorized = 3,
}

impl From<common_authorization::AuthError> for RmnProxyError {
    fn from(e: common_authorization::AuthError) -> Self {
        match e {
            common_authorization::AuthError::NotInitialized => RmnProxyError::NotInitialized,
            _ => RmnProxyError::Unauthorized,
        }
    }
}
