# Chainlink CCIP Stellar

## Contracts Commands

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
