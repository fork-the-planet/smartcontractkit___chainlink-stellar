package relayer

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"

	stellartypes "github.com/smartcontractkit/chainlink-common/pkg/types/chains/stellar"

	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
)

// scValToXDR converts the generic domain ScVal into a Stellar SDK
// xdr.ScVal so it can be passed as a Soroban contract-call argument.
func scValToXDR(v stellartypes.ScVal) (xdr.ScVal, error) {
	switch v.Type {
	case stellartypes.ScValTypeBool:
		if v.Bool == nil {
			return xdr.ScVal{}, errNilScValField("Bool")
		}
		b := *v.Bool
		return xdr.ScVal{Type: xdr.ScValTypeScvBool, B: &b}, nil

	case stellartypes.ScValTypeVoid:
		return xdr.ScVal{Type: xdr.ScValTypeScvVoid}, nil

	case stellartypes.ScValTypeU32:
		if v.U32 == nil {
			return xdr.ScVal{}, errNilScValField("U32")
		}
		u := xdr.Uint32(*v.U32)
		return xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: &u}, nil

	case stellartypes.ScValTypeI32:
		if v.I32 == nil {
			return xdr.ScVal{}, errNilScValField("I32")
		}
		i := xdr.Int32(*v.I32)
		return xdr.ScVal{Type: xdr.ScValTypeScvI32, I32: &i}, nil

	case stellartypes.ScValTypeU64:
		if v.U64 == nil {
			return xdr.ScVal{}, errNilScValField("U64")
		}
		u := xdr.Uint64(*v.U64)
		return xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &u}, nil

	case stellartypes.ScValTypeI64:
		if v.I64 == nil {
			return xdr.ScVal{}, errNilScValField("I64")
		}
		i := xdr.Int64(*v.I64)
		return xdr.ScVal{Type: xdr.ScValTypeScvI64, I64: &i}, nil

	case stellartypes.ScValTypeTimepoint:
		if v.Timepoint == nil {
			return xdr.ScVal{}, errNilScValField("Timepoint")
		}
		t := xdr.TimePoint(*v.Timepoint)
		return xdr.ScVal{Type: xdr.ScValTypeScvTimepoint, Timepoint: &t}, nil

	case stellartypes.ScValTypeDuration:
		if v.Duration == nil {
			return xdr.ScVal{}, errNilScValField("Duration")
		}
		d := xdr.Duration(*v.Duration)
		return xdr.ScVal{Type: xdr.ScValTypeScvDuration, Duration: &d}, nil

	case stellartypes.ScValTypeU128:
		if v.U128 == nil {
			return xdr.ScVal{}, errNilScValField("U128")
		}
		p := xdr.UInt128Parts{Hi: xdr.Uint64(v.U128.Hi), Lo: xdr.Uint64(v.U128.Lo)}
		return xdr.ScVal{Type: xdr.ScValTypeScvU128, U128: &p}, nil

	case stellartypes.ScValTypeI128:
		if v.I128 == nil {
			return xdr.ScVal{}, errNilScValField("I128")
		}
		p := xdr.Int128Parts{Hi: xdr.Int64(v.I128.Hi), Lo: xdr.Uint64(v.I128.Lo)}
		return xdr.ScVal{Type: xdr.ScValTypeScvI128, I128: &p}, nil

	case stellartypes.ScValTypeU256:
		if v.U256 == nil {
			return xdr.ScVal{}, errNilScValField("U256")
		}
		p := xdr.UInt256Parts{
			HiHi: xdr.Uint64(v.U256.HiHi),
			HiLo: xdr.Uint64(v.U256.HiLo),
			LoHi: xdr.Uint64(v.U256.LoHi),
			LoLo: xdr.Uint64(v.U256.LoLo),
		}
		return xdr.ScVal{Type: xdr.ScValTypeScvU256, U256: &p}, nil

	case stellartypes.ScValTypeI256:
		if v.I256 == nil {
			return xdr.ScVal{}, errNilScValField("I256")
		}
		p := xdr.Int256Parts{
			HiHi: xdr.Int64(v.I256.HiHi),
			HiLo: xdr.Uint64(v.I256.HiLo),
			LoHi: xdr.Uint64(v.I256.LoHi),
			LoLo: xdr.Uint64(v.I256.LoLo),
		}
		return xdr.ScVal{Type: xdr.ScValTypeScvI256, I256: &p}, nil

	case stellartypes.ScValTypeBytes:
		b := xdr.ScBytes(v.Bytes)
		return xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &b}, nil

	case stellartypes.ScValTypeString:
		if v.String == nil {
			return xdr.ScVal{}, errNilScValField("String")
		}
		s := xdr.ScString(*v.String)
		return xdr.ScVal{Type: xdr.ScValTypeScvString, Str: &s}, nil

	case stellartypes.ScValTypeSymbol:
		if v.Symbol == nil {
			return xdr.ScVal{}, errNilScValField("Symbol")
		}
		s := xdr.ScSymbol(*v.Symbol)
		return xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &s}, nil

	case stellartypes.ScValTypeVec:
		if v.Vec == nil {
			return xdr.ScVal{}, errNilScValField("Vec")
		}
		items := make([]xdr.ScVal, len(v.Vec.Values))
		for i, e := range v.Vec.Values {
			if e == nil {
				return xdr.ScVal{}, fmt.Errorf("vec element %d is nil", i)
			}
			c, err := scValToXDR(*e)
			if err != nil {
				return xdr.ScVal{}, fmt.Errorf("vec element %d: %w", i, err)
			}
			items[i] = c
		}
		vec := xdr.ScVec(items)
		vecPtr := &vec
		return xdr.ScVal{Type: xdr.ScValTypeScvVec, Vec: &vecPtr}, nil

	case stellartypes.ScValTypeMap:
		if v.Map == nil {
			return xdr.ScVal{}, errNilScValField("Map")
		}
		entries := make([]xdr.ScMapEntry, len(v.Map.Entries))
		for i, e := range v.Map.Entries {
			if e.Key == nil || e.Val == nil {
				return xdr.ScVal{}, fmt.Errorf("map entry %d has nil key or val", i)
			}
			k, err := scValToXDR(*e.Key)
			if err != nil {
				return xdr.ScVal{}, fmt.Errorf("map entry %d key: %w", i, err)
			}
			val, err := scValToXDR(*e.Val)
			if err != nil {
				return xdr.ScVal{}, fmt.Errorf("map entry %d val: %w", i, err)
			}
			entries[i] = xdr.ScMapEntry{Key: k, Val: val}
		}
		m := xdr.ScMap(entries)
		mapPtr := &m
		return xdr.ScVal{Type: xdr.ScValTypeScvMap, Map: &mapPtr}, nil

	case stellartypes.ScValTypeAddress:
		if v.Address == nil {
			return xdr.ScVal{}, errNilScValField("Address")
		}
		addr, err := scAddressToXDR(*v.Address)
		if err != nil {
			return xdr.ScVal{}, err
		}
		return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &addr}, nil

	default:
		return xdr.ScVal{}, fmt.Errorf("unsupported ScVal type %d as contract argument", v.Type)
	}
}

