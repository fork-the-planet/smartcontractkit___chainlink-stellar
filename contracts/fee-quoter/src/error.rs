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
    /// Price updater already exists
    AuthorizedCallerAlreadyExists = 20,
    /// Price updater not found
    AuthorizedCallerNotFound = 21,
}
