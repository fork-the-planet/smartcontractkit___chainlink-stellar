package mcmsutil

import (
	"fmt"

	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// EncodeSorobanMCMSInvokePayload builds StellarOp.data bytes: XDR for ScVal::Vec([Symbol(fn), ...args]),
// matching contracts/common/helpers/src/soroban_invoke.rs (decode_invoke_payload).
func EncodeSorobanMCMSInvokePayload(functionName string, argScVals []xdr.ScVal) ([]byte, error) {
	vec := make(xdr.ScVec, 0, 1+len(argScVals))
	vec = append(vec, scval.SymbolToScVal(functionName))
	vec = append(vec, argScVals...)
	inner := vec
	p := &inner
	sc := xdr.ScVal{
		Type: xdr.ScValTypeScvVec,
		Vec:  &p,
	}
	b, err := sc.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal soroban invoke payload %q: %w", functionName, err)
	}
	return b, nil
}
