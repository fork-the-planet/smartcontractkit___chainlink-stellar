# Chainlink CCIP Stellar — Top-level Makefile
# ============================================

WASM_DIR := target/wasm32v1-none/release
GENERATED_DIR := generated

# Contracts that expose cross-contract interfaces consumed by other contracts.
# Add new entries here when a contract needs to be called cross-contract.
INTERFACE_CONTRACTS := onramp rmn_proxy

.PHONY: build test check fmt clean generate-interfaces

# ─── Build ────────────────────────────────────────────────────
build:
	stellar contract build

# ─── Test ─────────────────────────────────────────────────────
test:
	cargo test --workspace

# ─── Check ────────────────────────────────────────────────────
check:
	cargo check --workspace

# ─── Format ───────────────────────────────────────────────────
fmt:
	cargo fmt --all

# ─── Clean ────────────────────────────────────────────────────
clean:
	cargo clean
	rm -rf $(GENERATED_DIR)

# ─── Generate Interfaces ─────────────────────────────────────
# Builds the specified contracts to WASM, then uses the Stellar CLI
# to generate Rust bindings from the compiled WASM. The output in
# generated/<contract>/ can be used to verify or update the interface
# traits in contracts/common/interfaces/src/.
#
# Usage:
#   make generate-interfaces
#
generate-interfaces: $(addprefix _gen-interface-,$(INTERFACE_CONTRACTS))
	@echo ""
	@echo "Generated reference bindings in $(GENERATED_DIR)/."
	@echo "Compare with contracts/common/interfaces/src/ and update traits if needed."

_gen-interface-%:
	@echo "──── Building $* ────"
	stellar contract build --package $*
	@echo "──── Generating Rust bindings for $* ────"
	@mkdir -p $(GENERATED_DIR)/$*
	stellar contract bindings rust \
		--wasm $(WASM_DIR)/$*.wasm \
		--output-dir $(GENERATED_DIR)/$*
	@echo "  → $(GENERATED_DIR)/$*/"
