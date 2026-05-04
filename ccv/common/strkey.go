package common

import (
	"fmt"

	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/stellar/go-stellar-sdk/strkey"
)

// ToUnknownAddress decodes a Stellar account (G…) or contract (C…) strkey into
// protocol.UnknownAddress (raw 32-byte contract ID or Ed25519 public key), matching
// bindings/scval.RawBytesFromAddressScVal for Soroban Address values.
func ToUnknownAddress(addr string) (protocol.UnknownAddress, error) {
	if len(addr) == 0 {
		return nil, fmt.Errorf("empty stellar address")
	}
	switch addr[0] {
	case 'C':
		raw, err := strkey.Decode(strkey.VersionByteContract, addr)
		if err != nil {
			return nil, fmt.Errorf("decode contract strkey: %w", err)
		}
		return protocol.UnknownAddress(raw), nil
	case 'G':
		raw, err := strkey.Decode(strkey.VersionByteAccountID, addr)
		if err != nil {
			return nil, fmt.Errorf("decode account strkey: %w", err)
		}
		return protocol.UnknownAddress(raw), nil
	default:
		return nil, fmt.Errorf("unsupported stellar strkey prefix %q (want G or C)", addr[:1])
	}
}
