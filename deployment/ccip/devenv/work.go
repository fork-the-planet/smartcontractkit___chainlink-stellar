package devenv

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"

	ccipOffchain "github.com/smartcontractkit/chainlink-ccip/deployment/v1_7_0/offchain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cvbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/committee_verifier"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	vvrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/versioned_verifier_resolver"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
)

// work holds shared state for phased Stellar CCIP devenv deployment.
type work struct {
	host Host
	ctx  context.Context

	selector uint64

	allSelectors []uint64
	topology     *ccipOffchain.EnvironmentTopology

	ds              *datastore.MemoryDataStore
	stellarRoot     string
	remoteSelectors []uint64

	feeTokenContractID string

	onrampContractID    string
	rmnRemoteContractID string
	rmnProxyContractID  string
	feeQuoterContractID string
	tarContractID       string
	poolContractID      string
	vvrContractID       string
	cvContractID        string
	offRampContractID   string
	routerContractID    string
	receiverContractID  string

	vvrClient    *vvrbindings.VersionedVerifierResolverClient
	cvClient     *cvbindings.CommitteeVerifierClient
	routerClient *routerbindings.RouterClient
}

func (w *work) contractHexAddr(name string) string {
	return hexutil.Encode(stellarutil.GenerateContractAddress(name, w.host.NetworkPassphrase()))
}

func (w *work) setup() error {
	host := w.host
	host.Logger().Info().Uint64("selector", w.selector).Msg("Deploying Stellar CCIP contracts")

	w.ds = datastore.NewMemoryDataStore()

	root, err := stellarutil.FindStellarRoot()
	if err != nil {
		return fmt.Errorf("failed to locate chainlink-stellar root: %w", err)
	}
	w.stellarRoot = root
	host.Logger().Info().Str("stellarRoot", root).Msg("Stellar root")

	w.remoteSelectors = stellarutil.FilterRemoteSelectors(w.allSelectors, w.selector)
	return nil
}
