default: build

all: test

test: build
	cargo test

build:
	stellar contract build
	@ls -l ../../../target/wasm32v1-none/release/ccvs_versioned_verifier_resolver.wasm

fmt:
	cargo fmt --all

clean:
	cargo clean
