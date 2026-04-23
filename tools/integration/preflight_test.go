//go:build integration

package integration

import (
	"math/big"
	"strings"
	"testing"

	"extension-scaffold/tools/pkg/validate"

	"github.com/ethereum/go-ethereum/crypto"
)

// --- 2.6: Pre-Flight Validation Against Chain ---

func TestAddressHasCode_DeployedContract(t *testing.T) {
	err := validate.AddressHasCode(
		testSupport.ChainClient,
		testSupport.Addresses.TeeExtensionRegistry,
		"TeeExtensionRegistry",
	)
	if err != nil {
		t.Fatalf("expected TeeExtensionRegistry to have code, got error: %v", err)
	}

	err = validate.AddressHasCode(
		testSupport.ChainClient,
		testSupport.Addresses.TeeMachineRegistry,
		"TeeMachineRegistry",
	)
	if err != nil {
		t.Fatalf("expected TeeMachineRegistry to have code, got error: %v", err)
	}
}

func TestAddressHasCode_RandomEOA(t *testing.T) {
	randomKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("key gen: %v", err)
	}
	eoaAddr := crypto.PubkeyToAddress(randomKey.PublicKey)

	err = validate.AddressHasCode(testSupport.ChainClient, eoaAddr, "RandomEOA")
	if err == nil {
		t.Fatal("expected error for random EOA with no code, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "no deployed code") {
		t.Errorf("expected error to mention 'no deployed code', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "RandomEOA") {
		t.Errorf("expected error to contain label 'RandomEOA', got: %s", errMsg)
	}
}

func TestKeyHasFunds_FundedAccount(t *testing.T) {
	err := validate.KeyHasFunds(
		testSupport.ChainClient,
		testSupport.Prv,
		validate.MinDeployBalance,
	)
	if err != nil {
		t.Fatalf("expected deployer key to have funds, got: %v", err)
	}
}

func TestKeyHasFunds_UnfundedAccount(t *testing.T) {
	unfundedKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("key gen: %v", err)
	}

	err = validate.KeyHasFunds(
		testSupport.ChainClient,
		unfundedKey,
		big.NewInt(1), // Even 1 wei should fail for a fresh random key
	)
	if err == nil {
		t.Fatal("expected error for unfunded account, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "insufficient funds") {
		t.Errorf("expected error to mention 'insufficient funds', got: %s", errMsg)
	}

	// Error should include the actual balance (0) and required amount
	if !strings.Contains(errMsg, "balance:") || !strings.Contains(errMsg, "minimum required:") {
		t.Errorf("expected error to include balance details, got: %s", errMsg)
	}
}
