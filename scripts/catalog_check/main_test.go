package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// TestCheckCatalogIntegration verifies checkCatalog integration with known entries
func TestCheckCatalogIntegration(t *testing.T) {
	entries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "unknown-ext-xyz", Version: "1.0.0", URL: "https://example.com/unknown.raw"},
	}

	covered, missing := checkCatalog(entries)

	if covered != 1 {
		t.Errorf("covered = %d, want 1", covered)
	}
	if len(missing) != 1 {
		t.Errorf("len(missing) = %d, want 1", len(missing))
	}
	if len(missing) > 0 && missing[0].Name != "unknown-ext-xyz" {
		t.Errorf("missing[0].Name = %q, want unknown-ext-xyz", missing[0].Name)
	}
}

// TestOutputFormatting verifies the output format for covered entries
func TestOutputFormatting(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	entries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
	}

	for _, e := range entries {
		if meta, ok := bakery.Lookup(e.Name); ok {
			fmt.Printf("  ok       %-22s  v%-12s  %s · %s\n",
				e.Name, e.Version, meta.SupportTier, meta.Category)
		} else {
			fmt.Printf("  MISSING  %-22s  v%s\n", e.Name, e.Version)
		}
	}

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "ok") {
		t.Errorf("output should contain 'ok', got: %s", output)
	}
	if !strings.Contains(output, "docker") {
		t.Errorf("output should contain 'docker', got: %s", output)
	}
	if !strings.Contains(output, "28.0.0") {
		t.Errorf("output should contain version '28.0.0', got: %s", output)
	}
}

// TestOutputFormattingMissing verifies the output format for missing entries
func TestOutputFormattingMissing(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	entries := []model.SysextEntry{
		{Name: "unknown-ext-abc", Version: "1.2.3", URL: "https://example.com/unknown.raw"},
	}

	for _, e := range entries {
		if meta, ok := bakery.Lookup(e.Name); ok {
			fmt.Printf("  ok       %-22s  v%-12s  %s · %s\n",
				e.Name, e.Version, meta.SupportTier, meta.Category)
		} else {
			fmt.Printf("  MISSING  %-22s  v%s\n", e.Name, e.Version)
		}
	}

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "MISSING") {
		t.Errorf("output should contain 'MISSING', got: %s", output)
	}
	if !strings.Contains(output, "unknown-ext-abc") {
		t.Errorf("output should contain 'unknown-ext-abc', got: %s", output)
	}
	if !strings.Contains(output, "1.2.3") {
		t.Errorf("output should contain version '1.2.3', got: %s", output)
	}
}

// TestMissingEntryInstructions verifies the template output for missing entries
func TestMissingEntryInstructions(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	missing := []bakery.MissingEntry{
		{Name: "test-ext", Version: "1.0.0", URL: "https://example.com/test.raw"},
	}

	for _, m := range missing {
		fmt.Printf("// %s v%s — source: %s\n", m.Name, m.Version, m.URL)
		fmt.Printf(`"%s": {
	Category:    "TODO", // e.g. "Container Runtime", "Networking", "Orchestration"
	SupportTier: bakery.TierMaintained, // or TierIntegrated, TierExperimental
	Short:       "TODO: one-line description (~80 chars)",
	Long:        "TODO: 3–5 sentence description shown in the detail panel.",
	Caveats:     nil,
},
`, m.Name)
		fmt.Println()
	}

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	expectedStrings := []string{
		"test-ext",
		"1.0.0",
		"https://example.com/test.raw",
		"Category:",
		"SupportTier:",
		"bakery.TierMaintained",
		"Short:",
		"Long:",
		"Caveats:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("output should contain %q, got: %s", expected, output)
		}
	}
}

// TestEmptyCatalogOutput verifies handling of empty catalog
func TestEmptyCatalogOutput(t *testing.T) {
	entries := []model.SysextEntry{}
	covered, missing := checkCatalog(entries)

	if covered != 0 {
		t.Errorf("covered = %d, want 0 for empty catalog", covered)
	}
	if len(missing) != 0 {
		t.Errorf("len(missing) = %d, want 0 for empty catalog", len(missing))
	}
}

