//go:build integration

package integration

import (
	"strings"
	"testing"

	"extension-scaffold/tools/pkg/fccutils"
	instrutils "extension-scaffold/tools/pkg/utils"
)

// --- 2.2: setExtensionId Error Handling ---

func TestSetExtensionId_NotRegistered(t *testing.T) {
	// Deploy a fresh InstructionSender but do NOT register it as an extension.
	addr, _ := deployFreshInstructionSender(t)

	// Call setExtensionId — should fail because this contract isn't registered.
	err := instrutils.SetExtensionId(testSupport, addr)
	if err == nil {
		t.Fatal("expected setExtensionId to fail on unregistered contract, but it succeeded")
	}

	t.Logf("Error: %v", err)

	// The error should contain the decoded revert reason.
	errMsg := err.Error()
	if strings.Contains(errMsg, "revert reason:") {
		// Good — our hardening extracted the reason
		if !strings.Contains(errMsg, "Extension ID not found") {
			t.Errorf("expected revert reason to mention 'Extension ID not found', got: %s", errMsg)
		}
	} else {
		// DecodeRevertReason might not work on all nodes — log for diagnosis
		t.Logf("Note: error does not contain 'revert reason:' — may need SimulateAndDecodeRevert fallback")
		// The error should at least indicate failure
		if !strings.Contains(errMsg, "failed to call setExtensionId") {
			t.Errorf("expected error to mention 'failed to call setExtensionId', got: %s", errMsg)
		}
	}
}

func TestSetExtensionId_AlreadySet(t *testing.T) {
	// Deploy and register a fresh extension
	addr, _ := deployFreshInstructionSender(t)
	registerExtensionForSender(t, addr)

	// First call should succeed
	err := instrutils.SetExtensionId(testSupport, addr)
	if err != nil {
		t.Fatalf("first setExtensionId call failed: %v", err)
	}
	t.Log("First setExtensionId succeeded")

	// Second call should fail with "Extension ID already set."
	err = instrutils.SetExtensionId(testSupport, addr)
	if err == nil {
		t.Fatal("expected second setExtensionId to fail, but it succeeded")
	}

	t.Logf("Error on second call: %v", err)

	errMsg := err.Error()
	if strings.Contains(errMsg, "revert reason:") {
		if !strings.Contains(errMsg, "Extension ID already set") {
			t.Errorf("expected revert reason to mention 'Extension ID already set', got: %s", errMsg)
		}
	} else {
		t.Logf("Note: error does not contain 'revert reason:' — revert data may not be available")
		if !strings.Contains(errMsg, "failed to call setExtensionId") {
			t.Errorf("expected error to mention 'failed to call setExtensionId', got: %s", errMsg)
		}
	}
}

func TestSetExtensionId_RevertReasonDecoded(t *testing.T) {
	// This test specifically verifies the revert decoding chain works.
	// Deploy but do NOT register — setExtensionId will revert.
	addr, _ := deployFreshInstructionSender(t)

	err := instrutils.SetExtensionId(testSupport, addr)
	if err == nil {
		t.Fatal("expected setExtensionId to fail")
	}

	errMsg := err.Error()

	// The hardening in SetExtensionId tries:
	// 1. DecodeRevertReason(err) — from the estimation error
	// 2. SimulateAndDecodeRevert() — replays the call via eth_call
	// At least one of these should produce a human-readable reason.
	if !strings.Contains(errMsg, "Extension ID not found") {
		t.Errorf("expected error to contain decoded revert reason 'Extension ID not found', got: %s", errMsg)
		t.Log("This indicates the revert decoding chain is not working correctly.")
		t.Log("Check that DecodeRevertReason or SimulateAndDecodeRevert extracts the reason.")

		// Additional diagnostic: try DecodeRevertReason directly on a fresh error
		directReason := fccutils.DecodeRevertReason(err)
		t.Logf("Direct DecodeRevertReason result: %q", directReason)
	}
}
