---
name: run-e2e-tests
description: "Run and debug E2E tests for the Stellar CCIP integration. Use when asked to run E2E tests, start/stop the devenv, rebuild Docker images, check container logs, or debug E2E test failures. Covers the full lifecycle: tearing down old containers, rebuilding images, starting the devenv, running tests, and inspecting logs."
---

# Run E2E Tests

All commands run from the `chainlink-stellar` repo root.

## Full Workflow

```
make down                  # 1. Stop existing containers
make docker-executor       # 2. Rebuild executor image (if needed)
make docker-verifier       # 3. Rebuild verifier image (if needed)
make up                    # 4. Start devenv (~5+ minutes)
make test-e2e              # 5. Run E2E tests
```

### Step Details

**Step 1 — `make down`**: Always run first to ensure a clean slate. Tears down all containers from prior runs.

**Steps 2 & 3 — Image Rebuilds**: Only required when source code changes affect the built images. See "When to Rebuild Images" below.

**Step 4 — `make up`**: Starts the full devenv topology defined in `tests/env/env-stellar-evm.toml`. This takes **5+ minutes** to become fully ready. It launches blockchains, verifiers, executors, aggregators, and an indexer. Wait for it to complete before proceeding — it writes `tests/env/env-stellar-evm-out.toml` which the tests read.

**Step 5 — `make test-e2e`**: Runs `go test -v -timeout 15m ./tests/e2e/...`. Run individual tests with:

```bash
go test -v -timeout 10m ./tests/e2e/... -run TestEVMToStellarExecution
go test -v -timeout 10m ./tests/e2e/... -run TestStellarToEVMExecution
```

### Convenience Targets

Rebuild images **and** restart the devenv in one step:

```bash
make restart-executor            # rebuild executor + down + up
make restart-verifier            # rebuild verifier + down + up
make restart-verifier-executor   # rebuild both + down + up
```

## When to Rebuild Images

Determine whether to rebuild based on what changed:

| Changed files | Rebuild executor? | Rebuild verifier? |
|---|---|---|
| `cmd/executor/` or executor Go code | Yes | No |
| `cmd/committee-verifier/` or verifier Go code | No | Yes |
| `ccv/` (shared chain/devenv Go code) | Yes | Yes |
| `bindings/` (contract bindings) | Yes | Yes |
| `contracts/` (Soroban contracts only) | No | No |
| `tests/` (test code only) | No | No |
| `go.mod` / `go.sum` | Yes | Yes |

- The executor image is `stellarexecutor:dev` (built by `Dockerfile.executor`, entry point `cmd/executor`).
- The verifier image is `stellarcommittee-verifier:dev` (built by `Dockerfile.verifier`, entry point `cmd/committee-verifier`).
- Both Dockerfiles drop local `chainlink-ccv` replace directives and resolve from the module proxy, so changes in `../chainlink-ccv` that are not yet published require publishing or updating the pseudo-version in `go.mod`.

## Container Topology

The devenv spins up these containers (configured in `tests/env/env-stellar-evm.toml`):

| Container | Image | Role |
|---|---|---|
| `blockchain-stellar` | `stellar/quickstart:testing` | Stellar localnet |
| `blockchain-src` | `ghcr.io/foundry-rs/foundry` | EVM chain (Anvil, chain 1337) |
| `blockchain-dst` | `ghcr.io/foundry-rs/foundry` | EVM chain (Anvil, chain 2337) |
| `stellar-verifier-1` | `stellarcommittee-verifier:dev` | Stellar committee verifier |
| `stellar-verifier-2` | `stellarcommittee-verifier:dev` | Stellar committee verifier |
| `evm-verifier-1` | `verifier:dev` | EVM committee verifier |
| `evm-verifier-2` | `verifier:dev` | EVM committee verifier |
| `stellar-executor-1` | `stellarexecutor:dev` | Stellar executor |
| `evm-executor-1` | `executor:dev` | EVM executor |
| `default-aggregator` | `aggregator:dev` | Committee aggregator |
| `indexer-1` | `indexer:dev` | Verification indexer |
| Various `-db` containers | `postgres:16-alpine` | Databases for verifiers/indexer/aggregator |

## Checking Container Logs

### List running containers

```bash
docker ps -a
```

### Follow logs for a specific container

```bash
docker logs -f stellar-verifier-1
docker logs -f stellar-executor-1
docker logs -f evm-executor-1
docker logs -f blockchain-stellar
docker logs -f indexer-1
docker logs -f default-aggregator
```

### Tail recent logs (last N lines)

```bash
docker logs --tail 50 stellar-verifier-1
```

### Search logs for a keyword

```bash
docker logs stellar-executor-1 2>&1 | grep -i "error"
docker logs stellar-verifier-1 2>&1 | grep -i "messageID"
```

### Dump all container logs (useful after failures)

```bash
for c in $(docker ps -a --format '{{.Names}}'); do
  echo "=== $c ==="
  docker logs "$c" 2>&1 | tail -50
done
```

### Key containers to check by failure type

| Symptom | Check these containers first |
|---|---|
| Message not sent | `blockchain-stellar` or `blockchain-src` (source chain) |
| Message sent but not verified | `stellar-verifier-1`, `stellar-verifier-2` (or `evm-verifier-*`) |
| Verified but not aggregated | `default-aggregator` |
| Aggregated but not indexed | `indexer-1` |
| Indexed but not executed | `stellar-executor-1` or `evm-executor-1` (destination executor) |
| Container crashing / restarting | The crashing container itself + its `-db` container |

## Debugging Tips

- The devenv config output (`tests/env/env-stellar-evm-out.toml`) contains the resolved addresses, ports, and CLDF datastore that tests use. Inspect it to verify contracts were deployed.
- Stellar verifiers have finality checking disabled for the Stellar chain selector (`17301180955411967724`) via `disable_finality_checkers` in the config.
- The committee threshold is 2 for all chains, meaning both verifiers must agree.
- The executor checks the aggregator/indexer for verified messages, so if execution stalls, check the upstream components (verifiers → aggregator → indexer) in order.
- If `make up` fails partway, always `make down` before retrying.
