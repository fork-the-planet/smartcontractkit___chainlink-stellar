package ccip

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v1_2_0/operations/router"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v1_6_0/operations/rmn_remote"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/committee_verifier"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/fee_quoter"
	offrampoperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/offramp"
	onrampoperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/onramp"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/versioned_verifier_resolver"
	devenvcommon "github.com/smartcontractkit/chainlink-ccv/build/devenv/common"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	"github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellarutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	rrops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/ramp_registry"
)

// MergeExistingAddressRefs upserts CCIP / environment address refs into the
// in-memory deploy datastore (EVM-style ExistingAddresses seeding).
func MergeExistingAddressRefs(ds *datastore.MemoryDataStore, refs []datastore.AddressRef) error {
	for _, ref := range refs {
		if err := ds.AddressRefStore.Upsert(ref); err != nil {
			return fmt.Errorf("merge existing address ref (%s %s): %w", ref.Type, ref.Version, err)
		}
	}
	return nil
}

// UpsertDeployedStrKey records a deployed Soroban contract strkey in the datastore.
func UpsertDeployedStrKey(
	ds *datastore.MemoryDataStore,
	chainSelector uint64,
	typ datastore.ContractType,
	version *semver.Version,
	qualifier string,
	contractStrkey string,
) error {
	hexAddr, err := stellarutil.StrkeyToHex(contractStrkey)
	if err != nil {
		return fmt.Errorf("strkey to hex for %s: %w", typ, err)
	}
	if err := ds.AddressRefStore.Upsert(datastore.AddressRef{
		Address:       hexAddr,
		ChainSelector: chainSelector,
		Type:          typ,
		Version:       version,
		Qualifier:     qualifier,
	}); err != nil {
		return fmt.Errorf("upsert address ref %s: %w", typ, err)
	}
	return nil
}

var (
	onRampDSVersion    = semver.MustParse(onrampoperations.Deploy.Version())
	offRampDSVersion   = semver.MustParse(offrampoperations.Deploy.Version())
	routerDSVersion    = semver.MustParse(router.Deploy.Version())
	feeQuoterDSVersion = semver.MustParse(fee_quoter.Deploy.Version())
	rmnRemoteDSVersion = semver.MustParse(rmn_remote.Deploy.Version())
	tarDSVersion       = semver.MustParse("1.0.0")
	ccipReceiverVer    = semver.MustParse("1.0.0")
	lockReleasePoolVer = semver.MustParse("1.0.0")
)

// RecordOnRamp records an OnRamp deployment in the datastore.
func RecordOnRamp(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(onrampoperations.ContractType), onRampDSVersion, "", contractStrkey)
}

// RecordOffRamp records an OffRamp deployment in the datastore.
func RecordOffRamp(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(offrampoperations.ContractType), offRampDSVersion, "", contractStrkey)
}

// RecordRouter records a Router deployment in the datastore.
func RecordRouter(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(router.ContractType), routerDSVersion, "", contractStrkey)
}

// RecordFeeQuoter records a FeeQuoter deployment in the datastore.
func RecordFeeQuoter(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(fee_quoter.ContractType), feeQuoterDSVersion, "", contractStrkey)
}

// RecordRMNRemote records an RMN Remote deployment in the datastore.
func RecordRMNRemote(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(rmn_remote.ContractType), rmnRemoteDSVersion, "", contractStrkey)
}

// RecordTokenAdminRegistry records TokenAdminRegistry in the datastore.
func RecordTokenAdminRegistry(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(TokenAdminRegistryContractType), tarDSVersion, "", contractStrkey)
}

// RecordVVR records the Versioned Verifier Resolver in the datastore.
func RecordVVR(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType), versioned_verifier_resolver.Version, devenvcommon.DefaultCommitteeVerifierQualifier, contractStrkey)
}

// RecordCommitteeVerifier records the Committee Verifier in the datastore.
func RecordCommitteeVerifier(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(committee_verifier.ContractType), committee_verifier.Version, devenvcommon.DefaultCommitteeVerifierQualifier, contractStrkey)
}

// RecordRampRegistry records the RampRegistry in the datastore.
func RecordRampRegistry(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(rrops.ContractType), stellarops.ContractDeploymentVersion, "", contractStrkey)
}

// RecordCCIPReceiver records the CCIP receiver example in the datastore.
func RecordCCIPReceiver(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(CcipReceiverContractType), ccipReceiverVer, "", contractStrkey)
}

// RecordLockReleasePool records the lock-release pool in the datastore.
func RecordLockReleasePool(ds *datastore.MemoryDataStore, chainSelector uint64, contractStrkey string) error {
	return UpsertDeployedStrKey(ds, chainSelector, datastore.ContractType(LockReleaseTokenPoolContractType), lockReleasePoolVer, DevenvTestTokenPoolQualifier, contractStrkey)
}

// LockReleasePoolAddressRefDataStore returns a sealed datastore containing the
// lock-release pool AddressRef and, when tokenContractID is non-empty, the
// test token AddressRef for devenv (qualifier DevenvTestTokenPoolQualifier).
func LockReleasePoolAddressRefDataStore(chainSelector uint64, poolContractID, tokenContractID string) (datastore.DataStore, error) {
	ds := datastore.NewMemoryDataStore()
	poolHex, err := stellarutil.StrkeyToHex(poolContractID)
	if err != nil {
		return nil, fmt.Errorf("convert pool address: %w", err)
	}
	if err := ds.AddressRefStore.Add(datastore.AddressRef{
		Address:       poolHex,
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(LockReleaseTokenPoolContractType),
		Version:       semver.MustParse("1.0.0"),
		Qualifier:     DevenvTestTokenPoolQualifier,
	}); err != nil {
		return nil, fmt.Errorf("add pool address ref: %w", err)
	}
	if tokenContractID != "" {
		tokenHex, err := stellarutil.StrkeyToHex(tokenContractID)
		if err != nil {
			return nil, fmt.Errorf("convert token address: %w", err)
		}
		if err := ds.AddressRefStore.Add(datastore.AddressRef{
			Address:       tokenHex,
			ChainSelector: chainSelector,
			Type:          datastore.ContractType(TestTokenContractType),
			Version:       semver.MustParse("1.0.0"),
			Qualifier:     DevenvTestTokenPoolQualifier,
		}); err != nil {
			return nil, fmt.Errorf("add token address ref: %w", err)
		}
	}
	return ds.Seal(), nil
}
