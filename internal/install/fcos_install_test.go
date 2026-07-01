package install

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestFCOSInstall_BasicArgs(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "fcos-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var call *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			call = &spy.Calls[i]
			break
		}
	}
	if call == nil {
		t.Fatalf("coreos-installer was not called; got: %v", spy.Calls)
		return
	}

	args := call.Args
	if len(args) < 1 || args[0] != "install" {
		t.Errorf("first arg must be 'install', got %v", args)
	}

	streamIdx := -1
	for i, a := range args {
		if a == "--stream" {
			streamIdx = i
			break
		}
	}
	if streamIdx == -1 {
		t.Fatalf("--stream flag not found in args: %v", args)
	}
	if args[streamIdx+1] != "stable" {
		t.Errorf("--stream value = %q, want 'stable'", args[streamIdx+1])
	}

	// Disk must be the last positional argument.
	if args[len(args)-1] != "/dev/sda" {
		t.Errorf("disk path must be last arg, got %q in %v", args[len(args)-1], args)
	}

	// --ignition-file must appear when no IgnitionURL is set.
	found := false
	for _, a := range args {
		if a == "--ignition-file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("--ignition-file not found in args: %v", args)
	}
}

func TestFCOSInstall_NoWipefs(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	_ = installer.Install(context.Background(), cfg, func(string) {})

	for _, call := range spy.Calls {
		if call.Name == "wipefs" {
			t.Error("wipefs must not be called for FCOS install")
		}
	}
}

func TestFCOSInstall_NoSfdisk(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	_ = installer.Install(context.Background(), cfg, func(string) {})

	for _, call := range spy.Calls {
		if call.Name == "sfdisk" {
			t.Error("sfdisk must not be called for FCOS install")
		}
	}
}

func TestFCOSInstall_ExternalURL(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:     "beta",
		Hostname:    "fcos-url-node",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/fcos.ign",
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var call *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			call = &spy.Calls[i]
			break
		}
	}
	if call == nil {
		t.Fatalf("coreos-installer was not called; got: %v", spy.Calls)
		return
	}

	urlIdx := -1
	for i, a := range call.Args {
		if a == "--ignition-url" {
			urlIdx = i
			break
		}
	}
	if urlIdx == -1 {
		t.Fatalf("--ignition-url not found in args: %v", call.Args)
	}
	if call.Args[urlIdx+1] != "https://example.com/fcos.ign" {
		t.Errorf("--ignition-url value = %q, want URL", call.Args[urlIdx+1])
	}

	for _, a := range call.Args {
		if a == "--ignition-file" {
			t.Error("--ignition-file must not appear when IgnitionURL is set")
		}
	}
}

func TestFCOSInstall_VersionIgnored(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Version:  "40.20240101.3.0",
		Hostname: "fcos-ver-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No --version flag should appear in coreos-installer args.
	for _, call := range spy.Calls {
		if call.Name == "coreos-installer" {
			for _, a := range call.Args {
				if strings.HasPrefix(a, "--version") {
					t.Errorf("--version flag must not be passed to coreos-installer, got %v", call.Args)
				}
			}
		}
	}
}

func TestFCOSInstall_ByIDPath(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk: model.DiskInfo{
			Path:    "/dev/disk/by-id/ata-fcos-test",
			DevPath: "/dev/sda",
		},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	_ = installer.Install(context.Background(), cfg, func(string) {})

	for _, call := range spy.Calls {
		if call.Name == "coreos-installer" {
			last := call.Args[len(call.Args)-1]
			if last != "/dev/disk/by-id/ata-fcos-test" {
				t.Errorf("disk arg = %q, want by-id path", last)
			}
			return
		}
	}
	t.Fatal("coreos-installer not called")
}

func TestFCOSInstall_NilConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())

	err := installer.Install(context.Background(), nil, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
}

