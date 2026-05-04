package devenv

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"

	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v1_2_0/operations/router"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v1_6_0/operations/rmn_remote"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/committee_verifier"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/executor"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/fee_quoter"
	offrampoperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/offramp"
	onrampoperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/onramp"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/proxy"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/versioned_verifier_resolver"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	recvops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/ccip_receiver"
)

func (w *work) deployReceiverAndWriteDatastore() error {
	h := w.host
	stellarRoot := w.stellarRoot
	ds := w.ds
	selector := w.selector

	receiverWasmPath := filepath.Join(stellarRoot, "target", "wasm32v1-none", "release", "ccip_receiver_example.wasm")
	if _, statErr := os.Stat(receiverWasmPath); os.IsNotExist(statErr) {
		return fmt.Errorf("ccip_receiver_example WASM not found at %s. Run 'make build' from the chainlink-stellar root to compile contracts.", receiverWasmPath)
	}

	h.Logger().Info().Str("wasmPath", receiverWasmPath).Msg("Deploying CCIP receiver example contract...")
	receiverSalt := stellardeployment.GenerateDeterministicSalt(h.DeployerKeypair().Address(), "ccip-receiver-example")
	recvOut, err := execStellarOp(w, recvops.Deploy, stellarops.DeployInput{WasmPath: receiverWasmPath, Salt: receiverSalt})
	if err != nil {
		return fmt.Errorf("failed to deploy ccip_receiver_example contract: %w", err)
	}
	receiverContractID := recvOut.ContractID

	if _, err := execStellarOp(w, recvops.Initialize, recvops.InitializeInput{
		ContractID: receiverContractID,
		Owner:      h.DeployerKeypair().Address(),
		Router:     w.routerContractID,
	}); err != nil {
		return fmt.Errorf("failed to initialize ccip_receiver_example: %w", err)
	}

	ownerAddr := h.DeployerKeypair().Address()
	placeholderExtra := []byte{0x01}
	for _, rs := range w.remoteSelectors {
		if _, err := execStellarOp(w, recvops.EnableRemoteChain, recvops.EnableRemoteChainInput{
			ContractID:            receiverContractID,
			Caller:                ownerAddr,
			RemoteChainSelector:   rs,
			ExtraArgs:             placeholderExtra,
			AllowedFinalityConfig: 0,
		}); err != nil {
			return fmt.Errorf("ccip_receiver_example EnableRemoteChain for source chain %d: %w", rs, err)
		}
	}

	w.receiverContractID = receiverContractID
	h.SetReceiver(receiverContractID)
	h.Logger().Info().Str("receiverContractID", receiverContractID).Msg("CCIP receiver example deployed and initialized")

	onrampContractID := w.onrampContractID
	offRampContractID := w.offRampContractID
	routerContractID := w.routerContractID
	tarContractID := w.tarContractID
	poolContractID := w.poolContractID
	vvrContractID := w.vvrContractID
	cvContractID := w.cvContractID
	rmnRemoteContractID := w.rmnRemoteContractID
	feeQuoterContractID := w.feeQuoterContractID

	receiverHex, err := stellarutil.StrkeyToHex(receiverContractID)
	if err != nil {
		return fmt.Errorf("failed to convert receiver address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       receiverHex,
		ChainSelector: selector,
		Type:          datastore.ContractType(CcipReceiverContractType),
		Version:       semver.MustParse("1.0.0"),
	})

	onrampHex, err := stellarutil.StrkeyToHex(onrampContractID)
	if err != nil {
		return fmt.Errorf("failed to convert OnRamp address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       onrampHex,
		ChainSelector: selector,
		Type:          datastore.ContractType(onrampoperations.ContractType),
		Version:       semver.MustParse(onrampoperations.Deploy.Version()),
	})

	offRampHex, err := stellarutil.StrkeyToHex(offRampContractID)
	if err != nil {
		return fmt.Errorf("failed to convert OffRamp address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       offRampHex,
		ChainSelector: selector,
		Type:          datastore.ContractType(offrampoperations.ContractType),
		Version:       semver.MustParse(offrampoperations.Deploy.Version()),
	})

	routerHex, err := stellarutil.StrkeyToHex(routerContractID)
	if err != nil {
		return fmt.Errorf("failed to convert Router address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       routerHex,
		ChainSelector: selector,
		Type:          datastore.ContractType(router.ContractType),
		Version:       semver.MustParse(router.Deploy.Version()),
	})

	tarHex, err := stellarutil.StrkeyToHex(tarContractID)
	if err != nil {
		return fmt.Errorf("failed to convert TokenAdminRegistry address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       tarHex,
		ChainSelector: selector,
		Type:          datastore.ContractType(TokenAdminRegistryContractType),
		Version:       semver.MustParse("1.0.0"),
	})

	if poolContractID != "" {
		poolHex, err := stellarutil.StrkeyToHex(poolContractID)
		if err != nil {
			return fmt.Errorf("failed to convert pool address: %w", err)
		}
		ds.AddressRefStore.Add(datastore.AddressRef{
			Address:       poolHex,
			ChainSelector: selector,
			Type:          datastore.ContractType(LockReleaseTokenPoolContractType),
			Version:       semver.MustParse("1.0.0"),
			Qualifier:     DevenvTestTokenPoolQualifier,
		})
	}

	vvrHex, err := stellarutil.StrkeyToHex(vvrContractID)
	if err != nil {
		return fmt.Errorf("failed to convert VVR address: %w", err)
	}
	for _, qualifier := range []string{
		devenvcommon.DefaultCommitteeVerifierQualifier,
	} {
		ds.AddressRefStore.Add(datastore.AddressRef{
			Address:       vvrHex,
			Type:          datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType),
			Version:       versioned_verifier_resolver.Version,
			Qualifier:     qualifier,
			ChainSelector: selector,
		})
	}

	cvHex, err := stellarutil.StrkeyToHex(cvContractID)
	if err != nil {
		return fmt.Errorf("failed to convert Committee Verifier address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       cvHex,
		Type:          datastore.ContractType(committee_verifier.ContractType),
		Version:       committee_verifier.Version,
		Qualifier:     devenvcommon.DefaultCommitteeVerifierQualifier,
		ChainSelector: selector,
	})

	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       w.contractHexAddr("stellar-executor"),
		Type:          datastore.ContractType(executor.ContractType),
		Version:       executor.Version,
		Qualifier:     devenvcommon.DefaultExecutorQualifier,
		ChainSelector: selector,
	})

	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       w.contractHexAddr("stellar-executor-proxy"),
		Type:          datastore.ContractType(proxy.ContractType),
		Version:       proxy.Version,
		Qualifier:     devenvcommon.DefaultExecutorQualifier,
		ChainSelector: selector,
	})

	rmnRemoteHex, err := stellarutil.StrkeyToHex(rmnRemoteContractID)
	if err != nil {
		return fmt.Errorf("failed to convert RMN Remote address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       rmnRemoteHex,
		Type:          datastore.ContractType(rmn_remote.ContractType),
		Version:       semver.MustParse(rmn_remote.Deploy.Version()),
		ChainSelector: selector,
	})

	feeQuoterHex, err := stellarutil.StrkeyToHex(feeQuoterContractID)
	if err != nil {
		return fmt.Errorf("failed to convert FeeQuoter address: %w", err)
	}
	ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       feeQuoterHex,
		Type:          datastore.ContractType(fee_quoter.ContractType),
		Version:       semver.MustParse(fee_quoter.Deploy.Version()),
		ChainSelector: selector,
	})

	return nil
}
