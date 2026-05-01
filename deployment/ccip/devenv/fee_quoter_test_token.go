package devenv

import (
	"context"

	"github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
)

// ApplyFeeQuoterTestTokenConfig forwards to [github.com/smartcontractkit/chainlink-stellar/deployment/ccip.ApplyFeeQuoterTestTokenConfig].
func ApplyFeeQuoterTestTokenConfig(
	ctx context.Context,
	feeQuoterClient *fee_quoter.FeeQuoterClient,
	priceUpdater string,
	testToken string,
	allSelectors []uint64,
) error {
	return stellarccip.ApplyFeeQuoterTestTokenConfig(ctx, feeQuoterClient, priceUpdater, testToken, allSelectors)
}
