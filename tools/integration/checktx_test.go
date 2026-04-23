//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"extension-scaffold/tools/pkg/contracts/helloworld"
	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

// --- 2.4: CheckTx Revert Reason Chain ---
//
// These tests verify that when a transaction is mined but reverts (Status=0),
// CheckTx correctly replays the call and decodes the revert reason.

func TestCheckTx_SuccessfulTx(t *testing.T) {
	// Deploy a fresh InstructionSender — the deployment tx itself is successful.
	opts, err := bind.NewKeyedTransactorWithChainID(testSupport.Prv, testSupport.ChainID)
	if err != nil {
		t.Fatalf("transactor: %v", err)
	}

	_, tx, _, err := helloworld.DeployHelloWorldInstructionSender(
		opts, testSupport.ChainClient,
		testSupport.Addresses.TeeExtensionRegistry,
		testSupport.Addresses.TeeMachineRegistry,
	)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	receipt, err := support.CheckTx(tx, testSupport.ChainClient)
	if err != nil {
		t.Fatalf("CheckTx failed on successful deployment: %v", err)
	}
	if receipt.Status != 1 {
		t.Fatalf("expected receipt status 1, got %d", receipt.Status)
	}
	t.Logf("CheckTx returned successful receipt (tx: %s)", tx.Hash().Hex())
}

func TestCheckTx_FailedTx_DecodesReason(t *testing.T) {
	// Deploy a fresh InstructionSender (valid).
	_, contract := deployFreshInstructionSender(t)

	// Call setExtensionId — it will revert because extension is not registered.
	// We bypass gas estimation by setting a manual gas limit so the tx actually
	// gets submitted and mined (rather than failing at estimation).
	opts, err := bind.NewKeyedTransactorWithChainID(testSupport.Prv, testSupport.ChainID)
	if err != nil {
		t.Fatalf("transactor: %v", err)
	}
	opts.GasLimit = 200000 // Bypass estimation — force tx submission

	tx, err := contract.SetExtensionId(opts)
	if err != nil {
		// Some nodes reject at send time even with manual gas — can't test
		// the mined-revert path on this chain.
		t.Skipf("Transaction rejected before mining: %v", err)
	}

	// Wait for the tx to be mined
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, testSupport.ChainClient, tx)
	if err != nil {
		t.Fatalf("WaitMined failed: %v", err)
	}

	if receipt.Status == 1 {
		t.Skip("Transaction succeeded (extension might already be registered) — cannot test revert path")
	}

	// Now use CheckTx — it should detect Status=0 and replay the call to get the reason.
	_, checkErr := support.CheckTx(tx, testSupport.ChainClient)
	if checkErr == nil {
		t.Fatal("expected CheckTx to return error for reverted transaction")
	}

	errMsg := checkErr.Error()
	t.Logf("CheckTx error: %s", errMsg)

	// Some chains (Avalanche C-chain / Coston2) don't support replaying
	// certain transaction types via eth_call. Skip rather than fail.
	if strings.Contains(errMsg, "transaction type not supported") {
		t.Skip("Chain does not support replaying this transaction type — skipping mined-revert test")
	}

	// The error should contain the decoded revert reason, NOT binary garbage.
	if strings.Contains(errMsg, "Extension ID not found") {
		t.Log("CheckTx correctly decoded revert reason from mined transaction")
	} else if strings.Contains(errMsg, "0x") {
		// Hex fallback — decoded but not as Error(string)
		t.Log("CheckTx returned hex-encoded revert (custom error or different encoding)")
	} else if strings.Contains(errMsg, "Transaction fail") {
		t.Log("CheckTx detected failure — checking if reason is readable")
		// Make sure we don't have binary garbage (non-printable characters)
		for _, r := range errMsg {
			if r < 32 && r != '\n' && r != '\r' && r != '\t' {
				t.Errorf("CheckTx error contains non-printable character (0x%02x) — binary garbage not properly decoded", r)
				break
			}
		}
	} else {
		t.Errorf("unexpected error format: %s", errMsg)
	}
}
