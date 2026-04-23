package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckExtensionEnvFormat_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extension.env")
	content := `# This is a comment
INSTRUCTION_SENDER=0x1234567890abcdef1234567890abcdef12345678
EXTENSION_ID=0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := CheckExtensionEnvFormat(path)

	for _, res := range results {
		if res.Status == FAIL {
			t.Errorf("unexpected FAIL: %s — %s", res.Name, res.Message)
		}
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Both should be PASS.
	for _, res := range results {
		if res.Status != PASS {
			t.Errorf("expected PASS for %s, got %s: %s", res.Name, res.Status, res.Message)
		}
		if res.Step != "deploy" {
			t.Errorf("expected step=deploy, got %s", res.Step)
		}
		if res.ID != "D7" {
			t.Errorf("expected ID=D7, got %s", res.ID)
		}
	}
}

func TestCheckExtensionEnvFormat_Malformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extension.env")
	content := `INSTRUCTION_SENDER=not-an-address
EXTENSION_ID=0xTOOSHORT
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := CheckExtensionEnvFormat(path)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	failCount := 0
	for _, res := range results {
		if res.Status == FAIL {
			failCount++
		}
	}

	if failCount != 2 {
		t.Errorf("expected 2 FAILs, got %d", failCount)
	}
}

func TestCheckExtensionEnvFormat_Missing(t *testing.T) {
	results := CheckExtensionEnvFormat("/nonexistent/path/extension.env")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Status != SKIP {
		t.Errorf("expected SKIP, got %s", results[0].Status)
	}

	if results[0].Step != "deploy" {
		t.Errorf("expected step=deploy, got %s", results[0].Step)
	}

	if results[0].ID != "D7" {
		t.Errorf("expected ID=D7, got %s", results[0].ID)
	}
}

func TestCheckDeployerKeySource_DevKey(t *testing.T) {
	t.Setenv("DEPLOYMENT_PRIVATE_KEY", "")
	t.Setenv("LOCAL_MODE", "false")

	result := CheckDeployerKeySource()

	if result.Status != WARN {
		t.Errorf("expected WARN, got %s: %s", result.Status, result.Message)
	}

	if result.Step != "deploy" {
		t.Errorf("expected step=deploy, got %s", result.Step)
	}

	if result.ID != "D5" {
		t.Errorf("expected ID=D5, got %s", result.ID)
	}

	if result.Fix == "" {
		t.Error("expected a Fix message, got empty")
	}
}

func TestCheckDeployerKeySource_RealKey(t *testing.T) {
	t.Setenv("DEPLOYMENT_PRIVATE_KEY", "abc")

	result := CheckDeployerKeySource()

	if result.Status != PASS {
		t.Errorf("expected PASS, got %s: %s", result.Status, result.Message)
	}

	if result.Step != "deploy" {
		t.Errorf("expected step=deploy, got %s", result.Step)
	}

	if result.ID != "D5" {
		t.Errorf("expected ID=D5, got %s", result.ID)
	}
}

func TestCheckDeployerKeySource_DevKeyLocal(t *testing.T) {
	t.Setenv("DEPLOYMENT_PRIVATE_KEY", "")
	t.Setenv("LOCAL_MODE", "true")

	result := CheckDeployerKeySource()

	if result.Status != PASS {
		t.Errorf("expected PASS, got %s: %s", result.Status, result.Message)
	}

	if result.Step != "deploy" {
		t.Errorf("expected step=deploy, got %s", result.Step)
	}

	if result.ID != "D5" {
		t.Errorf("expected ID=D5, got %s", result.ID)
	}
}

func TestParseExtensionEnv_Valid(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "extension.env")
	os.WriteFile(envFile, []byte(
		"# Auto-generated\n"+
			"EXTENSION_ID=0x00000000000000000000000000000000000000000000000000000000000000db\n"+
			"INSTRUCTION_SENDER=0x32F967bE8F35F73274Bd3d4130073547361A0d75\n",
	), 0644)

	extID, instrSender, err := parseExtensionEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extID != "0x00000000000000000000000000000000000000000000000000000000000000db" {
		t.Fatalf("unexpected EXTENSION_ID: %s", extID)
	}
	if instrSender != "0x32F967bE8F35F73274Bd3d4130073547361A0d75" {
		t.Fatalf("unexpected INSTRUCTION_SENDER: %s", instrSender)
	}
}

func TestParseExtensionEnv_Missing(t *testing.T) {
	extID, instrSender, err := parseExtensionEnv("/nonexistent/extension.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extID != "" || instrSender != "" {
		t.Fatalf("expected empty strings for missing file, got %q and %q", extID, instrSender)
	}
}

