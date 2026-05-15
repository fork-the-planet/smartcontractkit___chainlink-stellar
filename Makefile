WASM_DIR := target/wasm32v1-none/release

# Path to the chainlink-ccv repo that supplies the upstream dev images
# (verifier:dev, token-verifier:dev, executor:dev, indexer:dev, aggregator:dev,
# pricer:dev, ccv-fakes:dev). Override on the command line if your checkout
# lives elsewhere, e.g. `make docker-ccv-dev CCV_REPO=$HOME/code/chainlink-ccv`.
CCV_REPO ?= ../chainlink-ccv

.PHONY: build test test-e2e check fmt clean generate-interfaces generate-bindings docker-verifier docker-executor docker-ccv-dev restart-verifier restart-executor restart-verifier-executor docker-verifier-rc docker-executor-rc push-verifier push-executor push-all

build:
	stellar contract build

test-e2e:
	go test -v -timeout 30m ./tests/e2e/...

test-integration:
	go test -v -tags=integration -count=1 -p=1 -timeout=15m ./tests/integration/...

# Generate Rust interface files for all contracts from their WASM files.
# This can be run with `--no-build` to skip the build of the contracts.
generate-interfaces:
	./scripts/gen_interfaces.sh && cargo fmt -p common-interfaces

# Generate Go bindings for all contracts from their Rust interface files.
# This can be run with `--no-interfaces` to skip the 
# generation of the Rust interface files.
generate-bindings:
	./scripts/gen_bindings.sh && gofmt -w ./bindings/contracts/

test:
	cargo test --workspace

check:
	cargo check --workspace

lint:
	cargo fmt --all

clean:
	cargo clean

# Build the Stellar committee verifier Docker image used by E2E tests.
docker-verifier:
	docker build -f Dockerfile.verifier -t stellarcommittee-verifier:dev .

# Build the Stellar (standalone) executor Docker image used by E2E tests.
docker-executor:
	docker build -f Dockerfile.executor -t stellarexecutor:dev .

# Rebuild the chainlink-ccv dev Docker images that this devenv depends on. Run
# this after bumping chainlink-ccv in go.mod or whenever a structural move in
# chainlink-ccv (for example PR #1058 "Move verifier cmd") changes the WORKDIR
# baked into a dev image — the devenv container then fails to start with
# "open air.toml: no such file or directory" because the old image's WORKDIR
# no longer matches the freshly mounted source tree.
docker-ccv-dev:
	@if [ ! -d "$(CCV_REPO)" ]; then \
		echo "CCV_REPO=$(CCV_REPO) not found; clone smartcontractkit/chainlink-ccv next to chainlink-stellar or pass CCV_REPO=...";\
		exit 1; \
	fi
	cd $(CCV_REPO)/verifier   && just build-dev
	cd $(CCV_REPO)/executor   && just build-dev
	cd $(CCV_REPO)/indexer    && just build-dev
	cd $(CCV_REPO)/aggregator && just build-dev
	cd $(CCV_REPO)/pricer     && just build-dev
	cd $(CCV_REPO)/build/devenv/fakes && just build-dev

# Rebuild the verifier image and restart the devenv to pick up the new image.
restart-verifier: docker-verifier
	$(MAKE) down && $(MAKE) up

# Rebuild the executor image and restart the devenv to pick up the new image.
restart-executor: docker-executor
	$(MAKE) down && $(MAKE) up

# Rebuild both verifier and executor images and restart the devenv to pick up the new images.
restart-verifier-executor: docker-verifier docker-executor
	$(MAKE) down && $(MAKE) up

# ECR Registry and Image Configuration
# Override these variables when pushing to ECR:
#   make push-verifier ECR_REGISTRY=123456789012.dkr.ecr.us-west-2.amazonaws.com IMAGE_TAG=v1.2.3
ECR_REGISTRY ?= $(STELLAR_ECR_REGISTRY)
IMAGE_TAG ?= $(shell git rev-parse --short HEAD)

# Build release candidate images for ECR (tagged with :rc)
docker-verifier-rc:
	docker build -f Dockerfile.verifier -t stellarcommittee-verifier:rc .

docker-executor-rc:
	docker build -f Dockerfile.executor -t stellarexecutor:rc .

# Push committee-verifier image to ECR
# Requires: ECR_REGISTRY to be set (e.g., 123456789012.dkr.ecr.us-west-2.amazonaws.com)
# Usage: make push-verifier ECR_REGISTRY=<registry> [IMAGE_TAG=<tag>]
push-verifier: docker-verifier-rc
ifndef ECR_REGISTRY
	$(error ECR_REGISTRY is not set. Usage: make push-verifier ECR_REGISTRY=123456789012.dkr.ecr.us-west-2.amazonaws.com [IMAGE_TAG=v1.0.0])
endif
	@echo "Tagging and pushing stellarcommittee-verifier:$(IMAGE_TAG) to $(ECR_REGISTRY)..."
	docker tag stellarcommittee-verifier:rc $(ECR_REGISTRY)/stellarcommittee-verifier:$(IMAGE_TAG)
	docker tag stellarcommittee-verifier:rc $(ECR_REGISTRY)/stellarcommittee-verifier:latest
	docker push $(ECR_REGISTRY)/stellarcommittee-verifier:$(IMAGE_TAG)
	docker push $(ECR_REGISTRY)/stellarcommittee-verifier:latest
	@echo "Successfully pushed:"
	@echo "  - $(ECR_REGISTRY)/stellarcommittee-verifier:$(IMAGE_TAG)"
	@echo "  - $(ECR_REGISTRY)/stellarcommittee-verifier:latest"

# Push executor image to ECR
# Requires: ECR_REGISTRY to be set
# Usage: make push-executor ECR_REGISTRY=<registry> [IMAGE_TAG=<tag>]
push-executor: docker-executor-rc
ifndef ECR_REGISTRY
	$(error ECR_REGISTRY is not set. Usage: make push-executor ECR_REGISTRY=123456789012.dkr.ecr.us-west-2.amazonaws.com [IMAGE_TAG=v1.0.0])
endif
	@echo "Tagging and pushing stellarexecutor:$(IMAGE_TAG) to $(ECR_REGISTRY)..."
	docker tag stellarexecutor:rc $(ECR_REGISTRY)/stellarexecutor:$(IMAGE_TAG)
	docker tag stellarexecutor:rc $(ECR_REGISTRY)/stellarexecutor:latest
	docker push $(ECR_REGISTRY)/stellarexecutor:$(IMAGE_TAG)
	docker push $(ECR_REGISTRY)/stellarexecutor:latest
	@echo "Successfully pushed:"
	@echo "  - $(ECR_REGISTRY)/stellarexecutor:$(IMAGE_TAG)"
	@echo "  - $(ECR_REGISTRY)/stellarexecutor:latest"

# Push both images to ECR
# Usage: make push-all ECR_REGISTRY=<registry> [IMAGE_TAG=<tag>]
push-all: push-verifier push-executor
	@echo "All images pushed successfully to $(ECR_REGISTRY)"

up:
	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv up tests/env/env-stellar-evm.toml

down:
	CTF_CONFIGS=tests/env/env-stellar-evm.toml go run ./tests/testutils/cmd/devenv down tests/env/env-stellar-evm.toml
