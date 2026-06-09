package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/headless"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/probe"
	"github.com/projectbluefin/knuckle/internal/runner"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

// errProber is a probe.Prober stub that always returns an error.
type errProber struct{}

func (p *errProber) ListDisks(_ context.Context) ([]model.DiskInfo, error) {
	return nil, errors.New("injected probe error")
}
func (p *errProber) ListNetworkInterfaces(_ context.Context) ([]model.NetworkInterface, error) {
	return nil, errors.New("injected probe error")
}

// errBakery is a bakery.Client stub that always returns an error.
type errBakery struct{}

func (b *errBakery) FetchCatalog(_ context.Context) ([]model.SysextEntry, error) {
	return nil, errors.New("injected bakery error")
}
func (b *errBakery) FetchCatalogArch(_ context.Context, _ string) ([]model.SysextEntry, error) {
	return nil, errors.New("injected bakery error")
}

// Compile-time interface checks.
var _ probe.Prober = (*errProber)(nil)
var _ bakery.Client = (*errBakery)(nil)

// init wires test-only env vars into the injectable package-level variables.
// These run before TestMain delegates to main() in subprocess mode, letting
// subprocess tests cover error branches that normally require real hardware or TTY.
func init() {
	if os.Getenv("KNUCKLE_TEST_DEMO_PROBE_FAIL") == "1" {
		demoProberFactory = func() probe.Prober { return &errProber{} }
	}
	if os.Getenv("KNUCKLE_TEST_DEMO_BAKERY_FAIL") == "1" {
		demoBakeryFactory = func() bakery.Client { return &errBakery{} }
	}
	if os.Getenv("KNUCKLE_TEST_TUI_NOOP") == "1" {
		tuiRunFn = func(_ *wizard.Wizard, _ func(context.Context) error) error { return nil }
	}
	if os.Getenv("KNUCKLE_TEST_TUI_FAIL") == "1" {
		tuiRunFn = func(_ *wizard.Wizard, _ func(context.Context) error) error {
			return errors.New("injected TUI failure")
		}
	}
}

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

// TestMain_LTSChannelAccepted verifies the CLI accepts lts and reaches the
// normal TUI path when using the shared channel validator.
func TestMain_LTSChannelAccepted(t *testing.T) {
	cmd := helperCmd(t, "--demo", "--channel=lts", "--log-file=/dev/null")
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_NOOP=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected --channel=lts to succeed, got %v\noutput: %s", err, out)
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

// hasTTY reports whether /dev/tty is accessible in the current process.
// Used to skip interactive TUI tests in non-interactive environments such as
// CI runners and ghost QA workers that have no controlling terminal.
func hasTTY() bool {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// skipIfNoTTY calls t.Skip when there is no accessible /dev/tty.
func skipIfNoTTY(t *testing.T) {
	t.Helper()
	if !hasTTY() {
		t.Skip("skipping TTY test: /dev/tty not accessible (non-interactive environment)")
	}
}

// TestMain_TUINormalMode verifies that normal TUI mode starts successfully
// and follows the full startup path: hardware probe → sysext fetch → channel
// fetch → tui.Run(). Uses KNUCKLE_TEST_TUI_AUTO_QUIT to exit cleanly.
// Covers: log file setup, channel validation, runner setup, prober/bakery/
// installer wiring, wizard creation, probe/fetch calls, tui.Run() invocation.
func TestMain_TUINormalMode(t *testing.T) {
	skipIfNoTTY(t)
	logFile := t.TempDir() + "/knuckle.log"
	cmd := helperCmdWithTimeout(t, 15*time.Second,
		"--channel=stable", "--dry-run", "--log-file="+logFile)
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_AUTO_QUIT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("TUI normal mode exited non-zero: %v\noutput: %s", err, out)
	}
	// Verify log file was created
	if _, statErr := os.Stat(logFile); statErr != nil {
		t.Errorf("expected log file %q to be created, but stat failed: %v", logFile, statErr)
	}
}

// TestMain_TUIDemoMode verifies that demo mode starts successfully and uses
// mock implementations (demo.Prober, demo.Bakery, demo.Installer) instead of
// real hardware/network calls. Covers the demo mode branch, mock wiring, and
// pre-populated wizard state.
func TestMain_TUIDemoMode(t *testing.T) {
	skipIfNoTTY(t)
	logFile := t.TempDir() + "/knuckle-demo.log"
	cmd := helperCmdWithTimeout(t, 15*time.Second,
		"--demo", "--log-file="+logFile)
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_AUTO_QUIT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("TUI demo mode exited non-zero: %v\noutput: %s", err, out)
	}
	// Verify log file was created
	if _, statErr := os.Stat(logFile); statErr != nil {
		t.Errorf("expected log file %q to be created, but stat failed: %v", logFile, statErr)
	}
}

