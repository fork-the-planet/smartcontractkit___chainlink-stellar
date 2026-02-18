#![no_std]

/// Derive macro for generating `From<T>` implementations on error enums.
///
/// Annotate a unit variant with one or more `#[from(...)]` attributes:
///
/// ```ignore
/// use common_error::ErrorConversions;
///
/// #[derive(ErrorConversions)]
/// enum ContractError {
///     #[from(common_authorization::AuthError)]
///     Unauthorized,
/// }
/// ```
pub use common_error_derive::ErrorConversions;
