package stellarutil

import (
	"crypto/sha256"
	"fmt"

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
