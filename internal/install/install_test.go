package install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/ignition"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestInstallWithGeneratedConfig(t *testing.T) {
	spy := runner.NewSpyRunner()

	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "test-node",
		Disk: model.DiskInfo{
			DevPath: "/dev/sda",
		},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify butane compilation happened in-process (no CLI call)
	for i := range spy.Calls {
		if spy.Calls[i].Name == "butane" {
			t.Error("butane CLI should not be called — using Go library")
		}
	}

	// Verify ignition file was written securely via os.CreateTemp (not runner)
	// The write now happens in Go directly, not via runner shell command
	for i := range spy.Calls {
		if spy.Calls[i].Name == "sh" {
			t.Error("sh command should not be called — secure write uses os.CreateTemp")
		}
	}

	// Verify flatcar-install was called with -i flag pointing to a temp file
	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "flatcar-install" {
			installCall = &spy.Calls[i]
			break
		}
	}
	if installCall == nil {
		t.Fatal("flatcar-install was not called")
	}
	// Check basic structure: -d /dev/sda -C stable -i <some-temp-path>
	if len(installCall.Args) < 6 {
		t.Fatalf("flatcar-install args too short: %v", installCall.Args)
	}
	if installCall.Args[0] != "-d" || installCall.Args[1] != "/dev/sda" {
		t.Errorf("expected -d /dev/sda, got %v", installCall.Args[:2])
	}
	if installCall.Args[2] != "-C" || installCall.Args[3] != "stable" {
		t.Errorf("expected -C stable, got %v", installCall.Args[2:4])
	}
	if installCall.Args[4] != "-i" {
		t.Errorf("expected -i flag, got %q", installCall.Args[4])
	}
	// The path should be a temp file (not predictable /tmp/knuckle-ignition.json)
	if installCall.Args[5] == "/tmp/knuckle-ignition.json" {
		t.Error("ignition path should be randomized, not predictable")
	}
}

func TestInstallWithExternalURL(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:     "beta",
		Hostname:    "ext-node",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/config.ign",
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// butane must NOT be called
	for _, call := range spy.Calls {
		if call.Name == "butane" {
			t.Error("butane should not be called when IgnitionURL is set")
		}
	}

	// flatcar-install must use -I flag
	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "flatcar-install" {
			installCall = &spy.Calls[i]
			break
		}
	}
	if installCall == nil {
		t.Fatal("flatcar-install was not called")
	}
	wantArgs := []string{"-d", "/dev/vda", "-C", "beta", "-I", "https://example.com/config.ign"}
	if len(installCall.Args) != len(wantArgs) {
		t.Fatalf("flatcar-install args = %v, want %v", installCall.Args, wantArgs)
	}
	for i, arg := range wantArgs {
		if installCall.Args[i] != arg {
			t.Errorf("flatcar-install arg[%d] = %q, want %q", i, installCall.Args[i], arg)
		}
	}
}

func TestInstallNilConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())

	err := installer.Install(context.Background(), nil, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if got := err.Error(); got != "install config cannot be nil" {
		t.Errorf("error = %q, want %q", got, "install config cannot be nil")
	}
}

func TestInstallButaneCompilationFailure(t *testing.T) {
	// Test that CompileToIgnition properly rejects invalid Butane YAML.
	// Since the install path now uses the Go library directly, we test
	// the compilation function with intentionally malformed input.
	_, err := ignition.CompileToIgnition("not: valid: butane: {{{")
	if err == nil {
		t.Fatal("expected error for invalid Butane YAML")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestInstallFlatcarInstallFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	// Stub a generic error for any flatcar-install call
	// Since the ignition path is dynamic, we can't pre-stub the exact command.
	// Instead, use the AllError field to make ALL commands fail.
	spy.AllError = fmt.Errorf("command exited with code 1")

	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "fail-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when flatcar-install fails")
	}
}

