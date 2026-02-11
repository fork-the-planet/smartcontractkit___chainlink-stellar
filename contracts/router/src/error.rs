use soroban_sdk::contracterror;

// ============================================================
// Errors
// ============================================================

#[contracterror]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum RouterError {
    /// Contract already initialized
    AlreadyInitialized = 1,
    /// Contract not initialized
    NotInitialized = 2,
    /// Caller is not authorized
    Unauthorized = 3,
    /// Destination chain not supported (no OnRamp configured)
    UnsupportedDestinationChain = 4,
    /// Insufficient fee token amount provided
    InsufficientFeeTokenAmount = 5,
    /// Invalid message value (non-zero native sent with fee token)
    InvalidMsgValue = 6,
    /// Chain is cursed by RMN - operations halted
    BadRMNSignal = 7,
    /// OffRamp mismatch - trying to remove non-existent OffRamp
    OffRampMismatch = 8,
    /// Invalid recipient address
    InvalidRecipientAddress = 9,
    /// Failed to send value
    FailedToSendValue = 10,
    /// OnRamp returned an error
    OnRampError = 11,
    /// OffRamp already exists for this source chain
    OffRampAlreadyExists = 12,
}

impl From<common_authorization::AuthError> for RouterError {
    fn from(e: common_authorization::AuthError) -> Self {
        match e {
            common_authorization::AuthError::NotInitialized => RouterError::NotInitialized,
            _ => RouterError::Unauthorized,
        }
    }
}

impl From<onramp::error::OnRampError> for RouterError {
    fn from(_e: onramp::error::OnRampError) -> Self {
        RouterError::OnRampError
    }
}

impl From<rmn_proxy::error::RmnProxyError> for RouterError {
    fn from(e: rmn_proxy::error::RmnProxyError) -> Self {
        match e {
            rmn_proxy::error::RmnProxyError::NotInitialized => RouterError::NotInitialized,
            _ => RouterError::BadRMNSignal,
        }
    }
}
