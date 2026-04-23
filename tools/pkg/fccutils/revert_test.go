package fccutils

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

// mockDataError implements the ErrorData() interface that go-ethereum uses
// for JSON-RPC errors containing revert data.
type mockDataError struct {
	msg  string
	data interface{}
}

func (e *mockDataError) Error() string        { return e.msg }
func (e *mockDataError) ErrorData() interface{} { return e.data }

// makeRevertHex creates a valid ABI-encoded Error(string) revert in hex form.
func makeRevertHex(msg string) string {
	// Compute Error(string) selector via keccak256, matching what go-ethereum expects.
	selector := crypto.Keccak256([]byte("Error(string)"))[:4]
	stringTy, _ := abi.NewType("string", "", nil)
	args := abi.Arguments{{Type: stringTy}}
	encoded, _ := args.Pack(msg)
	return hex.EncodeToString(append(selector, encoded...))
}

// --- decodeRevertHex tests ---

func TestDecodeRevertHex_StandardError(t *testing.T) {
	hexStr := makeRevertHex("Extension ID already set.")
	result := decodeRevertHex(hexStr)
	if result != "Extension ID already set." {
		t.Errorf("expected %q, got %q", "Extension ID already set.", result)
	}
}

func TestDecodeRevertHex_WithPrefix(t *testing.T) {
	hexStr := "0x" + makeRevertHex("Extension ID already set.")
	result := decodeRevertHex(hexStr)
	if result != "Extension ID already set." {
		t.Errorf("expected %q, got %q", "Extension ID already set.", result)
	}
}

func TestDecodeRevertHex_CustomError(t *testing.T) {
	// 4-byte custom error selector + some encoded data (not Error(string))
	customSelector := "deadbeef"
	paddedArg := "0000000000000000000000000000000000000000000000000000000000000042"
	hexStr := customSelector + paddedArg
	result := decodeRevertHex(hexStr)
	// Should fall back to returning "0x" + hex since it can't unpack as Error(string)
	if result != "0x"+hexStr {
		t.Errorf("expected %q, got %q", "0x"+hexStr, result)
	}
}

func TestDecodeRevertHex_TooShort(t *testing.T) {
	// Only 2 bytes — less than the 4-byte minimum
	result := decodeRevertHex("abcd")
	if result != "" {
		t.Errorf("expected empty string for short input, got %q", result)
	}
}

func TestDecodeRevertHex_Empty(t *testing.T) {
	result := decodeRevertHex("")
	if result != "" {
		t.Errorf("expected empty string for empty input, got %q", result)
	}
}

func TestDecodeRevertHex_InvalidHex(t *testing.T) {
	result := decodeRevertHex("not-valid-hex!")
	if result != "" {
		t.Errorf("expected empty string for invalid hex, got %q", result)
	}
}

func TestDecodeRevertHex_ZeroAddressRevert(t *testing.T) {
	hexStr := makeRevertHex("TeeExtensionRegistry cannot be zero address")
	result := decodeRevertHex(hexStr)
	if result != "TeeExtensionRegistry cannot be zero address" {
		t.Errorf("expected constructor revert message, got %q", result)
	}
}

func TestDecodeRevertHex_NoCodeRevert(t *testing.T) {
	hexStr := makeRevertHex("TeeExtensionRegistry has no code")
	result := decodeRevertHex(hexStr)
	if result != "TeeExtensionRegistry has no code" {
		t.Errorf("expected constructor revert message, got %q", result)
	}
}

func TestDecodeRevertHex_ExtensionIdNotFound(t *testing.T) {
	hexStr := makeRevertHex("Extension ID not found.")
	result := decodeRevertHex(hexStr)
	if result != "Extension ID not found." {
		t.Errorf("expected %q, got %q", "Extension ID not found.", result)
	}
}

func TestDecodeRevertHex_ExtensionIdNotSet(t *testing.T) {
	hexStr := makeRevertHex("Extension ID is not set.")
	result := decodeRevertHex(hexStr)
	if result != "Extension ID is not set." {
		t.Errorf("expected %q, got %q", "Extension ID is not set.", result)
	}
}

// --- DecodeRevertReason tests ---

