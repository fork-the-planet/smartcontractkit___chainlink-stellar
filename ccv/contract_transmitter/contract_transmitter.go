package contracttransmitter

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/smartcontractkit/chainlink-ccv/executor"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-stellar/bindings"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
)

// DefaultGasLimitOverride is passed to every offramp execute call.
// Zero means "use the gas limit from the message itself".
const DefaultGasLimitOverride uint32 = 0

var _ chainaccess.ContractTransmitter = (*ContractTransmitter)(nil)

// ContractTransmitter transmits aggregated reports to the Stellar OffRamp
// contract by calling its `execute` entry point via a Soroban invoker.
type ContractTransmitter struct {
	invoker           bindings.Invoker
	offRampContractID string
	lggr              *zerolog.Logger
}

// NewContractTransmitter creates a Stellar ContractTransmitter.
func NewContractTransmitter(
	invoker bindings.Invoker,
	offRampContractID string,
	lggr *zerolog.Logger,
) (*ContractTransmitter, error) {
	if invoker == nil {
		return nil, fmt.Errorf("invoker is required")
	}
	if offRampContractID == "" {
		return nil, fmt.Errorf("offramp contract ID is required")
	}
	if lggr == nil {
		return nil, fmt.Errorf("logger is required")
	}

	return &ContractTransmitter{
		invoker:           invoker,
		offRampContractID: offRampContractID,
		lggr:              lggr,
	}, nil
}

// ConvertAndWriteMessageToChain encodes the report into ScVal arguments and
// invokes OffRamp.execute on Stellar.
//
// Stellar OffRamp.execute signature (Rust):
//
//	execute(env, encoded_message: Bytes, ccvs: Vec<Address>,
//	        verifier_results: Vec<Bytes>, gas_limit_override: u32)
func (ct *ContractTransmitter) ConvertAndWriteMessageToChain(ctx context.Context, report protocol.AbstractAggregatedReport) error {
	messageID := report.Message.MustMessageID()

	encodedMsg, err := report.Message.Encode()
	if err != nil {
		ct.lggr.Error().Err(err).
			Str("messageID", messageID.String()).
			Msg("Unable to submit txn: invalid message encoding")
		return errors.Join(executor.ErrMessageEncoding,
			fmt.Errorf("unable to submit txn: invalid message encoding: %w", err))
	}

	ccvScVals, err := unknownAddressesToStellarAddressScVals(report.CCVS)
	if err != nil {
		return fmt.Errorf("failed to convert CCV addresses: %w", err)
	}

	verifierResultScVals := make([]xdr.ScVal, len(report.CCVData))
	for i, data := range report.CCVData {
		verifierResultScVals[i] = scval.BytesToScVal(data)
	}

	args := []xdr.ScVal{
		scval.BytesToScVal(encodedMsg),
		scval.VecToScVal(ccvScVals),
		scval.VecToScVal(verifierResultScVals),
		scval.Uint32ToScVal(DefaultGasLimitOverride),
	}

	_, err = ct.invoker.InvokeContract(ctx, ct.offRampContractID, "execute", args)
	if err != nil {
		return fmt.Errorf("failed to invoke offramp execute: %w", err)
	}

	ct.lggr.Info().
		Str("messageID", messageID.String()).
		Str("offRampContractID", ct.offRampContractID).
		Int("ccvCount", len(report.CCVS)).
		Msg("Submitted execute transaction to Stellar offramp")

	return nil
}

// unknownAddressesToStellarAddressScVals converts raw 32-byte contract IDs
// (protocol.UnknownAddress) into Soroban Address ScVals.
// TODO: refactor, use what's in scval util instead
func unknownAddressesToStellarAddressScVals(addrs []protocol.UnknownAddress) ([]xdr.ScVal, error) {
	out := make([]xdr.ScVal, len(addrs))
	for i, raw := range addrs {
		if len(raw) != 32 {
			return nil, fmt.Errorf("CCV address at index %d has invalid length %d (expected 32)", i, len(raw))
		}
		stellarAddr, err := strkey.Encode(strkey.VersionByteContract, []byte(raw))
		if err != nil {
			return nil, fmt.Errorf("CCV address at index %d: strkey encode failed: %w", i, err)
		}
		out[i] = scval.AddressToScVal(stellarAddr)
	}
	return out, nil
}
