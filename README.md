# Chainlink CCIP Stellar

## Tooling Commands

### Build contracts 

```shell
stellar contract build
```

### Generate Contract Interfaces

```shell
make generate-interfaces
```

Note: This command also builds the contracts.

⚠️ Because interfaces are also used within contracts as a crate, it is required that it can compile.

### Generate Bindings

```shell
make generate-bindings
```

Note: This command also builds the contracts and generates interfaces. It then uses the interfaces to generate the Go bindings.

---

## Running Tests

### Integration Tests

❕ Make sure the contracts are built and Go bindings are generated before running the tests. Running `make generate-bindings` is sufficient to have the pre-requisites necessary.

To run all integration tests, use the following command from the base directory:

```shell
go test ./tests/integration/... -v -tags=integration -count=1 -p=1 -timeout=15m
```

To run a specific integration test, use the `-run` flag instead of specifying the file directly:

```shell
go test ./tests/integration/... -v -tags=integration -count=1 -p=1 -timeout=15m -run TestRmnRemote
```

> Note: This is required to make sure that a single test env setup is done when running multiple tests. This approach speeds up the spin up time to avoid having each test suite wait for 120-150 seconds for the local Stellar node to be ready.


### Contract Tests

To run the tests written in Rust (found in various `test.rs` files within `/contracts`), run the following command:

```shell
cargo test
```

To run a specific crate's tests, use the `-p` flag to specify the package:

```shell
cargo test -p ccvs-committee-verifier
```