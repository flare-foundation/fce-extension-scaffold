package validate

import (
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestAddressNotZero_ZeroAddress(t *testing.T) {
	err := AddressNotZero(common.Address{}, "TeeExtensionRegistry")
	if err == nil {
		t.Fatal("expected error for zero address, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "TeeExtensionRegistry") {
		t.Errorf("error should mention the label, got: %s", msg)
	}
	if !strings.Contains(msg, "zero address") {
		t.Errorf("error should mention 'zero address', got: %s", msg)
	}
	if !strings.Contains(msg, "0x0000000000000000000000000000000000000000") {
		t.Errorf("error should contain the zero address hex, got: %s", msg)
	}
}

func TestAddressNotZero_NonZeroAddress(t *testing.T) {
	addr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	err := AddressNotZero(addr, "TeeExtensionRegistry")
	if err != nil {
		t.Fatalf("expected nil error for non-zero address, got: %v", err)
	}
}

func TestAddressHasCode_NilClient(t *testing.T) {
	addr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	err := AddressHasCode(nil, addr, "TeeExtensionRegistry")
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "no chain client connected") {
		t.Errorf("error should mention 'no chain client connected', got: %s", msg)
	}
	if !strings.Contains(msg, "TeeExtensionRegistry") {
		t.Errorf("error should mention the label, got: %s", msg)
	}
}

func TestKeyHasFunds_NilClient(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	err = KeyHasFunds(nil, key, big.NewInt(1))
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "no chain client connected") {
		t.Errorf("error should mention 'no chain client connected', got: %s", msg)
	}
}

func TestIsUsingDevKey_WhenSet(t *testing.T) {
	original := os.Getenv("DEPLOYMENT_PRIVATE_KEY")
	defer os.Setenv("DEPLOYMENT_PRIVATE_KEY", original)

	os.Setenv("DEPLOYMENT_PRIVATE_KEY", "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	if IsUsingDevKey() {
		t.Error("expected IsUsingDevKey() to return false when DEPLOYMENT_PRIVATE_KEY is set")
	}
}

func TestIsUsingDevKey_WhenUnset(t *testing.T) {
	original := os.Getenv("DEPLOYMENT_PRIVATE_KEY")
	defer os.Setenv("DEPLOYMENT_PRIVATE_KEY", original)

	os.Unsetenv("DEPLOYMENT_PRIVATE_KEY")
	if !IsUsingDevKey() {
		t.Error("expected IsUsingDevKey() to return true when DEPLOYMENT_PRIVATE_KEY is unset")
	}
}

func TestAddressNotZero_PartialZero(t *testing.T) {
	// Address with only the last byte non-zero — NOT a zero address
	addr := common.HexToAddress("0x0000000000000000000000000000000000000001")
	err := AddressNotZero(addr, "TestRegistry")
	if err != nil {
		t.Fatalf("expected nil error for partial-zero address, got: %v", err)
	}
}

func TestAddressNotZero_ErrorContainsLabel(t *testing.T) {
	err := AddressNotZero(common.Address{}, "MyCustomLabel")
	if err == nil {
		t.Fatal("expected error for zero address")
	}
	if !strings.Contains(err.Error(), "MyCustomLabel") {
		t.Errorf("error should contain label 'MyCustomLabel', got: %s", err.Error())
	}
}

func TestAddressNotZero_ErrorContainsHexAddress(t *testing.T) {
	err := AddressNotZero(common.Address{}, "Reg")
	if err == nil {
		t.Fatal("expected error for zero address")
	}
	if !strings.Contains(err.Error(), "0x0000000000000000000000000000000000000000") {
		t.Errorf("error should contain the zero address hex, got: %s", err.Error())
	}
}

func TestIsUsingDevKey_WhenEmpty(t *testing.T) {
	t.Setenv("DEPLOYMENT_PRIVATE_KEY", "")
	if !IsUsingDevKey() {
		t.Error("expected IsUsingDevKey() to return true when DEPLOYMENT_PRIVATE_KEY is empty string")
	}
}

func TestMinDeployBalance_Value(t *testing.T) {
	// Verify the constant is 0.01 ETH = 10^16 wei
	expected := big.NewInt(10_000_000_000_000_000)
	if MinDeployBalance.Cmp(expected) != 0 {
		t.Errorf("MinDeployBalance = %s, want %s", MinDeployBalance.String(), expected.String())
	}
}