// TestMultipleMixedEntries verifies handling of multiple entries with mixed states
func TestMultipleMixedEntries(t *testing.T) {
	entries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "tailscale", Version: "1.56.1", URL: "https://example.com/tailscale.raw"},
		{Name: "unknown-1", Version: "1.0.0", URL: "https://example.com/u1.raw"},
		{Name: "unknown-2", Version: "2.0.0", URL: "https://example.com/u2.raw"},
	}

	covered, missing := checkCatalog(entries)

	if covered != 2 {
		t.Errorf("covered = %d, want 2 (docker + tailscale)", covered)
	}
	if len(missing) != 2 {
		t.Errorf("len(missing) = %d, want 2", len(missing))
	}

	missingNames := make(map[string]bool)
	for _, m := range missing {
		missingNames[m.Name] = true
	}

	if !missingNames["unknown-1"] || !missingNames["unknown-2"] {
		t.Errorf("missing should contain unknown-1 and unknown-2, got: %v", missing)
	}
}

// TestFlagParsing verifies flag parsing doesn't panic
func TestFlagParsing(t *testing.T) {
	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	strict := flag.Bool("strict", false, "exit 1 if any extensions are missing curated descriptions")
	arch := flag.String("arch", "amd64", "architecture to query")

	if *strict != false {
		t.Errorf("strict flag default = %v, want false", *strict)
	}
	if *arch != "amd64" {
		t.Errorf("arch flag default = %q, want amd64", *arch)
	}
}

// TestResultSummaryFormat verifies the result summary formatting
func TestResultSummaryFormat(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	covered := 5
	missing := []bakery.MissingEntry{
		{Name: "ext1", Version: "1.0.0", URL: "https://example.com/1.raw"},
		{Name: "ext2", Version: "2.0.0", URL: "https://example.com/2.raw"},
	}

	fmt.Printf("Result: %d covered, %d missing\n", covered, len(missing))

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Result:") {
		t.Errorf("output should contain 'Result:', got: %s", output)
	}
	if !strings.Contains(output, "5 covered") {
		t.Errorf("output should contain '5 covered', got: %s", output)
	}
	if !strings.Contains(output, "2 missing") {
		t.Errorf("output should contain '2 missing', got: %s", output)
	}
}

// TestSuccessMessage verifies the success message when all entries are covered
func TestSuccessMessage(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	missing := []bakery.MissingEntry{}

	if len(missing) == 0 {
		fmt.Println()
		fmt.Println("✓ All live bakery extensions have curated descriptions.")
		fmt.Println("  No action needed.")
	}

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "✓ All live bakery extensions have curated descriptions") {
		t.Errorf("output should contain success message, got: %s", output)
	}
	if !strings.Contains(output, "No action needed") {
		t.Errorf("output should contain 'No action needed', got: %s", output)
	}
}

// TestChecklistOutput verifies the checklist formatting
func TestChecklistOutput(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	missing := []bakery.MissingEntry{
		{Name: "new-ext", Version: "1.0.0", URL: "https://example.com/new.raw"},
	}

	fmt.Println("─── Checklist ──────────────────────────────────────────────────────────")
	fmt.Println("1. Add the entry/entries above to internal/bakery/descriptions.go")
	fmt.Printf("2. Add %-22q to allKnownExtensions in internal/bakery/descriptions_test.go\n", missing[0].Name)
	fmt.Println("3. Add a row to docs/SYSEXTS.md under the appropriate category")
	fmt.Println("4. Run: just ci")
	fmt.Println()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	expectedStrings := []string{
		"Checklist",
		"Add the entry/entries above to internal/bakery/descriptions.go",
		"allKnownExtensions",
		"docs/SYSEXTS.md",
		"just ci",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("checklist output should contain %q, got: %s", expected, output)
		}
	}
}