// scAddressToXDR converts a domain ScAddress (account or contract) into an
// xdr.ScAddress. Muxed/claimable-balance/liquidity-pool addresses are not valid
// Soroban argument addresses and are rejected.
func scAddressToXDR(a stellartypes.ScAddress) (xdr.ScAddress, error) {
	switch a.Type {
	case stellartypes.ScAddressTypeAccountID:
		if len(a.AccountID) != 32 {
			return xdr.ScAddress{}, fmt.Errorf("account id must be 32 bytes, got %d", len(a.AccountID))
		}
		gAddr, err := strkey.Encode(strkey.VersionByteAccountID, a.AccountID)
		if err != nil {
			return xdr.ScAddress{}, fmt.Errorf("encode account id: %w", err)
		}
		accountID, err := xdr.AddressToAccountId(gAddr)
		if err != nil {
			return xdr.ScAddress{}, fmt.Errorf("build account address: %w", err)
		}
		return xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeAccount, AccountId: &accountID}, nil

	case stellartypes.ScAddressTypeContractID:
		if len(a.ContractID) != 32 {
			return xdr.ScAddress{}, fmt.Errorf("contract id must be 32 bytes, got %d", len(a.ContractID))
		}
		addr := scval.BuildContractScAddress(a.ContractID)
		if addr == nil {
			return xdr.ScAddress{}, fmt.Errorf("failed to build contract address")
		}
		return *addr, nil

	default:
		return xdr.ScAddress{}, fmt.Errorf("unsupported ScAddress type %d as contract argument", a.Type)
	}
}

func errNilScValField(name string) error {
	return fmt.Errorf("scVal of declared type has nil %s field", name)
}
