use soroban_sdk::contracterror;

// ============================================================
// Errors
// ============================================================

#[contracterror]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum FeeQuoterError {
    /// Contract already initialized
    AlreadyInitialized = 1,
    /// Contract not initialized
    NotInitialized = 2,
    /// Caller is not authorized
    Unauthorized = 3,
    /// Token not supported (price not set)
    TokenNotSupported = 4,
    /// Fee token not supported
    FeeTokenNotSupported = 5,
    /// No gas price available for destination chain
    NoGasPriceAvailable = 6,
    /// Destination chain not enabled
    DestinationChainNotEnabled = 7,
    /// Invalid extra args tag
    InvalidExtraArgsTag = 8,
    /// Invalid extra args data
    InvalidExtraArgsData = 9,
    /// Message gas limit too high
    MessageGasLimitTooHigh = 10,
    /// Message too large
    MessageTooLarge = 11,
    /// Unsupported number of tokens
    UnsupportedNumberOfTokens = 12,
    /// Invalid destination chain config
    InvalidDestChainConfig = 13,
    /// Message fee too high (exceeds max_fee_juels_per_msg)
    MessageFeeTooHigh = 14,
    /// Invalid static config
    InvalidStaticConfig = 15,
    /// Invalid token receiver
    InvalidTokenReceiver = 16,
    /// Source token data too large
    SourceTokenDataTooLarge = 17,
    /// Invalid dest bytes overhead
    InvalidDestBytesOverhead = 18,
    /// Caller is not an authorized price updater
    CallerNotAuthorized = 19,
    /// Authorized caller already exists
    AuthorizedCallerAlreadyExists = 20,
    /// Authorized caller not found
    AuthorizedCallerNotFound = 21,
    /// No pending ownership transfer
    NoPendingOwner = 22,
    /// Reentrancy guard triggered
    ReentrancyGuardReentrantCall = 23,
    /// Authorization feature not enabled
    AuthFeatureNotEnabled = 24,
    /// Invalid token amount in message
    InvalidTokenAmount = 25,
    /// Invalid receiver address in message
    InvalidReceiverAddress = 26,
}

// ============================================================
// Error conversions from common libraries
// ============================================================

impl From<common_authorization::AuthError> for FeeQuoterError {
    fn from(error: common_authorization::AuthError) -> Self {
        match error {
            common_authorization::AuthError::NotInitialized => FeeQuoterError::NotInitialized,
            common_authorization::AuthError::Unauthorized => FeeQuoterError::Unauthorized,
            common_authorization::AuthError::NotOwner => FeeQuoterError::Unauthorized,
            common_authorization::AuthError::NoPendingOwner => FeeQuoterError::NoPendingOwner,
            common_authorization::AuthError::CallerNotAuthorized => {
                FeeQuoterError::CallerNotAuthorized
            }
            common_authorization::AuthError::CallerAlreadyAuthorized => {
                FeeQuoterError::AuthorizedCallerAlreadyExists
            }
            common_authorization::AuthError::CallerNotFound => {
                FeeQuoterError::AuthorizedCallerNotFound
            }
            common_authorization::AuthError::FeatureNotEnabled => {
                FeeQuoterError::AuthFeatureNotEnabled
            }
            // Role-based errors are not used by FeeQuoter, map to generic Unauthorized
            _ => FeeQuoterError::Unauthorized,
        }
    }
}

impl From<common_guard::GuardError> for FeeQuoterError {
    fn from(error: common_guard::GuardError) -> Self {
        match error {
            common_guard::GuardError::ReentrantCall => FeeQuoterError::ReentrancyGuardReentrantCall,
        }
    }
}

impl From<common_message::Error> for FeeQuoterError {
    fn from(error: common_message::Error) -> Self {
        match error {
            common_message::Error::InvalidTokenAmount => FeeQuoterError::InvalidTokenAmount,
            common_message::Error::InvalidReceiverAddress => FeeQuoterError::InvalidReceiverAddress,
        }
    }
}
