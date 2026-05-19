package helpers

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/stretchr/testify/require"

	mcmsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/mcms"
	timelockbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/timelock"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/deployment/mcmsutil"
	stellarops "github.com/smartcontractkit/chainlink-stellar/deployment/operations"
	mcmsops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/mcms"
	"github.com/smartcontractkit/chainlink-stellar/deployment/operations/stellardeps"
	timelockops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/timelock"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const DefaultMCMSTimelockMinDelaySec uint64 = 3

// MCMSGovernanceStack holds deployed MCMS + timelock contracts wired for MCMS-mediated governance.
type MCMSGovernanceStack struct {
	MCMSID         string
	TimelockID     string
	MCMSClient     *mcmsbindings.McmsClient
	TimelockClient *timelockbindings.TimelockClient
	MCMSRaw        [32]byte
	TimelockRaw    [32]byte
	ChainNetID     [32]byte
	SignerPK       *ecdsa.PrivateKey
	MinDelaySec    uint64
}

// ContractIDToBytes32 decodes a Soroban contract strkey into a 32-byte contract id.
func ContractIDToBytes32(contractID string) ([32]byte, error) {
	var out [32]byte
	raw, err := strkey.Decode(strkey.VersionByteContract, contractID)
	if err != nil {
		return out, err
	}
	if len(raw) != 32 {
		return out, fmt.Errorf("contract id raw length %d, want 32", len(raw))
	}
	copy(out[:], raw)
	return out, nil
}

// SorobanScheduleBatch encodes timelock schedule_batch Call.data for MCMS StellarOp payloads.
func SorobanScheduleBatch(
	caller string,
	calls timelockbindings.Calls,
	predecessor, salt [32]byte,
	delay uint64,
) ([]byte, error) {
	callsVal, err := calls.ToScVal()
	if err != nil {
		return nil, err
	}
	val := scval.VecToScVal([]xdr.ScVal{
		scval.SymbolToScVal("schedule_batch"),
		scval.AddressToScVal(caller),
		callsVal,
		scval.Bytes32ToScVal(predecessor),
		scval.Bytes32ToScVal(salt),
		scval.Uint64ToScVal(delay),
	})
	return val.MarshalBinary()
}

// SorobanExecuteBatch encodes timelock execute_batch Call.data for MCMS StellarOp payloads.
func SorobanExecuteBatch(
	caller string,
	calls timelockbindings.Calls,
	predecessor, salt [32]byte,
) ([]byte, error) {
	callsVal, err := calls.ToScVal()
	if err != nil {
		return nil, err
	}
	val := scval.VecToScVal([]xdr.ScVal{
		scval.SymbolToScVal("execute_batch"),
		scval.AddressToScVal(caller),
		callsVal,
		scval.Bytes32ToScVal(predecessor),
		scval.Bytes32ToScVal(salt),
	})
	return val.MarshalBinary()
}

// MCMSValidUntilSeconds returns a deadline for MCMS set_root: must be >= host ledger timestamp
// and ≤ now + 90d (contracts/mcms MAX_ROOT_VALIDITY_SECS).
func MCMSValidUntilSeconds(ctx context.Context, rpc *rpcclient.Client) (uint32, error) {
	latest, err := rpc.GetLatestLedger(ctx)
	if err != nil {
		return 0, fmt.Errorf("GetLatestLedger: %w", err)
	}
	now := latest.LedgerCloseTime
	if now < 0 {
		return 0, fmt.Errorf("unexpected negative ledger close time: %d", now)
	}
	const marginSec int64 = 90 * 24 * 3600 // must stay ≤ MCMS MAX_ROOT_VALIDITY_SECS (contracts/mcms)
	sum := now + marginSec
	maxU32 := int64(^uint32(0))
	if sum > maxU32 {
		return ^uint32(0), nil
	}
	return uint32(sum), nil
}

