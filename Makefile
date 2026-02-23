WASM_DIR := target/wasm32v1-none/release

.PHONY: build test check fmt clean generate-interfaces

build:
	stellar contract build

generate-interfaces:
	./scripts/gen_interfaces.sh

test:
	cargo test --workspace

check:
	cargo check --workspace

lint:
	cargo fmt --all

clean:
	cargo clean
