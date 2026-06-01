package main

import (
	"testing"
)

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
