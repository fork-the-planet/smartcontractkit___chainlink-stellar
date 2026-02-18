use soroban_sdk::contracterror;
use common_verifier::BaseVerifierError;

#[contracterror]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum CommitteeVerifierError {
    AlreadyInitialized = 1,
    NotInitialized = 2,
    Unauthorized = 3,
    InvalidConfig = 4,
    InvalidVerifierResults = 5,
    InvalidCCVVersion = 6,
    RemoteChainNotSupported = 7,
    SenderNotAllowed = 8,
    SourceNotConfigured = 9,
    WrongNumberOfSignatures = 10,
    InvalidSignatureConfig = 11,
    InvalidStorageLocationsAdmin = 12,
}

impl From<BaseVerifierError> for CommitteeVerifierError {
    fn from(value: BaseVerifierError) -> Self {
        match value {
            BaseVerifierError::InvalidConfig => CommitteeVerifierError::InvalidConfig,
            BaseVerifierError::RemoteChainNotSupported => {
                CommitteeVerifierError::RemoteChainNotSupported
            }
            BaseVerifierError::SenderNotAllowed => CommitteeVerifierError::SenderNotAllowed,
            BaseVerifierError::NotInitialized => CommitteeVerifierError::NotInitialized,
        }
    }
}
