//go:build integration

package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	chainsel "github.com/smartcontractkit/chain-selectors"

	mcmsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/mcms"
	timelockbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/timelock"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func sorobanScheduleBatch(
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

func sorobanExecuteBatch(
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

// mcmsValidUntilSeconds returns a deadline for MCMS set_root: must be >= host ledger timestamp
// and ≤ now + 90d (contracts/mcms MAX_ROOT_VALIDITY_SECS).
func mcmsValidUntilSeconds(ctx context.Context, rpc *rpcclient.Client) (uint32, error) {
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

// TestMcmsMerkleTimelockScheduleAndExecute wires MCMS SetRoot + Execute into timelock schedule_batch
// and execute_batch after min delay (Go Merkle + EIP-191).
//
// Caller identity matches EVM-style wiring: the ManyChainMultiSig contract invokes the timelock, so
// msg.sender-style authority is the multisig contract. On Soroban, schedule_batch and execute_batch
// pass caller = MCMS contract address; MCMS holds PROPOSER and EXECUTOR on the timelock so nested
// require_auth() succeeds when MCMS.execute invokes the timelock (see contracts/timelock roles.rs).
func TestMcmsMerkleTimelockScheduleAndExecute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, rpcClient, _, _ := GetSharedTestEnv(ctx, t)

	pk, err := crypto.HexToECDSA(anvil0SKHex)
	if err != nil {
		t.Fatalf("HexToECDSA: %v", err)
	}
	paddedSigner := PaddedEthAddress(&pk.PublicKey)

	chainNetID, err := chainNetworkIDFromHex(chainsel.STELLAR_LOCALNET.ChainID)
	if err != nil {
		t.Fatalf("chain network id: %v", err)
	}

	deploy := func(name, wasm string) string {
		t.Helper()
		salt := deployment.GenerateDeterministicSalt(deployerKP.Address(), name)
		p := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", wasm)
		id, err := deployer.DeployContract(ctx, p, salt)
		if err != nil {
			t.Fatalf("deploy %s: %v", name, err)
		}
		return id
	}

	mcmsID := deploy("mcms-tl-mcms", "mcms.wasm")
	tlID := deploy("mcms-tl-timelock", "timelock.wasm")

	mcmsClient := mcmsbindings.NewMcmsClient(deployer, mcmsID)
	tlClient := timelockbindings.NewTimelockClient(deployer, tlID)

	if err := mcmsClient.Initialize(ctx, deployerKP.Address(), chainNetID); err != nil {
		t.Fatalf("MCMS Initialize: %v", err)
	}

	var groupQuorums [32]byte
	groupQuorums[0] = 1
	var groupParents [32]byte
	if err := mcmsClient.SetConfig(ctx,
		mcmsbindings.SignerAddresses{Inner: [][32]byte{paddedSigner}},
		mcmsbindings.SignerGroups{Inner: []uint32{0}},
		groupQuorums,
		groupParents,
		false,
	); err != nil {
		t.Fatalf("MCMS SetConfig: %v", err)
	}

	const minDelaySec uint64 = 3
	// Admin is deployer; MCMS is both proposer and executor so MCMS-mediated ops authenticate (same
	// logical caller as ManyChainMultiSig calling RBACTimelock on EVM).
	if err := tlClient.Initialize(ctx, minDelaySec, deployerKP.Address(),
		[]string{mcmsID},
		[]string{mcmsID},
		[]string{},
		[]string{},
	); err != nil {
		t.Fatalf("Timelock Initialize: %v", err)
	}

	mcmsRaw, err := contractIDToBytes32(mcmsID)
	if err != nil {
		t.Fatalf("mcms id bytes: %v", err)
	}
	tlRaw, err := contractIDToBytes32(tlID)
	if err != nil {
		t.Fatalf("timelock id bytes: %v", err)
	}

	var predecessor [32]byte
	var saltSched [32]byte
	saltSched[31] = 42

	emptyCalls := timelockbindings.Calls{Inner: []timelockbindings.Call{}}

	scheduleData, err := sorobanScheduleBatch(mcmsID, emptyCalls, predecessor, saltSched, minDelaySec)
	if err != nil {
		t.Fatalf("encode schedule_batch: %v", err)
	}

	validUntil, err := mcmsValidUntilSeconds(ctx, rpcClient)
	if err != nil {
		t.Fatalf("mcms valid_until: %v", err)
	}

	opSchedule := mcmsbindings.StellarOp{
		ChainId:  chainNetID,
		Multisig: mcmsRaw,
		Nonce:    0,
		To:       tlRaw,
		Value:    [32]byte{},
		Data:     scheduleData,
	}
	meta1 := mcmsbindings.StellarRootMetadata{
		ChainId:              chainNetID,
		Multisig:             mcmsRaw,
		PreOpCount:           0,
		PostOpCount:          1,
		OverridePreviousRoot: false,
	}

	metaLeaf1, err := HashRootMetadata(meta1)
	if err != nil {
		t.Fatalf("hash meta1: %v", err)
	}
	opLeaf1, err := HashStellarOp(opSchedule)
	if err != nil {
		t.Fatalf("hash op schedule: %v", err)
	}
	leaves1 := [2][32]byte{metaLeaf1, opLeaf1}
	root1 := MerkleRootTwoLeaves(leaves1[0], leaves1[1])

	proofMeta1 := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leaves1, 0)}
	sigs1, err := SignaturesForSetRoot(pk, root1, validUntil)
	if err != nil {
		t.Fatalf("sign set_root 1: %v", err)
	}
	if err := mcmsClient.SetRoot(ctx, root1, validUntil, meta1, proofMeta1, sigs1); err != nil {
		t.Fatalf("SetRoot 1: %v", err)
	}

	proofOp1 := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leaves1, 1)}
	if err := mcmsClient.Execute(ctx, opSchedule, proofOp1); err != nil {
		t.Fatalf("Execute schedule_batch: %v", err)
	}

	opID, err := tlClient.HashOperationBatch(ctx, emptyCalls, predecessor, saltSched)
	if err != nil {
		t.Fatalf("HashOperationBatch: %v", err)
	}
	pending, err := tlClient.IsOperationPending(ctx, opID)
	if err != nil || !pending {
		t.Fatalf("expected pending scheduled op: pending=%v err=%v", pending, err)
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ready, err := tlClient.IsOperationReady(ctx, opID)
		if err != nil {
			t.Fatalf("IsOperationReady: %v", err)
		}
		if ready {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	okReady, err := tlClient.IsOperationReady(ctx, opID)
	if err != nil || !okReady {
		t.Fatalf("scheduled op never became ready: ready=%v err=%v", okReady, err)
	}

	execData, err := sorobanExecuteBatch(mcmsID, emptyCalls, predecessor, saltSched)
	if err != nil {
		t.Fatalf("encode execute_batch: %v", err)
	}

	opExec := mcmsbindings.StellarOp{
		ChainId:  chainNetID,
		Multisig: mcmsRaw,
		Nonce:    1,
		To:       tlRaw,
		Value:    [32]byte{},
		Data:     execData,
	}
	meta2 := mcmsbindings.StellarRootMetadata{
		ChainId:              chainNetID,
		Multisig:             mcmsRaw,
		PreOpCount:           1,
		PostOpCount:          2,
		OverridePreviousRoot: false,
	}

	metaLeaf2, err := HashRootMetadata(meta2)
	if err != nil {
		t.Fatalf("hash meta2: %v", err)
	}
	opLeaf2, err := HashStellarOp(opExec)
	if err != nil {
		t.Fatalf("hash op exec: %v", err)
	}
	leaves2 := [2][32]byte{metaLeaf2, opLeaf2}
	root2 := MerkleRootTwoLeaves(leaves2[0], leaves2[1])

	proofMeta2 := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leaves2, 0)}
	sigs2, err := SignaturesForSetRoot(pk, root2, validUntil)
	if err != nil {
		t.Fatalf("sign set_root 2: %v", err)
	}
	if err := mcmsClient.SetRoot(ctx, root2, validUntil, meta2, proofMeta2, sigs2); err != nil {
		t.Fatalf("SetRoot 2: %v", err)
	}

	proofOp2 := mcmsbindings.MerkleProof{Inner: MerkleProofTwoLeaves(leaves2, 1)}
	if err := mcmsClient.Execute(ctx, opExec, proofOp2); err != nil {
		t.Fatalf("Execute execute_batch: %v", err)
	}

	done, err := tlClient.IsOperationDone(ctx, opID)
	if err != nil || !done {
		t.Fatalf("expected operation done: done=%v err=%v", done, err)
	}
}
