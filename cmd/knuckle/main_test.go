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

// helperCmd builds a subprocess that runs main() with the supplied args.
// A 10-second timeout is applied so a future blocking-before-exit regression
// fails fast rather than hanging CI indefinitely.
func helperCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), "KNUCKLE_TEST_MAIN=1")
	return cmd
}

// TestMain_Version verifies --version prints "knuckle <ver>" and exits 0.
func TestMain_Version(t *testing.T) {
	cmd := helperCmd(t, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version exited non-zero: %v\noutput: %s", err, out)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out)), "knuckle ") {
		t.Errorf("--version output %q does not match 'knuckle <version>'", out)
	}
}

// TestMain_InvalidChannel verifies an unrecognised channel exits 1 with a
// descriptive error on stderr.
func TestMain_InvalidChannel(t *testing.T) {
	cmd := helperCmd(t, "--channel=bogus", "--log-file=/dev/null")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid channel, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "bogus") {
		t.Errorf("expected output to contain the bad channel name %q; got: %s", "bogus", out)
	}
}

// TestMain_HeadlessRequiresConfig verifies that --headless without --config
// exits 1 and prints the usage hint.
func TestMain_HeadlessRequiresConfig(t *testing.T) {
	cmd := helperCmd(t, "--headless")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for --headless without --config, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "--config") {
		t.Errorf("expected output to mention '--config'; got: %s", out)
	}
}

// TestMain_HeadlessConfigNotFound verifies that --headless with a
// non-existent config file exits 1 with an error message.
func TestMain_HeadlessConfigNotFound(t *testing.T) {
	cmd := helperCmd(t, "--headless",
		"--config=/nonexistent-knuckle-config-xyz.json",
		"--log-file=/dev/null")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing config file, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error") {
		t.Errorf("expected output to contain 'Error'; got: %s", out)
	}
}

// TestMain_ConfigWithoutHeadless verifies that --config alone (no --headless)
// also triggers the headless path and exits 1 when the file is missing.
// This covers the `configFile != ""` branch of the headless guard.
func TestMain_ConfigWithoutHeadless(t *testing.T) {
	cmd := helperCmd(t, "--config=/nonexistent-knuckle-config-xyz.json",
		"--log-file=/dev/null")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing config file, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error") {
		t.Errorf("expected output to contain 'Error'; got: %s", out)
	}
}

// TestMain_HeadlessInvalidJSON verifies that a config file with malformed JSON
// exits 1 with a parsing error.
func TestMain_HeadlessInvalidJSON(t *testing.T) {
	cfg := writeTempConfig(t, `{not valid json}`)
	cmd := helperCmd(t, "--headless", "--config="+cfg, "--log-file=/dev/null")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid JSON config, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error") {
		t.Errorf("expected output to contain 'Error'; got: %s", out)
	}
}

// minimalDryRunConfig is a minimal valid headless config with dry_run: true.
// Uses /dev/sda as the target disk — DryRunner never executes disk commands,
// so the disk need not exist in the test environment.
const minimalDryRunConfig = `{
  "channel": "stable",
  "hostname": "testhost",
  "disk": "/dev/sda",
  "network": {"mode": "dhcp"},
  "users": [{"username": "core", "ssh_keys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@knuckle"]}],
  "update_strategy": "reboot",
  "dry_run": true
}`

// writeTempConfig writes content to a temp file and returns the path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "knuckle-test-config-*.json")
	if err != nil {
		t.Fatalf("creating temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	_ = f.Close()
	return f.Name()
}

// helperCmdWithTimeout builds a subprocess with a custom timeout.
func helperCmdWithTimeout(t *testing.T, timeout time.Duration, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), "KNUCKLE_TEST_MAIN=1")
	return cmd
}

// TestMain_HeadlessDryRun verifies a valid dry-run config exits 0 and runs the
// full headless path (config load → validate → install via DryRunner).
// Covers: log setup, cfg load, DryRunner branch, headless.Run success path.
func TestMain_HeadlessDryRun(t *testing.T) {
	cfg := writeTempConfig(t, minimalDryRunConfig)
	cmd := helperCmdWithTimeout(t, 30*time.Second,
		"--headless", "--config="+cfg, "--log-file=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run headless exited non-zero: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "finished successfully") {
		t.Errorf("expected success message in output; got: %s", out)
	}
}

// TestMain_HeadlessDryRunFlag verifies that --dry-run CLI flag forces dry-run
// even when the config does not set dry_run: true.
// Covers the `if dryRun { cfg.DryRun = true }` branch in runHeadless.
func TestMain_HeadlessDryRunFlag(t *testing.T) {
	// Config has dry_run omitted (defaults false) — CLI flag must enable it.
	cfg := writeTempConfig(t, `{
  "channel": "stable",
  "hostname": "testhost",
  "disk": "/dev/sda",
  "network": {"mode": "dhcp"},
  "users": [{"username": "core", "ssh_keys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@knuckle"]}],
  "update_strategy": "reboot"
}`)
	cmd := helperCmdWithTimeout(t, 30*time.Second,
		"--headless", "--config="+cfg, "--dry-run", "--log-file=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--dry-run flag headless exited non-zero: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "finished successfully") {
		t.Errorf("expected success message in output; got: %s", out)
	}
}

// TestMain_HeadlessUnwriteableLogFile verifies that an unwriteable log file
// path exits 1 — covers the log-file open-error branch in runHeadless.
func TestMain_HeadlessUnwriteableLogFile(t *testing.T) {
	cfg := writeTempConfig(t, minimalDryRunConfig)
	badLog := t.TempDir() + "/nonexistent-subdir/knuckle.log"
	cmd := helperCmd(t, "--headless", "--config="+cfg, "--log-file="+badLog)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unwriteable log file, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error") {
		t.Errorf("expected output to contain 'Error'; got: %s", out)
	}
}
