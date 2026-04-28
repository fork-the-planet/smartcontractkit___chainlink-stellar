//! RBACTimelock contract errors.

use soroban_sdk::contracterror;

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq, PartialOrd, Ord)]
#[repr(u32)]
pub enum TimelockError {
    // Initialization
    NotInitialized = 1,
    AlreadyInitialized = 2,
    // Authorization
    NotAuthorized = 3,
    // Scheduling
    OperationAlreadyScheduled = 20,
    InsufficientDelay = 21,
    SelectorIsBlocked = 22,
    // Execution
    OperationNotReady = 30,
    MissingPredecessor = 31,
    CallReverted = 32,
    // Cancellation
    OperationCannotBeCancelled = 40,
    // Misc
    InvalidInvokeData = 50,
    IndexOutOfBounds = 51,
}

impl From<common_error::CCIPError> for TimelockError {
    fn from(e: common_error::CCIPError) -> Self {
        match e {
            common_error::CCIPError::AlreadyInitialized => TimelockError::AlreadyInitialized,
            common_error::CCIPError::NotInitialized => TimelockError::NotInitialized,
            _ => TimelockError::NotAuthorized,
        }
    }
}
