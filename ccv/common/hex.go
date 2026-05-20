package common

import "encoding/hex"

// HexEncode returns b as a 0x-prefixed lowercase hex string.
func HexEncode(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}
