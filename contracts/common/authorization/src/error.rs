use soroban_sdk::contracterror;

// ============================================================
// Authorization Errors
// ============================================================

/// Errors that can occur in the authorization library.
#[contracterror]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum AuthError {
    /// Owner has not been initialized.
    NotInitialized = 1,
    /// Caller is not authorized.
    Unauthorized = 2,
    /// Caller is not the owner.
    NotOwner = 3,
    /// No pending ownership transfer.
    NoPendingOwner = 4,
    /// Caller is not in the authorized callers list.
    CallerNotAuthorized = 5,
    /// Caller is already in the authorized callers list.
    CallerAlreadyAuthorized = 6,
    /// Caller not found in the authorized callers list.
    CallerNotFound = 7,
    /// Address does not have the required role.
    RoleNotGranted = 8,
    /// The feature (AuthorizedCallers or AccessControl) has not been enabled.
    FeatureNotEnabled = 9,
    /// Address already has the role.
    RoleAlreadyGranted = 10,
    /// Cannot renounce a role you don't have.
    CannotRenounceRole = 11,
}
