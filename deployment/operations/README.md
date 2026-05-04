# Stellar deployment operations

This directory holds **Chainlink Deployments Framework (CLDF)** operations for Stellar CCIP Soroban contracts: small, composable units that perform **at most one on-chain side effect** per execution (one deploy, one contract `invoke`, etc.). Read-only `simulate` flows and event polling helpers belong in callers or dedicated read operations, not mixed into mutating ops.

## References (patterns to mirror)

- **Solana (CLDF + per-program packages):** [chains/solana/deployment/v1_6_0/operations/](https://github.com/smartcontractkit/chainlink-ccip/tree/main/chains/solana/deployment/v1_6_0/operations/)
- **EVM (CLDF + codegen-heavy ops):** [chains/evm/deployment/v2_0_0/operations/](https://github.com/smartcontractkit/chainlink-ccip/tree/main/chains/evm/deployment/v2_0_0/operations/)

Stellar bindings are hand-written Soroban clients under `bindings/contracts/…`; operations should wrap those clients and accept **`stellardeps.StellarDeps`** (see `stellardeps/`) for deploy + invoke, rather than reaching into `deployment.Deployer` directly from every op.

## Conventions

- **Operation IDs:** `kebab-case`, namespaced by contract and action (e.g. `offramp:deploy`, `offramp:apply-source-chain-cfg-updates`).
- **Semver:** Each `operations.NewOperation` gets a `*semver.Version`. Shared baseline for this repo: `operations.ContractDeploymentVersion` (`1.0.0`) until per-contract release lines exist.
- **ContractType (datastore alignment):** Use PascalCase strings consistent with CCIP lane tooling, e.g. `OffRamp`, `OnRamp`, `Router`, `FeeQuoter`, `RmnRemote`, `RmnProxy`, `RampRegistry`, `TokenAdminRegistry`, `CommitteeVerifier`, `VersionedVerifierResolver`, `LockReleasePool`, `BurnMintPool`, `SiloedLockReleasePool`, `TokenLockBox`, `MCMS`, `Timelock`, `CCIPReceiver` (example receiver).
- **WASM paths:** Built artifacts live at `target/wasm32v1-none/release/<crate>.wasm` relative to the repo root; `<crate>` follows Cargo’s underscore rules (hyphens in `[package].name` become underscores).

## Bundle usage (tests and runners)

CLDF expects a **Bundle** with context, logger, and reporter:

```go
bundle := operations.NewBundle(
    func() context.Context { return ctx },
    cldflogger.Test(t), // or cldflogger.Nop()
    operations.NewMemoryReporter(),
)
_, err := operations.ExecuteOperation(bundle, someOp, deps, input)
```

Handlers use the signature `func(b operations.Bundle, deps DEP, input IN) (OUT, error)` (see `chainlink-deployments-framework/operations`). Inputs and outputs must be JSON-serializable unless you opt into framework escape hatches.

## Contract inventory (bindings → WASM → role)

| Bindings package | Suggested `ContractType` | `release/*.wasm` | Notes |
|------------------|--------------------------|------------------|--------|
| `offramp` | `OffRamp` | `offramp.wasm` | Lane sink: init, source chain config, execution, fee updates, ownership |
| `onramp` | `OnRamp` | `onramp.wasm` | Lane source: init, dest chain allowlists, fee/family config, sends |
| `router` | `Router` | `router.wasm` | OffRamp routing, RMN config, ownership |
| `fee_quoter` | `FeeQuoter` | `fee_quoter.wasm` | Token price / fee config mutators |
| `rmn_remote` | `RmnRemote` | `rmn_remote.wasm` | RMN curse / config updates |
| `rmn_proxy` | `RmnProxy` | `rmn_proxy.wasm` | Proxy admin / routing to remote |
| `ramp_registry` | `RampRegistry` | `ccip_ramp_registry.wasm` | Ramp registration and lookups |
| `token_admin_registry` | `TokenAdminRegistry` | `token_admin_registry.wasm` | Token admin / pool registry updates |
| `committee_verifier` | `CommitteeVerifier` | `ccvs_committee_verifier.wasm` | Verifier set / digest config |
| `versioned_verifier_resolver` | `VersionedVerifierResolver` | `ccvs_versioned_verifier_resolver.wasm` | Resolver binding updates |
| `lock_release_pool` | `LockReleasePool` | `pools_lock_release_pool.wasm` | Pool-specific init and lock/release I/O |
| `burn_mint_pool` | `BurnMintPool` | `pools_burn_mint_pool.wasm` | Burn/mint style pool |
| `siloed_lock_release_pool` | `SiloedLockReleasePool` | `pools_siloed_lock_release_pool.wasm` | Siloed L/R pool |
| `token_lock_box` | `TokenLockBox` | `pools_token_lock_box.wasm` | Lock box transfers / admin |
| `token_pool` | (per deployment) | (see pool crates) | Shared interface used by concrete pool clients |
| `mcms` | `MCMS` | `mcms.wasm` | MCMS roots, ops, role changes |
| `timelock` | `Timelock` | `timelock.wasm` | Timelock schedule / execute |
| `ccip_receiver` | `CCIPReceiver` | `ccip_receiver_example.wasm` | Example receiver (dev / tests) |

WASM filenames follow `cargo build --release` output for workspace members under `contracts/`.

## Layout

- `stellardeps/` — `StellarDeps` (`Deploy` + `Invoker`) and `FromDeployer(*deployment.Deployer)`.
- `types.go` / `deploy.go` — shared `Void`, `DeployInput` / `DeployOutput`, and `NewDeployOperation(id, description)` for WASM deploy ops.
- **Per-contract packages** (each exports `ContractType` and `operations.NewOperation` vars): `offramp/`, `onramp/`, `router/`, `rmn_remote/`, `rmn_proxy/`, `fee_quoter/`, `ramp_registry/`, `committee_verifier/`, `versioned_verifier_resolver/`, `ccip_receiver/`, `token_admin_registry/`, `token_pool/`, `burn_mint_pool/`, `siloed_lock_release_pool/`, `token_lock_box/`, `mcms/`, `timelock/`.

## Devenv (Phase 5)

`deployment/ccip/devenv` phases build a CLDF `Bundle` (`cldflogger.Nop()`, in-memory reporter) on `work` and run Stellar ops via `execStellarOp` (`work_ops.go`). **FeeQuoter `update_prices`** still uses the generated client directly because `PriceUpdates` embeds `scval.U128` types that are not CLDF JSON-serialization friendly.
