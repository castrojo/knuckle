package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestMain re-uses the compiled test binary as the knuckle subprocess.
// When KNUCKLE_TEST_MAIN=1, the process delegates directly to main() —
// so early-exit flag-validation paths can be tested without starting the TUI.
func TestMain(m *testing.M) {
	if os.Getenv("KNUCKLE_TEST_MAIN") == "1" {
		main()
		os.Exit(0) // reached only if main() returned without calling os.Exit
	}
	os.Exit(m.Run())
}

// helperResult captures the output, exit code, and timeout state of a
// subprocess run via runHelper.
type helperResult struct {
	output   []byte
	exitCode int
	timedOut bool
}

// runHelper executes the test binary as a knuckle subprocess with the given
// args and returns the combined output, exit code, and whether the 10-second
// timeout fired. Using a timeout means a future blocking-before-exit
// regression fails fast with a clear message rather than hanging CI.
func runHelper(t *testing.T, args ...string) helperResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), "KNUCKLE_TEST_MAIN=1")
	out, _ := cmd.CombinedOutput()
	return helperResult{
		output:   out,
		exitCode: cmd.ProcessState.ExitCode(),
		timedOut: ctx.Err() != nil,
	}
}

// TestMain_Version verifies --version prints "knuckle <ver>" and exits 0.
func TestMain_Version(t *testing.T) {
	r := runHelper(t, "--version")
	if r.timedOut {
		t.Fatal("subprocess timed out — possible hang before --version exit")
	}
	if r.exitCode != 0 {
		t.Fatalf("--version exited %d; output: %s", r.exitCode, r.output)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(r.output)), "knuckle ") {
		t.Errorf("--version output %q does not match 'knuckle <version>'", r.output)
	}
}

// TestMain_InvalidChannel verifies an unrecognised channel exits 1 with the
// bad channel name in the error.
func TestMain_InvalidChannel(t *testing.T) {
	r := runHelper(t, "--channel=bogus", "--log-file=/dev/null")
	if r.timedOut {
		t.Fatal("subprocess timed out — possible hang before channel-validation exit")
	}
	if r.exitCode != 1 {
		t.Fatalf("expected exit code 1 for invalid channel, got %d; output: %s", r.exitCode, r.output)
	}
	if !strings.Contains(string(r.output), "bogus") {
		t.Errorf("expected output to contain the bad channel name %q; got: %s", "bogus", r.output)
	}
}

// TestMain_HeadlessRequiresConfig verifies that --headless without --config
// exits 1 and prints the usage hint. The guard fires before the log file is
// opened, so no --log-file flag is needed.
func TestMain_HeadlessRequiresConfig(t *testing.T) {
	r := runHelper(t, "--headless")
	if r.timedOut {
		t.Fatal("subprocess timed out — possible hang before headless-guard exit")
	}
	if r.exitCode != 1 {
		t.Fatalf("expected exit code 1 for --headless without --config, got %d; output: %s", r.exitCode, r.output)
	}
	if !strings.Contains(string(r.output), "--config") {
		t.Errorf("expected output to mention '--config'; got: %s", r.output)
	}
	// Must NOT have entered runHeadless — no file-error message expected.
	if strings.Contains(string(r.output), "no such file") {
		t.Errorf("unexpectedly reached runHeadless; output: %s", r.output)
	}
}

// TestMain_HeadlessConfigNotFound verifies that --headless with a
// non-existent config file exits 1 with a file-error message.
// This covers the headlessMode=true branch of the headless guard.
func TestMain_HeadlessConfigNotFound(t *testing.T) {
	r := runHelper(t, "--headless",
		"--config=/nonexistent-knuckle-config-xyz.json",
		"--log-file=/dev/null")
	if r.timedOut {
		t.Fatal("subprocess timed out — possible hang in runHeadless")
	}
	if r.exitCode != 1 {
		t.Fatalf("expected exit code 1 for missing config, got %d; output: %s", r.exitCode, r.output)
	}
	// Must have entered runHeadless and hit the LoadConfig error.
	if !strings.Contains(string(r.output), "no such file") && !strings.Contains(string(r.output), "Error") {
		t.Errorf("expected file-not-found error; got: %s", r.output)
	}
	// Must NOT have hit the headless guard (configFile was provided).
	if strings.Contains(string(r.output), "requires --config") {
		t.Errorf("unexpectedly hit headless guard despite --config being set; output: %s", r.output)
	}
}

// TestMain_ConfigWithoutHeadless verifies that --config alone (no --headless)
// also triggers the headless path and exits 1 when the file is missing.
// This covers the configFile != "" branch of the headless guard — distinct
// from TestMain_HeadlessConfigNotFound which covers headlessMode == true.
func TestMain_ConfigWithoutHeadless(t *testing.T) {
	r := runHelper(t, "--config=/nonexistent-knuckle-config-xyz.json",
		"--log-file=/dev/null")
	if r.timedOut {
		t.Fatal("subprocess timed out — possible hang in runHeadless")
	}
	if r.exitCode != 1 {
		t.Fatalf("expected exit code 1 for missing config, got %d; output: %s", r.exitCode, r.output)
	}
	// Must have entered runHeadless and hit the LoadConfig error, not the guard.
	if !strings.Contains(string(r.output), "no such file") && !strings.Contains(string(r.output), "Error") {
		t.Errorf("expected file-not-found error; got: %s", r.output)
	}
	// Must NOT have hit the headless guard (configFile != "" bypasses it).
	if strings.Contains(string(r.output), "requires --config") {
		t.Errorf("unexpectedly hit headless guard despite --config being set; output: %s", r.output)
	}
}

// TestMain_LogFileOpenFailure verifies that an unwritable log file path exits 1
// with a descriptive error. Uses a path whose parent directory does not exist
// so the failure is permission-independent.
func TestMain_LogFileOpenFailure(t *testing.T) {
	r := runHelper(t, "--log-file=/nonexistent-dir/knuckle.log")
	if r.timedOut {
		t.Fatal("subprocess timed out — possible hang before log-open exit")
	}
	if r.exitCode != 1 {
		t.Fatalf("expected exit code 1 for bad log file, got %d; output: %s", r.exitCode, r.output)
	}
	if !strings.Contains(string(r.output), "Error opening log file") {
		t.Errorf("expected 'Error opening log file' message; got: %s", r.output)
	}
}