func TestInstallWipesTargetDiskBeforeFlatcarInstall(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "wipe-node",
		Disk: model.DiskInfo{
			DevPath: "/dev/sda",
			Path:    "/dev/disk/by-id/ata-test-disk",
		},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.Calls) < 3 {
		t.Fatalf("expected wipefs, flatcar-install, and sfdisk calls, got %v", spy.Calls)
	}
	if spy.Calls[0].Name != "wipefs" {
		t.Fatalf("first command = %q, want wipefs", spy.Calls[0].Name)
	}
	if got, want := spy.Calls[0].Args, []string{"--all", "--force", "/dev/disk/by-id/ata-test-disk"}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("wipefs args = %v, want %v", got, want)
	}
	if spy.Calls[1].Name != "flatcar-install" {
		t.Fatalf("second command = %q, want flatcar-install", spy.Calls[1].Name)
	}
	if spy.Calls[2].Name != "sfdisk" {
		t.Fatalf("third command = %q, want sfdisk", spy.Calls[2].Name)
	}
	if got, want := spy.Calls[2].Args, []string{"--relocate", "gpt-bak-std", "/dev/disk/by-id/ata-test-disk"}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("sfdisk args = %v, want %v", got, want)
	}
}

func TestInstallWipeFailureStopsBeforeFlatcarInstall(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubError("wipefs --all --force /dev/sda", fmt.Errorf("permission denied"))
	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "wipe-fail-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when wipefs fails")
	}
	if !strings.Contains(err.Error(), "wiping target disk") {
		t.Fatalf("error = %q, want wipefs context", err)
	}
	for _, call := range spy.Calls {
		if call.Name == "flatcar-install" {
			t.Fatalf("flatcar-install should not run after wipefs failure: %#v", spy.Calls)
		}
	}
}

func TestInstallGPTRepairFailureStopsAfterFlatcarInstall(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubError("sfdisk --relocate gpt-bak-std /dev/sda", fmt.Errorf("permission denied"))
	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "repair-fail-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when GPT repair fails")
	}
	if !strings.Contains(err.Error(), "repairing GPT backup header") {
		t.Fatalf("error = %q, want GPT repair context", err)
	}
	if len(spy.Calls) < 3 || spy.Calls[1].Name != "flatcar-install" || spy.Calls[2].Name != "sfdisk" {
		t.Fatalf("unexpected call order: %#v", spy.Calls)
	}
}

type stderrFailRunner struct{}

func (stderrFailRunner) Run(ctx context.Context, name string, args ...string) (*runner.Result, error) {
	// Only fail flatcar-install with stderr; wipefs and sfdisk succeed so the
	// test exercises the flatcar-install error path specifically.
	if name != "flatcar-install" {
		return &runner.Result{Command: name, Args: args, ExitCode: 0}, nil
	}
	return &runner.Result{
		Command:  name,
		Args:     args,
		Stderr:   "partition table write failed: GPT headers are invalid",
		ExitCode: 1,
	}, fmt.Errorf("command %q exited with code 1", name)
}

func (stderrFailRunner) RunWithInput(ctx context.Context, input string, name string, args ...string) (*runner.Result, error) {
	return nil, fmt.Errorf("unexpected RunWithInput call")
}

func TestInstallFlatcarInstallFailureIncludesStderr(t *testing.T) {
	installer := NewFlatcarInstaller(stderrFailRunner{}, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "fail-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when flatcar-install fails")
	}
	if !strings.Contains(err.Error(), "GPT headers are invalid") {
		t.Fatalf("error = %q, want stderr from flatcar-install", err)
	}
}

