package adapters

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stellar/go-stellar-sdk/keypair"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/committee_verifier"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/proxy"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/versioned_verifier_resolver"
	dsutils "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	ccipdevenvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	ccvdeploymentadapters "github.com/smartcontractkit/chainlink-ccv/deployment/adapters"
	"github.com/smartcontractkit/chainlink-ccv/executor"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	"github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	ccvbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/committee_verifier"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
)

// StellarCCVDeploymentAggregatorConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.AggregatorConfigAdapter
// for the devenv GenerateAggregatorConfig changeset (distinct from the chainlink-ccip
// AggregatorConfigAdapter used by CLD-style registries).
type StellarCCVDeploymentAggregatorConfigAdapter struct{}

var _ ccvdeploymentadapters.AggregatorConfigAdapter = (*StellarCCVDeploymentAggregatorConfigAdapter)(nil)

func (a *StellarCCVDeploymentAggregatorConfigAdapter) ScanCommitteeStates(
	ctx context.Context,
	env deployment.Environment,
	chainSelector uint64,
) ([]*ccvdeploymentadapters.CommitteeState, error) {
	refs := env.DataStore.Addresses().Filter(
		datastore.AddressRefByType(datastore.ContractType(committee_verifier.ContractType)),
		datastore.AddressRefByChainSelector(chainSelector),
	)

	if len(refs) == 0 {
		return nil, nil
	}

	stellarChains := env.BlockChains.StellarChains()
	chain, ok := stellarChains[chainSelector]
	if !ok {
		return nil, fmt.Errorf("Stellar chain %d not found in environment", chainSelector)
	}

	kp, err := keypair.Random()
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral keypair: %w", err)
	}
	deployer := stellardeployment.NewDeployer(chain.Client, chain.NetworkPassphrase, kp)

	states := make([]*ccvdeploymentadapters.CommitteeState, 0, len(refs))
	for _, ref := range refs {
		contractID, err := hexToStellarContractID(ref.Address)
		if err != nil {
			return nil, fmt.Errorf("convert address %s to Stellar contract ID: %w", ref.Address, err)
		}

		client := ccvbindings.NewCommitteeVerifierClient(deployer, contractID)
		configs, err := client.GetAllSignatureConfigs(ctx)
		if err != nil {
			return nil, fmt.Errorf("get signature configs from %s on chain %d: %w", ref.Address, chainSelector, err)
		}

		sigConfigs := make([]ccvdeploymentadapters.SignatureConfig, 0, len(configs))
		for _, cfg := range configs {
			signers := make([]string, 0, len(cfg.Signers))
			for _, signer := range cfg.Signers {
				signers = append(signers, common.BytesToAddress(signer[12:32]).Hex())
			}
			sigConfigs = append(sigConfigs, ccvdeploymentadapters.SignatureConfig{
				SourceChainSelector: cfg.SourceChainSelector,
				Signers:             signers,
				Threshold:           uint8(cfg.Threshold),
			})
		}

		states = append(states, &ccvdeploymentadapters.CommitteeState{
			Qualifier:        ref.Qualifier,
			ChainSelector:    chainSelector,
			Address:          ref.Address,
			SignatureConfigs: sigConfigs,
		})
	}

	return states, nil
}

func (a *StellarCCVDeploymentAggregatorConfigAdapter) ResolveVerifierAddress(
	ds datastore.DataStore,
	chainSelector uint64,
	qualifier string,
) (string, error) {
	return dsutils.FindAndFormatFirstRef(ds, chainSelector,
		func(r datastore.AddressRef) (string, error) { return r.Address, nil },
		datastore.AddressRef{
			Type:      datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType),
			Qualifier: qualifier,
		},
		datastore.AddressRef{
			Type:      datastore.ContractType(committee_verifier.ContractType),
			Qualifier: qualifier,
		},
	)
}

// StellarCCVDeploymentExecutorConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.ExecutorConfigAdapter.
type StellarCCVDeploymentExecutorConfigAdapter struct{}

var _ ccvdeploymentadapters.ExecutorConfigAdapter = (*StellarCCVDeploymentExecutorConfigAdapter)(nil)

func (a *StellarCCVDeploymentExecutorConfigAdapter) GetDeployedChains(ds datastore.DataStore, qualifier string) []uint64 {
	if ds == nil {
		return nil
	}
	refs := ds.Addresses().Filter(
		datastore.AddressRefByQualifier(qualifier),
		datastore.AddressRefByType(datastore.ContractType(proxy.ContractType)),
	)
	seen := make(map[uint64]struct{}, len(refs))
	chains := make([]uint64, 0, len(refs))
	for _, ref := range refs {
		if _, exists := seen[ref.ChainSelector]; exists {
			continue
		}
		family, err := chainsel.GetSelectorFamily(ref.ChainSelector)
		if err != nil || family != chainsel.FamilyStellar {
			continue
		}
		seen[ref.ChainSelector] = struct{}{}
		chains = append(chains, ref.ChainSelector)
	}
	return chains
}

