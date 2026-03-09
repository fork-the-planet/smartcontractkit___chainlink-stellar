package destinationreader

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	"github.com/smartcontractkit/chainlink-stellar/bindings"
	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	rmnremotebindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
)

var _ chainaccess.DestinationReader = (*DestinationReader)(nil)

// Config is the configuration required to create a Stellar destination reader.
type Config struct {
	// OffRampContractID is the Stellar contract address of the OffRamp.
	OffRampContractID string `toml:"offramp_contract_id"`
	// RMNRemoteContractID is the Stellar contract address of the RMN Remote.
	RMNRemoteContractID string `toml:"rmn_remote_contract_id"`
}

// DestinationReader reads execution state and CCV data from the Stellar OffRamp contract.
type DestinationReader struct {
	lggr             *zerolog.Logger
	invoker          bindings.Invoker
	offrampClient    *offrampbindings.OffRampClient
	rmnRemoteAddress string
}

// New creates a new Stellar DestinationReader.
func New(
	invoker bindings.Invoker,
	offRampContractID string,
	rmnRemoteContractID string,
	lggr *zerolog.Logger,
) (*DestinationReader, error) {
	if invoker == nil {
		return nil, fmt.Errorf("invoker is required")
	}
	if offRampContractID == "" {
		return nil, fmt.Errorf("offramp contract ID is required")
	}
	if rmnRemoteContractID == "" {
		return nil, fmt.Errorf("rmn remote contract ID is required")
	}
	if lggr == nil {
		return nil, fmt.Errorf("logger is required")
	}

	return &DestinationReader{
		lggr:             lggr,
		invoker:          invoker,
		offrampClient:    offrampbindings.NewOffRampClient(invoker, offRampContractID),
		rmnRemoteAddress: rmnRemoteContractID,
	}, nil
}

// Start implements services.Service.
func (d *DestinationReader) Start(_ context.Context) error {
	d.lggr.Info().Msg("Stellar DestinationReader started")
	return nil
}

// Close implements services.Service.
func (d *DestinationReader) Close() error {
	d.lggr.Info().Msg("Stellar DestinationReader stopped")
	return nil
}

// Ready implements services.Service.
func (d *DestinationReader) Ready() error {
	return nil
}

// HealthReport implements services.Service.
func (d *DestinationReader) HealthReport() map[string]error {
	return map[string]error{"StellarDestinationReader": nil}
}

// Name implements services.Service.
func (d *DestinationReader) Name() string {
	return "StellarDestinationReader"
}

// GetMessageSuccess queries the OffRamp contract for the execution state of a message
// and returns true if the message has been successfully executed.
func (d *DestinationReader) GetMessageSuccess(ctx context.Context, message protocol.Message) (bool, error) {
	msgID, err := message.MessageID()
	if err != nil {
		return false, fmt.Errorf("failed to compute message ID: %w", err)
	}
	state, err := d.offrampClient.GetExecutionState(ctx, msgID)
	if err != nil {
		return false, fmt.Errorf("failed to get execution state for message %x: %w", msgID, err)
	}
	return state == offrampbindings.MessageExecutionStateSuccess, nil
}

// GetCCVSForMessage returns the cross-chain verification addresses for the message.
// TODO: implement by querying the OffRamp contract's source chain config for CCV addresses.
func (d *DestinationReader) GetCCVSForMessage(_ context.Context, _ protocol.Message) (protocol.CCVAddressInfo, error) {
	return protocol.CCVAddressInfo{}, fmt.Errorf("GetCCVSForMessage not yet implemented for Stellar")
}

// GetExecutionAttempts returns the execution attempts for a given message.
// TODO: implement by querying OffRamp execution events or state for the message.
func (d *DestinationReader) GetExecutionAttempts(_ context.Context, _ protocol.Message) ([]protocol.ExecutionAttempt, error) {
	return nil, fmt.Errorf("GetExecutionAttempts not yet implemented for Stellar")
}

// GetRMNCursedSubjects queries the RMN Remote contract for cursed subjects.
func (d *DestinationReader) GetRMNCursedSubjects(ctx context.Context) ([]protocol.Bytes16, error) {
	rmnRemoteClient := rmnremotebindings.NewRmnRemoteClient(d.invoker, d.rmnRemoteAddress)
	cursedSubjects, err := rmnRemoteClient.GetCursedSubjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cursed subjects: %w", err)
	}

	result := make([]protocol.Bytes16, len(cursedSubjects))
	for i, s := range cursedSubjects {
		result[i] = protocol.Bytes16(s)
	}
	return result, nil
}
