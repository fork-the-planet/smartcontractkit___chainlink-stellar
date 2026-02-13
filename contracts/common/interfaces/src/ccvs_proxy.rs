
#[soroban_sdk::contractclient(name = "CCVSProxyClient")]
pub trait CCVSProxyInterface {
    fn hello(
        env: soroban_sdk::Env,
        to: soroban_sdk::String,
    ) -> soroban_sdk::Vec<soroban_sdk::String>;
}

