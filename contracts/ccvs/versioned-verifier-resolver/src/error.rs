use common_authorization::AuthError;
use common_error::ErrorConversions;
use soroban_sdk::contracterror;

#[contracterror]
#[derive(Clone, Copy, Debug, Eq, PartialEq, ErrorConversions)]
#[repr(u32)]
pub enum VerifierResolverError {
    /// Contract already initialized
    AlreadyInitialized = 1,
    /// Contract not initialized
    NotInitialized = 2,
    /// Caller is not authorized (not the owner)
    #[from(AuthError)]
    Unauthorized = 3,
    /// Verifier results data is too short (must be at least 4 bytes for version prefix)
    InvalidVerifierResultsLength = 4,
    /// No inbound implementation found for the given version
    InboundImplementationNotFound = 5,
    /// No outbound implementation found for the given destination chain
    OutboundImplementationNotFound = 6,
    /// Invalid configuration: zero address not allowed
    InvalidAddress = 7,
    /// Invalid configuration: zero chain selector not allowed
    InvalidChainSelector = 8,
    /// Invalid version: zero version (bytes4(0)) not allowed when setting
    InvalidVersion = 9,
}

impl From<VerifierResolverError> for soroban_sdk::xdr::Error {
    fn from(error: VerifierResolverError) -> Self {
        error.into()
    }
}
