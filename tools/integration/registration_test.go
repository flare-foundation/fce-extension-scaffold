//go:build integration

package integration

import (
	"testing"

	"extension-scaffold/tools/pkg/fccutils"

	"github.com/ethereum/go-ethereum/common"
)

// --- 2.5: Extension Registration ---

func TestSetupExtension_FirstTime(t *testing.T) {
	addr, _ := deployFreshInstructionSender(t)

	governanceHash := common.Hash{}
	extID, err := fccutils.SetupExtension(testSupport, governanceHash, addr, common.Address{})
	if err != nil {
		t.Fatalf("SetupExtension failed on first run: %v", err)
	}

	if extID == nil {
		t.Fatal("expected non-nil extension ID")
	}
	t.Logf("Extension registered with ID: %s", extID.String())
}

func TestSetupExtension_DuplicateSenderFails(t *testing.T) {
	addr, _ := deployFreshInstructionSender(t)

	governanceHash := common.Hash{}

	// First registration should succeed
	extID1, err := fccutils.SetupExtension(testSupport, governanceHash, addr, common.Address{})
	if err != nil {
		t.Fatalf("first SetupExtension failed: %v", err)
	}
	t.Logf("First registration: ID=%s", extID1.String())

	// Second registration with the same InstructionSender should fail —
	// the contract rejects duplicate senders. Duplicate detection belongs in
	// verify-deploy (check R7), not in the deploy hot path.
	_, err = fccutils.SetupExtension(testSupport, governanceHash, addr, common.Address{})
	if err == nil {
		t.Fatal("expected second SetupExtension with same sender to fail, but it succeeded")
	}
	t.Logf("Second registration correctly failed: %v", err)
}
