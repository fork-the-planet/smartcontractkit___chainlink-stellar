package stellardeploy

import (
	"fmt"
	"os"
	"path/filepath"

	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	recvops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/ccip_receiver"
)

func (w *deployRun) deployReceiverAndWriteDatastore() error {
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
	if err := stellarccip.RecordCCIPReceiver(w.ds, w.selector, receiverContractID); err != nil {
		return fmt.Errorf("record CCIP receiver in datastore: %w", err)
	}

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
	ds.AddressRefStore.Upsert(stellarccip.CCIPReceiverDatastoreRef().FullAddressRef(selector, receiverHex))

	onrampHex, err := stellarutil.StrkeyToHex(onrampContractID)
	if err != nil {
		return fmt.Errorf("failed to convert OnRamp address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.OnRampDatastoreRef().FullAddressRef(selector, onrampHex))

	offRampHex, err := stellarutil.StrkeyToHex(offRampContractID)
	if err != nil {
		return fmt.Errorf("failed to convert OffRamp address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.OffRampDatastoreRef().FullAddressRef(selector, offRampHex))

	routerHex, err := stellarutil.StrkeyToHex(routerContractID)
	if err != nil {
		return fmt.Errorf("failed to convert Router address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.RouterDatastoreRef().FullAddressRef(selector, routerHex))

	tarHex, err := stellarutil.StrkeyToHex(tarContractID)
	if err != nil {
		return fmt.Errorf("failed to convert TokenAdminRegistry address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.TokenAdminRegistryDatastoreRef().FullAddressRef(selector, tarHex))

	if poolContractID != "" {
		poolHex, err := stellarutil.StrkeyToHex(poolContractID)
		if err != nil {
			return fmt.Errorf("failed to convert pool address: %w", err)
		}
		ds.AddressRefStore.Upsert(stellarccip.LockReleasePoolDevenvDatastoreRef().FullAddressRef(selector, poolHex))
	}

	vvrHex, err := stellarutil.StrkeyToHex(vvrContractID)
	if err != nil {
		return fmt.Errorf("failed to convert VVR address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.VVRDatastoreRef().FullAddressRef(selector, vvrHex))

	cvHex, err := stellarutil.StrkeyToHex(cvContractID)
	if err != nil {
		return fmt.Errorf("failed to convert Committee Verifier address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.CommitteeVerifierDatastoreRef().FullAddressRef(selector, cvHex))

	ds.AddressRefStore.Upsert(stellarccip.DefaultExecutorDatastoreRef().FullAddressRef(selector, w.contractHexAddr("stellar-executor")))

	ds.AddressRefStore.Upsert(stellarccip.ExecutorProxyDatastoreRef(devenvcommon.DefaultExecutorQualifier).FullAddressRef(selector, w.contractHexAddr("stellar-executor-proxy")))

	rmnRemoteHex, err := stellarutil.StrkeyToHex(rmnRemoteContractID)
	if err != nil {
		return fmt.Errorf("failed to convert RMN Remote address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.RMNRemoteDatastoreRef().FullAddressRef(selector, rmnRemoteHex))

	feeQuoterHex, err := stellarutil.StrkeyToHex(feeQuoterContractID)
	if err != nil {
		return fmt.Errorf("failed to convert FeeQuoter address: %w", err)
	}
	ds.AddressRefStore.Upsert(stellarccip.FeeQuoterDatastoreRef().FullAddressRef(selector, feeQuoterHex))

	return nil
}
