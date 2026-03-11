package contracttransmitter

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-ccv/executor"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-stellar/bindings"
	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
)

// DefaultGasLimitOverride is passed to every offramp execute call.
// Zero means "use the gas limit from the message itself".
const DefaultGasLimitOverride uint32 = 0

var _ chainaccess.ContractTransmitter = (*ContractTransmitter)(nil)

// ContractTransmitterConfig is the configuration required to create a Stellar contract transmitter.
type ContractTransmitterConfig struct {
	// NetworkPassphrase is the Stellar network passphrase (e.g., "Standalone Network ; February 2017").
	NetworkPassphrase string `toml:"network_passphrase"`
	// OffRampContractID is the contract ID of the Stellar OffRamp contract.
	OffRampContractID string `toml:"offramp_contract_id"`
	// CCIPOfframpAddress is the address of the CCIP OffRamp contract.
	CCIPOfframpAddress string `toml:"ccip_offramp_address"`
	// CCIPStateChangedTopic is the topic of the CCIP StateChanged event.
	CCIPStateChangedTopic string `toml:"ccip_state_changed_topic"`
	// RMNRemoteAddress is the address of the RMN Remote contract.
	RMNRemoteAddress string `toml:"rmn_remote_address"`
}

// ContractTransmitter transmits aggregated reports to the Stellar OffRamp
// contract by calling its `execute` entry point via a Soroban invoker.
type ContractTransmitter struct {
	lggr                  *zerolog.Logger
	invoker               bindings.Invoker
	ccipOfframpAddress    string
	ccipStateChangedTopic string
	rmnRemoteAddress      string
	offrampClient         *offrampbindings.OffRampClient
}

// NewContractTransmitter creates a Stellar ContractTransmitter.
func NewContractTransmitterWithClient(
	invoker bindings.Invoker,
	ccipOfframpAddress string,
	ccipStateChangedTopic string,
	rmnRemoteAddress string,
	lggr *zerolog.Logger,
) (*ContractTransmitter, error) {
	if invoker == nil {
		return nil, fmt.Errorf("invoker is required")
	}
	if ccipOfframpAddress == "" {
		return nil, fmt.Errorf("ccip offramp address is required")
	}
	if ccipStateChangedTopic == "" {
		return nil, fmt.Errorf("ccip state changed topic is required")
	}
	if lggr == nil {
		return nil, fmt.Errorf("logger is required")
	}

	offrampClient := offrampbindings.NewOffRampClient(invoker, ccipOfframpAddress)

	return &ContractTransmitter{
		invoker:               invoker,
		ccipOfframpAddress:    ccipOfframpAddress,
		ccipStateChangedTopic: ccipStateChangedTopic,
		rmnRemoteAddress:      rmnRemoteAddress,
		lggr:                  lggr,
		offrampClient:         offrampClient,
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

	ccvScVals := make([]string, len(report.CCVS))
	for i, ccv := range report.CCVS {
		// CCV addresses from EVM are 32-byte hex values. The OffRamp expects Stellar Address (Vec<Address>),
		// which requires Stellar strkey format (C...). ParseAddress only accepts C... or G... strings;
		// passing raw bytes as string causes ParseAddress to return nil, leading to nil pointer panic during XDR encoding.
		ccvStrkey, convErr := scval.HexToContractStrkey("0x" + hex.EncodeToString(ccv.Bytes()))
		if convErr != nil {
			ct.lggr.Error().Err(convErr).
				Str("messageID", messageID.String()).
				Msg("Unable to submit txn: invalid CCV address encoding")
			return errors.Join(executor.ErrMessageEncoding,
				fmt.Errorf("unable to submit txn: invalid CCV address at index %d: %w", i, convErr))
		}
		ccvScVals[i] = ccvStrkey
	}

	err = ct.offrampClient.Execute(ctx, encodedMsg, ccvScVals, report.CCVData, DefaultGasLimitOverride)
	return err
}