// DeployMCMSAndTimelock deploys and initializes MCMS + RBAC timelock via CLDF operations
// (mcmsops/timelockops Deploy, Initialize, SetConfig), using mcmsutil WASM paths and deploy salts.
//
// MCMS is configured as both proposer and executor on the timelock so MCMS.execute can drive
// schedule_batch and execute_batch (same wiring as integration TestMcmsMerkleTimelockScheduleAndExecute).
func DeployMCMSAndTimelock(
	t *testing.T,
	ctx context.Context,
	env *E2ETestEnv,
	chainSelector uint64,
	qualifier string,
) *MCMSGovernanceStack {
	t.Helper()

	pk, err := crypto.HexToECDSA(Anvil0SKHex)
	require.NoError(t, err)

	chainNetID := mcmsutil.ChainNetworkID(env.NetworkPassphrase)

	projectRoot := FindProjectRoot(t)
	mcmsWasm := filepath.Join(projectRoot, mcmsutil.DefaultMCMSWasmRelative)
	tlWasm := filepath.Join(projectRoot, mcmsutil.DefaultTimelockWasmRelative)

	bundle := cldfops.NewBundle(
		func() context.Context { return ctx },
		cldflogger.Test(t),
		cldfops.NewMemoryReporter(),
	)
	deps := stellardeps.FromDeployer(env.Deployer)

	mcmsDep, err := cldfops.ExecuteOperation(bundle, mcmsops.Deploy, deps, stellarops.DeployInput{
		WasmPath: mcmsWasm,
		Salt:     mcmsutil.MCMSDeploySalt(chainSelector, qualifier),
	})
	require.NoError(t, err)
	mcmsID := mcmsDep.Output.ContractID

	_, err = cldfops.ExecuteOperation(bundle, mcmsops.Initialize, deps, mcmsops.InitializeInput{
		ContractID:     mcmsID,
		Owner:          env.DeployerKP.Address(),
		ChainNetworkID: chainNetID,
	})
	require.NoError(t, err)

	var groupQuorums [32]byte
	groupQuorums[0] = 1
	var groupParents [32]byte
	paddedSigner := PaddedEthAddress(&pk.PublicKey)
	_, err = cldfops.ExecuteOperation(bundle, mcmsops.SetConfig, deps, mcmsops.SetConfigInput{
		ContractID:      mcmsID,
		SignerAddresses: mcmsbindings.SignerAddresses{Inner: [][32]byte{paddedSigner}},
		SignerGroups:    mcmsbindings.SignerGroups{Inner: []uint32{0}},
		GroupQuorums:    groupQuorums,
		GroupParents:    groupParents,
		ClearRoot:       true,
	})
	require.NoError(t, err)

	tlDep, err := cldfops.ExecuteOperation(bundle, timelockops.Deploy, deps, stellarops.DeployInput{
		WasmPath: tlWasm,
		Salt:     mcmsutil.TimelockDeploySalt(chainSelector, qualifier),
	})
	require.NoError(t, err)
	tlID := tlDep.Output.ContractID

	minDelay := DefaultMCMSTimelockMinDelaySec
	_, err = cldfops.ExecuteOperation(bundle, timelockops.Initialize, deps, timelockops.InitializeInput{
		ContractID: tlID,
		MinDelay:   minDelay,
		Admin:      env.DeployerKP.Address(),
		Proposers:  []string{mcmsID},
		Executors:  []string{mcmsID},
		Cancellers: []string{},
		Bypassers:  []string{},
	})
	require.NoError(t, err)

	mcmsRaw, err := ContractIDToBytes32(mcmsID)
	require.NoError(t, err)
	tlRaw, err := ContractIDToBytes32(tlID)
	require.NoError(t, err)

	return &MCMSGovernanceStack{
		MCMSID:         mcmsID,
		TimelockID:     tlID,
		MCMSClient:     mcmsbindings.NewMcmsClient(env.Deployer, mcmsID),
		TimelockClient: timelockbindings.NewTimelockClient(env.Deployer, tlID),
		MCMSRaw:        mcmsRaw,
		TimelockRaw:    tlRaw,
		ChainNetID:     chainNetID,
		SignerPK:       pk,
		MinDelaySec:    minDelay,
	}
}
