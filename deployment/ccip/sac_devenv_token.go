package ccip

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
)

// DevenvSACTokenParams configures classic-asset + SAC deploy used for CCIP fee and test tokens in devenv.
type DevenvSACTokenParams struct {
	Ctx               context.Context
	RPCClient         *rpcclient.Client
	MainDeployer      *stellardeployment.Deployer
	OwnerKeypair      *keypair.Full
	NetworkPassphrase string
	FriendbotURL      string
	IssuerSeedLabel   string // e.g. "fee-token-issuer-" + passphrase
	AssetCode         string
	Logger            *zerolog.Logger
}

// DeployDevenvSACToken creates a deterministic issuer, optional friendbot funding, trust + payment, and SAC deploy.
func DeployDevenvSACToken(p DevenvSACTokenParams) (contractID string, issuerKP *keypair.Full, err error) {
	if p.MainDeployer == nil || p.OwnerKeypair == nil || p.RPCClient == nil {
		return "", nil, fmt.Errorf("rpc client, main deployer, and owner keypair are required")
	}
	if p.Logger == nil {
		z := zerolog.Nop()
		p.Logger = &z
	}

	issuerSeed := sha256.Sum256([]byte(p.IssuerSeedLabel))
	issuerKP, err = keypair.FromRawSeed(issuerSeed)
	if err != nil {
		return "", nil, fmt.Errorf("create issuer keypair: %w", err)
	}

	if p.FriendbotURL != "" {
		if err := FundAccountViaFriendbot(p.FriendbotURL, issuerKP.Address(), p.Logger); err != nil {
			return "", nil, fmt.Errorf("fund issuer: %w", err)
		}
	}

	issuerDeployer := stellardeployment.NewDeployer(p.RPCClient, p.NetworkPassphrase, issuerKP)
	asset := txnbuild.CreditAsset{Code: p.AssetCode, Issuer: issuerKP.Address()}

	if err := p.MainDeployer.SubmitClassicOperation(p.Ctx, &txnbuild.ChangeTrust{
		Line:          asset.MustToChangeTrustAsset(),
		SourceAccount: p.OwnerKeypair.Address(),
	}); err != nil {
		return "", nil, fmt.Errorf("establish trustline: %w", err)
	}

	if err := issuerDeployer.SubmitClassicOperation(p.Ctx, &txnbuild.Payment{
		Destination:   p.OwnerKeypair.Address(),
		Amount:        "100000000",
		Asset:         asset,
		SourceAccount: issuerKP.Address(),
	}); err != nil {
		return "", nil, fmt.Errorf("issue tokens: %w", err)
	}

	xdrAsset, err := asset.ToXDR()
	if err != nil {
		return "", nil, fmt.Errorf("convert asset to XDR: %w", err)
	}
	contractID, err = p.MainDeployer.DeploySACToken(p.Ctx, xdrAsset)
	if err != nil {
		return "", nil, fmt.Errorf("deploy SAC: %w", err)
	}

	p.Logger.Info().
		Str("contractID", contractID).
		Str("issuer", issuerKP.Address()).
		Str("asset", p.AssetCode).
		Msg("Devenv SAC token deployed")

	return contractID, issuerKP, nil
}
