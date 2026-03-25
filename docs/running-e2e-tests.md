# Running E2E Tests (locally)

## Setup

### Repo setup

To run things locally, make sure you're on the `feature/setup-stellar` branch of `chainlink-ccv` (this is only required for that remaining portion of `NewCLDF...` method but otherwise not needed).

You'll notice that I've removed any StellarConfig definition from that repo.

Run `go mod tidy` in `chainlink-ccv` and `chainlink-stellar`

### Network setup

Build the committee-verifier image with `make docker-verifier`

Run `make up`

This will up all the containers incl. that of the stellar verifier referenced above.

It will also output the `....-out.toml` file which the E2E test will use.

> Note: something that's a bit weird now is that debugging still requires looking into the logs of the docker container running the verifier.

Use `docker ps` and `docker logs -f $CONTAINER_ID` is useful to inspect the status of containers and read through their logs.

### Running E2E tests

Now you can run the E2E tests with `go test -v -timeout 15m ./tests/e2e/...`


---

## Debugging

...


---

## Important Notes

### Where are contracts deployed?

Contracts are deployed as a part of the toplogy spin up step (when `make up` is called) which invokes the chain's `DeployContractsForSelector(...)` method:

1. The registeration of various chain-specific implementations is done in [/ccv/devenv/register.go](../ccv/devenv/register.go)
2. One of the registered implementations is a `ChainImplFactory` (see [ccv/chain/chain_impl_factory.go](../ccv/chain/impl_factory.go)) which provides an instance of the chain implementation (in this case, Stellar - see [ccv/chain/chain.go](../ccv/chain/chain.go))
3. The chain implmenetation incldues a method `DeployContractsForSelector(...)` which is responsible for deploying contracts to teh provided chain selector.


### Is it required to re-build all the images when making changes?

...