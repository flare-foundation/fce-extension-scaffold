package fccutils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestLoadState_NoFile(t *testing.T) {
	state, err := loadState("/nonexistent/path/register-tee.state")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if state.CompletedSteps != "" {
		t.Errorf("expected empty CompletedSteps, got %q", state.CompletedSteps)
	}
	if state.TeeAttestInstructionID != (common.Hash{}) {
		t.Errorf("expected zero TeeAttestInstructionID, got %s", state.TeeAttestInstructionID.Hex())
	}
	if state.InstructionID != (common.Hash{}) {
		t.Errorf("expected zero InstructionID, got %s", state.InstructionID.Hex())
	}
}

func TestLoadState_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register-tee.state")

	expectedHash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	state := registrationState{
		CompletedSteps:         "rR",
		TeeAttestInstructionID: expectedHash,
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.CompletedSteps != "rR" {
		t.Errorf("expected CompletedSteps %q, got %q", "rR", loaded.CompletedSteps)
	}
	if loaded.TeeAttestInstructionID != expectedHash {
		t.Errorf("expected TeeAttestInstructionID %s, got %s",
			expectedHash.Hex(), loaded.TeeAttestInstructionID.Hex())
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register-tee.state")
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadState(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if msg := err.Error(); msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestLoadState_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register-tee.state")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadState(path)
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}

func TestSaveState_WritesCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register-tee.state")

	instrID := common.HexToHash("0xdeadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678")
	state := &registrationState{
		CompletedSteps:         "rRa",
		TeeAttestInstructionID: common.HexToHash("0xaaaa"),
		InstructionID:          instrID,
	}

	err := saveState(path, state)
	if err != nil {
		t.Fatalf("saveState failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var loaded registrationState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal saved state: %v", err)
	}

	if loaded.CompletedSteps != "rRa" {
		t.Errorf("expected CompletedSteps %q, got %q", "rRa", loaded.CompletedSteps)
	}
	if loaded.InstructionID != instrID {
		t.Errorf("expected InstructionID %s, got %s", instrID.Hex(), loaded.InstructionID.Hex())
	}
}

func TestSaveState_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register-tee.state")

	// Save initial state
	state1 := &registrationState{CompletedSteps: "r"}
	if err := saveState(path, state1); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	// Overwrite with updated state
	state2 := &registrationState{CompletedSteps: "rR"}
	if err := saveState(path, state2); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	// Read back — should see "rR", not "r" or "rrR"
	loaded, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState failed: %v", err)
	}
	if loaded.CompletedSteps != "rR" {
		t.Errorf("expected %q after overwrite, got %q", "rR", loaded.CompletedSteps)
	}
}

func TestSaveState_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(readOnlyDir, 0755)
	})

	path := filepath.Join(readOnlyDir, "register-tee.state")
	state := &registrationState{CompletedSteps: "r"}
	err := saveState(path, state)
	if err == nil {
		t.Fatal("expected error writing to read-only directory, got nil")
	}
}

func TestSaveAndLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register-tee.state")

	original := &registrationState{
		CompletedSteps:         "rRap",
		TeeAttestInstructionID: common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
		InstructionID:          common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"),
	}

	if err := saveState(path, original); err != nil {
		t.Fatalf("saveState failed: %v", err)
	}

	loaded, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState failed: %v", err)
	}

	if loaded.CompletedSteps != original.CompletedSteps {
		t.Errorf("CompletedSteps: expected %q, got %q", original.CompletedSteps, loaded.CompletedSteps)
	}
	if loaded.TeeAttestInstructionID != original.TeeAttestInstructionID {
		t.Errorf("TeeAttestInstructionID: expected %s, got %s",
			original.TeeAttestInstructionID.Hex(), loaded.TeeAttestInstructionID.Hex())
	}
	if loaded.InstructionID != original.InstructionID {
		t.Errorf("InstructionID: expected %s, got %s",
			original.InstructionID.Hex(), loaded.InstructionID.Hex())
	}
}

func TestLoadState_ExtraFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register-tee.state")
	// State file with an extra field that doesn't exist in the struct
	content := `{"completed_steps":"r","tee_attest_instruction_id":"0x0000000000000000000000000000000000000000000000000000000000000000","extra_field":"should be ignored"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.CompletedSteps != "r" {
		t.Errorf("expected CompletedSteps %q, got %q", "r", loaded.CompletedSteps)
	}
}
