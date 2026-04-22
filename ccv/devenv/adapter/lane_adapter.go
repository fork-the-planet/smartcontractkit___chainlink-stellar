package adapter

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/Masterminds/semver/v3"

	routeroperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v1_2_0/operations/router"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/fee_quoter"
	offrampoperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/offramp"
	onrampoperations "github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/onramp"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/deployment/v2_0_0/operations/proxy"
	"github.com/smartcontractkit/chainlink-ccip/deployment/finality"
	"github.com/smartcontractkit/chainlink-ccip/deployment/lanes"
	datastore_utils "github.com/smartcontractkit/chainlink-ccip/deployment/utils/datastore"
	seq_core "github.com/smartcontractkit/chainlink-ccip/deployment/utils/sequences"
	ccvadapters "github.com/smartcontractkit/chainlink-ccip/deployment/v2_0_0/adapters"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf_ops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// StellarLaneAdapter implements lanes.LaneAdapter for the Stellar chain family.
// Stellar lane configuration is performed during DeployContractsForSelector,
// so the sequences here are intentional no-ops.
type StellarLaneAdapter struct{}

var (
	_ lanes.LaneAdapter       = (*StellarLaneAdapter)(nil)
	_ ccvadapters.ChainFamily = (*StellarLaneAdapter)(nil)
)

// stellarFeeQuoterChainFamilySelector is bytes4(keccak256("CCIP ChainFamilySelector EVM")).
// It is used as a stand-in for Stellar until a Stellar-specific selector is registered
// in the EVM FeeQuoter contract (see ccv/chain/chain.go).
var stellarFeeQuoterChainFamilySelector = [4]byte{0x28, 0x12, 0xd5, 0x2c}

var stellarNoOpSource = cldf_ops.NewSequence(
	"StellarConfigureLaneLegAsSource",
	semver.MustParse("2.0.0"),
	"No-op: Stellar source lane config is applied during contract deployment",
	func(_ cldf_ops.Bundle, _ cldf_chain.BlockChains, _ lanes.UpdateLanesInput) (seq_core.OnChainOutput, error) {
		return seq_core.OnChainOutput{}, nil
	},
)

var stellarNoOpDest = cldf_ops.NewSequence(
	"StellarConfigureLaneLegAsDest",
	semver.MustParse("2.0.0"),
	"No-op: Stellar dest lane config is applied during contract deployment",
	func(_ cldf_ops.Bundle, _ cldf_chain.BlockChains, _ lanes.UpdateLanesInput) (seq_core.OnChainOutput, error) {
		return seq_core.OnChainOutput{}, nil
	},
)

var stellarNoOpDisable = cldf_ops.NewSequence(
	"StellarDisableRemoteChain",
	semver.MustParse("2.0.0"),
	"No-op: Stellar disable remote chain",
	func(_ cldf_ops.Bundle, _ cldf_chain.BlockChains, _ lanes.DisableRemoteChainInput) (seq_core.OnChainOutput, error) {
		return seq_core.OnChainOutput{}, nil
	},
)

func (a *StellarLaneAdapter) ConfigureLaneLegAsSource() *cldf_ops.Sequence[lanes.UpdateLanesInput, seq_core.OnChainOutput, cldf_chain.BlockChains] {
	return stellarNoOpSource
}

func (a *StellarLaneAdapter) ConfigureLaneLegAsDest() *cldf_ops.Sequence[lanes.UpdateLanesInput, seq_core.OnChainOutput, cldf_chain.BlockChains] {
	return stellarNoOpDest
}

func (a *StellarLaneAdapter) DisableRemoteChain() *cldf_ops.Sequence[lanes.DisableRemoteChainInput, seq_core.OnChainOutput, cldf_chain.BlockChains] {
	return stellarNoOpDisable
}

func toStellarAddressBytes(ref datastore.AddressRef) ([]byte, error) {
	addr := strings.TrimPrefix(ref.Address, "0x")
	b, err := hex.DecodeString(addr)
	if err != nil {
		return nil, fmt.Errorf("decode stellar hex address %q: %w", ref.Address, err)
	}
	return b, nil
}

func (a *StellarLaneAdapter) GetOnRampAddress(ds datastore.DataStore, chainSelector uint64) ([]byte, error) {
	return datastore_utils.FindAndFormatRef(ds, datastore.AddressRef{
		Type:    datastore.ContractType(onrampoperations.ContractType),
		Version: semver.MustParse(onrampoperations.Deploy.Version()),
	}, chainSelector, toStellarAddressBytes)
}

func (a *StellarLaneAdapter) GetOffRampAddress(ds datastore.DataStore, chainSelector uint64) ([]byte, error) {
	return datastore_utils.FindAndFormatRef(ds, datastore.AddressRef{
		Type:    datastore.ContractType(offrampoperations.ContractType),
		Version: semver.MustParse(offrampoperations.Deploy.Version()),
	}, chainSelector, toStellarAddressBytes)
}

func (a *StellarLaneAdapter) GetRouterAddress(ds datastore.DataStore, chainSelector uint64) ([]byte, error) {
	return datastore_utils.FindAndFormatRef(ds, datastore.AddressRef{
		Type:    datastore.ContractType(routeroperations.ContractType),
		Version: routeroperations.Version,
	}, chainSelector, toStellarAddressBytes)
}

func (a *StellarLaneAdapter) GetFQAddress(ds datastore.DataStore, chainSelector uint64) ([]byte, error) {
	return datastore_utils.FindAndFormatRef(ds, datastore.AddressRef{
		Type:    datastore.ContractType(fee_quoter.ContractType),
		Version: semver.MustParse(fee_quoter.Deploy.Version()),
	}, chainSelector, toStellarAddressBytes)
}

