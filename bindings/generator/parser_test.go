package main

import (
	"reflect"
	"testing"
)

// TestParseEnums_PureUnit covers the historical (pre-fix) C-style enum case
// that we must keep emitting as a Go `uint32` newtype.
//
// It also pins the Rust auto-numbering rule: bare unit variants get
// sequential values starting at 0 in declaration order, matching the
// on-chain ScVal::U32 Soroban emits. The previous implementation left
// every bare-variant value at 0, which collapsed Outbound and Inbound to
// the same wire value.
func TestParseEnums_PureUnit(t *testing.T) {
	src := `
#[soroban_sdk::contracttype]
#[derive(Debug, Clone)]
pub enum MessageDirection {
    Outbound,
    Inbound,
}
`
	enums := parseEnums(src)
	if len(enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(enums))
	}
	got := enums[0]
	if got.Name != "MessageDirection" {
		t.Fatalf("name: %q", got.Name)
	}
	if !got.IsUnit() {
		t.Fatalf("expected IsUnit=true")
	}
	want := []EnumVariant{
		{Name: "Outbound", Kind: EnumVariantUnit, Value: 0},
		{Name: "Inbound", Kind: EnumVariantUnit, Value: 1},
	}
	if !reflect.DeepEqual(got.Variants, want) {
		t.Fatalf("variants: got %+v want %+v", got.Variants, want)
	}
}

// TestParseEnums_ImplicitDiscriminants covers the full Rust auto-numbering
// rule including discriminant resets:
//   - bare variants get +1 from the previous variant
//   - an explicit `= N` resets the counter so the next bare gets `N+1`
//
// This is the exact behaviour Soroban's #[contracttype] derive uses for
// the on-chain wire value, so anything that diverges from this leaks
// wrong discriminants into Go bindings.
func TestParseEnums_ImplicitDiscriminants(t *testing.T) {
	src := `
#[soroban_sdk::contracttype]
pub enum E {
    A,
    B,
    C = 10,
    D,
    Reset = 0,
    F,
}
`
	enums := parseEnums(src)
	if len(enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(enums))
	}
	want := []EnumVariant{
		{Name: "A", Kind: EnumVariantUnit, Value: 0},
		{Name: "B", Kind: EnumVariantUnit, Value: 1},
		{Name: "C", Kind: EnumVariantUnit, Value: 10},
		{Name: "D", Kind: EnumVariantUnit, Value: 11},
		{Name: "Reset", Kind: EnumVariantUnit, Value: 0},
		{Name: "F", Kind: EnumVariantUnit, Value: 1},
	}
	if !reflect.DeepEqual(enums[0].Variants, want) {
		t.Fatalf("variants:\n  got %+v\n  want %+v", enums[0].Variants, want)
	}
}

// TestParseEnums_Tuple covers the regression that motivated this fix:
// McmsDataKey carries a BytesN<32> payload and must be detected as
// non-unit so codegen emits the discriminated-union shape.
func TestParseEnums_Tuple(t *testing.T) {
	src := `
#[soroban_sdk::contracttype]
#[derive(Debug, Clone)]
pub enum McmsDataKey {
    SeenHash(soroban_sdk::BytesN<32>),
}
`
	enums := parseEnums(src)
	if len(enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(enums))
	}
	got := enums[0]
	if got.IsUnit() {
		t.Fatalf("expected IsUnit=false for tuple-variant enum")
	}
	if len(got.Variants) != 1 {
		t.Fatalf("variants: got %d", len(got.Variants))
	}
	v := got.Variants[0]
	if v.Name != "SeenHash" || v.Kind != EnumVariantTuple {
		t.Fatalf("variant header: %+v", v)
	}
	if len(v.Payload) != 1 || v.Payload[0].Type != "soroban_sdk::BytesN<32>" {
		t.Fatalf("payload: %+v", v.Payload)
	}
	if v.Payload[0].Name != "" {
		t.Fatalf("expected anonymous tuple field, got name=%q", v.Payload[0].Name)
	}
}

// TestParseEnums_Mixed exercises the real-world PoolDataKey shape:
// unit and tuple variants in one enum. Bare-identifier and discriminant-less
// unit variants must coexist with parameterised ones.
func TestParseEnums_Mixed(t *testing.T) {
	src := `
#[soroban_sdk::contracttype]
pub enum PoolDataKey {
    Token,
    RemoteChainConfig(u64),
    SupportedChains,
    OutboundRateLimit(u64),
}
`
	enums := parseEnums(src)
	if len(enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(enums))
	}
	got := enums[0]
	if got.IsUnit() {
		t.Fatalf("expected IsUnit=false for mixed enum")
	}
	// Discriminants for unit variants follow Rust's auto-numbering even when
	// interleaved with tuple variants (the non-unit codegen path doesn't
	// consult Value, but the parser pins this for consistency and so that
	// IsUnit-derived behaviour stays predictable if the shape ever changes).
	want := []EnumVariant{
		{Name: "Token", Kind: EnumVariantUnit, Value: 0},
		{Name: "RemoteChainConfig", Kind: EnumVariantTuple, Payload: []Field{{Type: "u64"}}},
		{Name: "SupportedChains", Kind: EnumVariantUnit, Value: 1},
		{Name: "OutboundRateLimit", Kind: EnumVariantTuple, Payload: []Field{{Type: "u64"}}},
	}
	if !reflect.DeepEqual(got.Variants, want) {
		t.Fatalf("variants: got %+v want %+v", got.Variants, want)
	}
}

