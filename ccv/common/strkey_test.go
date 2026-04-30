package common

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/strkey"
)

func TestToUnknownAddress_Contract(t *testing.T) {
	contractID := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	c, err := strkey.Encode(strkey.VersionByteContract, contractID[:])
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := ToUnknownAddress(c)
	if err != nil {
		t.Fatalf("ToUnknownAddress: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("len got %d want 32", len(got))
	}
	for i := range contractID {
		if got[i] != contractID[i] {
			t.Fatalf("byte %d: got %d want %d", i, got[i], contractID[i])
		}
	}
}

func TestToUnknownAddress_Account(t *testing.T) {
	pubKeyBytes := [32]byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160,
		170, 180, 190, 200, 210, 220, 230, 240, 250, 1, 2, 3, 4, 5, 6, 7}
	g, err := strkey.Encode(strkey.VersionByteAccountID, pubKeyBytes[:])
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := ToUnknownAddress(g)
	if err != nil {
		t.Fatalf("ToUnknownAddress: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("len got %d want 32", len(got))
	}
	for i := range pubKeyBytes {
		if got[i] != pubKeyBytes[i] {
			t.Fatalf("byte %d: got %d want %d", i, got[i], pubKeyBytes[i])
		}
	}
}

func TestToUnknownAddress_Invalid(t *testing.T) {
	if _, err := ToUnknownAddress(""); err == nil {
		t.Fatal("expected error for empty")
	}
	if _, err := ToUnknownAddress("XBAD"); err == nil {
		t.Fatal("expected error for bad prefix")
	}
}
