package relayer

import (
	"context"
	"fmt"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	relaytypes "github.com/smartcontractkit/chainlink-common/pkg/types"
	stellartypes "github.com/smartcontractkit/chainlink-common/pkg/types/chains/stellar"

	"github.com/smartcontractkit/chainlink-stellar/relayer/chain"
)

type stellarService struct {
	relaytypes.UnimplementedStellarService
	chain chain.Chain
}

var _ relaytypes.StellarService = (*stellarService)(nil)

func newStellarService(ch chain.Chain) stellarService {
	return stellarService{chain: ch}
}

func (s *stellarService) GetLedgerEntries(ctx context.Context, req stellartypes.GetLedgerEntriesRequest) (stellartypes.GetLedgerEntriesResponse, error) {
	rpc, err := s.chain.GetClient()
	if err != nil {
		return stellartypes.GetLedgerEntriesResponse{}, fmt.Errorf("GetLedgerEntries: get client: %w", err)
	}

	keys := make([]string, len(req.Keys))
	for i, k := range req.Keys {
		keys[i] = string(k)
	}

	resp, err := rpc.GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{Keys: keys})
	if err != nil {
		return stellartypes.GetLedgerEntriesResponse{}, fmt.Errorf("GetLedgerEntries: %w", err)
	}

	entries := make([]stellartypes.LedgerEntryResult, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entry := stellartypes.LedgerEntryResult{
			KeyXDR:             stellartypes.XDR(e.KeyXDR),
			DataXDR:            stellartypes.XDR(e.DataXDR),
			LastModifiedLedger: e.LastModifiedLedger,
			ExtensionXDR:       stellartypes.XDR(e.ExtensionXDR),
		}
		if e.LiveUntilLedgerSeq != nil {
			v := *e.LiveUntilLedgerSeq
			entry.LiveUntilLedgerSeq = &v
		}
		entries = append(entries, entry)
	}

	return stellartypes.GetLedgerEntriesResponse{
		Entries:      entries,
		LatestLedger: resp.LatestLedger,
	}, nil
}

func (s *stellarService) GetLatestLedger(ctx context.Context) (stellartypes.GetLatestLedgerResponse, error) {
	rpc, err := s.chain.GetClient()
	if err != nil {
		return stellartypes.GetLatestLedgerResponse{}, fmt.Errorf("failed to get client: %w", err)
	}

	resp, err := rpc.GetLatestLedger(ctx)
	if err != nil {
		return stellartypes.GetLatestLedgerResponse{}, err
	}

	return stellartypes.GetLatestLedgerResponse{
		Hash:              stellartypes.LedgerHash(resp.Hash),
		ProtocolVersion:   resp.ProtocolVersion,
		Sequence:          resp.Sequence,
		LedgerCloseTime:   resp.LedgerCloseTime,
		LedgerHeaderXDR:   stellartypes.XDR(resp.LedgerHeader),
		LedgerMetadataXDR: stellartypes.XDR(resp.LedgerMetadata),
	}, nil
}
