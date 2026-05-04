//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	rampbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/ramp_registry"
	timelockbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/timelock"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	helpers "github.com/smartcontractkit/chainlink-stellar/tests/testutils"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// sorobanInvokePayload encodes timelock Call.data as XDR for ScVec([Symbol(fn), ...args]),
// matching contracts/common/helpers/src/soroban_invoke.rs (decode_invoke_payload).
func sorobanInvokePayload(fnName string, args ...xdr.ScVal) ([]byte, error) {
	items := make([]xdr.ScVal, 0, 1+len(args))
	items = append(items, scval.SymbolToScVal(fnName))
	items = append(items, args...)
	val := scval.VecToScVal(items)
	return val.MarshalBinary()
}

// encodeApplyRampUpdatesSingleOnramp builds timelock Call.data for ramp registry
// apply_ramp_updates(Vec<OnRampEntry>, Vec<OffRampEntry>, Vec<OffRampEntry>) with one on-ramp upsert.
func encodeApplyRampUpdatesSingleOnramp(destChainSelector uint64, onramp string) ([]byte, error) {
	update := rampbindings.OnRampEntry{
		DestChainSelector: destChainSelector,
		Onramp:            onramp,
	}
	return sorobanInvokePayload(
		"apply_ramp_updates",
		scval.StructSliceToScVal([]rampbindings.OnRampEntry{update}),
		scval.StructSliceToScVal([]rampbindings.OffRampEntry{}),
		scval.StructSliceToScVal([]rampbindings.OffRampEntry{}),
	)
}

