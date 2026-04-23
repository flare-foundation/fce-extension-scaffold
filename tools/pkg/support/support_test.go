package support

import (
	"encoding/hex"
	stderrors "errors"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

// mockDataError implements the ErrorData() interface used by go-ethereum
// JSON-RPC error types.
type mockDataError struct {
	msg  string
	data interface{}
}

func (e *mockDataError) Error() string        { return e.msg }
func (e *mockDataError) ErrorData() interface{} { return e.data }

// makeRevertHex creates valid ABI-encoded Error(string) revert data as hex.
func makeRevertHex(msg string) string {
	selector := crypto.Keccak256([]byte("Error(string)"))[:4]
	stringTy, _ := abi.NewType("string", "", nil)
	args := abi.Arguments{{Type: stringTy}}
	encoded, _ := args.Pack(msg)
	return hex.EncodeToString(append(selector, encoded...))
}

// --- decodeRevertFromError tests ---

func TestDecodeRevertFromError_StandardRevert(t *testing.T) {
	revertHex := "0x" + makeRevertHex("Extension ID already set.")
	err := &mockDataError{
		msg:  "execution reverted",
		data: revertHex,
	}
	result := decodeRevertFromError(err)
	if result != "Extension ID already set." {
		t.Errorf("expected %q, got %q", "Extension ID already set.", result)
	}
}

func TestDecodeRevertFromError_NoPrefix(t *testing.T) {
	revertHex := makeRevertHex("Extension ID not found.")
	err := &mockDataError{
		msg:  "execution reverted",
		data: revertHex,
	}
	result := decodeRevertFromError(err)
	if result != "Extension ID not found." {
		t.Errorf("expected %q, got %q", "Extension ID not found.", result)
	}
}

func TestDecodeRevertFromError_NoInterface(t *testing.T) {
	result := decodeRevertFromError(stderrors.New("plain error without ErrorData"))
	if result != "" {
		t.Errorf("expected empty string for plain error, got %q", result)
	}
}

func TestDecodeRevertFromError_InvalidHex(t *testing.T) {
	err := &mockDataError{
		msg:  "execution reverted",
		data: "zzzz-not-hex",
	}
	result := decodeRevertFromError(err)
	if result != "" {
		t.Errorf("expected empty string for invalid hex, got %q", result)
	}
}

func TestDecodeRevertFromError_ShortData(t *testing.T) {
	// Only 2 bytes — too short for ABI selector
	err := &mockDataError{
		msg:  "execution reverted",
		data: "0x1234",
	}
	result := decodeRevertFromError(err)
	if result != "" {
		t.Errorf("expected empty string for short data, got %q", result)
	}
}

func TestDecodeRevertFromError_NilData(t *testing.T) {
	err := &mockDataError{
		msg:  "execution reverted",
		data: nil,
	}
	result := decodeRevertFromError(err)
	if result != "" {
		t.Errorf("expected empty string for nil data, got %q", result)
	}
}

func TestDecodeRevertFromError_NonStringData(t *testing.T) {
	err := &mockDataError{
		msg:  "execution reverted",
		data: 12345,
	}
	result := decodeRevertFromError(err)
	if result != "" {
		t.Errorf("expected empty string for non-string data, got %q", result)
	}
}

func TestDecodeRevertFromError_WrongSelector(t *testing.T) {
	// Valid hex, 4+ bytes, but wrong selector (not Error(string))
	err := &mockDataError{
		msg:  "execution reverted",
		data: "0xdeadbeef00000000000000000000000000000000000000000000000000000000",
	}
	result := decodeRevertFromError(err)
	// decodeRevertFromError only tries abi.UnpackRevert which requires the Error(string) selector.
	// Wrong selector → UnpackRevert fails → returns ""
	if result != "" {
		t.Errorf("expected empty string for wrong selector, got %q", result)
	}
}

func TestDecodeRevertFromError_WrappedDataError(t *testing.T) {
	revertHex := "0x" + makeRevertHex("insufficient funds")
	innerErr := &mockDataError{
		msg:  "execution reverted",
		data: revertHex,
	}
	wrappedErr := stderrors.Join(stderrors.New("outer"), innerErr)
	result := decodeRevertFromError(wrappedErr)
	if result != "insufficient funds" {
		t.Errorf("expected %q, got %q", "insufficient funds", result)
	}
}

func TestDecodeRevertFromError_AllContractReverts(t *testing.T) {
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
			result := decodeRevertFromError(err)
			if result != msg {
				t.Errorf("expected %q, got %q", msg, result)
			}
		})
	}
}
