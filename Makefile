WASM_DIR := target/wasm32v1-none/release
GENERATED_DIR := generated/interfaces

.PHONY: build test check fmt clean

build:
	stellar contract build

test:
	cargo test --workspace

check:
	cargo check --workspace

lint:
	cargo fmt --all

clean:
	cargo clean