func (a *StellarLaneAdapter) GetFeeQuoterDestChainConfig() lanes.FeeQuoterDestChainConfig {
	return lanes.FeeQuoterDestChainConfig{
		IsEnabled:                   true,
		MaxDataBytes:                50_000,
		MaxPerMsgGasLimit:           4_000_000,
		DestGasOverhead:             350_000,
		DestGasPerPayloadByteBase:   16,
		ChainFamilySelector:         binary.BigEndian.Uint32(stellarFeeQuoterChainFamilySelector[:]),
		DefaultTokenFeeUSDCents:     50,
		DefaultTokenDestGasOverhead: 50_000,
		DefaultTxGasLimit:           200_000,
		NetworkFeeUSDCents:          100,
		V1Params: &lanes.FeeQuoterV1Params{
			MaxNumberOfTokensPerMsg:           10,
			DestGasPerPayloadByteHigh:         40,
			DestGasPerPayloadByteThreshold:    3000,
			DestDataAvailabilityOverheadGas:   100,
			DestGasPerDataAvailabilityByte:    16,
			DestDataAvailabilityMultiplierBps: 1,
			GasMultiplierWeiPerEth:            11e17,
		},
		V2Params: &lanes.FeeQuoterV2Params{
			LinkFeeMultiplierPercent: 90,
			USDPerUnitGas:            big.NewInt(1e6),
		},
	}
}

func (a *StellarLaneAdapter) GetDefaultGasPrice() *big.Int {
	return big.NewInt(1e9)
}

// ---------------------------------------------------------------------------
// ccvadapters.ChainFamily implementation
// ---------------------------------------------------------------------------

var stellarNoOpConfigureChainForLanes = cldf_ops.NewSequence(
	"StellarConfigureChainForLanes",
	semver.MustParse("2.0.0"),
	"No-op: Stellar lane config is applied during contract deployment",
	func(_ cldf_ops.Bundle, _ cldf_chain.BlockChains, _ ccvadapters.ConfigureChainForLanesInput) (seq_core.OnChainOutput, error) {
		return seq_core.OnChainOutput{}, nil
	},
)

func (a *StellarLaneAdapter) ConfigureChainForLanes() *cldf_ops.Sequence[ccvadapters.ConfigureChainForLanesInput, seq_core.OnChainOutput, cldf_chain.BlockChains] {
	return stellarNoOpConfigureChainForLanes
}

func (a *StellarLaneAdapter) AddressRefToBytes(ref datastore.AddressRef) ([]byte, error) {
	return toStellarAddressBytes(ref)
}

func (a *StellarLaneAdapter) GetTestRouter(ds datastore.DataStore, chainSelector uint64) ([]byte, error) {
	return a.GetRouterAddress(ds, chainSelector)
}

func (a *StellarLaneAdapter) ResolveExecutor(ds datastore.DataStore, chainSelector uint64, qualifier string) (string, error) {
	toAddress := func(ref datastore.AddressRef) (string, error) { return ref.Address, nil }
	return datastore_utils.FindAndFormatRef(ds, datastore.AddressRef{
		Type:      datastore.ContractType(proxy.ContractType),
		Version:   proxy.Version,
		Qualifier: qualifier,
	}, chainSelector, toAddress)
}

func (a *StellarLaneAdapter) GetAddressBytesLength() uint8 {
	// Stellar contract IDs are 32 bytes on-chain / in cross-chain payloads.
	return 32
}

func (a *StellarLaneAdapter) GetChainFamilySelector() [4]byte {
	return stellarFeeQuoterChainFamilySelector
}

func (a *StellarLaneAdapter) GetDefaultFeeQuoterDestChainConfig() ccvadapters.FeeQuoterDestChainConfig {
	return ccvadapters.FeeQuoterDestChainConfig{
		IsEnabled:                   true,
		MaxDataBytes:                50_000,
		MaxPerMsgGasLimit:           4_000_000,
		DestGasOverhead:             350_000,
		DestGasPerPayloadByteBase:   16,
		ChainFamilySelector:         stellarFeeQuoterChainFamilySelector,
		DefaultTokenFeeUSDCents:     50,
		DefaultTokenDestGasOverhead: 50_000,
		DefaultTxGasLimit:           200_000,
		NetworkFeeUSDCents:          100,
		LinkFeeMultiplierPercent:    90,
		USDPerUnitGas:               big.NewInt(1e6),
	}
}

func (a *StellarLaneAdapter) GetDefaultRemoteChainConfig() ccvadapters.RemoteChainDefaults {
	return ccvadapters.RemoteChainDefaults{
		AllowTrafficFrom:          true,
		ExecutorDestChainConfig:   ccvadapters.ExecutorDestChainConfig{USDCentsFee: 0, Enabled: true},
		BaseExecutionGasCost:      175_000,
		TokenReceiverAllowed:      false,
		MessageNetworkFeeUSDCents: 10,
		TokenNetworkFeeUSDCents:   25,
	}
}

func (a *StellarLaneAdapter) GetDefaultCommitteeVerifierRemoteChainConfig() ccvadapters.CommitteeVerifierRemoteChainDefaults {
	return ccvadapters.CommitteeVerifierRemoteChainDefaults{
		AllowlistEnabled:   false,
		FeeUSDCents:        0,
		GasForVerification: 60_000,
		PayloadSizeBytes:   390,
	}
}

func (a *StellarLaneAdapter) GetDefaultFinalityConfig() finality.Config {
	return finality.Config{
		WaitForFinality: true,
		WaitForSafe:     true,
		BlockDepth:      1,
	}
}
