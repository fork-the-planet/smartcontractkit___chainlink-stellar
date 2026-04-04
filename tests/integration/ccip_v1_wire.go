//go:build integration

package integration

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
)

// Soroban / contracts/common/message: MESSAGE_V1_VERSION.
const ccipMessageV1Version byte = 1

// ccipV1Wire holds fields for canonical CcipMessageV1 encoding (ToBytes layout).
type ccipV1Wire struct {
	SourceChainSelector uint64
	DestChainSelector   uint64
	SequenceNumber      uint64
	ExecutionGasLimit   uint32
	CcipReceiveGasLimit uint32
	Finality            uint32
	CcvExecutorHash     [32]byte
	OnRampAddress       []byte
	OffRampAddress      []byte
	Sender              []byte
	Receiver            []byte
	DestBlob            []byte
	TokenTransfer       []byte
	Data                []byte
}

func encodeCcipMessageV1(m ccipV1Wire) ([]byte, error) {
	if len(m.OnRampAddress) > 255 || len(m.OffRampAddress) > 255 || len(m.Sender) > 255 || len(m.Receiver) > 255 {
		return nil, fmt.Errorf("1-byte length field overflow for address field")
	}
	if len(m.DestBlob) > 65535 || len(m.TokenTransfer) > 65535 || len(m.Data) > 65535 {
		return nil, fmt.Errorf("2-byte length field overflow for blob field")
	}
	var b []byte
	b = append(b, ccipMessageV1Version)
	b = binary.BigEndian.AppendUint64(b, m.SourceChainSelector)
	b = binary.BigEndian.AppendUint64(b, m.DestChainSelector)
	b = binary.BigEndian.AppendUint64(b, m.SequenceNumber)
	b = binary.BigEndian.AppendUint32(b, m.ExecutionGasLimit)
	b = binary.BigEndian.AppendUint32(b, m.CcipReceiveGasLimit)
	b = binary.BigEndian.AppendUint32(b, m.Finality)
	b = append(b, m.CcvExecutorHash[:]...)
	b = append(b, byte(len(m.OnRampAddress)))
	b = append(b, m.OnRampAddress...)
	b = append(b, byte(len(m.OffRampAddress)))
	b = append(b, m.OffRampAddress...)
	b = append(b, byte(len(m.Sender)))
	b = append(b, m.Sender...)
	b = append(b, byte(len(m.Receiver)))
	b = append(b, m.Receiver...)
	b = binary.BigEndian.AppendUint16(b, uint16(len(m.DestBlob)))
	b = append(b, m.DestBlob...)
	b = binary.BigEndian.AppendUint16(b, uint16(len(m.TokenTransfer)))
	b = append(b, m.TokenTransfer...)
	b = binary.BigEndian.AppendUint16(b, uint16(len(m.Data)))
	b = append(b, m.Data...)
	return b, nil
}

func keccak256MessageID(encoded []byte) [32]byte {
	h := crypto.Keccak256(encoded)
	var out [32]byte
	copy(out[:], h)
	return out
}

// contractAddressScValSuffix32 returns the last 32 bytes of Soroban ScVal XDR for a contract
// strkey (C...), matching CcipMessageV1::address_raw_bytes / OffRamp execute's offramp check.
func contractAddressScValSuffix32(contractStrkey string) ([]byte, error) {
	v := scval.AddressToScVal(contractStrkey)
	buf, err := v.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if len(buf) < 32 {
		return nil, fmt.Errorf("ScVal XDR shorter than 32 bytes: %d", len(buf))
	}
	return buf[len(buf)-32:], nil
}