func contractIDToBytes32(contractID string) ([32]byte, error) {
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

func randSalt(t *testing.T) [32]byte {
	t.Helper()
	var s [32]byte
	if _, err := rand.Read(s[:]); err != nil {
		t.Fatalf("rand salt: %v", err)
	}
	return s
}

// Uses ccip-ramp-registry as the Ownable target: transfer_ownership → timelock schedules
// accept_ownership → execute; owner-only apply_ramp_updates is denied for the former owner and strangers,
// and only succeeds via schedule → wait → execute with PROPOSER / EXECUTOR roles.
func TestGovernanceTimelockRampRegistry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, rpcClient, passphrase, friendbotURL := GetSharedTestEnv(ctx, t)

	proposerKP := keypair.MustRandom()
	executorKP := keypair.MustRandom()
	strangerKP := keypair.MustRandom()
	for _, label := range []struct {
		name string
		kp   *keypair.Full
	}{
		{"proposer", proposerKP},
		{"executor", executorKP},
		{"stranger", strangerKP},
	} {
		if err := helpers.FundViaFriendbot(friendbotURL, label.kp.Address()); err != nil {
			t.Fatalf("Friendbot fund %s: %v", label.name, err)
		}
	}

	proposerDep := deployment.NewDeployer(rpcClient, passphrase, proposerKP)
	executorDep := deployment.NewDeployer(rpcClient, passphrase, executorKP)
	strangerDep := deployment.NewDeployer(rpcClient, passphrase, strangerKP)

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

	registryID := deploy("gov-tl-ramp-registry", "ccip_ramp_registry.wasm")
	timelockID := deploy("gov-tl-timelock", "timelock.wasm")

	tlAdmin := timelockbindings.NewTimelockClient(deployer, timelockID)
	tlProposer := timelockbindings.NewTimelockClient(proposerDep, timelockID)
	tlExecutor := timelockbindings.NewTimelockClient(executorDep, timelockID)
	tlStranger := timelockbindings.NewTimelockClient(strangerDep, timelockID)

	const minDelaySec uint64 = 3

	if err := tlAdmin.Initialize(ctx, minDelaySec, deployerKP.Address(),
		[]string{proposerKP.Address()},
		[]string{executorKP.Address()},
		[]string{},
		[]string{},
	); err != nil {
		t.Fatalf("Timelock Initialize: %v", err)
	}

	reg := rampbindings.NewRampRegistryClient(deployer, registryID)
	if err := reg.Initialize(ctx, deployerKP.Address()); err != nil {
		t.Fatalf("RampRegistry Initialize: %v", err)
	}

	if err := reg.TransferOwnership(ctx, timelockID); err != nil {
		t.Fatalf("TransferOwnership to timelock: %v", err)
	}
	pending, err := reg.GetPendingOwner(ctx)
	if err != nil {
		t.Fatalf("GetPendingOwner: %v", err)
	}
	if pending == nil || *pending != timelockID {
		t.Fatalf("pending owner = %v, want timelock %s", pending, timelockID)
	}

	acceptData, err := sorobanInvokePayload("accept_ownership")
	if err != nil {
		t.Fatalf("encode accept_ownership payload: %v", err)
	}
	registryRaw, err := contractIDToBytes32(registryID)
	if err != nil {
		t.Fatalf("registry id bytes: %v", err)
	}

	var predecessor [32]byte
	saltAccept := randSalt(t)
	callsAccept := timelockbindings.Calls{
		Inner: []timelockbindings.Call{
			{To: registryRaw, Data: acceptData},
		},
	}

	if err := tlProposer.ScheduleBatch(ctx, proposerKP.Address(), callsAccept, predecessor, saltAccept, minDelaySec); err != nil {
		t.Fatalf("ScheduleBatch accept_ownership: %v", err)
	}

	opIDAccept, err := tlAdmin.HashOperationBatch(ctx, callsAccept, predecessor, saltAccept)
	if err != nil {
		t.Fatalf("HashOperationBatch accept: %v", err)
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ready, err := tlAdmin.IsOperationReady(ctx, opIDAccept)
		if err != nil {
			t.Fatalf("IsOperationReady: %v", err)
		}
		if ready {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}

	okAccept, err := tlAdmin.IsOperationReady(ctx, opIDAccept)
	if err != nil || !okAccept {
		t.Fatalf("accept operation never became ready: ready=%v err=%v", okAccept, err)
	}

	if err := tlExecutor.ExecuteBatch(ctx, executorKP.Address(), callsAccept, predecessor, saltAccept); err != nil {
		t.Fatalf("ExecuteBatch accept_ownership: %v", err)
	}

	ownerAfter, err := reg.Owner(ctx)
	if err != nil {
		t.Fatalf("Owner after accept: %v", err)
	}
	if ownerAfter == nil || *ownerAfter != timelockID {
		t.Fatalf("owner = %v, want timelock %s", ownerAfter, timelockID)
	}

	const (
		chainReject   uint64 = 910001
		chainEarly    uint64 = 910002
		chainGate     uint64 = 910003
		chainProposer uint64 = 910004
		chainExecutor uint64 = 910005
		chainStranger uint64 = 910006
	)

	mockReject := helpers.GenerateMockContractID(t, deployerKP.Address(), "gov-tl-mock-reject")
	mockEarly := helpers.GenerateMockContractID(t, deployerKP.Address(), "gov-tl-mock-early")
	mockGate := helpers.GenerateMockContractID(t, deployerKP.Address(), "gov-tl-mock-gate")
	mockProposer := helpers.GenerateMockContractID(t, deployerKP.Address(), "gov-tl-mock-proposer")
	mockExecutor := helpers.GenerateMockContractID(t, deployerKP.Address(), "gov-tl-mock-exec")
	mockStranger := helpers.GenerateMockContractID(t, deployerKP.Address(), "gov-tl-mock-stranger")

	// Negative auth checks: simulate rather than InvokeContract so the host's
	// `Error(Contract, #code)` survives in the error string. A submitted invocation
	// can collapse a contract revert into a generic "transaction failed" message
	// without the code, making the assertion ambiguous. Same pattern as
	// `router_test.go`'s `ccip_send rejects when destination chain is cursed`.
	t.Run("former owner cannot apply_ramp_updates", func(t *testing.T) {
		args := []xdr.ScVal{
			scval.StructSliceToScVal([]rampbindings.OnRampEntry{{
				DestChainSelector: chainReject,
				Onramp:            mockReject,
			}}),
			scval.StructSliceToScVal([]rampbindings.OffRampEntry{}),
			scval.StructSliceToScVal([]rampbindings.OffRampEntry{}),
		}
		_, err := deployer.SimulateContract(ctx, registryID, "apply_ramp_updates", args)
		if err == nil {
			t.Fatal("expected apply_ramp_updates simulation to fail for non-owner deployer")
		}
		assertHostContractErrorContainsCode(t, err, timelockbindings.CCIPErrorUnauthorized)
	})

	t.Run("stranger cannot apply_ramp_updates", func(t *testing.T) {
		args := []xdr.ScVal{
			scval.StructSliceToScVal([]rampbindings.OnRampEntry{{
				DestChainSelector: chainStranger,
				Onramp:            mockStranger,
			}}),
			scval.StructSliceToScVal([]rampbindings.OffRampEntry{}),
			scval.StructSliceToScVal([]rampbindings.OffRampEntry{}),
		}
		_, err := strangerDep.SimulateContract(ctx, registryID, "apply_ramp_updates", args)
		if err == nil {
			t.Fatal("expected apply_ramp_updates simulation to fail for unrelated account")
		}
		assertHostContractErrorContainsCode(t, err, timelockbindings.CCIPErrorUnauthorized)
	})

	t.Run("executor cannot schedule", func(t *testing.T) {
		opData, err := encodeApplyRampUpdatesSingleOnramp(chainExecutor, mockExecutor)
		if err != nil {
			t.Fatal(err)
		}
		saltBump := randSalt(t)
		callsOp := timelockbindings.Calls{
			Inner: []timelockbindings.Call{{To: registryRaw, Data: opData}},
		}
		err = tlExecutor.ScheduleBatch(ctx, executorKP.Address(), callsOp, predecessor, saltBump, minDelaySec)
		if err == nil {
			t.Fatal("expected ScheduleBatch to fail when caller is EXECUTOR but not PROPOSER")
		}
	})

	t.Run("apply_ramp_updates before delay cannot execute", func(t *testing.T) {
		opData, err := encodeApplyRampUpdatesSingleOnramp(chainEarly, mockEarly)
		if err != nil {
			t.Fatal(err)
		}
		saltEarly := randSalt(t)
		callsOp := timelockbindings.Calls{
			Inner: []timelockbindings.Call{{To: registryRaw, Data: opData}},
		}
		if err := tlProposer.ScheduleBatch(ctx, proposerKP.Address(), callsOp, predecessor, saltEarly, minDelaySec); err != nil {
			t.Fatalf("ScheduleBatch apply_ramp_updates: %v", err)
		}
		err = tlExecutor.ExecuteBatch(ctx, executorKP.Address(), callsOp, predecessor, saltEarly)
		if err == nil {
			t.Fatal("expected ExecuteBatch before min_delay to fail")
		}
	})

	t.Run("gated apply_ramp_updates via timelock", func(t *testing.T) {
		if _, err := reg.GetOnramp(ctx, chainGate); err == nil {
			t.Fatal("expected GetOnramp to fail before route is configured")
		}

		opData, err := encodeApplyRampUpdatesSingleOnramp(chainGate, mockGate)
		if err != nil {
			t.Fatal(err)
		}
		saltBump := randSalt(t)
		callsOp := timelockbindings.Calls{
			Inner: []timelockbindings.Call{{To: registryRaw, Data: opData}},
		}
		if err := tlProposer.ScheduleBatch(ctx, proposerKP.Address(), callsOp, predecessor, saltBump, minDelaySec); err != nil {
			t.Fatalf("ScheduleBatch apply_ramp_updates: %v", err)
		}
		opBump, err := tlAdmin.HashOperationBatch(ctx, callsOp, predecessor, saltBump)
		if err != nil {
			t.Fatal(err)
		}

		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			ready, err := tlAdmin.IsOperationReady(ctx, opBump)
			if err != nil {
				t.Fatalf("IsOperationReady: %v", err)
			}
			if ready {
				break
			}
			time.Sleep(400 * time.Millisecond)
		}
		if ok, _ := tlAdmin.IsOperationReady(ctx, opBump); !ok {
			t.Fatal("apply_ramp_updates operation never became ready")
		}

		if err := tlExecutor.ExecuteBatch(ctx, executorKP.Address(), callsOp, predecessor, saltBump); err != nil {
			t.Fatalf("ExecuteBatch apply_ramp_updates: %v", err)
		}

		got, err := reg.GetOnramp(ctx, chainGate)
		if err != nil {
			t.Fatalf("GetOnramp after: %v", err)
		}
		if got != mockGate {
			t.Fatalf("get_onramp = %s, want %s", got, mockGate)
		}
	})

	t.Run("proposer cannot execute", func(t *testing.T) {
		opData, err := encodeApplyRampUpdatesSingleOnramp(chainProposer, mockProposer)
		if err != nil {
			t.Fatal(err)
		}
		saltBump := randSalt(t)
		callsOp := timelockbindings.Calls{
			Inner: []timelockbindings.Call{{To: registryRaw, Data: opData}},
		}
		if err := tlProposer.ScheduleBatch(ctx, proposerKP.Address(), callsOp, predecessor, saltBump, minDelaySec); err != nil {
			t.Fatalf("ScheduleBatch: %v", err)
		}
		opBump, err := tlAdmin.HashOperationBatch(ctx, callsOp, predecessor, saltBump)
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			ready, err := tlAdmin.IsOperationReady(ctx, opBump)
			if err != nil {
				t.Fatalf("IsOperationReady: %v", err)
			}
			if ready {
				break
			}
			time.Sleep(400 * time.Millisecond)
		}
		err = tlProposer.ExecuteBatch(ctx, proposerKP.Address(), callsOp, predecessor, saltBump)
		if err == nil {
			t.Fatal("expected ExecuteBatch to fail when caller is PROPOSER but not EXECUTOR")
		}
	})

	t.Run("stranger cannot schedule or execute", func(t *testing.T) {
		opData, err := encodeApplyRampUpdatesSingleOnramp(chainGate, mockGate)
		if err != nil {
			t.Fatal(err)
		}
		saltBump := randSalt(t)
		callsOp := timelockbindings.Calls{
			Inner: []timelockbindings.Call{{To: registryRaw, Data: opData}},
		}
		err = tlStranger.ScheduleBatch(ctx, strangerKP.Address(), callsOp, predecessor, saltBump, minDelaySec)
		if err == nil {
			t.Fatal("expected ScheduleBatch to fail for account without PROPOSER")
		}
		err = tlStranger.ExecuteBatch(ctx, strangerKP.Address(), callsOp, predecessor, saltBump)
		if err == nil {
			t.Fatal("expected ExecuteBatch to fail for account without EXECUTOR")
		}
	})
}
