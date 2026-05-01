package devenv

import (
	"fmt"
	"os"
	"path/filepath"

	chainsel "github.com/smartcontractkit/chain-selectors"
	cvbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/committee_verifier"
	fqbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	vvrbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/versioned_verifier_resolver"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	cvops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/committee_verifier"
	fqops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/fee_quoter"
	vvrops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/versioned_verifier_resolver"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func (w *work) configureVerificationAndFeeQuoter() error {
	h := w.host
	ctx := w.ctx
	stellarRoot := w.stellarRoot
	allSelectors := w.allSelectors
	topology := w.topology

	mockFeeAggregator := stellarutil.MustGenerateMockContractID(h.DeployerKeypair().Address(), "fee-aggregator")
	feeQuoterClient := h.FeeQuoterClient()

	vvrWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "ccvs_versioned_verifier_resolver.wasm")
	if _, statErr := os.Stat(vvrWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("VVR WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", vvrWasmPath)
	}

	h.Logger().Info().Str("wasmPath", vvrWasmPath).Msg("Deploying Versioned Verifier Resolver contract...")

	vvrSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "versioned-verifier-resolver")
	vvrOut, err := execStellarOp(w, vvrops.Deploy, stellarops.DeployInput{WasmPath: vvrWasmPath, Salt: vvrSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy VVR contract: %w", err)
	}
	vvrContractID := vvrOut.ContractID
	w.vvrContractID = vvrContractID
	h.Logger().Info().Str("contractID", vvrContractID).Msg("VVR contract deployed")
	h.SetVVR(vvrContractID)

	vvrClient := vvrbindings.NewVersionedVerifierResolverClient(h.Deployer(), vvrContractID)
	w.vvrClient = vvrClient

	if _, err := execStellarOp(w, vvrops.Initialize, vvrops.InitializeInput{
		ContractID: vvrContractID,
		Owner:      h.DeployerKeypair().Address(),
		// TODO: replace with real fee aggregator value
		FeeAggregator: mockFeeAggregator,
	}); err != nil {
		return fmt.Errorf("failed to initialize VVR: %w", err)
	}

	h.Logger().Info().
		Str("vvrContractID", vvrContractID).
		Msg("VVR client initialized")

	cvWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "ccvs_committee_verifier.wasm")
	if _, statErr := os.Stat(cvWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("Committee Verifier WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", cvWasmPath)
	}

	h.Logger().Info().Str("wasmPath", cvWasmPath).Msg("Deploying Committee Verifier contract...")

	cvSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "committee-verifier")
	cvOut, err := execStellarOp(w, cvops.Deploy, stellarops.DeployInput{WasmPath: cvWasmPath, Salt: cvSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy Committee Verifier contract: %w", err)
	}
	cvContractID := cvOut.ContractID
	w.cvContractID = cvContractID
	h.Logger().Info().Str("contractID", cvContractID).Msg("Committee Verifier contract deployed")

	cvClient := cvbindings.NewCommitteeVerifierClient(h.Deployer(), cvContractID)
	w.cvClient = cvClient

	allowlistAdmin := h.DeployerKeypair().Address()
	mockStorageLocation := stellarutil.GenerateContractAddress("storage-location", h.NetworkPassphrase())
	if _, err := execStellarOp(w, cvops.Initialize, cvops.InitializeInput{
		ContractID: cvContractID,
		Owner:      h.DeployerKeypair().Address(),
		DynamicConfig: cvbindings.DynamicConfig{
			AllowlistAdmin: &allowlistAdmin,
			FeeAggregator:  &mockFeeAggregator,
		},
		StorageLocations: [][]byte{mockStorageLocation},
		RmnProxy:         w.rmnProxyContractID,
		VersionTag:       stellarutil.DefaultCommitteeVerifierVersionTag(),
	}); err != nil {
		return fmt.Errorf("failed to initialize Committee Verifier: %w", err)
	}

	h.SetCV(cvContractID)
	h.Logger().Info().
		Str("cvContractID", cvContractID).
		Msg("Committee Verifier client initialized")

	outboundImplUpdates := []vvrbindings.OutboundImplementationUpdate{}
	for _, remoteSelector := range allSelectors {
		outboundImplUpdates = append(outboundImplUpdates, vvrbindings.OutboundImplementationUpdate{
			DestChainSelector: remoteSelector,
			Verifier:          &cvContractID,
		})
	}

	if _, err := execStellarOp(w, vvrops.ApplyOutboundImplUpdates, vvrops.ApplyOutboundImplUpdatesInput{
		ContractID:      vvrContractID,
		Implementations: outboundImplUpdates,
	}); err != nil {
		return fmt.Errorf("failed to apply outbound implementation updates: %w", err)
	}

	inboundImplUpdates := []vvrbindings.InboundImplementationUpdate{
		{
			Version:  stellarutil.DefaultCommitteeVerifierVersionTag(),
			Verifier: &cvContractID,
		},
	}
	if _, err := execStellarOp(w, vvrops.ApplyInboundImplUpdates, vvrops.ApplyInboundImplUpdatesInput{
		ContractID:      vvrContractID,
		Implementations: inboundImplUpdates,
	}); err != nil {
		return fmt.Errorf("failed to apply inbound implementation updates: %w", err)
	}

	h.Logger().Info().Msg("Inbound implementation and outbound updates applied")

	remoteChainConfigs := make([]cvbindings.RemoteChainConfig, 0, len(allSelectors))
	for _, rs := range allSelectors {
		router := h.DeployerKeypair().Address()
		remoteChainConfigs = append(remoteChainConfigs, cvbindings.RemoteChainConfig{
			RemoteChainSelector: rs,
			FeeUsdCents:         0,
			GasForVerification:  10000, // CANNOT be zero
			PayloadSizeBytes:    0,
			AllowlistEnabled:    false,
			Router:              &router,
		})
	}
	if _, err := execStellarOp(w, cvops.ApplyRemoteChainCfgUpdates, cvops.ApplyRemoteChainCfgUpdatesInput{
		ContractID: cvContractID,
		Configs:    remoteChainConfigs,
	}); err != nil {
		return fmt.Errorf("failed to apply remote chain config updates on committee verifier: %w", err)
	}
	h.Logger().Info().Int("count", len(remoteChainConfigs)).Msg("Committee Verifier remote chain configs applied")

	signatureQuorumConfigs := make([]cvbindings.SignatureQuorumConfig, 0, len(allSelectors))
	for _, rs := range allSelectors {
		signers, threshold := stellarutil.ResolveSignersFromTopology(topology, rs, chainsel.FamilyStellar)
		if len(signers) == 0 {
			h.Logger().Warn().Uint64("sourceChainSelector", rs).Msg("No signers found in topology, using placeholder")
			signers = [][32]byte{{1}}
			threshold = 1
		}
		signatureQuorumConfigs = append(signatureQuorumConfigs, cvbindings.SignatureQuorumConfig{
			SourceChainSelector: rs,
			Threshold:           threshold,
			Signers:             signers,
		})
	}
	if _, err := execStellarOp(w, cvops.ApplySignatureConfigs, cvops.ApplySignatureConfigsInput{
		ContractID:             cvContractID,
		RemoveSelectors:        []uint64{},
		SignatureQuorumConfigs: signatureQuorumConfigs,
	}); err != nil {
		return fmt.Errorf("failed to apply signature quorum configs: %w", err)
	}
	h.Logger().Info().Int("count", len(signatureQuorumConfigs)).Msg("Signature quorum configs applied")

	fqDestChainConfigs := []fqbindings.DestChainConfigArgs{}
	for _, rs := range allSelectors {
		fqDestChainConfigs = append(fqDestChainConfigs, fqbindings.DestChainConfigArgs{
			DestChainSelector: rs,
			Config: fqbindings.DestChainConfig{
				IsEnabled:             true,
				MaxDataBytes:          50000,
				MaxPerMsgGasLimit:     4_000_000,
				DestGasOverhead:       350_000,
				DestGasPerPayloadByte: 16,
				DefaultTokenFeeUsd:    50,
				DefaultTokenDestGas:   50_000,
				DefaultTxGasLimit:     200_000,
				NetworkFeeUsdCents:    100,
				LinkPremiumPercent:    90,
			},
		})
	}
	if _, err := execStellarOp(w, fqops.ApplyDestChainConfigs, fqops.ApplyDestChainConfigsInput{
		ContractID: w.feeQuoterContractID,
		Configs:    fqDestChainConfigs,
	}); err != nil {
		return fmt.Errorf("failed to apply dest chain configs on FeeQuoter: %w", err)
	}
	h.Logger().Info().Int("count", len(fqDestChainConfigs)).Msg("FeeQuoter dest chain configs applied")

	gasPriceUpdates := make([]fqbindings.GasPriceUpdate, 0, len(allSelectors))
	for _, rs := range allSelectors {
		gasPriceUpdates = append(gasPriceUpdates, fqbindings.GasPriceUpdate{
			DestChainSelector: rs,
			UsdPerUnitGas:     scval.U128(xdr.UInt128Parts{Hi: 0, Lo: 100_000_000_000_000}), // 1e14
		})
	}
	if err := feeQuoterClient.UpdatePrices(ctx, h.DeployerKeypair().Address(), fqbindings.PriceUpdates{
		TokenPriceUpdates: []fqbindings.TokenPriceUpdate{
			{
				Token:       w.feeTokenContractID,
				UsdPerToken: scval.U128(xdr.UInt128Parts{Hi: 0, Lo: 1_000_000_000_000_000_000}), // $1
			},
		},
		GasPriceUpdates: gasPriceUpdates,
	}); err != nil {
		return fmt.Errorf("failed to update prices on FeeQuoter: %w", err)
	}
	h.Logger().Info().Msg("FeeQuoter prices updated")

	if testToken := h.TestTokenContractID(); testToken != "" {
		if err := ApplyFeeQuoterTestTokenConfig(ctx, feeQuoterClient, h.DeployerKeypair().Address(), testToken, allSelectors); err != nil {
			return err
		}
		h.Logger().Info().Int("count", len(allSelectors)).Msg("FeeQuoter token transfer fee configs applied")
	}

	return nil
}
