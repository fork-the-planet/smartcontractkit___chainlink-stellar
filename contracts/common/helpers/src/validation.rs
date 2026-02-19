use common_error::CCIPError;

/// A trait to define abstract behavior for validating a type.
pub trait Validatable {
    fn validate(&self) -> Result<(), CCIPError>;
}