func TestDecodeRevertReason_NilError(t *testing.T) {
	result := DecodeRevertReason(nil)
	if result != "" {
		t.Errorf("expected empty string for nil error, got %q", result)
	}
}

func TestDecodeRevertReason_PlainError(t *testing.T) {
	result := DecodeRevertReason(errors.New("something went wrong"))
	if result != "" {
		t.Errorf("expected empty string for plain error (no ErrorData), got %q", result)
	}
}

func TestDecodeRevertReason_WithDataError(t *testing.T) {
	revertHex := "0x" + makeRevertHex("Extension ID already set.")
	err := &mockDataError{
		msg:  "execution reverted",
		data: revertHex,
	}
	result := DecodeRevertReason(err)
	if result != "Extension ID already set." {
		t.Errorf("expected %q, got %q", "Extension ID already set.", result)
	}
}

func TestDecodeRevertReason_WithDataError_NoPrefix(t *testing.T) {
	revertHex := makeRevertHex("Extension ID not found.")
	err := &mockDataError{
		msg:  "execution reverted",
		data: revertHex,
	}
	result := DecodeRevertReason(err)
	if result != "Extension ID not found." {
		t.Errorf("expected %q, got %q", "Extension ID not found.", result)
	}
}

func TestDecodeRevertReason_WithNilData(t *testing.T) {
	err := &mockDataError{
		msg:  "execution reverted",
		data: nil,
	}
	result := DecodeRevertReason(err)
	if result != "" {
		t.Errorf("expected empty string for nil data, got %q", result)
	}
}

func TestDecodeRevertReason_WithNonStringData(t *testing.T) {
	err := &mockDataError{
		msg:  "execution reverted",
		data: 42,
	}
	result := DecodeRevertReason(err)
	if result != "" {
		t.Errorf("expected empty string for non-string data, got %q", result)
	}
}

func TestDecodeRevertReason_WrappedError(t *testing.T) {
	revertHex := "0x" + makeRevertHex("insufficient funds")
	innerErr := &mockDataError{
		msg:  "execution reverted",
		data: revertHex,
	}
	// Wrap the error — errors.As should unwrap and find ErrorData
	wrappedErr := errors.Join(errors.New("outer context"), innerErr)
	result := DecodeRevertReason(wrappedErr)
	if result != "insufficient funds" {
		t.Errorf("expected %q, got %q", "insufficient funds", result)
	}
}

func TestDecodeRevertReason_CustomErrorSelector(t *testing.T) {
	// Custom error with unknown selector — should return hex
	err := &mockDataError{
		msg:  "execution reverted",
		data: "0xdeadbeef0000000000000000000000000000000000000000000000000000000000000042",
	}
	result := DecodeRevertReason(err)
	if result == "" {
		t.Error("expected non-empty result for custom error (should return hex)")
	}
	if result == "execution reverted" {
		t.Error("should not return the error message itself")
	}
	// Should start with "0x" since it's a hex fallback
	if len(result) < 2 || result[:2] != "0x" {
		t.Errorf("expected hex fallback starting with 0x, got %q", result)
	}
}

func TestDecodeRevertReason_EmptyHexData(t *testing.T) {
	err := &mockDataError{
		msg:  "execution reverted",
		data: "",
	}
	result := DecodeRevertReason(err)
	if result != "" {
		t.Errorf("expected empty string for empty hex data, got %q", result)
	}
}

// --- Verify actual contract revert messages decode correctly ---
// These are the specific revert strings from InstructionSender.sol

func TestDecodeRevertReason_AllContractReverts(t *testing.T) {
	reverts := []string{
		"TeeExtensionRegistry cannot be zero address",
		"TeeMachineRegistry cannot be zero address",
		"TeeExtensionRegistry has no code",
		"TeeMachineRegistry has no code",
		"Extension ID already set.",
		"Extension ID not found.",
		"Extension ID is not set.",
	}

	for _, msg := range reverts {
		t.Run(msg, func(t *testing.T) {
			revertHex := "0x" + makeRevertHex(msg)
			err := &mockDataError{
				msg:  "execution reverted",
				data: revertHex,
			}
			result := DecodeRevertReason(err)
			if result != msg {
				t.Errorf("expected %q, got %q", msg, result)
			}
		})
	}
}
