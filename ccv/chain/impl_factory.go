package ccvchain

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"

	chainsel "github.com/smartcontractkit/chain-selectors"
	ccv "github.com/smartcontractkit/chainlink-ccv/build/devenv"
	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	ccvservices "github.com/smartcontractkit/chainlink-ccv/build/devenv/services"
	"github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-testing-framework/framework/components/blockchain"

	fqbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/fee_quoter"
	offrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/offramp"
	onrampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/onramp"
	rmnproxybindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_proxy"
	rmnremotebindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
	tokenpoolbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/token_pool"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarccip "github.com/smartcontractkit/chainlink-stellar/deployment/ccip"
)

var _ ccv.ImplFactory = &ImplFactory{}

// ImplFactory creates Stellar CCIP17 chain implementations.
// It implements [registry.ImplFactory] and is registered with the global factory
// registry via RegisterImplFactory(chainsel.FamilyStellar, NewImplFactory()).
type ImplFactory struct{}

// NewImplFactory returns a new Stellar ImplFactory.
func NewImplFactory() *ImplFactory {
	return &ImplFactory{}
}

// DefaultSignerKey returns the default verifier-result signer key for this
// chain family given the bootstrap keys from a verifier node. devenv calls
// this from enrichEnvironmentTopology; the returned address is recorded in
// the verifier's commit.Config.SignerAddress and ends up as the signer
// expected by the on-chain committee_verifier.
//
// Despite Stellar transmitting Soroban transactions with an Ed25519 keypair,
// the committee_verifier contract on Stellar stores 20-byte ETH-style signer
// addresses (see contracts/common/verifier ETH_ADDRESS_OFFSET and
// ccv/chain/adapter/aggregator_config_adapter.go reading signer[12:32]).
// That means the verifier signs results with its ECDSA key — the same one
// EVM verifiers use — so we return keys.ECDSAAddress here. The Stellar
// Ed25519 transmitter / deployer keypair is a separate concern handled by
// the accessor's KeystoreSetter path.
//
// Pre-2026-05-01 this read keys.EdDSAPublicKey and produced a Stellar G...
// address, but that never matched the 20-byte ETH-style on-chain entry; the
// fallback "fetch signing keys from JD" path masked the mismatch. With
// chainlink-ccv changelog/2026-05-01_executor_keystore_transmitter.md the
// EdDSA field was removed from BootstrapKeys, so we now return the correct
// ECDSA address directly.
func (f *ImplFactory) DefaultSignerKey(keys ccvservices.BootstrapKeys) string {
	return keys.ECDSAAddress
}

// DefaultFeeAggregator implements [ccv.ImplFactory].
// Returns the CLDF Stellar deployer account address when topology omits fee_aggregator for this chain.
func (f *ImplFactory) DefaultFeeAggregator(env *deployment.Environment, chainSelector uint64) string {
	stellarChains := env.BlockChains.StellarChains()
	if chain, ok := stellarChains[chainSelector]; ok && chain.Signer != nil {
		return chain.Signer.Address()
	}
	return ""
}

// SupportsFunding reports whether this chain family supports native token
// funding of executor addresses. Families that lack on-chain transfer
// primitives in devenv (e.g. Canton) return false.
func (f *ImplFactory) SupportsFunding() bool {
	return true
}

// SupportsBootstrapExecutor reports whether executors for this family
// use the bootstrap.Run lifecycle (JD-managed with DB). Families that
// use standalone executors (legacy mode, no bootstrap) return false.
func (f *ImplFactory) SupportsBootstrapExecutor() bool {
	return true
}

// NewEmpty implements [registry.ImplFactory].
// Returns a bare Chain used by NewEnvironment() to call DeployLocalNetwork and
// the shared DeployChainContracts path (Pre/GetDeployChainContractsCfg/Post).
func (f *ImplFactory) NewEmpty() cciptestinterfaces.CCIP17Configuration {
	return New(
		zerolog.New(os.Stderr).With().Str("component", "Stellar").Logger(),
		chainsel.STELLAR_LOCALNET.Selector,
	)
}