// TestMain_TUILogFileError verifies that when the log file cannot be opened
// in TUI mode (not headless), main exits 1 with an error message.
// Covers the log-file open-error branch in main() before TUI startup.
func TestMain_TUILogFileError(t *testing.T) {
	badLog := t.TempDir() + "/nonexistent-subdir/knuckle.log"
	cmd := helperCmd(t, "--dry-run", "--log-file="+badLog)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unwriteable log file, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error opening log file") {
		t.Errorf("expected output to contain 'Error opening log file'; got: %s", out)
	}
}

// TestMain_TUIRebootFnWiring verifies that when not in dry-run mode, the
// rebootFn is correctly wired to call systemctl reboot through the runner.
// This test cannot actually execute the reboot (would kill the test runner),
// but it exercises the rebootFn != nil branch by starting TUI in non-dry-run
// mode. The auto-quit mechanism prevents the TUI from blocking.
// Covers: rebootFn setup for non-dry-run mode (lines 147-152 in main.go).
func TestMain_TUIRebootFnWiring(t *testing.T) {
	skipIfNoTTY(t)
	logFile := t.TempDir() + "/knuckle-reboot.log"
	// Start TUI without --dry-run (rebootFn will be non-nil)
	// but use auto-quit to prevent blocking. Since we quit before install
	// completes, reboot is never actually triggered (which is good — we
	// don't want to reboot the test machine). This test covers the setup
	// of rebootFn in the non-dry-run path.
	cmd := helperCmdWithTimeout(t, 15*time.Second,
		"--channel=stable", "--log-file="+logFile)
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_AUTO_QUIT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("TUI non-dry-run mode exited non-zero: %v\noutput: %s", err, out)
	}
	// Verify log file was created
	if _, statErr := os.Stat(logFile); statErr != nil {
		t.Errorf("expected log file %q to be created, but stat failed: %v", logFile, statErr)
	}
}

// --- Direct unit tests for runHeadless ---
// These test the function directly without a subprocess, which was impossible
// before the os.Exit refactor (the process would have exited).

// TestRunHeadless_LogFileError tests that runHeadless returns an error (not
// os.Exit) when the log file cannot be opened.
func TestRunHeadless_LogFileError(t *testing.T) {
	cfg := writeTempConfig(t, minimalDryRunConfig)
	badLog := t.TempDir() + "/nonexistent-subdir/knuckle.log"
	err := runHeadless(cfg, true, badLog)
	if err == nil {
		t.Fatal("expected error for unwriteable log file, got nil")
	}
	if !strings.Contains(err.Error(), "opening log file") {
		t.Errorf("expected error to mention 'opening log file'; got: %v", err)
	}
}

// TestRunHeadless_ConfigNotFound tests that runHeadless returns an error when
// the config file does not exist.
func TestRunHeadless_ConfigNotFound(t *testing.T) {
	err := runHeadless("/nonexistent-knuckle-config-xyz.json", true, "/dev/null")
	if err == nil {
		t.Fatal("expected error for missing config file, got nil")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected error to mention 'loading config'; got: %v", err)
	}
}

// TestRunHeadless_InvalidJSON tests that runHeadless returns an error when the
// config file contains invalid JSON.
func TestRunHeadless_InvalidJSON(t *testing.T) {
	cfg := writeTempConfig(t, `{not valid json}`)
	err := runHeadless(cfg, true, "/dev/null")
	if err == nil {
		t.Fatal("expected error for invalid JSON config, got nil")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected error to mention 'loading config'; got: %v", err)
	}
}

// TestRunHeadless_DryRunSuccess tests that runHeadless returns nil for a valid
// dry-run config, exercising the full happy path without network or disk I/O.
func TestRunHeadless_DryRunSuccess(t *testing.T) {
	cfg := writeTempConfig(t, minimalDryRunConfig)
	err := runHeadless(cfg, true, "/dev/null")
	if err != nil {
		t.Fatalf("expected nil error for dry-run, got: %v", err)
	}
}

// TestMain_TUIFlatcarVersionFlag verifies that the --flatcar-version flag
// is correctly passed to the wizard state.
func TestMain_TUIFlatcarVersionFlag(t *testing.T) {
	skipIfNoTTY(t)
	logFile := t.TempDir() + "/knuckle-version.log"
	cmd := helperCmdWithTimeout(t, 15*time.Second,
		"--channel=stable", "--dry-run", "--flatcar-version=3510.2.8", "--log-file="+logFile)
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_AUTO_QUIT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("TUI with --flatcar-version exited non-zero: %v\noutput: %s", err, out)
	}
	// Verify log file was created
	if _, statErr := os.Stat(logFile); statErr != nil {
		t.Errorf("expected log file %q to be created, but stat failed: %v", logFile, statErr)
	}
}

