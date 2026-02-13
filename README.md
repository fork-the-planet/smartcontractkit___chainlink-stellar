# Chainlink CCIP Stellar

## Contracts Commands

### Build contracts 

```shell
stellar contract build
```

### Generate Contract Interfaces

Generate Rust Bindings (for the purpose of interfaces). The example below is for the `ccvs/proxy` contract

```shell
stellar contract bindings rust --wasm ./target/wasm32v1-none/release/ccvs_proxy.wasm > contracts/common/interfaces/ccvs_proxy.rs
```

Then we go into the generated file and remove any non-necessary code for the interfaces.


### Generate Bindings

> ⚠️ This is not fully functional yet.

```shell
stellar contract bindings rust --wasm ../target/wasm32v1-none/release/fee_quoter.wasm | go run ./generator -name FeeQuoter -pkg fee_quoter -out ./fee_quoter
```