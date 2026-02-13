# Generate Cross-Contract Interfaces

Generate or update `#[contractclient]` interface traits in `contracts/common/interfaces/` for all deployable contracts in `contracts/` (excluding `common/` and `examples/`).

## Scope

Target these contracts (from `Cargo.toml` workspace members):

- `contracts/fee-quoter`
- `contracts/onramp`
- `contracts/offramp`
- `contracts/rmn_proxy`
- `contracts/router`
- `contracts/registry`
- `contracts/ccvs/*` (proxy, committee-verifier, base-verifier)
- `contracts/pools/*` (burn-mint-pool, base-pool, lock-release-pool)

**Exclude:** `contracts/common/*`, `contracts/examples/*`

## Steps

For each target contract:

1. **Read the contract** – Inspect `contracts/<name>/src/lib.rs` for all `pub fn` methods in the `#[contractimpl]` block (exclude `initialize` unless it is meant to be called cross-contract).

2. **Create or update the interface module** – Add or edit `contracts/common/interfaces/src/<snake_case_name>.rs`:
   - Use snake_case for the file (e.g. `fee_quoter.rs`, `rmn_proxy.rs`).
   - Add module doc comment describing the interface and primary callers.
   - Use `#[contractclient(name = "<PascalCase>Client")]` and `pub trait <PascalCase>Interface`.
   - Mirror each public function’s signature exactly (including `Result<T, E>` if the contract returns it).
   - Import types and errors from the contract crate (e.g. `fee_quoter::types::*`, `fee_quoter::error::FeeQuoterError`).
   - Use `soroban_sdk::{contractclient, Address, Env, Vec, ...}` as needed.

3. **Expose contract internals** – If the contract has private `mod error` or `mod types`, change them to `pub mod` so the interface crate can import them.

4. **Wire up dependencies** – In `contracts/common/interfaces/Cargo.toml`:
   - Add the contract as a dependency (e.g. `fee-quoter = { workspace = true }`).
   - In the workspace root `Cargo.toml`, add the contract to `[workspace.dependencies]` if missing (e.g. `fee-quoter = { path = "contracts/fee-quoter" }`).

5. **Register the module** – In `contracts/common/interfaces/src/lib.rs`, add `pub mod <snake_case_name>;` if not already present.

## Reference implementations

- **Simple:** `rmn_proxy.rs` – single function, no custom types.
- **With shared types:** `onramp.rs` – uses `common_message::StellarToAnyMessage`.
- **With contract types/errors:** `fee_quoter.rs` – imports `fee_quoter::types::*` and `fee_quoter::error::FeeQuoterError`.

## Output

- One interface file per contract under `contracts/common/interfaces/src/`.
- `common-interfaces` builds successfully (`cargo check -p common-interfaces` or `make check`).
