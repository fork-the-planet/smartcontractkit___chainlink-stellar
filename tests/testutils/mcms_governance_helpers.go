package helpers

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"path/filepath"
	"testing"
	"time"

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

// EncodeTimelockInvokePayload builds timelock Call.data for a Soroban contract function invocation.
func EncodeTimelockInvokePayload(functionName string, argScVals []xdr.ScVal) ([]byte, error) {
	return mcmsutil.EncodeSorobanMCMSInvokePayload(functionName, argScVals)
}

// WaitTimelockOperationReady polls until the scheduled timelock batch is executable.
func WaitTimelockOperationReady(
	ctx context.Context,
	t *testing.T,
	tlClient *timelockbindings.TimelockClient,
	calls timelockbindings.Calls,
	predecessor, salt [32]byte,
) {
	t.Helper()

	opID, err := tlClient.HashOperationBatch(ctx, calls, predecessor, salt)
	require.NoError(t, err)

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ready, err := tlClient.IsOperationReady(ctx, opID)
		require.NoError(t, err)
		if ready {
			return
		}
		time.Sleep(400 * time.Millisecond)
	}

	ready, err := tlClient.IsOperationReady(ctx, opID)
	require.NoError(t, err)
	require.True(t, ready, "timelock operation never became ready")
}

// MCMSTimelockScheduleAndExecute drives schedule_batch → wait → execute_batch through MCMS
// SetRoot + Execute (two-leaf Merkle tree, EIP-191 signing). Uses the current MCMS op count
// as the schedule nonce; execute uses nonce+1.
func MCMSTimelockScheduleAndExecute(
	t *testing.T,
	ctx context.Context,
	env *E2ETestEnv,
	gov *MCMSGovernanceStack,
	calls timelockbindings.Calls,
	predecessor, salt [32]byte,
) {
	t.Helper()

	preOpCount, err := gov.MCMSClient.GetOpCount(ctx)
	require.NoError(t, err)

	scheduleData, err := SorobanScheduleBatch(gov.MCMSID, calls, predecessor, salt, gov.MinDelaySec)
	require.NoError(t, err)

	validUntil, err := MCMSValidUntilSeconds(ctx, env.RPCClient)
	require.NoError(t, err)

	opSchedule := mcmsbindings.StellarOp{
		ChainId:  gov.ChainNetID,
		Multisig: gov.MCMSRaw,
		Nonce:    preOpCount,
		To:       gov.TimelockRaw,
		Value:    [32]byte{},
		Data:     scheduleData,
	}
	metaSchedule := mcmsbindings.StellarRootMetadata{
		ChainId:              gov.ChainNetID,
		Multisig:             gov.MCMSRaw,
		PreOpCount:           preOpCount,
		PostOpCount:          preOpCount + 1,
		OverridePreviousRoot: false,
	}

	metaLeaf, err := HashRootMetadata(metaSchedule)
	require.NoError(t, err)
	opLeaf, err := HashStellarOp(opSchedule)
	require.NoError(t, err)
	leaves := [2][32]byte{metaLeaf, opLeaf}
	root := MerkleRootTwoLeaves(leaves[0], leaves[1])

	proofMeta := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leaves, 0)}
	sigs, err := SignaturesForSetRoot(gov.SignerPK, root, validUntil)
	require.NoError(t, err)
	require.NoError(t, gov.MCMSClient.SetRoot(ctx, root, validUntil, metaSchedule, proofMeta, sigs))

	proofOp := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leaves, 1)}
	require.NoError(t, gov.MCMSClient.Execute(ctx, opSchedule, proofOp))

	pending, err := gov.TimelockClient.IsOperationPending(ctx, mustHashOperationBatch(t, ctx, gov.TimelockClient, calls, predecessor, salt))
	require.NoError(t, err)
	require.True(t, pending, "expected scheduled operation to be pending")

	WaitTimelockOperationReady(ctx, t, gov.TimelockClient, calls, predecessor, salt)

	execData, err := SorobanExecuteBatch(gov.MCMSID, calls, predecessor, salt)
	require.NoError(t, err)

	execNonce := preOpCount + 1
	opExec := mcmsbindings.StellarOp{
		ChainId:  gov.ChainNetID,
		Multisig: gov.MCMSRaw,
		Nonce:    execNonce,
		To:       gov.TimelockRaw,
		Value:    [32]byte{},
		Data:     execData,
	}
	metaExec := mcmsbindings.StellarRootMetadata{
		ChainId:              gov.ChainNetID,
		Multisig:             gov.MCMSRaw,
		PreOpCount:           execNonce,
		PostOpCount:          execNonce + 1,
		OverridePreviousRoot: false,
	}

	metaLeafExec, err := HashRootMetadata(metaExec)
	require.NoError(t, err)
	opLeafExec, err := HashStellarOp(opExec)
	require.NoError(t, err)
	leavesExec := [2][32]byte{metaLeafExec, opLeafExec}
	rootExec := MerkleRootTwoLeaves(leavesExec[0], leavesExec[1])

	proofMetaExec := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leavesExec, 0)}
	sigsExec, err := SignaturesForSetRoot(gov.SignerPK, rootExec, validUntil)
	require.NoError(t, err)
	require.NoError(t, gov.MCMSClient.SetRoot(ctx, rootExec, validUntil, metaExec, proofMetaExec, sigsExec))

	proofOpExec := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leavesExec, 1)}
	require.NoError(t, gov.MCMSClient.Execute(ctx, opExec, proofOpExec))

	opID := mustHashOperationBatch(t, ctx, gov.TimelockClient, calls, predecessor, salt)
	done, err := gov.TimelockClient.IsOperationDone(ctx, opID)
	require.NoError(t, err)
	require.True(t, done, "expected timelock operation to be done after execute_batch")
}

func mustHashOperationBatch(
	t *testing.T,
	ctx context.Context,
	tlClient *timelockbindings.TimelockClient,
	calls timelockbindings.Calls,
	predecessor, salt [32]byte,
) [32]byte {
	t.Helper()
	opID, err := tlClient.HashOperationBatch(ctx, calls, predecessor, salt)
	require.NoError(t, err)
	return opID
}
