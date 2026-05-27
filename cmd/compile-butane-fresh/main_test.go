package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestMain re-uses the compiled test binary as the compile-butane-fresh
// subprocess. When COMPILE_BUTANE_FRESH_TEST_MAIN=1 the process delegates
// directly to main(), allowing early-exit and error-path testing without a
// separately compiled binary.
func TestMain(m *testing.M) {
	if os.Getenv("COMPILE_BUTANE_FRESH_TEST_MAIN") == "1" {
		main()
		os.Exit(0) // reached only if main() returned without calling os.Exit
	}
	os.Exit(m.Run())
}

// butaneFilePath is the hardcoded path that compile-butane-fresh reads from.
const butaneFilePath = "/tmp/karnataka-fresh.butane"

// helperCmd builds a subprocess that invokes main() via the test binary.
func helperCmd(t *testing.T) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), "COMPILE_BUTANE_FRESH_TEST_MAIN=1")
	return cmd
}

// writeBataneFile writes content to the butane input file and removes it on
// test cleanup.
func writeBataneFile(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(butaneFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("writeBataneFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(butaneFilePath) })
}

// TestMain_FileNotFound verifies that a missing butane file exits 1 and prints
// an "Error:" message to stderr. Covers os.ReadFile error branch.
func TestMain_FileNotFound(t *testing.T) {
	_ = os.Remove(butaneFilePath) // ensure absent

	cmd := helperCmd(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing butane file, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("exit code: got %d, want 1", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error:") {
		t.Errorf("expected 'Error:' in output; got: %s", out)
	}
}

// TestMain_InvalidButane verifies that syntactically invalid butane exits 1
// and prints a "Compile error:" message. Covers CompileToIgnition error branch.
func TestMain_InvalidButane(t *testing.T) {
	writeBataneFile(t, "this is: not: valid: butane: {{{")

	cmd := helperCmd(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid butane, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("exit code: got %d, want 1", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Compile error:") {
		t.Errorf("expected 'Compile error:' in output; got: %s", out)
	}
}

// TestMain_ValidButane verifies that a minimal valid butane config exits 0
// and prints ignition JSON (containing the "ignition" key) to stdout.
// Covers the success path.
func TestMain_ValidButane(t *testing.T) {
	const minimalButane = `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      mode: 0644
      contents:
        inline: "testhost"
`
	writeBataneFile(t, minimalButane)

	cmd := helperCmd(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 for valid butane, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), `"ignition"`) {
		t.Errorf("expected ignition JSON in stdout; got: %s", out)
	}
}
