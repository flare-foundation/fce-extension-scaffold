package validate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Status represents the outcome of a single verification check.
type Status int

const (
	PASS Status = iota
	WARN
	FAIL
	SKIP
)

// String returns the human-readable label for a Status.
func (s Status) String() string {
	switch s {
	case PASS:
		return "PASS"
	case WARN:
		return "WARN"
	case FAIL:
		return "FAIL"
	case SKIP:
		return "SKIP"
	default:
		return "UNKNOWN"
	}
}

// CheckResult captures the outcome of a single verification check.
type CheckResult struct {
	Step    string // "deploy", "register", "services", "tee-version", "tee-machine", "test"
	ID      string // "D1", "D2", etc.
	Name    string // human-readable check name
	Status  Status
	Message string // what was found
	Fix     string // how to fix (only on FAIL/WARN)
}

// Report collects the results of all verification checks.
type Report struct {
	Results []CheckResult
}

// Add appends a CheckResult to the report.
func (r *Report) Add(result CheckResult) {
	r.Results = append(r.Results, result)
}

// HasFailures returns true if any result has FAIL status.
func (r *Report) HasFailures() bool {
	for _, res := range r.Results {
		if res.Status == FAIL {
			return true
		}
	}
	return false
}

// Summary returns a string like "N passed, N warning, N failed, N skipped".
func (r *Report) Summary() string {
	var passed, warning, failed, skipped int
	for _, res := range r.Results {
		switch res.Status {
		case PASS:
			passed++
		case WARN:
			warning++
		case FAIL:
			failed++
		case SKIP:
			skipped++
		}
	}
	return fmt.Sprintf("%d passed, %d warning, %d failed, %d skipped",
		passed, warning, failed, skipped)
}

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// stepLabel maps step identifiers to human-readable labels.
var stepLabel = map[string]string{
	"deploy":      "Contract Deployment",
	"register":    "Extension Registration",
	"services":    "Service Startup",
	"tee-version": "TEE Version Registration",
	"tee-machine": "TEE Machine Registration",
	"test":        "Testing",
}

// statusColor returns the ANSI color code for a given status.
func statusColor(s Status) string {
	switch s {
	case PASS:
		return colorGreen
	case WARN:
		return colorYellow
	case FAIL:
		return colorRed
	case SKIP:
		return colorGray
	default:
		return colorReset
	}
}

// Fprint writes a colored terminal report to w, grouped by Step.
func (r *Report) Fprint(w io.Writer) {
	fmt.Fprintf(w, "\n%s=== Deployment Verification Report ===%s\n\n", colorCyan, colorReset)

	// Collect unique steps in order of first appearance.
	var steps []string
	seen := make(map[string]bool)
	for _, res := range r.Results {
		if !seen[res.Step] {
			seen[res.Step] = true
			steps = append(steps, res.Step)
		}
	}

	for _, step := range steps {
		label := stepLabel[step]
		if label == "" {
			label = step
		}
		fmt.Fprintf(w, "%s:\n", label)

		for _, res := range r.Results {
			if res.Step != step {
				continue
			}
			color := statusColor(res.Status)
			fmt.Fprintf(w, "  %s[%s]%s %s  %s\n", color, res.Status, colorReset, res.ID, res.Name)

			if res.Status == FAIL || res.Status == WARN {
				if res.Message != "" {
					fmt.Fprintf(w, "         %s\n", res.Message)
				}
				if res.Fix != "" {
					fmt.Fprintf(w, "         Fix: %s\n", res.Fix)
				}
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n", r.Summary())
}

// Print writes the colored report to stdout.
func (r *Report) Print() {
	r.Fprint(os.Stdout)
}

// jsonResult is the JSON-serializable form of a CheckResult.
type jsonResult struct {
	Step    string `json:"step"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

// jsonReport is the top-level JSON output structure.
type jsonReport struct {
	Results []jsonResult `json:"results"`
	Summary string       `json:"summary"`
}

// FprintJSON writes the report as pretty-printed JSON to w.
func (r *Report) FprintJSON(w io.Writer) {
	jr := jsonReport{
		Results: make([]jsonResult, len(r.Results)),
		Summary: r.Summary(),
	}
	for i, res := range r.Results {
		jr.Results[i] = jsonResult{
			Step:    res.Step,
			ID:      res.ID,
			Name:    res.Name,
			Status:  res.Status.String(),
			Message: strings.TrimSpace(res.Message),
			Fix:     strings.TrimSpace(res.Fix),
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(jr)
}

// PrintJSON writes the JSON report to stdout.
func (r *Report) PrintJSON() {
	r.FprintJSON(os.Stdout)
}
