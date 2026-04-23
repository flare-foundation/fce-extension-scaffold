//go:build integration

package integration

import (
	"strings"
	"testing"

	"extension-scaffold/tools/pkg/fccutils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// --- 2.1: Contract Constructor Validation ---

func TestDeploy_ZeroRegistryAddress(t *testing.T) {
	err := deployInstructionSenderRaw(t,
		common.Address{}, // zero address for TeeExtensionRegistry
		testSupport.Addresses.TeeMachineRegistry,
	)
	if err == nil {
		t.Fatal("expected deployment to fail with zero TeeExtensionRegistry address, but it succeeded")
	}

	reason := fccutils.DecodeRevertReason(err)
	t.Logf("Raw error: %v", err)
	t.Logf("Decoded revert reason: %q", reason)

	if reason == "" {
		// Fallback: check the raw error message
		if !strings.Contains(err.Error(), "zero address") && !strings.Contains(err.Error(), "TeeExtensionRegistry") {
			t.Errorf("expected error to mention zero address or TeeExtensionRegistry, got: %v", err)
		}
		t.Log("Note: DecodeRevertReason returned empty — revert data may not be available from this RPC node")
		return
	}

	if !strings.Contains(reason, "TeeExtensionRegistry cannot be zero address") {
		t.Errorf("expected revert reason to contain %q, got %q",
			"TeeExtensionRegistry cannot be zero address", reason)
	}
}

func TestDeploy_ZeroMachineRegistryAddress(t *testing.T) {
	err := deployInstructionSenderRaw(t,
		testSupport.Addresses.TeeExtensionRegistry,
		common.Address{}, // zero address for TeeMachineRegistry
	)
	if err == nil {
		t.Fatal("expected deployment to fail with zero TeeMachineRegistry address, but it succeeded")
	}

	reason := fccutils.DecodeRevertReason(err)
	t.Logf("Raw error: %v", err)
	t.Logf("Decoded revert reason: %q", reason)

	if reason == "" {
		if !strings.Contains(err.Error(), "zero address") && !strings.Contains(err.Error(), "TeeMachineRegistry") {
			t.Errorf("expected error to mention zero address or TeeMachineRegistry, got: %v", err)
		}
		t.Log("Note: DecodeRevertReason returned empty — revert data may not be available from this RPC node")
		return
	}

	if !strings.Contains(reason, "TeeMachineRegistry cannot be zero address") {
		t.Errorf("expected revert reason to contain %q, got %q",
			"TeeMachineRegistry cannot be zero address", reason)
	}
}

func TestDeploy_EOAAsRegistry(t *testing.T) {
	// Generate a random EOA address — it has no code deployed
	randomKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	eoaAddr := crypto.PubkeyToAddress(randomKey.PublicKey)

	deployErr := deployInstructionSenderRaw(t,
		eoaAddr, // EOA with no code
		testSupport.Addresses.TeeMachineRegistry,
	)
	if deployErr == nil {
		t.Fatal("expected deployment to fail with EOA as TeeExtensionRegistry, but it succeeded")
	}

	reason := fccutils.DecodeRevertReason(deployErr)
	t.Logf("Raw error: %v", deployErr)
	t.Logf("Decoded revert reason: %q", reason)

	if reason == "" {
		if !strings.Contains(deployErr.Error(), "no code") && !strings.Contains(deployErr.Error(), "TeeExtensionRegistry") {
			t.Errorf("expected error to mention 'no code' or TeeExtensionRegistry, got: %v", deployErr)
		}
		t.Log("Note: DecodeRevertReason returned empty — revert data may not be available from this RPC node")
		return
	}

	if !strings.Contains(reason, "TeeExtensionRegistry has no code") {
		t.Errorf("expected revert reason to contain %q, got %q",
			"TeeExtensionRegistry has no code", reason)
	}
}

func TestDeploy_ValidAddresses(t *testing.T) {
	addr, _ := deployFreshInstructionSender(t)
	if addr == (common.Address{}) {
		t.Fatal("expected non-zero deployed address")
	}
}