func TestBuildFCOSInstallArgs_Generated(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
	}
	args := buildFCOSInstallArgs(cfg, "/tmp/ignition-abc.json")

	want := []string{"install", "--stream", "stable", "--ignition-file", "/tmp/ignition-abc.json", "/dev/sda"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestBuildFCOSInstallArgs_ExternalURL(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel:     "beta",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/fcos.ign",
	}
	args := buildFCOSInstallArgs(cfg, "")

	want := []string{"install", "--stream", "beta", "--ignition-url", "https://example.com/fcos.ign", "/dev/vda"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestFCOSInstall_ButaneError(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
	}
	origCompile := compileToIgnitionFunc
	t.Cleanup(func() { compileToIgnitionFunc = origCompile })
	compileToIgnitionFunc = func(_ string) (string, error) {
		return "", fmt.Errorf("compile error")
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from butane/compile, got nil")
	}
}

func TestFCOSInstall_TempFileError(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	origTempFile := newIgnitionTempFile
	t.Cleanup(func() { newIgnitionTempFile = origTempFile })
	newIgnitionTempFile = func() (ignitionTempFile, error) {
		return nil, fmt.Errorf("disk full")
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from temp file creation, got nil")
	}
	if !strings.Contains(err.Error(), "writing ignition file") {
		t.Errorf("error = %q, want to contain 'writing ignition file'", err.Error())
	}
}

func TestFCOSInstall_RunnerError(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.AllError = fmt.Errorf("coreos-installer not found")
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:     "stable",
		Disk:        model.DiskInfo{DevPath: "/dev/sda"},
		IgnitionURL: "https://example.com/fcos.ign",
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from runner, got nil")
	}
}

type fcosFailRunner struct{}

func (fcosFailRunner) Run(_ context.Context, name string, args ...string) (*runner.Result, error) {
	return &runner.Result{Command: name, Args: args, ExitCode: 1, Stderr: "installation failed"}, nil
}
func (fcosFailRunner) RunWithInput(_ context.Context, _ string, name string, args ...string) (*runner.Result, error) {
	return nil, fmt.Errorf("unexpected RunWithInput")
}

func TestFCOSInstall_RunnerNonZeroExit(t *testing.T) {
	installer := NewFCOSInstaller(fcosFailRunner{}, testLogger())
	cfg := &model.InstallConfig{
		Channel:     "stable",
		Disk:        model.DiskInfo{DevPath: "/dev/sda"},
		IgnitionURL: "https://example.com/fcos.ign",
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from non-zero exit, got nil")
	}
}

func TestFCOSCleanup_EmptyPath(t *testing.T) {
	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	installer.ignitionPath = ""
	installer.cleanupFCOSIgnitionFile() // should not panic
}

func TestFCOSCleanup_RemoveError(t *testing.T) {
	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	installer.ignitionPath = "/nonexistent/path/abc.json"

	origRemove := removeIgnitionFile
	t.Cleanup(func() { removeIgnitionFile = origRemove })
	removeIgnitionFile = func(_ string) error {
		return fmt.Errorf("permission denied")
	}

	installer.cleanupFCOSIgnitionFile() // should log warning, not panic
	if installer.ignitionPath != "" {
		t.Error("ignitionPath should be cleared even on remove error")
	}
}

func TestFCOSInstall_GenerateButaneError(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	// A sysext with a non-HTTPS URL triggers GenerateFCOSButane to return an error.
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
		Sysexts: []model.SysextEntry{{Name: "bad", URL: "http://example.com/bad.raw", Selected: true}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected butane error for non-HTTPS sysext URL, got nil")
	}
	if !strings.Contains(err.Error(), "generating fcos butane config") {
		t.Errorf("error = %q, want 'generating fcos butane config'", err.Error())
	}
}

type writeErrFile struct{ name string }

func (w *writeErrFile) Name() string                      { return w.name }
func (w *writeErrFile) WriteString(_ string) (int, error) { return 0, fmt.Errorf("write error") }
func (w *writeErrFile) Close() error                      { return nil }

func TestFCOSInstall_WriteIgnitionWriteError(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	origTempFile := newIgnitionTempFile
	t.Cleanup(func() { newIgnitionTempFile = origTempFile })
	newIgnitionTempFile = func() (ignitionTempFile, error) {
		return &writeErrFile{name: "/tmp/knuckle-test-write-err.json"}, nil
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from write failure, got nil")
	}
	if !strings.Contains(err.Error(), "writing ignition file") {
		t.Errorf("error = %q, want 'writing ignition file'", err.Error())
	}
}

type closeErrFile struct {
	name    string
	content strings.Builder
}

func (c *closeErrFile) Name() string                      { return c.name }
func (c *closeErrFile) WriteString(s string) (int, error) { return c.content.WriteString(s) }
func (c *closeErrFile) Close() error                      { return fmt.Errorf("close error") }

func TestFCOSInstall_WriteIgnitionCloseError(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	origTempFile := newIgnitionTempFile
	t.Cleanup(func() { newIgnitionTempFile = origTempFile })
	newIgnitionTempFile = func() (ignitionTempFile, error) {
		return &closeErrFile{name: "/tmp/knuckle-test-close-err.json"}, nil
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from close failure, got nil")
	}
	if !strings.Contains(err.Error(), "writing ignition file") {
		t.Errorf("error = %q, want 'writing ignition file'", err.Error())
	}
}