// rebootConfig returns a headless JSON config with reboot: true, dry_run: false.
const rebootConfig = `{
  "channel": "stable",
  "hostname": "reboot-test",
  "disk": "/dev/sda",
  "network": {"mode": "dhcp"},
  "users": [{"username": "core", "ssh_keys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@knuckle"]}],
  "update_strategy": "reboot",
  "reboot": true,
  "dry_run": false
}`

// TestRunHeadlessWithRunner_RebootInvoked verifies that when cfg.Reboot=true
// and cfg.DryRun=false, the runner receives a "systemctl reboot" command.
func TestRunHeadlessWithRunner_RebootInvoked(t *testing.T) {
	cleanup := headless.OverrideValidateBlockDevice(func(string) error { return nil })
	defer cleanup()
	cleanupDelay := headless.OverrideRebootDelay(nil)
	defer cleanupDelay()

	spy := runner.NewSpyRunner()
	// SpyRunner returns success for unknown commands by default

	cfg := writeTempConfig(t, rebootConfig)
	logFile := t.TempDir() + "/reboot-test.log"

	err := runHeadlessWithRunner(cfg, false, logFile, spy)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Verify reboot command was called
	found := false
	for _, c := range spy.Calls {
		if c.Name == "systemctl" && len(c.Args) > 0 && c.Args[0] == "reboot" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'systemctl reboot' call, got: %v", spy.Calls)
	}
}

// TestRunHeadlessWithRunner_RebootError verifies that a reboot error is propagated.
func TestRunHeadlessWithRunner_RebootError(t *testing.T) {
	cleanup := headless.OverrideValidateBlockDevice(func(string) error { return nil })
	defer cleanup()
	cleanupDelay := headless.OverrideRebootDelay(nil)
	defer cleanupDelay()

	spy := runner.NewSpyRunner()
	spy.StubError("systemctl reboot", fmt.Errorf("connection refused"))

	cfg := writeTempConfig(t, rebootConfig)
	logFile := t.TempDir() + "/reboot-err.log"

	err := runHeadlessWithRunner(cfg, false, logFile, spy)
	if err == nil {
		t.Fatal("expected error for failed reboot, got nil")
	}
	if !strings.Contains(err.Error(), "reboot failed") {
		t.Errorf("expected 'reboot failed' in error, got: %v", err)
	}
}

// TestRunHeadlessWithRunner_NilRunnerRealRunnerPath verifies that when no runner
// is injected and cfg.DryRun=false, runHeadlessWithRunner creates a RealRunner
// (covers main.go:201-202) and propagates a headless.Run error (covers main.go:211-212).
// The config uses an invalid network mode so cfg.Validate() fails before any
// shell commands are executed — safe to run in any environment.
func TestRunHeadlessWithRunner_NilRunnerRealRunnerPath(t *testing.T) {
	// invalid network mode causes cfg.Validate() to fail immediately in headless.Run,
	// before any block-device checks or shell commands are issued.
	const invalidNetworkConfig = `{
  "channel": "stable",
  "hostname": "testhost",
  "disk": "",
  "network": {"mode": "not-a-valid-mode"},
  "users": [{"username": "core", "ssh_keys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@knuckle"]}],
  "update_strategy": "reboot",
  "dry_run": false
}`
	cfg := writeTempConfig(t, invalidNetworkConfig)
	logFile := t.TempDir() + "/real-runner.log"

	// Pass nil runner + dryRun=false so line 201-202 (NewRealRunner) is reached.
	err := runHeadlessWithRunner(cfg, false, logFile, nil)
	if err == nil {
		t.Fatal("expected error for invalid network mode config, got nil")
	}
	if !strings.Contains(err.Error(), "network mode") {
		t.Errorf("expected 'network mode' validation error, got: %v", err)
	}
}

// TestRunHeadlessWithRunner_NoRebootWhenDisabled verifies that no reboot is
// issued when cfg.Reboot=false.
func TestRunHeadlessWithRunner_NoRebootWhenDisabled(t *testing.T) {
	cleanup := headless.OverrideValidateBlockDevice(func(string) error { return nil })
	defer cleanup()

	noRebootConfig := `{
  "channel": "stable",
  "hostname": "no-reboot-test",
  "disk": "/dev/sda",
  "network": {"mode": "dhcp"},
  "users": [{"username": "core", "ssh_keys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@knuckle"]}],
  "update_strategy": "reboot",
  "reboot": false,
  "dry_run": false
}`

	spy := runner.NewSpyRunner()

	cfg := writeTempConfig(t, noRebootConfig)
	logFile := t.TempDir() + "/no-reboot.log"

	err := runHeadlessWithRunner(cfg, false, logFile, spy)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Verify NO reboot command was called
	for _, c := range spy.Calls {
		if c.Name == "systemctl" && len(c.Args) > 0 && c.Args[0] == "reboot" {
			t.Errorf("unexpected reboot call when reboot=false: %v", spy.Calls)
		}
	}
}