// New implements [registry.ImplFactory].
// Returns a fully initialised Chain for test interactions against an already-deployed
// network. It reconstructs all necessary state (RPC client, deployer keypair, OnRamp
// client) from the blockchain.Input output and the deployment datastore.
func (f *ImplFactory) New(ctx context.Context, cfg *ccv.Cfg, lggr zerolog.Logger, env *deployment.Environment, bc *blockchain.Input) (cciptestinterfaces.CCIP17, error) {
	details, err := chainsel.GetChainDetailsByChainIDAndFamily(bc.ChainID, chainsel.FamilyStellar)
	if err != nil {
		return nil, fmt.Errorf("get chain details for Stellar chain %s: %w", bc.ChainID, err)
	}

	if bc.Out == nil {
		return nil, fmt.Errorf("blockchain output is nil for chain %s", bc.ChainID)
	}
	if len(bc.Out.Nodes) == 0 {
		return nil, fmt.Errorf("no nodes in blockchain output for Stellar chain %s", bc.ChainID)
	}
	if bc.Out.NetworkSpecificData == nil || bc.Out.NetworkSpecificData.StellarNetwork == nil {
		return nil, fmt.Errorf("missing Stellar network info in blockchain output for chain %s", bc.ChainID)
	}

	sorobanRPCURL := bc.Out.Nodes[0].ExternalHTTPUrl
	networkPassphrase := bc.Out.NetworkSpecificData.StellarNetwork.NetworkPassphrase

	// Derive the same deployer keypair used during DeployLocalNetwork.
	deployerSeed := sha256.Sum256([]byte(fmt.Sprintf("deployer-%s", networkPassphrase)))
	deployerKP, err := keypair.FromRawSeed(deployerSeed)
	if err != nil {
		return nil, fmt.Errorf("derive deployer keypair for Stellar chain %s: %w", bc.ChainID, err)
	}

	rpcClient := rpcclient.NewClient(sorobanRPCURL, &http.Client{Timeout: 60 * time.Second})
	deployer := stellardeployment.NewDeployer(rpcClient, networkPassphrase, deployerKP)

	chain := &Chain{
		chainSelector:     details.ChainSelector,
		logger:            lggr,
		rpcClient:         rpcClient,
		networkPassphrase: networkPassphrase,
		deployerKeypair:   deployerKP,
		deployer:          deployer,
	}

	// Look up deployed contract addresses from the datastore and wire up clients.
	if env.DataStore != nil {
		onrampRef, err := env.DataStore.Addresses().Get(stellarccip.OnRampDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && onrampRef.Address != "" {
			onrampContractID, convErr := scval.HexToContractStrkey(onrampRef.Address)
			if convErr == nil {
				chain.onRampContractID = onrampContractID
				chain.onRampClient = onrampbindings.NewOnRampClient(deployer, onrampContractID)
			}
		}

		offrampRef, err := env.DataStore.Addresses().Get(stellarccip.OffRampDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && offrampRef.Address != "" {
			offrampContractID, convErr := scval.HexToContractStrkey(offrampRef.Address)
			if convErr == nil {
				chain.offRampContractID = offrampContractID
				chain.offRampClient = offrampbindings.NewOffRampClient(deployer, offrampContractID)
			}
		}

		routerRef, err := env.DataStore.Addresses().Get(stellarccip.RouterDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && routerRef.Address != "" {
			routerContractID, convErr := scval.HexToContractStrkey(routerRef.Address)
			if convErr == nil {
				chain.routerContractID = routerContractID
				chain.routerClient = routerbindings.NewRouterClient(deployer, routerContractID)
			}
		}

		fqRef, err := env.DataStore.Addresses().Get(stellarccip.FeeQuoterDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && fqRef.Address != "" {
			fqContractID, convErr := scval.HexToContractStrkey(fqRef.Address)
			if convErr == nil {
				chain.feeQuoterClient = fqbindings.NewFeeQuoterClient(deployer, fqContractID)
			}
		}

		vvrRef, err := env.DataStore.Addresses().Get(stellarccip.VVRDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && vvrRef.Address != "" {
			vvrContractID, convErr := scval.HexToContractStrkey(vvrRef.Address)
			if convErr == nil {
				chain.vvrContractID = vvrContractID
			}
		}

		cvRef, err := env.DataStore.Addresses().Get(stellarccip.CommitteeVerifierDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && cvRef.Address != "" {
			cvContractID, convErr := scval.HexToContractStrkey(cvRef.Address)
			if convErr == nil {
				chain.cvContractID = cvContractID
			}
		}

		receiverRef, err := env.DataStore.Addresses().Get(stellarccip.CCIPReceiverDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && receiverRef.Address != "" {
			receiverContractID, convErr := scval.HexToContractStrkey(receiverRef.Address)
			if convErr == nil {
				chain.receiverContractID = receiverContractID
			}
		}

		rmnProxyRef, err := env.DataStore.Addresses().Get(stellarccip.RMNProxyDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && rmnProxyRef.Address != "" {
			rmnProxyContractID, convErr := scval.HexToContractStrkey(rmnProxyRef.Address)
			if convErr == nil {
				chain.rmnProxyContractID = rmnProxyContractID
				chain.rmnProxyClient = rmnproxybindings.NewRmnProxyClient(deployer, rmnProxyContractID)
			}
		}

		rmnRemoteRef, err := env.DataStore.Addresses().Get(stellarccip.RMNRemoteDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && rmnRemoteRef.Address != "" {
			rmnRemoteContractID, convErr := scval.HexToContractStrkey(rmnRemoteRef.Address)
			if convErr == nil {
				chain.rmnRemoteContractID = rmnRemoteContractID
				chain.rmnRemoteClient = rmnremotebindings.NewRmnRemoteClient(deployer, rmnRemoteContractID)
			}
		}

		poolRef, err := env.DataStore.Addresses().Get(stellarccip.LockReleasePoolDevenvDatastoreRef().AddressRefKey(details.ChainSelector))
		if err == nil && poolRef.Address != "" {
			poolContractID, convErr := scval.HexToContractStrkey(poolRef.Address)
			if convErr == nil {
				chain.tokenPoolContractID = poolContractID
				chain.tokenPoolClient = tokenpoolbindings.NewTokenPoolClient(deployer, poolContractID)
			}
		}

		// TokenAdminRegistry and RampRegistry are not loaded above; fill from DS using shared lookups.
		chain.hydrateDevenvClientsFromDataStore(env.DataStore, details.ChainSelector)
	}

	// Re-derive the deterministic test SAC token address so that token transfer
	// tests can call GetTokenAddress() without re-running deployment.
	issuerSeed := sha256.Sum256([]byte(fmt.Sprintf("test-token-issuer-%s", networkPassphrase)))
	issuerKP, err := keypair.FromRawSeed(issuerSeed)
	if err == nil {
		asset := txnbuild.CreditAsset{Code: testTokenAssetCode, Issuer: issuerKP.Address()}
		xdrAsset, xdrErr := asset.ToXDR()
		if xdrErr == nil {
			if contractID, sacErr := stellardeployment.ComputeSACContractID(networkPassphrase, xdrAsset); sacErr == nil {
				chain.testTokenContractID = contractID
			}
		}
	}

	// Re-derive the deterministic fee SAC token address so that SendMessage
	// can reference it without re-running deployment.
	feeIssuerSeed := sha256.Sum256([]byte(fmt.Sprintf("fee-token-issuer-%s", networkPassphrase)))
	feeIssuerKP, err := keypair.FromRawSeed(feeIssuerSeed)
	if err == nil {
		feeAsset := txnbuild.CreditAsset{Code: feeTokenAssetCode, Issuer: feeIssuerKP.Address()}
		xdrAsset, xdrErr := feeAsset.ToXDR()
		if xdrErr == nil {
			if contractID, sacErr := stellardeployment.ComputeSACContractID(networkPassphrase, xdrAsset); sacErr == nil {
				chain.feeTokenContractID = contractID
			}
		}
	}

	return chain, nil
}