func TestBuildInstallArgs(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *model.InstallConfig
		ignitionJSON string
		want         []string
	}{
		{
			name: "basic with ignition file",
			cfg: &model.InstallConfig{
				Channel: "stable",
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
			},
			ignitionJSON: `{"ignition":{}}`,
			want:         []string{"-d", "/dev/sda", "-C", "stable", "-i", "/tmp/test-ign.json"},
		},
		{
			name: "external URL",
			cfg: &model.InstallConfig{
				Channel:     "alpha",
				Disk:        model.DiskInfo{DevPath: "/dev/nvme0n1"},
				IgnitionURL: "https://example.com/ign.json",
			},
			ignitionJSON: "",
			want:         []string{"-d", "/dev/nvme0n1", "-C", "alpha", "-I", "https://example.com/ign.json"},
		},
		{
			name: "no ignition",
			cfg: &model.InstallConfig{
				Channel: "beta",
				Disk:    model.DiskInfo{DevPath: "/dev/vda"},
			},
			ignitionJSON: "",
			want:         []string{"-d", "/dev/vda", "-C", "beta"},
		},
		{
			name: "version pinning",
			cfg: &model.InstallConfig{
				Channel: "stable",
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
				Version: "3510.2.8",
			},
			ignitionJSON: `{"ignition":{}}`,
			want:         []string{"-d", "/dev/sda", "-C", "stable", "-V", "3510.2.8", "-i", "/tmp/test-ign.json"},
		},
		{
			name: "prefers by-id path over devpath",
			cfg: &model.InstallConfig{
				Channel: "stable",
				Disk: model.DiskInfo{
					DevPath: "/dev/sda",
					Path:    "/dev/disk/by-id/ata-Samsung_SSD_870_S5PXNG0R312345",
				},
			},
			ignitionJSON: `{"ignition":{}}`,
			want:         []string{"-d", "/dev/disk/by-id/ata-Samsung_SSD_870_S5PXNG0R312345", "-C", "stable", "-i", "/tmp/test-ign.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ignitionPath is set when ignitionJSON is non-empty
			ignPath := ""
			if tt.ignitionJSON != "" {
				ignPath = "/tmp/test-ign.json"
			}
			got := buildInstallArgs(tt.cfg, ignPath)
			if len(got) != len(tt.want) {
				t.Fatalf("buildInstallArgs() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestProgressCallback(t *testing.T) {
	spy := runner.NewSpyRunner()

	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "progress-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
	}

	var steps []string
	err := installer.Install(context.Background(), cfg, func(step string) {
		steps = append(steps, step)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSteps := []string{
		"Generating Butane config...",
		"Compiling Ignition config...",
		"Writing Ignition config...",
		"Wiping target disk signatures...",
		"Running flatcar-install...",
		"Repairing GPT backup header...",
		"Installation complete!",
	}

	if len(steps) != len(expectedSteps) {
		t.Fatalf("got %d progress steps, want %d\nsteps: %v", len(steps), len(expectedSteps), steps)
	}
	for i, want := range expectedSteps {
		if steps[i] != want {
			t.Errorf("step[%d] = %q, want %q", i, steps[i], want)
		}
	}
}

func TestWriteIgnitionFileSecure(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())

	// Test successful write
	path, err := installer.WriteIgnitionFile(`{"ignition":{"version":"3.3.0"}}`)
	if err != nil {
		t.Fatalf("WriteIgnitionFile: %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	// Verify file exists and has correct content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading ignition file: %v", err)
	}
	if string(content) != `{"ignition":{"version":"3.3.0"}}` {
		t.Errorf("content mismatch: got %q", string(content))
	}

	// Verify permissions are restrictive (0600)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}

	// Verify path is unique (not predictable)
	if path == "/tmp/knuckle-ignition.json" {
		t.Error("path should be randomized, not predictable")
	}
}

func TestInstallWithStaticNetwork(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "static-node",
		Disk:     model.DiskInfo{DevPath: "/dev/vda"},
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "eth0",
			Address:   "10.0.2.15/24",
			Gateway:   "10.0.2.2",
		},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify flatcar-install was called
	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "flatcar-install" {
			installCall = &spy.Calls[i]
			break
		}
	}
	if installCall == nil {
		t.Fatal("flatcar-install was not called")
	}
}

func TestInstallWithSysexts(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "sysext-node",
		Disk:     model.DiskInfo{DevPath: "/dev/vda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			{Name: "docker", Version: "24.0.7", URL: "https://github.com/flatcar/sysext-bakery/releases/download/latest/docker-24.0.7-x86-64.raw", Selected: true},
			{Name: "tailscale", Version: "1.56.1", URL: "https://github.com/flatcar/sysext-bakery/releases/download/latest/tailscale-1.56.1-x86-64.raw", Selected: true},
		},
	}

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
	}
	// The ignition file passed to -i must have been generated (sysext URLs included)
	if len(installCall.Args) < 5 || installCall.Args[4] != "-i" {
		t.Errorf("expected -i flag in flatcar-install args: %v", installCall.Args)
	}
}

func TestWriteIgnitionFileCleanup(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())

	path, err := installer.WriteIgnitionFile(`{"test":"cleanup"}`)
	if err != nil {
		t.Fatalf("WriteIgnitionFile: %v", err)
	}
	installer.ignitionPath = path

	// Cleanup should remove the file
	installer.cleanupIgnitionFile()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed after cleanup, got err=%v", err)
	}
}
