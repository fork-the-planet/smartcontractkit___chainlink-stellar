package stellarutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stellar/go-stellar-sdk/strkey"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
)

// GenerateContractAddress returns a deterministic Soroban contract address seed (32 bytes)
// from a logical name and network passphrase.
func GenerateContractAddress(name, networkPassphrase string) []byte {
	networkID := sha256.Sum256([]byte(networkPassphrase))
	combined := append(networkID[:], []byte(name)...)
	hash := sha256.Sum256(combined)
	return hash[:]
}

// ParseFeeAggregatorAddress normalizes a fee-aggregator address for Stellar OnRamp, VVR, and CommitteeVerifier.
// It accepts a Stellar strkey (G… account or C… contract) or 0x-prefixed 32-byte hex (raw address bytes).
// Hex input is encoded as an account strkey (G…); for a contract fee sink pass the C… strkey explicitly.
func ParseFeeAggregatorAddress(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty fee aggregator address")
	}
	if len(s) > 0 && s[0] == 'G' {
		if _, err := strkey.Decode(strkey.VersionByteAccountID, s); err != nil {
			return "", fmt.Errorf("decode account strkey: %w", err)
		}
		return s, nil
	}
	if len(s) > 0 && s[0] == 'C' {
		if _, err := strkey.Decode(strkey.VersionByteContract, s); err != nil {
			return "", fmt.Errorf("decode contract strkey: %w", err)
		}
		return s, nil
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return "", fmt.Errorf("fee aggregator must be G/C strkey or 0x-hex: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("hex fee aggregator must be 32 bytes, got %d", len(raw))
	}
	out, err := strkey.Encode(strkey.VersionByteAccountID, raw)
	if err != nil {
		return "", fmt.Errorf("encode account strkey: %w", err)
	}
	return out, nil
}

// MustGenerateMockContractID returns a deterministic mock contract strkey (C…) for tests/devenv.
func MustGenerateMockContractID(deployerAddress, contractName string) string {
	salt := stellardeployment.GenerateDeterministicSalt(deployerAddress, contractName)
	encoded, err := strkey.Encode(strkey.VersionByteContract, salt[:])
	if err != nil {
		panic(fmt.Errorf("failed to encode mock contract ID: %w", err))
	}
	return encoded
}

// StrkeyToHex decodes a strkey address (C… contract or G… account) to a 0x-prefixed hex string.
func StrkeyToHex(addr string) (string, error) {
	var vb strkey.VersionByte
	switch {
	case len(addr) > 0 && addr[0] == 'C':
		vb = strkey.VersionByteContract
	case len(addr) > 0 && addr[0] == 'G':
		vb = strkey.VersionByteAccountID
	default:
		return "", fmt.Errorf("unsupported strkey prefix: %s", addr)
	}
	raw, err := strkey.Decode(vb, addr)
	if err != nil {
		return "", fmt.Errorf("decode strkey %s: %w", addr, err)
	}
	return hexutil.Encode(raw), nil
}
