package stellardeploy

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cvbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/committee_verifier"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	vvrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/versioned_verifier_resolver"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
)

// deployRun holds shared state for phased Stellar CCIP deployment.
type deployRun struct {
	host Host
	ctx  context.Context

	selector uint64

	allSelectors []uint64
	topology     *ccvdeployment.EnvironmentTopology

	existingAddresses []datastore.AddressRef

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

	// opBundle is the CLDF bundle used for all execStellarOp calls (caller supplies it, same as EVM/Solana sequences).
	opBundle cldfops.Bundle
}

func (w *deployRun) contractHexAddr(name string) string {
	return hexutil.Encode(stellarutil.GenerateContractAddress(name, w.host.NetworkPassphrase()))
}

func (w *deployRun) setup() error {
	host := w.host
	host.Logger().Info().Uint64("selector", w.selector).Msg("Deploying Stellar CCIP contracts")

	w.ds = datastore.NewMemoryDataStore()
	if err := stellarccip.MergeExistingAddressRefs(w.ds, w.existingAddresses); err != nil {
		return fmt.Errorf("merge existing address refs: %w", err)
	}

	root, err := stellarutil.FindStellarRoot()
	if err != nil {
		return fmt.Errorf("failed to locate chainlink-stellar root: %w", err)
	}
	w.stellarRoot = root
	host.Logger().Info().Str("stellarRoot", root).Msg("Stellar root")

	w.remoteSelectors = stellarutil.FilterRemoteSelectors(w.allSelectors, w.selector)
	return nil
}