func TestRegisterServicesChecks_NoConfig(t *testing.T) {
	r := &Report{}
	RegisterServicesChecks(r, "")
	// Should have at least one SKIP
	hasSkip := false
	for _, res := range r.Results {
		if res.Status == SKIP {
			hasSkip = true
		}
	}
	if !hasSkip {
		t.Fatal("expected at least one SKIP result for empty config path")
	}
}

func TestRegisterServicesChecks_WithValidConfig(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "extension.env")
	os.WriteFile(envFile, []byte(
		"EXTENSION_ID=0x00000000000000000000000000000000000000000000000000000000000000db\n"+
			"INSTRUCTION_SENDER=0x32F967bE8F35F73274Bd3d4130073547361A0d75\n",
	), 0644)

	r := &Report{}
	RegisterServicesChecks(r, envFile)
	// Should have S2 PASS for extension ID format
	found := false
	for _, res := range r.Results {
		if res.ID == "S2" && res.Status == PASS {
			found = true
		}
	}
	if !found {
		t.Fatal("expected S2 PASS for valid extension ID")
	}
}

func TestRegisterRegistrationChecks_NoConfig(t *testing.T) {
	r := &Report{}
	RegisterRegistrationChecks(r, nil, nil, nil, nil, "")
	if len(r.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Results))
	}
	if r.Results[0].Status != SKIP {
		t.Fatalf("expected SKIP, got %s", r.Results[0].Status)
	}
}

func TestRegisterRegistrationChecks_MissingFile(t *testing.T) {
	r := &Report{}
	RegisterRegistrationChecks(r, nil, nil, nil, nil, "/nonexistent/extension.env")
	if len(r.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Results))
	}
	if r.Results[0].Status != SKIP {
		t.Fatalf("expected SKIP, got %s", r.Results[0].Status)
	}
}

func TestRegisterTeeMachineChecks_NoConfig(t *testing.T) {
	r := &Report{}
	RegisterTeeMachineChecks(r, "")
	if len(r.Results) < 1 {
		t.Fatalf("expected at least 1 result, got %d", len(r.Results))
	}
}

func TestRegisterTeeMachineChecks_SimulatedTeeWarning(t *testing.T) {
	t.Setenv("SIMULATED_TEE", "true")
	t.Setenv("LOCAL_MODE", "false")
	r := &Report{}
	RegisterTeeMachineChecks(r, "")
	foundWarn := false
	for _, res := range r.Results {
		if res.ID == "T1" && res.Status == WARN {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Fatal("expected T1 WARN when SIMULATED_TEE=true and LOCAL_MODE=false")
	}
}

func TestRegisterTeeVersionChecks_NoConfig(t *testing.T) {
	r := &Report{}
	RegisterTeeVersionChecks(r, "")
	// Should have results for V2, and SKIP for V4
	if len(r.Results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(r.Results))
	}
}

func TestRegisterTestChecks_NoConfig(t *testing.T) {
	r := &Report{}
	RegisterTestChecks(r, "")
	hasSkip := false
	for _, res := range r.Results {
		if res.Status == SKIP {
			hasSkip = true
		}
	}
	if !hasSkip {
		t.Fatal("expected at least one SKIP for empty config")
	}
}

func TestRegisterTestChecks_WithValidConfig(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "extension.env")
	os.WriteFile(envFile, []byte(
		"EXTENSION_ID=0x00000000000000000000000000000000000000000000000000000000000000db\n"+
			"INSTRUCTION_SENDER=0x32F967bE8F35F73274Bd3d4130073547361A0d75\n",
	), 0644)

	r := &Report{}
	RegisterTestChecks(r, envFile)
	foundE1 := false
	for _, res := range r.Results {
		if res.ID == "E1" && res.Status == PASS {
			foundE1 = true
		}
	}
	if !foundE1 {
		t.Fatal("expected E1 PASS for valid INSTRUCTION_SENDER")
	}
}

func TestRegisterTeeVersionChecks_WithConfig(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "extension.env")
	os.WriteFile(envFile, []byte(
		"EXTENSION_ID=0x00000000000000000000000000000000000000000000000000000000000000db\n"+
			"INSTRUCTION_SENDER=0x32F967bE8F35F73274Bd3d4130073547361A0d75\n",
	), 0644)

	t.Setenv("DEPLOYMENT_PRIVATE_KEY", "abc123")
	r := &Report{}
	RegisterTeeVersionChecks(r, envFile)
	// Should have V2 PASS and V4 PASS at minimum
	foundV2 := false
	foundV4 := false
	for _, res := range r.Results {
		if res.ID == "V2" && res.Status == PASS {
			foundV2 = true
		}
		if res.ID == "V4" && res.Status == PASS {
			foundV4 = true
		}
	}
	if !foundV2 {
		t.Fatal("expected V2 PASS")
	}
	if !foundV4 {
		t.Fatal("expected V4 PASS")
	}
}

