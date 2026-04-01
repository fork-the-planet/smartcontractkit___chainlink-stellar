# Default: show available recipes
default:
    just --list

# Coverage exclusion regex (mockery-generated files), aligned with chainlink-ccv Justfile
COVERAGE_EXCLUDE_REGEX := '(/mock_.*\\.go:|/_mocks/.*:|/mocks/.*:)'

# Host target triple (needed to override .cargo/config.toml wasm target for tests)
host_target := `rustc -vV | grep host | awk '{print $2}'`

# Run all Soroban contract tests
test-contracts:
    cargo test --workspace --target {{host_target}} --verbose

# Check all contracts compile (faster than full test)
check-contracts:
    cargo check --workspace --target {{host_target}}

# Build all contract WASMs
build-contracts:
    stellar contract build

# Format all Rust code
fmt-contracts:
    cargo fmt --all

# Check Rust formatting (CI mode, no changes)
fmt-contracts-check:
    cargo fmt --all -- --check

# Run Go unit tests (root module) with coverage; optional second arg "short" runs -short tests only.
# Excludes tests/e2e (requires running devenv; same idea as chainlink-ccv -short for heavy tests).
# Writes filtered coverprofile to coverage_file (strips mock files matching COVERAGE_EXCLUDE_REGEX).
test-coverage coverage_file="coverage.out" short="":
    #!/usr/bin/env bash
    set -euo pipefail
    pkgs=$(go list ./... | grep -v 'github.com/smartcontractkit/chainlink-stellar/tests/e2e$' || true)
    go test -v -race -fullpath -shuffle on {{ if short != "" { "-short" } else { "" } }} -coverprofile={{ coverage_file }} $pkgs
    { head -n1 {{ coverage_file }}; tail -n +2 {{ coverage_file }} | grep -v -E '{{ COVERAGE_EXCLUDE_REGEX }}' || true; } > {{ coverage_file }}.filtered
    mv {{ coverage_file }}.filtered {{ coverage_file }}

# Run Go unit tests (root module) with coverage
test-go:
    go test -v -race -fullpath -shuffle on -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Run Go unit tests (bindings module) with coverage
test-go-bindings:
    cd bindings && go test -v -race -fullpath -shuffle on -coverprofile=coverage-bindings.out ./...
    cd bindings && go tool cover -func=coverage-bindings.out

# Run all Go unit tests
test-go-all: test-go test-go-bindings

# Run Go integration tests (requires running Stellar localnet)
test-go-integration:
    go test -tags integration -v -timeout 5m ./tests/integration/...

# Generate mocks using mockery
mock:
    @echo "Cleaning existing mocks..."
    find ./internal/mocks -type f -name 'mock_*.go' -delete 2>/dev/null || true
    @echo "Generating mocks with mockery..."
    mockery

# Run all tests (contracts + Go)
test-all: test-contracts test-go-all