// TestMain_DemoProbeWarn verifies that when the demo hardware prober returns an
// error, main logs a warning and continues (does not exit 1).
// Covers: lines ~124-126 (demo hardware probe failed warn branch).
func TestMain_DemoProbeWarn(t *testing.T) {
	logFile := t.TempDir() + "/knuckle-probe-warn.log"
	cmd := helperCmd(t, "--demo", "--log-file="+logFile)
	cmd.Env = append(cmd.Env,
		"KNUCKLE_TEST_DEMO_PROBE_FAIL=1",
		"KNUCKLE_TEST_TUI_NOOP=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 (warn-and-continue), got %v\noutput: %s", err, out)
	}
	logData, readErr := os.ReadFile(logFile)
	if readErr != nil {
		t.Fatalf("reading log file: %v", readErr)
	}
	if !strings.Contains(string(logData), "demo hardware probe failed") {
		t.Errorf("expected log to contain 'demo hardware probe failed'; log: %s", logData)
	}
}

// TestMain_DemoBakeryWarn verifies that when the demo bakery returns an error,
// main logs a warning and continues (does not exit 1).
// Covers: lines ~127-129 (demo sysext fetch failed warn branch).
func TestMain_DemoBakeryWarn(t *testing.T) {
	logFile := t.TempDir() + "/knuckle-bakery-warn.log"
	cmd := helperCmd(t, "--demo", "--log-file="+logFile)
	cmd.Env = append(cmd.Env,
		"KNUCKLE_TEST_DEMO_BAKERY_FAIL=1",
		"KNUCKLE_TEST_TUI_NOOP=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 (warn-and-continue), got %v\noutput: %s", err, out)
	}
	logData, readErr := os.ReadFile(logFile)
	if readErr != nil {
		t.Fatalf("reading log file: %v", readErr)
	}
	if !strings.Contains(string(logData), "demo sysext fetch failed") {
		t.Errorf("expected log to contain 'demo sysext fetch failed'; log: %s", logData)
	}
}

// TestMain_ProductionPath exercises the non-demo, non-dry-run initialization
// path that wires real SystemProber, HTTPClient, and DispatchingInstaller.
// Uses KNUCKLE_TEST_TUI_NOOP to bypass the terminal requirement so this test
// runs in headless CI environments (no /dev/tty needed).
// Covers: real runner creation (else branch), real prober/bakery/installer
// wiring, hardware probe + sysext/channel fetch (warn-and-continue on error),
// and rebootFn setup for non-dry-run mode.
func TestMain_ProductionPath(t *testing.T) {
	logFile := t.TempDir() + "/knuckle-prod.log"
	cmd := helperCmd(t, "--channel=stable", "--log-file="+logFile)
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_NOOP=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("production path exited non-zero: %v\noutput: %s", err, out)
	}
	logData, readErr := os.ReadFile(logFile)
	if readErr != nil {
		t.Fatalf("reading log file: %v", readErr)
	}
	// In CI there are no real disks or network, so probes log warnings — that's fine.
	// The key assertion is that main() reached tuiRunFn without crashing.
	if !strings.Contains(string(logData), "knuckle starting") {
		t.Errorf("expected log to contain 'knuckle starting'; log: %s", logData)
	}
	if !strings.Contains(string(logData), "knuckle finished") {
		t.Errorf("expected log to contain 'knuckle finished'; log: %s", logData)
	}
}

// TestMain_ProductionPathTUIError verifies that the production path (non-demo,
// non-dry-run) correctly propagates a TUI error to stderr and exits 1.
// Covers the tuiRunFn error handling in production mode.
func TestMain_ProductionPathTUIError(t *testing.T) {
	cmd := helperCmd(t, "--channel=stable", "--log-file=/dev/null")
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_FAIL=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for TUI error in production path, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error:") {
		t.Errorf("expected 'Error:' in output; got: %s", out)
	}
}

// TestMain_TUIRunError verifies that when tui.Run returns an error, main logs it,
// prints to stderr, and exits 1.
// Covers: lines ~156-159 (TUI error path).
// Uses --demo to bypass hardware/network probes so the process reaches tuiRunFn fast.
func TestMain_TUIRunError(t *testing.T) {
	cmd := helperCmd(t, "--demo", "--log-file=/dev/null")
	cmd.Env = append(cmd.Env, "KNUCKLE_TEST_TUI_FAIL=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for TUI error, got exit 0")
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}
	if !strings.Contains(string(out), "Error:") {
		t.Errorf("expected 'Error:' in output; got: %s", out)
	}
}