// --- Additional env parsing edge case tests ---

func TestParseExtensionEnv_CommentsOnly(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "extension.env")
	os.WriteFile(envFile, []byte(
		"# This is a comment\n"+
			"# Another comment\n"+
			"\n"+
			"  \n",
	), 0644)

	extID, instrSender, err := parseExtensionEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extID != "" {
		t.Errorf("expected empty EXTENSION_ID, got %q", extID)
	}
	if instrSender != "" {
		t.Errorf("expected empty INSTRUCTION_SENDER, got %q", instrSender)
	}
}

func TestParseExtensionEnv_ExtraKeys(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "extension.env")
	os.WriteFile(envFile, []byte(
		"EXTENSION_ID=0x00000000000000000000000000000000000000000000000000000000000000db\n"+
			"INSTRUCTION_SENDER=0x32F967bE8F35F73274Bd3d4130073547361A0d75\n"+
			"EXTRA_KEY=some_value\n"+
			"ANOTHER_KEY=another_value\n",
	), 0644)

	extID, instrSender, err := parseExtensionEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extID != "0x00000000000000000000000000000000000000000000000000000000000000db" {
		t.Fatalf("unexpected EXTENSION_ID: %s", extID)
	}
	if instrSender != "0x32F967bE8F35F73274Bd3d4130073547361A0d75" {
		t.Fatalf("unexpected INSTRUCTION_SENDER: %s", instrSender)
	}
}

func TestParseExtensionEnv_NoEquals(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "extension.env")
	os.WriteFile(envFile, []byte(
		"EXTENSION_ID_NO_EQUALS\n"+
			"INSTRUCTION_SENDER=0x32F967bE8F35F73274Bd3d4130073547361A0d75\n",
	), 0644)

	extID, instrSender, err := parseExtensionEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extID != "" {
		t.Errorf("expected empty EXTENSION_ID for line without equals, got %q", extID)
	}
	if instrSender != "0x32F967bE8F35F73274Bd3d4130073547361A0d75" {
		t.Errorf("unexpected INSTRUCTION_SENDER: %s", instrSender)
	}
}

func TestCheckExtensionEnvFormat_LowercaseHex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extension.env")
	content := `INSTRUCTION_SENDER=0xabcdef1234567890abcdef1234567890abcdef12
EXTENSION_ID=0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := CheckExtensionEnvFormat(path)
	for _, res := range results {
		if res.Status == FAIL {
			t.Errorf("unexpected FAIL for lowercase hex: %s — %s", res.Name, res.Message)
		}
	}
}

func TestCheckExtensionEnvFormat_MixedCaseChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extension.env")
	// EIP-55 checksum address format
	content := `INSTRUCTION_SENDER=0x32F967bE8F35F73274Bd3d4130073547361A0d75
EXTENSION_ID=0x00000000000000000000000000000000000000000000000000000000000000Db
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := CheckExtensionEnvFormat(path)
	for _, res := range results {
		if res.Status == FAIL {
			t.Errorf("unexpected FAIL for mixed-case (EIP-55) address: %s — %s", res.Name, res.Message)
		}
	}
}

func TestCheckExtensionEnvFormat_EmptyValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extension.env")
	content := `INSTRUCTION_SENDER=
EXTENSION_ID=
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := CheckExtensionEnvFormat(path)
	failCount := 0
	for _, res := range results {
		if res.Status == FAIL {
			failCount++
		}
	}
	if failCount != 2 {
		t.Errorf("expected 2 FAILs for empty values, got %d", failCount)
	}
}

func TestCheckExtensionEnvFormat_NoPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extension.env")
	// Missing 0x prefix
	content := `INSTRUCTION_SENDER=1234567890abcdef1234567890abcdef12345678
EXTENSION_ID=1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results := CheckExtensionEnvFormat(path)
	failCount := 0
	for _, res := range results {
		if res.Status == FAIL {
			failCount++
		}
	}
	if failCount != 2 {
		t.Errorf("expected 2 FAILs for missing 0x prefix, got %d", failCount)
	}
}
