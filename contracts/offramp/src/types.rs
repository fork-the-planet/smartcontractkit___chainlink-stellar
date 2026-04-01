use common_error::CCIPError;
use common_helpers::validation::Validatable;
use soroban_sdk::{contracttype, Address, Bytes, BytesN, Vec};

// ============================================================
// Storage Key Enum
// ============================================================

#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    ExecState(BytesN<32>),
}

// ============================================================
// MessageExecutionState
// ============================================================

/// Execution state of a CCIP message on the destination chain.
/// Mirrors the EVM `Internal.MessageExecutionState`.
#[contracttype]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u32)]
pub enum MessageExecutionState {
    Untouched = 0,
    InProgress = 1,
    Success = 2,
    Failure = 3,
}

// ============================================================
// StaticConfig
// ============================================================

/// Immutable configuration set once at initialization.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StaticConfig {
    /// Local chain selector identifying this chain
    pub chain_selector: u64,
    /// RMN proxy contract address (for curse checking)
    pub rmn_proxy: Address,
    /// Token admin registry contract address (for pool lookups)
    pub token_admin_registry: Address,
}

// ============================================================
// SourceChainConfig
// ============================================================

/// Per-source-chain configuration controlling which lanes are enabled.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SourceChainConfig {
    /// Router address for routing verified messages to receivers
    pub router: Address,
    /// Whether this source chain lane is enabled
    pub is_enabled: bool,
    /// Allowed OnRamp addresses (raw bytes, one per historical deployment)
    pub on_ramps: Vec<Bytes>,
    /// Default CCVs used when the receiver doesn't specify its own
    pub default_ccvs: Vec<Address>,
    /// Lane-mandated CCVs required for all messages on this lane
    pub lane_mandated_ccvs: Vec<Address>,
}

// ============================================================
// SourceChainConfigArgs
// ============================================================

/// Arguments for updating source chain configuration.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SourceChainConfigArgs {
    /// Source chain selector
    pub source_chain_selector: u64,
    /// Router address
    pub router: Address,
    /// Whether this lane is enabled
    pub is_enabled: bool,
    /// Allowed OnRamp addresses
    pub on_ramps: Vec<Bytes>,
    /// Default CCVs
    pub default_ccvs: Vec<Address>,
    /// Lane-mandated CCVs
    pub lane_mandated_ccvs: Vec<Address>,
}

impl Validatable for SourceChainConfigArgs {
    fn validate(&self) -> Result<(), CCIPError> {
        if self.source_chain_selector == 0 {
            return Err(CCIPError::InvalidSourceChainConfig);
        }

        if self.on_ramps.is_empty() {
            return Err(CCIPError::InvalidSourceChainConfig);
        }

        if self.default_ccvs.is_empty() && self.lane_mandated_ccvs.is_empty() {
            return Err(CCIPError::InvalidSourceChainConfig);
        }

        Ok(())
    }
}
