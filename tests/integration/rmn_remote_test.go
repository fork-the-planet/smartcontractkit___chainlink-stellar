//go:build integration

package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	rmnbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/rmn_remote"
	deployment "github.com/smartcontractkit/chainlink-stellar/deployment"
)

var globalCurseSubject = [16]byte{
	0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
}

func TestRmnRemote(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectRoot, deployerKP, deployer, _, _, _ := GetSharedTestEnv(ctx, t)

	t.Log("Deploying RmnRemote contract...")
	salt := deployment.GenerateDeterministicSalt(deployerKP.Address(), "rmn-remote")
	wasmPath := filepath.Join(projectRoot, "target", "wasm32v1-none", "release", "rmn_remote.wasm")

	contractID, err := deployer.DeployContract(ctx, wasmPath, salt)
	if err != nil {
		t.Fatalf("Failed to deploy RmnRemote: %v", err)
	}
	t.Logf("RmnRemote deployed at: %s", contractID)

	client := rmnbindings.NewRmnRemoteClient(deployer, contractID)

	t.Run("initialize", func(t *testing.T) {
		err := client.Initialize(ctx, deployerKP.Address(), nil)
		if err != nil {
			t.Fatalf("Failed to initialize RmnRemote: %v", err)
		}
		t.Log("RmnRemote initialized successfully")
	})

	t.Run("double initialize fails", func(t *testing.T) {
		err := client.Initialize(ctx, deployerKP.Address(), nil)
		if err == nil {
			t.Fatal("Expected error on double initialize, got nil")
		}
		t.Logf("Double initialize correctly rejected: %v", err)
	})

	t.Run("verify owner", func(t *testing.T) {
		owner, err := client.Owner(ctx)
		if err != nil {
			t.Fatalf("Failed to get owner: %v", err)
		}
		if owner == nil || *owner != deployerKP.Address() {
			t.Errorf("Owner mismatch: expected %s, got %v", deployerKP.Address(), owner)
		}
		t.Logf("Owner verified: %s", *owner)
	})

	t.Run("initially not cursed", func(t *testing.T) {
		subjects, err := client.GetCursedSubjects(ctx)
		if err != nil {
			t.Fatalf("Failed to get cursed subjects: %v", err)
		}
		if len(subjects) != 0 {
			t.Errorf("Expected no cursed subjects, got %d", len(subjects))
		}
		t.Log("No cursed subjects initially (as expected)")
	})

	t.Run("curse a specific subject", func(t *testing.T) {
		laneSubject := [16]byte{0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
			0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02}

		err := client.Curse(ctx, deployerKP.Address(), [][16]byte{laneSubject})
		if err != nil {
			t.Fatalf("Failed to curse subject: %v", err)
		}

		subjects, err := client.GetCursedSubjects(ctx)
		if err != nil {
			t.Fatalf("Failed to get cursed subjects: %v", err)
		}
		if len(subjects) != 1 {
			t.Fatalf("Expected 1 cursed subject, got %d", len(subjects))
		}
		if subjects[0] != laneSubject {
			t.Errorf("Cursed subject mismatch: expected %x, got %x", laneSubject, subjects[0])
		}
		t.Logf("Subject cursed and verified: %x", subjects[0])
	})

	t.Run("curse already cursed subject silent skip", func(t *testing.T) {
		laneSubject := [16]byte{0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
			0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02}

		err := client.Curse(ctx, deployerKP.Address(), [][16]byte{laneSubject})
		if err != nil {
			t.Fatalf("Re-curse should succeed (silent skip): %v", err)
		}
		subjects, err := client.GetCursedSubjects(ctx)
		if err != nil {
			t.Fatalf("Failed to get cursed subjects: %v", err)
		}
		if len(subjects) != 1 {
			t.Fatalf("Expected 1 cursed subject after re-curse, got %d", len(subjects))
		}
		t.Log("Re-curse on already-cursed subject is a no-op (EVM-aligned)")
	})

	t.Run("uncurse a subject", func(t *testing.T) {
		laneSubject := [16]byte{0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
			0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02}

		err := client.Uncurse(ctx, [][16]byte{laneSubject})
		if err != nil {
			t.Fatalf("Failed to uncurse subject: %v", err)
		}

		subjects, err := client.GetCursedSubjects(ctx)
		if err != nil {
			t.Fatalf("Failed to get cursed subjects after uncurse: %v", err)
		}
		if len(subjects) != 0 {
			t.Errorf("Expected 0 cursed subjects after uncurse, got %d", len(subjects))
		}
		t.Log("Subject uncursed and verified")
	})

	t.Run("uncurse not cursed subject fails", func(t *testing.T) {
		notCursed := [16]byte{0xAA, 0xBB, 0xCC, 0xDD, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

		err := client.Uncurse(ctx, [][16]byte{notCursed})
		if err == nil {
			t.Fatal("Expected error when uncursing non-cursed subject, got nil")
		}
		t.Logf("Uncurse non-cursed correctly rejected: %v", err)
	})

	t.Run("curse and uncurse multiple subjects", func(t *testing.T) {
		s1 := [16]byte{0x10}
		s2 := [16]byte{0x20}
		s3 := [16]byte{0x30}

		err := client.Curse(ctx, deployerKP.Address(), [][16]byte{s1, s2, s3})
		if err != nil {
			t.Fatalf("Failed to curse multiple subjects: %v", err)
		}

		subjects, err := client.GetCursedSubjects(ctx)
		if err != nil {
			t.Fatalf("Failed to get cursed subjects: %v", err)
		}
		if len(subjects) != 3 {
			t.Fatalf("Expected 3 cursed subjects, got %d", len(subjects))
		}
		t.Logf("3 subjects cursed successfully")

		err = client.Uncurse(ctx, [][16]byte{s2})
		if err != nil {
			t.Fatalf("Failed to uncurse s2: %v", err)
		}

		subjects, err = client.GetCursedSubjects(ctx)
		if err != nil {
			t.Fatalf("Failed to get cursed subjects after partial uncurse: %v", err)
		}
		if len(subjects) != 2 {
			t.Errorf("Expected 2 cursed subjects, got %d", len(subjects))
		}
		t.Logf("Partial uncurse verified: %d subjects remain cursed", len(subjects))

		err = client.Uncurse(ctx, [][16]byte{s1, s3})
		if err != nil {
			t.Fatalf("Failed to uncurse remaining subjects: %v", err)
		}
	})

	t.Run("global curse subject", func(t *testing.T) {
		err := client.Curse(ctx, deployerKP.Address(), [][16]byte{globalCurseSubject})
		if err != nil {
			t.Fatalf("Failed to curse global subject: %v", err)
		}

		isCursed, err := client.IsCursed(ctx)
		if err != nil {
			t.Fatalf("IsCursed failed after global curse: %v", err)
		}
		if !isCursed {
			t.Fatal("IsCursed should return true after global curse")
		}
		t.Log("Global curse applied and IsCursed returned successfully")

		arbitrary := [16]byte{0xFF}
		isCursedBySubject, err := client.IsCursedBySubject(ctx, arbitrary)
		if err != nil {
			t.Fatalf("IsCursedBySubject failed after global curse: %v", err)
		}
		if !isCursedBySubject {
			t.Fatal("IsCursedBySubject should return true after global curse")
		}
		t.Log("IsCursedBySubject correctly responds after global curse")

		err = client.Uncurse(ctx, [][16]byte{globalCurseSubject})
		if err != nil {
			t.Fatalf("Failed to uncurse global subject: %v", err)
		}
		t.Log("Global curse removed")
	})

	t.Log("RmnRemote integration test passed!")
}
