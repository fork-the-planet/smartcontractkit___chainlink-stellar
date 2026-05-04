package main

import (
	"strings"
	"testing"
)

// TestGenerateEnum_UnitOnly is a regression guard: pre-existing unit-only
// enums (CCIPError, MessageDirection, ...) must keep emitting the legacy
// `type X uint32` newtype shape so existing call sites continue to compile.
func TestGenerateEnum_UnitOnly(t *testing.T) {
	c := &Contract{Enums: []Enum{
		{Name: "MessageDirection", Variants: []EnumVariant{
			{Name: "Outbound", Kind: EnumVariantUnit, Value: 0},
			{Name: "Inbound", Kind: EnumVariantUnit, Value: 1},
		}},
	}}
	out := GenerateTypes("test", c)
	mustContain(t, out,
		"type MessageDirection uint32",
		"MessageDirectionOutbound MessageDirection = 0",
		"MessageDirectionInbound MessageDirection = 1",
		"return scval.Uint32ToScVal(uint32(e)), nil",
	)
	mustNotContain(t, out, "type MessageDirection struct")
}

// TestGenerateEnum_TupleEmitsUnion is the core regression test for the
// reviewer's report: an enum with a tuple variant must emit a discriminated
// union, must use ScVec(Symbol+payloads), and must NOT use Uint32ToScVal.
func TestGenerateEnum_TupleEmitsUnion(t *testing.T) {
	c := &Contract{Enums: []Enum{
		{Name: "McmsDataKey", Variants: []EnumVariant{
			{Name: "SeenHash", Kind: EnumVariantTuple, Payload: []Field{
				{Type: "soroban_sdk::BytesN<32>"},
			}},
		}},
	}}
	out := GenerateTypes("test", c)

	mustContain(t, out,
		"type McmsDataKey struct {",
		"SeenHash *McmsDataKeySeenHash",
		"type McmsDataKeySeenHash struct {",
		"Field0 [32]byte",
		// ToScVal: discriminant symbol + Bytes32 payload, returned as a vec.
		"scval.SymbolToScVal(\"SeenHash\")",
		"scval.Bytes32ToScVal(e.SeenHash.Field0)",
		"scval.VecToScVal(items)",
		// FromScVal: parse vec, dispatch on tag symbol, decode payload.
		"vecPtr, ok := val.GetVec()",
		"tag, err := scval.SymbolFromScVal(vec[0])",
		"case \"SeenHash\":",
		"scval.Bytes32FromScVal(vec[1])",
	)
	// Critical: the broken behaviour must be gone.
	mustNotContain(t, out,
		"type McmsDataKey uint32",
		"return scval.Uint32ToScVal(uint32(e)), nil",
	)
}

// TestGenerateEnum_MixedUnitAndTuple covers PoolDataKey: the union path must
// handle unit variants alongside tuple variants without losing the variant.
func TestGenerateEnum_MixedUnitAndTuple(t *testing.T) {
	c := &Contract{Enums: []Enum{
		{Name: "PoolDataKey", Variants: []EnumVariant{
			{Name: "Token", Kind: EnumVariantUnit},
			{Name: "RemoteChainConfig", Kind: EnumVariantTuple, Payload: []Field{{Type: "u64"}}},
			{Name: "SupportedChains", Kind: EnumVariantUnit},
		}},
	}}
	out := GenerateTypes("test", c)

	mustContain(t, out,
		"type PoolDataKey struct {",
		"Token *PoolDataKeyToken",
		"RemoteChainConfig *PoolDataKeyRemoteChainConfig",
		"SupportedChains *PoolDataKeySupportedChains",
		"type PoolDataKeyToken struct{}",
		"type PoolDataKeyRemoteChainConfig struct {",
		"Field0 uint64",
		"type PoolDataKeySupportedChains struct{}",
		// Each variant's ToScVal branch should emit a vec with the right tag.
		"scval.SymbolToScVal(\"Token\")",
		"scval.SymbolToScVal(\"RemoteChainConfig\")",
		"scval.SymbolToScVal(\"SupportedChains\")",
		// FromScVal must dispatch all three variants.
		"case \"Token\":",
		"case \"RemoteChainConfig\":",
		"case \"SupportedChains\":",
		// Payload-bearing variant must check the right element count.
		"PoolDataKey::RemoteChainConfig: expected 2 elements",
	)
	// Unit variants must accept exactly 1 element (just the symbol).
	mustContain(t, out,
		"PoolDataKey::Token: expected 1 elements",
		"PoolDataKey::SupportedChains: expected 1 elements",
	)
}

// TestGenerateEnum_StructVariant covers the struct-variant payload shape.
// We exercise it via codegen even though no current contract enum uses it,
// because the parser supports it and we want symmetric encode/decode.
func TestGenerateEnum_StructVariant(t *testing.T) {
	c := &Contract{Enums: []Enum{
		{Name: "Op", Variants: []EnumVariant{
			{Name: "Mint", Kind: EnumVariantStruct, Payload: []Field{
				{Name: "to", Type: "soroban_sdk::Address"},
				{Name: "amount", Type: "i128"},
			}},
		}},
	}}
	out := GenerateTypes("test", c)

	mustContain(t, out,
		"type Op struct {",
		"Mint *OpMint",
		"type OpMint struct {",
		"To string",
		"Amount int64",
		// Struct-variant fields are passed positionally in the same order
		// they appear in Rust, after the discriminant symbol.
		"scval.AddressToScVal(e.Mint.To)",
		"scval.I128ToScVal(e.Mint.Amount)",
	)
}

// TestGenerateEnum_ZeroValue makes sure tuple/return-position uses pick the
// correct Go zero literal: `0` for unit-only, `T{}` for unions. Without
// this, a tuple-returning function whose tuple contains a discriminated
// union would emit `return 0, ...` and fail to compile.
func TestGenerateEnum_ZeroValue(t *testing.T) {
	knownEnumNames = map[string]bool{
		"MessageDirection": true,  // unit-only
		"McmsDataKey":      false, // union
	}
	if got := zeroValue("MessageDirection"); got != "0" {
		t.Errorf("unit enum zero: got %q want \"0\"", got)
	}
	if got := zeroValue("McmsDataKey"); got != "McmsDataKey{}" {
		t.Errorf("union enum zero: got %q want \"McmsDataKey{}\"", got)
	}
}

// helpers

func mustContain(t *testing.T, s string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(s, n) {
			t.Errorf("output missing required snippet:\n  %q", n)
		}
	}
}

func mustNotContain(t *testing.T, s string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(s, n) {
			t.Errorf("output unexpectedly contains:\n  %q", n)
		}
	}
}
