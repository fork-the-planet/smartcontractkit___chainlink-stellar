#[soroban_sdk::contractclient(name = "FeeQuoterClient")]
pub trait FeeQuoterInterface {
    fn initialize(
        env: soroban_sdk::Env,
        owner: soroban_sdk::Address,
        static_config: StaticConfig,
        authorized_callers: soroban_sdk::Vec<soroban_sdk::Address>,
    ) -> Result<(), FeeQuoterError>;
    fn update_prices(
        env: soroban_sdk::Env,
        price_updates: PriceUpdates,
    ) -> Result<(), FeeQuoterError>;
    fn get_fee_tokens(
        env: soroban_sdk::Env,
    ) -> Result<soroban_sdk::Vec<soroban_sdk::Address>, FeeQuoterError>;
    fn get_message_fee(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
        message: StellarToAnyMessage,
    ) -> Result<i128, FeeQuoterError>;
    fn get_token_price(
        env: soroban_sdk::Env,
        token: soroban_sdk::Address,
    ) -> Result<TimestampedPrice, FeeQuoterError>;
    fn get_token_prices(
        env: soroban_sdk::Env,
        tokens: soroban_sdk::Vec<soroban_sdk::Address>,
    ) -> Result<soroban_sdk::Vec<TimestampedPrice>, FeeQuoterError>;
    fn get_static_config(env: soroban_sdk::Env) -> Result<StaticConfig, FeeQuoterError>;
    fn remove_fee_tokens(
        env: soroban_sdk::Env,
        tokens: soroban_sdk::Vec<soroban_sdk::Address>,
    ) -> Result<(), FeeQuoterError>;
    fn quote_gas_for_exec(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
        non_calldata_gas: u32,
        calldata_size: u32,
        fee_token: soroban_sdk::Address,
    ) -> Result<GasQuoteResult, FeeQuoterError>;
    fn convert_token_amount(
        env: soroban_sdk::Env,
        from_token: soroban_sdk::Address,
        from_token_amount: i128,
        to_token: soroban_sdk::Address,
    ) -> Result<i128, FeeQuoterError>;
    fn get_all_dest_configs(
        env: soroban_sdk::Env,
    ) -> Result<(soroban_sdk::Vec<u64>, soroban_sdk::Vec<DestChainConfig>), FeeQuoterError>;
    fn get_token_fee_config(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
        token: soroban_sdk::Address,
    ) -> Result<TokenTransferFeeConfig, FeeQuoterError>;
    fn get_dest_chain_config(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
    ) -> Result<DestChainConfig, FeeQuoterError>;
    fn get_token_transfer_fee(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
        token: soroban_sdk::Address,
    ) -> Result<TokenTransferFeeResult, FeeQuoterError>;
    fn apply_token_fee_configs(
        env: soroban_sdk::Env,
        config_args: soroban_sdk::Vec<TokenFeeConfigArgs>,
        remove_args: soroban_sdk::Vec<TokenFeeConfigRemoveArgs>,
    ) -> Result<(), FeeQuoterError>;
    fn apply_dest_chain_configs(
        env: soroban_sdk::Env,
        config_args: soroban_sdk::Vec<DestChainConfigArgs>,
    ) -> Result<(), FeeQuoterError>;
    fn get_dest_chain_gas_price(
        env: soroban_sdk::Env,
        dest_chain_selector: u64,
    ) -> Result<TimestampedPrice, FeeQuoterError>;
    fn get_validated_token_price(
        env: soroban_sdk::Env,
        token: soroban_sdk::Address,
    ) -> Result<u128, FeeQuoterError>;
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct PriceUpdates {
    pub gas_price_updates: soroban_sdk::Vec<GasPriceUpdate>,
    pub token_price_updates: soroban_sdk::Vec<TokenPriceUpdate>,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct StaticConfig {
    pub link_token: soroban_sdk::Address,
    pub max_fee_juels_per_msg: i128,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct GasPriceUpdate {
    pub dest_chain_selector: u64,
    pub usd_per_unit_gas: u128,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct GasQuoteResult {
    pub fee_token_price: u128,
    pub gas_cost_usd_cents: u128,
    pub premium_multiplier: u32,
    pub total_gas: u32,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct DestChainConfig {
    pub default_token_dest_gas: u32,
    pub default_token_fee_usd: u32,
    pub default_tx_gas_limit: u32,
    pub dest_gas_overhead: u32,
    pub dest_gas_per_payload_byte: u32,
    pub is_enabled: bool,
    pub link_premium_percent: u32,
    pub max_data_bytes: u32,
    pub max_per_msg_gas_limit: u32,
    pub network_fee_usd_cents: u32,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct TimestampedPrice {
    pub timestamp: u64,
    pub value: u128,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct TokenPriceUpdate {
    pub token: soroban_sdk::Address,
    pub usd_per_token: u128,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct TokenFeeConfigArgs {
    pub config: TokenTransferFeeConfig,
    pub dest_chain_selector: u64,
    pub token: soroban_sdk::Address,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct DestChainConfigArgs {
    pub config: DestChainConfig,
    pub dest_chain_selector: u64,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct TokenTransferFeeConfig {
    pub dest_bytes_overhead: u32,
    pub dest_gas_overhead: u32,
    pub fee_usd_cents: u32,
    pub is_enabled: bool,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct TokenTransferFeeResult {
    pub dest_bytes_overhead: u32,
    pub dest_gas_overhead: u32,
    pub fee_usd_cents: u32,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct TokenFeeConfigRemoveArgs {
    pub dest_chain_selector: u64,
    pub token: soroban_sdk::Address,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct TokenAmount {
    pub amount: i128,
    pub token: soroban_sdk::Address,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct AnyToStellarMessage {
    pub placeholder: u64,
}
#[soroban_sdk::contracttype(export = false)]
#[derive(Debug, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub struct StellarToAnyMessage {
    pub data: soroban_sdk::Bytes,
    pub extra_args: soroban_sdk::Bytes,
    pub fee_token: soroban_sdk::Address,
    pub receiver: soroban_sdk::Bytes,
    pub token_amounts: soroban_sdk::Vec<TokenAmount>,
}
#[soroban_sdk::contracterror(export = false)]
#[derive(Debug, Copy, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub enum FeeQuoterError {
    AlreadyInitialized = 1,
    NotInitialized = 2,
    Unauthorized = 3,
    TokenNotSupported = 4,
    FeeTokenNotSupported = 5,
    NoGasPriceAvailable = 6,
    DestinationChainNotEnabled = 7,
    InvalidExtraArgsTag = 8,
    InvalidExtraArgsData = 9,
    MessageGasLimitTooHigh = 10,
    MessageTooLarge = 11,
    UnsupportedNumberOfTokens = 12,
    InvalidDestChainConfig = 13,
    MessageFeeTooHigh = 14,
    InvalidStaticConfig = 15,
    InvalidTokenReceiver = 16,
    SourceTokenDataTooLarge = 17,
    InvalidDestBytesOverhead = 18,
    CallerNotAuthorized = 19,
    AuthorizedCallerAlreadyExists = 20,
    AuthorizedCallerNotFound = 21,
    NoPendingOwner = 22,
    ReentrancyGuardReentrantCall = 23,
    AuthFeatureNotEnabled = 24,
    InvalidTokenAmount = 25,
    InvalidReceiverAddress = 26,
}
#[soroban_sdk::contracterror(export = false)]
#[derive(Debug, Copy, Clone, Eq, PartialEq, Ord, PartialOrd)]
pub enum AuthError {
    NotInitialized = 1,
    Unauthorized = 2,
    NotOwner = 3,
    NoPendingOwner = 4,
    CallerNotAuthorized = 5,
    CallerAlreadyAuthorized = 6,
    CallerNotFound = 7,
    RoleNotGranted = 8,
    FeatureNotEnabled = 9,
    RoleAlreadyGranted = 10,
    CannotRenounceRole = 11,
}
