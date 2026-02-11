package ccvcodec

import (
	"fmt"

	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/onramp"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Type aliases -- the bindings types are the source of truth for contract types.
// These aliases preserve backward compatibility for consumers that reference the codec types.
type (
	OnRampStaticConfig  = onrampbindings.StaticConfig
	OnRampDynamicConfig = onrampbindings.DynamicConfig
	DestChainConfig     = onrampbindings.DestChainConfig
	DestChainConfigArgs = onrampbindings.DestChainConfigArgs
	StellarToAnyMessage = onrampbindings.StellarToAnyMessage
	TokenAmount         = onrampbindings.TokenAmount
)

// MessageSentResult contains the result of sending a CCIP message.
type MessageSentResult struct {
	MessageID      [32]byte
	SequenceNumber uint64
	Ledger         uint32
	TxHash         string
}

// CCIPMessageSentEvent represents the event emitted by the OnRamp when a message is sent.
type CCIPMessageSentEvent struct {
	DestChainSelector     uint64
	SequenceNumber        uint64
	Sender                string
	MessageID             [32]byte
	FeeToken              string
	TokenAmountBeforeFees int64
	EncodedMessage        []byte
	Ledger                uint32
	TxHash                string
}

// ToScVal converts CCIPMessageSentEvent to an xdr.ScVal for contract calls.
func (c *CCIPMessageSentEvent) ToScVal() (xdr.ScVal, error) {
	scVal, err := buildStructScVal(map[string]xdr.ScVal{
		"dest_chain_selector":      uint64ToScVal(c.DestChainSelector),
		"sequence_number":          uint64ToScVal(c.SequenceNumber),
		"sender":                   addressToScVal(c.Sender),
		"message_id":               bytesToScVal(c.MessageID[:]),
		"fee_token":                addressToScVal(c.FeeToken),
		"token_amount_before_fees": i128ToScVal(c.TokenAmountBeforeFees),
		"encoded_message":          bytesToScVal(c.EncodedMessage),
		"ledger":                   uint32ToScVal(c.Ledger),
		"tx_hash":                  bytesToScVal([]byte(c.TxHash)),
	})

	if err != nil {
		return xdr.ScVal{}, err
	}

	return scVal, nil
}

// -----------------------------------------------------------------------
// XDR helper functions used by codec-specific types (CCIPMessageSentEvent)
// -----------------------------------------------------------------------

func uint64ToScVal(v uint64) xdr.ScVal {
	xdrU64 := xdr.Uint64(v)
	return xdr.ScVal{
		Type: xdr.ScValTypeScvU64,
		U64:  &xdrU64,
	}
}

func uint32ToScVal(v uint32) xdr.ScVal {
	xdrU32 := xdr.Uint32(v)
	return xdr.ScVal{
		Type: xdr.ScValTypeScvU32,
		U32:  &xdrU32,
	}
}

func i128ToScVal(v int64) xdr.ScVal {
	var hi int64
	if v < 0 {
		hi = -1 // Sign extend for negative numbers
	}
	lo := uint64(v)
	parts := xdr.Int128Parts{
		Hi: xdr.Int64(hi),
		Lo: xdr.Uint64(lo),
	}
	return xdr.ScVal{
		Type: xdr.ScValTypeScvI128,
		I128: &parts,
	}
}

func bytesToScVal(b []byte) xdr.ScVal {
	bytes := xdr.ScBytes(b)
	return xdr.ScVal{
		Type:  xdr.ScValTypeScvBytes,
		Bytes: &bytes,
	}
}

func addressToScVal(addr string) xdr.ScVal {
	scAddr := parseAddress(addr)
	return xdr.ScVal{
		Type:    xdr.ScValTypeScvAddress,
		Address: scAddr,
	}
}

func vecToScVal(items []xdr.ScVal) xdr.ScVal {
	scVec := xdr.ScVec(items)
	// ScVal.Vec field is **ScVec, so we need to allocate and take address
	vecPtr := &scVec
	return xdr.ScVal{
		Type: xdr.ScValTypeScvVec,
		Vec:  &vecPtr,
	}
}

func buildStructScVal(fields map[string]xdr.ScVal) (xdr.ScVal, error) {
	entries := make([]xdr.ScMapEntry, 0, len(fields))
	for k, v := range fields {
		sym := xdr.ScSymbol(k)
		entries = append(entries, xdr.ScMapEntry{
			Key: xdr.ScVal{
				Type: xdr.ScValTypeScvSymbol,
				Sym:  &sym,
			},
			Val: v,
		})
	}
	scMap := xdr.ScMap(entries)
	// ScVal.Map field is **ScMap, so we need to allocate and take address
	mapPtr := &scMap
	return xdr.ScVal{
		Type: xdr.ScValTypeScvMap,
		Map:  &mapPtr,
	}, nil
}

// parseAddress parses a Stellar address string (G... or C...) into an xdr.ScAddress.
func parseAddress(addr string) *xdr.ScAddress {
	if len(addr) == 0 {
		return nil
	}

	// Handle contract addresses (C...)
	if addr[0] == 'C' {
		decoded, err := strkey.Decode(strkey.VersionByteContract, addr)
		if err != nil {
			return nil
		}
		return buildContractScAddress(decoded)
	}

	// Handle account addresses (G...)
	if addr[0] == 'G' {
		decoded, err := strkey.Decode(strkey.VersionByteAccountID, addr)
		if err != nil {
			return nil
		}
		var pubKey xdr.Uint256
		copy(pubKey[:], decoded)
		accountID := xdr.AccountId{
			Type:    xdr.PublicKeyTypePublicKeyTypeEd25519,
			Ed25519: &pubKey,
		}
		return &xdr.ScAddress{
			Type:      xdr.ScAddressTypeScAddressTypeAccount,
			AccountId: &accountID,
		}
	}

	return nil
}

// addressFromScVal extracts a strkey address from an xdr.ScVal.
func addressFromScVal(val xdr.ScVal) (string, error) {
	addr, ok := val.GetAddress()
	if !ok {
		return "", fmt.Errorf("not an address type: %v", val.Type)
	}

	switch addr.Type {
	case xdr.ScAddressTypeScAddressTypeAccount:
		accountID := addr.MustAccountId()
		pubKey := accountID.Ed25519
		if pubKey == nil {
			return "", fmt.Errorf("account ID has no Ed25519 key")
		}
		return strkey.Encode(strkey.VersionByteAccountID, (*pubKey)[:])
	case xdr.ScAddressTypeScAddressTypeContract:
		contractID := addr.MustContractId()
		return strkey.Encode(strkey.VersionByteContract, contractID[:])
	default:
		return "", fmt.Errorf("unsupported address type: %s", addr.Type)
	}
}

// uint64FromScVal extracts a uint64 from an xdr.ScVal.
func uint64FromScVal(val xdr.ScVal) (uint64, error) {
	u64, ok := val.GetU64()
	if !ok {
		return 0, fmt.Errorf("not a u64 type: %v", val.Type)
	}
	return uint64(u64), nil
}

// bytes32FromScVal extracts a [32]byte from an xdr.ScVal containing BytesN<32>.
func bytes32FromScVal(val xdr.ScVal) ([32]byte, error) {
	bytes, ok := val.GetBytes()
	if !ok {
		return [32]byte{}, fmt.Errorf("not a bytes type: %v", val.Type)
	}
	if len(bytes) != 32 {
		return [32]byte{}, fmt.Errorf("expected 32 bytes, got %d", len(bytes))
	}
	var result [32]byte
	copy(result[:], bytes)
	return result, nil
}

// i128FromScVal extracts an int64 from an xdr.ScVal containing i128.
// Note: This truncates to int64 for simplicity.
func i128FromScVal(val xdr.ScVal) (int64, error) {
	i128, ok := val.GetI128()
	if !ok {
		return 0, fmt.Errorf("not an i128 type: %v", val.Type)
	}
	// For simplicity, assume the value fits in int64
	return int64(i128.Lo), nil
}

// buildContractScAddress creates an ScAddress for a contract from raw bytes.
// Uses XDR marshaling to properly construct the address with correct types.
func buildContractScAddress(contractIDBytes []byte) *xdr.ScAddress {
	if len(contractIDBytes) != 32 {
		return nil
	}
	// Construct via XDR encoding to handle SDK type requirements
	// ScAddress union: type (4 bytes) + data (32 bytes for contract)
	xdrBytes := make([]byte, 0, 36)
	// Type discriminant for contract: ScAddressTypeScAddressTypeContract = 1
	xdrBytes = append(xdrBytes, 0, 0, 0, 1) // Big-endian uint32
	xdrBytes = append(xdrBytes, contractIDBytes...)

	var addr xdr.ScAddress
	if err := addr.UnmarshalBinary(xdrBytes); err != nil {
		return nil
	}
	return &addr
}