func (a *StellarCCVDeploymentExecutorConfigAdapter) BuildChainConfig(
	ds datastore.DataStore,
	chainSelector uint64,
	qualifier string,
) (executor.ChainConfiguration, error) {
	cfg, err := (&StellarExecutorConfigAdapter{}).BuildChainConfig(ds, chainSelector, qualifier)
	if err != nil {
		return executor.ChainConfiguration{}, err
	}
	return executor.ChainConfiguration{
		DestinationChainConfig: chainaccess.DestinationChainConfig{
			OffRampAddress: cfg.OffRampAddress,
			RmnAddress:     cfg.RmnAddress,
		},
		DefaultExecutorAddress: cfg.ExecutorProxyAddress,
	}, nil
}

// StellarCCVDeploymentVerifierConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.VerifierConfigAdapter.
type StellarCCVDeploymentVerifierConfigAdapter struct{}

var _ ccvdeploymentadapters.VerifierConfigAdapter = (*StellarCCVDeploymentVerifierConfigAdapter)(nil)

func (a *StellarCCVDeploymentVerifierConfigAdapter) GetSignerAddressFamily() string {
	return chainsel.FamilyStellar
}

func (a *StellarCCVDeploymentVerifierConfigAdapter) ResolveVerifierContractAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	committeeQualifier string,
	executorQualifier string,
) (*ccvdeploymentadapters.VerifierContractAddresses, error) {
	addrs, err := (&StellarVerifierConfigAdapter{}).ResolveVerifierContractAddresses(
		ds, chainSelector, committeeQualifier, executorQualifier)
	if err != nil {
		return nil, err
	}
	return &ccvdeploymentadapters.VerifierContractAddresses{
		CommitteeVerifierAddress: addrs.CommitteeVerifierAddress,
		OnRampAddress:            addrs.OnRampAddress,
		ExecutorProxyAddress:     addrs.ExecutorProxyAddress,
		RMNRemoteAddress:         addrs.RMNRemoteAddress,
	}, nil
}

// StellarCCVDeploymentIndexerConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.IndexerConfigAdapter.
type StellarCCVDeploymentIndexerConfigAdapter struct{}

var _ ccvdeploymentadapters.IndexerConfigAdapter = (*StellarCCVDeploymentIndexerConfigAdapter)(nil)

func (a *StellarCCVDeploymentIndexerConfigAdapter) ResolveVerifierAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	qualifier string,
	kind ccvdeploymentadapters.VerifierKind,
) ([]string, error) {
	switch kind {
	case ccvdeploymentadapters.CommitteeVerifierKind:
		return (&StellarIndexerConfigAdapter{}).ResolveVerifierAddresses(
			ds, chainSelector, qualifier, ccipdevenvadapters.VerifierKind(kind))
	default:
		return nil, fmt.Errorf("Stellar does not support verifier kind %q", kind)
	}
}

// StellarCCVDeploymentTokenVerifierConfigAdapter implements
// github.com/smartcontractkit/chainlink-ccv/deployment/adapters.TokenVerifierConfigAdapter.
type StellarCCVDeploymentTokenVerifierConfigAdapter struct{}

var _ ccvdeploymentadapters.TokenVerifierConfigAdapter = (*StellarCCVDeploymentTokenVerifierConfigAdapter)(nil)

func (a *StellarCCVDeploymentTokenVerifierConfigAdapter) ResolveTokenVerifierAddresses(
	ds datastore.DataStore,
	chainSelector uint64,
	cctpQualifier string,
	lombardQualifier string,
) (*ccvdeploymentadapters.TokenVerifierChainAddresses, error) {
	addrs, err := (&StellarTokenVerifierConfigAdapter{}).ResolveTokenVerifierAddresses(
		ds, chainSelector, cctpQualifier, lombardQualifier)
	if err != nil {
		return nil, err
	}
	return &ccvdeploymentadapters.TokenVerifierChainAddresses{
		OnRampAddress:                  addrs.OnRampAddress,
		RMNRemoteAddress:               addrs.RMNRemoteAddress,
		CCTPVerifierAddress:            addrs.CCTPVerifierAddress,
		CCTPVerifierResolverAddress:    addrs.CCTPVerifierResolverAddress,
		LombardVerifierResolverAddress: addrs.LombardVerifierResolverAddress,
	}, nil
}
