# CCIP Stellar: chainlink-ccv alignment (keystore, accessors, devenv, adapters)

## Summary

This batch aligns `chainlink-stellar` with recent `chainlink-ccv` changes (including bootstrap key declarations via `WithKey`, `KeystoreSetter` / `KeystoreRegistry`, composable `cciptestinterfaces` updates, and deployment adapter consolidation). Production-facing Stellar signing no longer relies on randomly generated keypairs for Soroban deploy/read paths where the CCV bootstrap keystore is available.

Upstream references worth reading alongside this changelog:

- `chainlink-ccv` changelog: `changelog/2026-04-27_evm_adapter_consolidation.md` — `CommitteeVerifierOnchainAdapter`, merged `GetRegistry().Register`, removal of `ScanCommitteeStates` from CCV’s `AggregatorConfigAdapter`.
- Bootstrap / keys: PR [#1061](https://github.com/smartcontractkit/chainlink-ccv/pull/1061) (“bootstrap: allow explicit key declarations via WithKey”).
- Verifier layout / dev images: upstream PR #1058 (“Move verifier cmd”) — stale `verifier:dev` images can fail with missing `air.toml` until images are rebuilt.

---

## Keystore-backed Stellar transaction signing

### `deployment.TxSigner`

- Added `deployment.TxSigner` (`Address`, `SignTransaction`) so Soroban submission can use either an in-memory `*keypair.Full` or an external signer.
- `NewDeployer` wraps a full keypair in `NewKeypairSigner`; `NewDeployerWithSigner` accepts any `TxSigner`.
- All former `tx.Sign(...)` sites call `d.signer.SignTransaction(...)`.

### `chainlink-common` keystore bridge

- New `ccv/accessors/stellar_keystore_signer.go`: `LoadStellarKeystoreSigner` builds a signer that hashes the envelope locally, calls `keystore.Sign`, and attaches a `DecoratedSignature` (hint from Ed25519 pubkey), matching Stellar SDK semantics without extracting the private key.

### Stellar transmitter key name

- `ccv/common/consts.go`: `StellarTransmitterKeyName = "stellar/tx/stellar_transmitter_ed25519_key"` — mirrors the `evm/tx/` naming convention used upstream for executor transmitters.

---

## Accessor factory (`KeystoreSetter`, executor path)

- `ccv/accessors/factory.go`: Factory holds separate reader config (`sourcereader.ReaderConfig`) and destination config (`destinationConfig`: OffRamp / RMN remote strkeys, CCIP state-changed topic, optional per-chain transmitter key name).
- `SourceReader`, `DestinationReader`, and `ContractTransmitter` are built lazily after `SetKeystore(keystore.Keystore)` (implements `bootstrap.KeystoreSetter`).
- `NewReaderFactory` preserves committee-verifier-only tests (`NewFactory(..., nil dest map, 0)`).
- Pre-validation errors vs `errKeystoreNotInjected` vs `errDestConfigMissing` distinguish missing config, missing keystore injection, and incomplete executor TOML.

### Constructor (`CreateStellarAccessorFactory`)

- `ccv/accessors/factory_constructor.go`: `buildStellarDestConfigs` merges Stellar file TOML (`transmitter_configs`, `destination_reader_configs`) with `GenericConfig.ChainConfiguration` hex addresses (converted via `scval.HexToContractStrkey`). Entries without OffRamp + state-changed topic are dropped so verifier-only deployments do not claim executor capabilities.

### Tests

- `factory_test.go` updated for `NewReaderFactory` and keystore expectations.
- New `factory_keystore_test.go` exercises `SetKeystore`, missing keys, destination build, and default key-name fallback.

---

## Binaries

### Executor (`cmd/executor`)

- Uses `executorcmd.NewFactory()` from `chainlink-ccv`.
- Declares `bootstrap.WithKey(common.StellarTransmitterKeyName, "transmitting", keystore.Ed25519)`.
- Removed bespoke `cmd/executor/bootstrap.go` (logic folded into accessors + upstream bootstrap).

### Committee verifier (`cmd/committee-verifier`)

- Declares both ECDSA signing key (`commit.DefaultECDSASigningKeyName`) and `StellarTransmitterKeyName` (Ed25519) for the Soroban deployer used by the source reader path.

---

## CCV deployment adapters & registration (`ccv/chain`)

### `RegisterStellarComponents` (`ccv/chain/register.go`)

- Registers `CommitteeVerifierOnchain: &adapter.StellarCCVCommitteeVerifierOnchainAdapter{}` into `chainlink-ccv/deployment/adapters.GetRegistry()` so changesets such as `GenerateAggregatorConfig` can scan Stellar committee verifier state (parity with EVM’s ccip `init()` registration).

### `StellarCCVCommitteeVerifierOnchainAdapter` (`ccv/chain/adapter/ccv_committee_verifier_onchain.go`)

- `ScanCommitteeStates`: datastore `CommitteeVerifier` refs → Soroban `get_all_signature_configs`, signer addresses normalized to 20-byte EVM-style hex (same padding convention as existing Stellar aggregator adapter logic).
- `ApplySignatureConfigs`: resolves contract by qualifier, maps hex signers to left-padded 32-byte keys, invokes `apply_signature_configs`.

### `StellarCCVDeploymentAggregatorConfigAdapter` (`ccv/chain/adapter/ccv_deployment_adapters.go`)

- Implements only CCV’s slim `AggregatorConfigAdapter` (`ResolveVerifierAddress`). On-chain committee scanning was removed from this type because CCV moved it to `CommitteeVerifierOnchainAdapter`.

### Other devenv / chain wiring (high level)

- **Executor modifier** (`ccv/chain/modifier/executor.go`): reads executor app config from `GeneratedJobSpecs[0].AppConfig` instead of removed `GeneratedConfig`.
- **`ImplFactory.DefaultSignerKey`** (`ccv/chain/impl_factory.go`): returns the ECDSA address for verifier signing — Stellar’s `committee_verifier` expects ETH-style 20-byte signer identities; this also fixes devenv topology enrichment so standalone verifiers do not fall through to JD “nodeIDs must be specified” lookups.
- **Composable / legacy chain API** (`ccv/chain/composable.go`, `ccv/chain/chain.go`): updated for `cciptestinterfaces` (`SendMessage` signature with message version / extra-args provider, `GenericChainMessage`, `ChainSendOption`, etc.).
- **Extra-args serializers** (`ccv/chain/register.go`): registers EVM serializers for Stellar destination `(family, version)` tuples so EVM-as-source messages encode correctly for Stellar destinations.

### CLDF / deployment adapters

- `ccv/chain/adapter/ccv_deployment_adapters.go`, `aggregator_config_adapter.go`: stop using ephemeral random keypairs for read-only simulations; use `NewDeployerWithSigner(..., NewSDKSigner(chain.Signer))`.

---

## Topology & local tooling

- **`tests/env/env-stellar-evm.toml`**: removed deprecated `port` fields from `[[executor]]` blocks (upstream executor config no longer accepts them).
- **`Makefile`**: `docker-ccv-dev` target (with `CCV_REPO`) rebuilds upstream `chainlink-ccv` dev images (`verifier`, `executor`, `indexer`, `aggregator`, `pricer`, `build/devenv/fakes`) after bumps or Dockerfile `WORKDIR` changes.

---

## E2E tests

- Shared message V3 version constant; all `SendMessage` calls updated for the new `Chain` signature.
- Removed use of deprecated `MessageOptions.Version` where applicable.
- `evm_to_stellar_exec_test.go`: inlined scenario helper using `ExtraArgsSerializer` registry where needed for Stellar destination encoding.

---

## Operational notes

1. After upgrading `chainlink-ccv` or changing verifier paths, run `make docker-ccv-dev` (or rebuild equivalent images) before `make up` if containers fail with missing `air.toml`.
2. Ensure `RegisterStellarComponents()` is invoked for any binary or test harness that runs CCV deployment changesets against Stellar selectors — otherwise `CommitteeVerifierOnchain` remains unset for Stellar.

---

## Files touched (reference)

| Area | Paths |
|------|--------|
| Keystore / signing | `deployment/deployer.go`, `ccv/accessors/stellar_keystore_signer.go`, `ccv/common/consts.go` |
| Accessors | `ccv/accessors/factory.go`, `factory_constructor.go`, `factory_test.go`, `factory_keystore_test.go` |
| Commands | `cmd/executor/main.go`, `cmd/committee-verifier/main.go`; removed `cmd/executor/bootstrap.go` |
| CCV chain | `ccv/chain/register.go`, `impl_factory.go`, `chain.go`, `composable.go`, `modifier/executor.go`, `adapter/*.go` |
| Env / Makefile | `tests/env/env-stellar-evm.toml`, `Makefile` |
| E2E | `tests/e2e/evm_to_stellar_*.go`, `tests/e2e/stellar_to_evm_*.go` |

This list may omit tiny edits; `git diff main` is authoritative for the branch.
