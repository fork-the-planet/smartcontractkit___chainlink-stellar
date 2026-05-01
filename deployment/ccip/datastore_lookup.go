package ccip

import (
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
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	rrops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/ramp_registry"
)

// GetOnRampStrkey resolves the local OnRamp contract strkey for a chain from the datastore.
func GetOnRampStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(onrampoperations.ContractType), semver.MustParse(onrampoperations.Deploy.Version()), "")
}

// GetOffRampStrkey resolves the local OffRamp contract strkey for a chain from the datastore.
func GetOffRampStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(offrampoperations.ContractType), semver.MustParse(offrampoperations.Deploy.Version()), "")
}

// GetRouterStrkey resolves the local Router contract strkey for a chain from the datastore.
func GetRouterStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(router.ContractType), semver.MustParse(router.Deploy.Version()), "")
}

// GetFeeQuoterStrkey resolves the FeeQuoter contract strkey for a chain from the datastore.
func GetFeeQuoterStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(fee_quoter.ContractType), semver.MustParse(fee_quoter.Deploy.Version()), "")
}

// GetTokenAdminRegistryStrkey resolves TokenAdminRegistry for a chain from the datastore.
func GetTokenAdminRegistryStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(TokenAdminRegistryContractType), semver.MustParse("1.0.0"), "")
}

// GetVVRStrkey resolves the Versioned Verifier Resolver strkey for a chain from the datastore.
func GetVVRStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(versioned_verifier_resolver.CommitteeVerifierResolverType), versioned_verifier_resolver.Version, devenvcommon.DefaultCommitteeVerifierQualifier)
}

// GetCommitteeVerifierStrkey resolves the Committee Verifier strkey for a chain from the datastore.
func GetCommitteeVerifierStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(committee_verifier.ContractType), committee_verifier.Version, devenvcommon.DefaultCommitteeVerifierQualifier)
}

// GetRampRegistryStrkey resolves the RampRegistry strkey for a chain from the datastore.
func GetRampRegistryStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(rrops.ContractType), stellarops.ContractDeploymentVersion, "")
}

// GetRMNRemoteStrkey resolves RMN Remote strkey for a chain from the datastore.
func GetRMNRemoteStrkey(ds datastore.DataStore, chainSelector uint64) (string, error) {
	return LookupStellarContractStrkey(ds, chainSelector, datastore.ContractType(rmn_remote.ContractType), semver.MustParse(rmn_remote.Deploy.Version()), "")
}
