use soroban_sdk::contracterror;

// ============================================================
// Errors
// TODO: numeric errors are fine but too basic, need to change error types below to use thiserror or anyhow instead
// ============================================================

#[contracterror]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum OnRampError {
    /// Contract already initialized
    AlreadyInitialized = 1,
    /// Contract not initialized
    NotInitialized = 2,
    /// Caller is not authorized
    Unauthorized = 3,
    /// Invalid configuration provided
    InvalidConfig = 4,
    /// Destination chain not supported
    DestinationChainNotSupported = 5,
    /// Caller must be the router
    MustBeCalledByRouter = 6,
    /// Original sender must be set
    RouterMustSetOriginalSender = 7,
    /// Cannot send zero tokens
    CannotSendZeroTokens = 8,
    /// Can only send one token per message
    CanOnlySendOneTokenPerMessage = 9,
    /// Unsupported token
    UnsupportedToken = 10,
    /// Invalid destination chain address
    InvalidDestChainAddress = 11,
    /// Fee exceeds maximum allowed
    FeeExceedsMaxAllowed = 12,
    /// Insufficient fee token amount
    InsufficientFeeTokenAmount = 13,
    /// Reentrancy guard triggered
    ReentrancyGuardReentrantCall = 14,
    /// CCV does not support destination chain
    DestinationChainNotSupportedByCCV = 15,
    /// Token receiver not allowed for this destination
    TokenReceiverNotAllowed = 16,
    /// Source token data exceeds maximum length
    SourceTokenDataTooLarge = 17,
    /// Chain is cursed by RMN
    CursedByRMN = 18,
    /// Invalid token amount
    InvalidTokenAmount = 19,
    /// Invalid receiver address
    InvalidReceiverAddress = 20,
}

impl From<common_message::Error> for OnRampError {
    fn from(error: common_message::Error) -> Self {
        match error {
            common_message::Error::InvalidTokenAmount => OnRampError::InvalidTokenAmount,
            common_message::Error::InvalidReceiverAddress => OnRampError::InvalidReceiverAddress,
        }
    }
}

impl From<common_guard::GuardError> for OnRampError {
    fn from(error: common_guard::GuardError) -> Self {
        match error {
            common_guard::GuardError::ReentrantCall => OnRampError::ReentrancyGuardReentrantCall,
        }
    }
}