// TestParseEnums_Struct covers struct-variant payloads. These are valid in
// #[contracttype] enums and currently produced by no project enums, but the
// parser must not silently mis-classify them as unit when they appear.
func TestParseEnums_Struct(t *testing.T) {
	src := `
#[soroban_sdk::contracttype]
pub enum Op {
    Mint { to: soroban_sdk::Address, amount: i128 },
    Burn,
}
`
	enums := parseEnums(src)
	if len(enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(enums))
	}
	got := enums[0]
	if got.IsUnit() {
		t.Fatalf("expected IsUnit=false for struct-variant enum")
	}
	if len(got.Variants) != 2 {
		t.Fatalf("variants: got %d", len(got.Variants))
	}
	mint := got.Variants[0]
	if mint.Name != "Mint" || mint.Kind != EnumVariantStruct {
		t.Fatalf("Mint header: %+v", mint)
	}
	wantPayload := []Field{
		{Name: "to", Type: "soroban_sdk::Address"},
		{Name: "amount", Type: "i128"},
	}
	if !reflect.DeepEqual(mint.Payload, wantPayload) {
		t.Fatalf("Mint payload: got %+v want %+v", mint.Payload, wantPayload)
	}
	burn := got.Variants[1]
	if burn.Name != "Burn" || burn.Kind != EnumVariantUnit || len(burn.Payload) != 0 {
		t.Fatalf("Burn: %+v", burn)
	}
}

// TestParseEnums_ExportFalseStillEmitted documents the intentional choice
// that `#[contracttype(export = false)]` does NOT cause us to skip Go
// binding generation. That attribute controls whether the type appears in
// the on-chain contract schema, which is orthogonal to whether Go callers
// need to encode it for invocations or for RestoreFootprintOp ledger keys.
//
// Several public ABI surfaces (MessageDirection on pools,
// MessageExecutionState on offramp) are tagged `export = false` and still
// appear as function parameters / return types. Skipping them would break
// every package that uses them.
func TestParseEnums_ExportFalseStillEmitted(t *testing.T) {
	src := `
#[soroban_sdk::contracttype(export = false)]
pub enum MessageDirection {
    Outbound,
    Inbound,
}

#[soroban_sdk::contracttype]
pub enum PublicKey {
    Bar(u32),
}
`
	enums := parseEnums(src)
	if len(enums) != 2 {
		t.Fatalf("expected both enums to be emitted, got %d", len(enums))
	}
	names := []string{enums[0].Name, enums[1].Name}
	if names[0] != "MessageDirection" || names[1] != "PublicKey" {
		t.Fatalf("unexpected enums: %v", names)
	}
}

// TestParseEnums_GenericCommas guards the top-level comma splitter:
// a payload like `Vec<u32, u64>` must not split mid-generic.
func TestParseEnums_GenericCommas(t *testing.T) {
	src := `
#[soroban_sdk::contracttype]
pub enum X {
    A(soroban_sdk::Map<u32, soroban_sdk::Address>),
    B,
}
`
	enums := parseEnums(src)
	if len(enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(enums))
	}
	v := enums[0].Variants[0]
	if v.Kind != EnumVariantTuple || len(v.Payload) != 1 {
		t.Fatalf("A should have one payload field: %+v", v)
	}
	if v.Payload[0].Type != "soroban_sdk::Map<u32, soroban_sdk::Address>" {
		t.Fatalf("payload type wrong: %q", v.Payload[0].Type)
	}
}

// TestSplitTopLevel directly exercises the splitter that all three parsers
// rely on. Failing here would silently corrupt every shape above.
func TestSplitTopLevel(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a, b, c", []string{"a", " b", " c"}},
		{"Vec<u32, u64>, X", []string{"Vec<u32, u64>", " X"}},
		{"Foo { a: T, b: T }, Bar", []string{"Foo { a: T, b: T }", " Bar"}},
		{"Foo(T1, T2), Bar", []string{"Foo(T1, T2)", " Bar"}},
	}
	for _, tc := range cases {
		got := splitTopLevel(tc.in, ',')
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitTopLevel(%q):\n  got  %#v\n  want %#v", tc.in, got, tc.want)
		}
	}
}
