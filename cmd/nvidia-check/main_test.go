package main

import (
	"fmt"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// docWith returns a fake Flatcar docs string containing the listed driver series.
func docWith(series ...string) string {
	var sb strings.Builder
	for _, s := range series {
		fmt.Fprintf(&sb, "SYSEXTNAME=%s\n", s)
	}
	return sb.String()
}

func fetchOK(series ...string) func() (string, error) {
	return func() (string, error) { return docWith(series...), nil }
}

func fetchErr(msg string) func() (string, error) {
	return func() (string, error) { return "", fmt.Errorf("%s", msg) }
}

// ── run() tests ───────────────────────────────────────────────────────────────

func TestRun_FetchError_Returns2(t *testing.T) {
	var w strings.Builder
	rc := run(&w, fetchErr("network failure"))
	if rc != 2 {
		t.Errorf("expected exit code 2, got %d", rc)
	}
	if !strings.Contains(w.String(), "ERROR") {
		t.Error("expected ERROR in output on fetch failure")
	}
}

func TestRun_NoMissingDrivers_Returns0(t *testing.T) {
	// Serve exactly the series that model.go already contains.
	// If model is consistent with docs, run() must return 0.
	var w strings.Builder
	// Use an empty doc — nothing in docs, so nothing is missing from model.
	rc := run(&w, fetchOK())
	if rc != 0 {
		t.Errorf("expected exit code 0 (consistent), got %d; output:\n%s", rc, w.String())
	}
	if !strings.Contains(w.String(), "consistent") {
		t.Error("expected 'consistent' message in output")
	}
}

func TestRun_MissingInModel_Returns1(t *testing.T) {
	// Docs mention a series that model.go does not contain → ACTION REQUIRED.
	var w strings.Builder
	rc := run(&w, fetchOK("nvidia-drivers-9999-open"))
	if rc != 1 {
		t.Errorf("expected exit code 1 (missing drivers), got %d; output:\n%s", rc, w.String())
	}
	if !strings.Contains(w.String(), "MISSING IN MODEL") {
		t.Error("expected 'MISSING IN MODEL' in output")
	}
	if !strings.Contains(w.String(), "ACTION REQUIRED") {
		t.Error("expected 'ACTION REQUIRED' in output")
	}
}

func TestRun_ModelExtraNotInDocs_Returns0(t *testing.T) {
	// model.go has entries not in docs — that is normal (just a NOTE), not an error.
	var w strings.Builder
	// Fetch returns empty docs → model has entries, docs don't → NOTE, not error.
	rc := run(&w, fetchOK())
	if rc != 0 {
		t.Errorf("expected exit code 0 when model has extras not in docs, got %d", rc)
	}
}

func TestRun_OutputContainsBanner(t *testing.T) {
	var w strings.Builder
	_ = run(&w, fetchOK())
	out := w.String()
	if !strings.Contains(out, "nvidia_check") {
		t.Error("expected 'nvidia_check' banner in output")
	}
}

func TestRun_EmptyDocs_ListsNoDocSeries(t *testing.T) {
	var w strings.Builder
	_ = run(&w, fetchOK())
	out := w.String()
	if !strings.Contains(out, "none found") {
		t.Error("expected '(none found)' message when docs are empty")
	}
}

func TestRun_NonEmptyDocs_ListsDocSeries(t *testing.T) {
	var w strings.Builder
	_ = run(&w, fetchOK("nvidia-drivers-550-open", "nvidia-drivers-535-open"))
	out := w.String()
	if !strings.Contains(out, "550-open") {
		t.Errorf("expected '550-open' in output; got:\n%s", out)
	}
}

// ── extractDriverSeries tests (pre-existing, kept) ───────────────────────────

func TestExtractDriverSeries_Empty(t *testing.T) {
	result := extractDriverSeries("")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestExtractDriverSeries_Single(t *testing.T) {
	doc := "Install with nvidia-drivers-latest and reboot."
	result := extractDriverSeries(doc)
	if len(result) != 1 || result[0] != "nvidia-drivers-latest" {
		t.Errorf("expected [nvidia-drivers-latest], got %v", result)
	}
}

func TestExtractDriverSeries_Multiple(t *testing.T) {
	doc := `Available driver series:
- SYSEXTNAME=nvidia-drivers-550
- SYSEXTNAME=nvidia-drivers-535
- SYSEXTNAME=nvidia-drivers-470`
	result := extractDriverSeries(doc)
	if len(result) != 3 {
		t.Errorf("expected 3 results, got %d: %v", len(result), result)
	}
	expected := []string{"nvidia-drivers-550", "nvidia-drivers-535", "nvidia-drivers-470"}
	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("result[%d] = %q, want %q", i, result[i], exp)
		}
	}
}

func TestExtractDriverSeries_Dedup(t *testing.T) {
	doc := "nvidia-drivers-latest and nvidia-drivers-latest again"
	result := extractDriverSeries(doc)
	if len(result) != 1 || result[0] != "nvidia-drivers-latest" {
		t.Errorf("expected deduped [nvidia-drivers-latest], got %v", result)
	}
}

func TestExtractDriverSeries_Production(t *testing.T) {
	// Production-like examples (open-source and proprietary variants)
	doc := `Example: SYSEXTNAME=nvidia-drivers-latest SYSEXTOPTS="--strip"
For older GPUs: SYSEXTNAME=nvidia-drivers-550
Also available: nvidia-drivers-production nvidia-drivers-470`
	result := extractDriverSeries(doc)
	if len(result) != 4 {
		t.Errorf("expected 4 results, got %d: %v", len(result), result)
	}
	seen := make(map[string]bool)
	for _, r := range result {
		seen[r] = true
	}
	for _, want := range []string{"nvidia-drivers-latest", "nvidia-drivers-550", "nvidia-drivers-production", "nvidia-drivers-470"} {
		if !seen[want] {
			t.Errorf("missing expected driver series: %s", want)
		}
	}
}
