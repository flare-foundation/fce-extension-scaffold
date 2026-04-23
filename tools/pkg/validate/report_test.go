package validate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReport_Summary(t *testing.T) {
	var r Report
	r.Add(CheckResult{Step: "deploy", ID: "D1", Name: "Contract deployed", Status: PASS})
	r.Add(CheckResult{Step: "deploy", ID: "D2", Name: "Balance check", Status: WARN, Message: "Low balance", Fix: "Add funds"})
	r.Add(CheckResult{Step: "services", ID: "S1", Name: "Service skipped", Status: SKIP})

	want := "1 passed, 1 warning, 0 failed, 1 skipped"
	got := r.Summary()
	if got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestReport_HasFailures(t *testing.T) {
	var r Report
	r.Add(CheckResult{Step: "deploy", ID: "D1", Name: "OK check", Status: PASS})
	r.Add(CheckResult{Step: "deploy", ID: "D2", Name: "Warn check", Status: WARN})

	if r.HasFailures() {
		t.Error("HasFailures() = true, want false (no FAIL results)")
	}

	r.Add(CheckResult{Step: "register", ID: "R1", Name: "Bad check", Status: FAIL, Message: "broken", Fix: "fix it"})

	if !r.HasFailures() {
		t.Error("HasFailures() = false, want true (FAIL result added)")
	}
}

func TestReport_PrintJSON(t *testing.T) {
	var r Report
	r.Add(CheckResult{Step: "deploy", ID: "D1", Name: "Contract deployed", Status: PASS, Message: "found code"})
	r.Add(CheckResult{Step: "register", ID: "R1", Name: "Extension registered", Status: FAIL, Message: "not registered", Fix: "run register cmd"})

	var buf bytes.Buffer
	r.FprintJSON(&buf)

	var parsed struct {
		Results []struct {
			Step    string `json:"step"`
			ID      string `json:"id"`
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
			Fix     string `json:"fix"`
		} `json:"results"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\nraw: %s", err, buf.String())
	}

	if len(parsed.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(parsed.Results))
	}

	// Status must be the string "PASS", not a number.
	if parsed.Results[0].Status != "PASS" {
		t.Errorf("results[0].status = %q, want \"PASS\"", parsed.Results[0].Status)
	}
	if parsed.Results[1].Status != "FAIL" {
		t.Errorf("results[1].status = %q, want \"FAIL\"", parsed.Results[1].Status)
	}

	// Verify fields.
	if parsed.Results[0].Step != "deploy" {
		t.Errorf("results[0].step = %q, want \"deploy\"", parsed.Results[0].Step)
	}
	if parsed.Results[1].Fix != "run register cmd" {
		t.Errorf("results[1].fix = %q, want \"run register cmd\"", parsed.Results[1].Fix)
	}

	// Summary should be present.
	if !strings.Contains(parsed.Summary, "passed") {
		t.Errorf("summary missing 'passed': %q", parsed.Summary)
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{PASS, "PASS"},
		{WARN, "WARN"},
		{FAIL, "FAIL"},
		{SKIP, "SKIP"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", int(tt.s), got, tt.want)
		}
	}
}

func TestReport_EmptyReport(t *testing.T) {
	var r Report

	want := "0 passed, 0 warning, 0 failed, 0 skipped"
	got := r.Summary()
	if got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}

	if r.HasFailures() {
		t.Error("HasFailures() = true for empty report, want false")
	}

	// JSON output should be valid with empty results array
	var buf bytes.Buffer
	r.FprintJSON(&buf)
	var parsed struct {
		Results []interface{} `json:"results"`
		Summary string        `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse empty report JSON: %v", err)
	}
	if len(parsed.Results) != 0 {
		t.Errorf("expected 0 results in JSON, got %d", len(parsed.Results))
	}
	if parsed.Summary != want {
		t.Errorf("JSON summary = %q, want %q", parsed.Summary, want)
	}
}

func TestReport_AllStatuses(t *testing.T) {
	var r Report
	r.Add(CheckResult{Step: "deploy", ID: "D1", Name: "Pass check", Status: PASS})
	r.Add(CheckResult{Step: "deploy", ID: "D2", Name: "Warn check", Status: WARN, Message: "warning"})
	r.Add(CheckResult{Step: "register", ID: "R1", Name: "Fail check", Status: FAIL, Message: "failed", Fix: "fix it"})
	r.Add(CheckResult{Step: "services", ID: "S1", Name: "Skip check", Status: SKIP})

	if !r.HasFailures() {
		t.Error("HasFailures() = false, want true (has FAIL)")
	}

	want := "1 passed, 1 warning, 1 failed, 1 skipped"
	got := r.Summary()
	if got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestStatus_String_Unknown(t *testing.T) {
	unknown := Status(99)
	if got := unknown.String(); got != "UNKNOWN" {
		t.Errorf("Status(99).String() = %q, want %q", got, "UNKNOWN")
	}
}

func TestReport_FprintJSON_OmitsEmptyFields(t *testing.T) {
	var r Report
	r.Add(CheckResult{Step: "deploy", ID: "D1", Name: "OK", Status: PASS})

	var buf bytes.Buffer
	r.FprintJSON(&buf)

	// The JSON output should NOT contain "fix" or "message" keys for PASS results
	// because they use omitempty
	raw := buf.String()
	if strings.Contains(raw, `"fix"`) {
		t.Error("JSON contains 'fix' field for PASS result — should be omitted")
	}
	if strings.Contains(raw, `"message"`) {
		t.Error("JSON contains 'message' field for PASS result — should be omitted")
	}
}

func TestReport_Fprint_UnknownStep(t *testing.T) {
	var r Report
	r.Add(CheckResult{Step: "unknown-step", ID: "X1", Name: "Custom check", Status: PASS})

	var buf bytes.Buffer
	r.Fprint(&buf)
	out := buf.String()

	// For unknown steps, the step key itself should be used as the label
	if !strings.Contains(out, "unknown-step") {
		t.Error("output should use step key as label for unknown steps")
	}
}

func TestReport_Fprint(t *testing.T) {
	var r Report
	r.Add(CheckResult{Step: "deploy", ID: "D1", Name: "Contract deployed", Status: PASS})
	r.Add(CheckResult{Step: "deploy", ID: "D2", Name: "Balance check", Status: FAIL, Message: "No funds", Fix: "Add ETH"})
	r.Add(CheckResult{Step: "services", ID: "S1", Name: "API running", Status: WARN, Message: "Slow start", Fix: "Check logs"})

	var buf bytes.Buffer
	r.Fprint(&buf)
	out := buf.String()

	// Header present.
	if !strings.Contains(out, "Deployment Verification Report") {
		t.Error("output missing header")
	}

	// Step grouping.
	if !strings.Contains(out, "Contract Deployment") {
		t.Error("output missing 'Contract Deployment' group header")
	}
	if !strings.Contains(out, "Service Startup") {
		t.Error("output missing 'Service Startup' group header")
	}

	// Status labels present.
	if !strings.Contains(out, "[PASS]") {
		t.Error("output missing [PASS]")
	}
	if !strings.Contains(out, "[FAIL]") {
		t.Error("output missing [FAIL]")
	}
	if !strings.Contains(out, "[WARN]") {
		t.Error("output missing [WARN]")
	}

	// Fix line present for FAIL.
	if !strings.Contains(out, "Fix: Add ETH") {
		t.Error("output missing fix line for FAIL result")
	}

	// Summary at the end.
	if !strings.Contains(out, "1 passed, 1 warning, 1 failed, 0 skipped") {
		t.Errorf("output missing correct summary line, got:\n%s", out)
	}
}
