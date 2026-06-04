package install

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// --- buildFCOSInstallArgs unit tests ---

func TestBuildFCOSInstallArgs_Basic(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
	}
	args := buildFCOSInstallArgs(cfg, "/tmp/ign.json")
	want := []string{"install", "--stream", "stable", "--ignition-file", "/tmp/ign.json", "/dev/sda"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestBuildFCOSInstallArgs_ByIdDisk(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel: "testing",
		Disk: model.DiskInfo{
			DevPath: "/dev/sda",
			Path:    "/dev/disk/by-id/ata-some-disk",
		},
	}
	args := buildFCOSInstallArgs(cfg, "/tmp/ign.json")
	// Path (by-id) should be preferred over DevPath.
	last := args[len(args)-1]
	if last != "/dev/disk/by-id/ata-some-disk" {
		t.Errorf("disk arg = %q, want %q", last, "/dev/disk/by-id/ata-some-disk")
	}
}

func TestBuildFCOSInstallArgs_ExternalURL(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel:     "next",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/config.ign",
	}
	args := buildFCOSInstallArgs(cfg, "")
	want := []string{"install", "--stream", "next", "--ignition-url", "https://example.com/config.ign", "/dev/vda"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", args, want)
	}
	// Must not contain --ignition-file when URL is set.
	for _, a := range args {
		if a == "--ignition-file" {
			t.Error("--ignition-file should not appear when IgnitionURL is set")
		}
	}
}

func TestBuildFCOSInstallArgs_NoIgnition(t *testing.T) {
	cfg := &model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
	}
	args := buildFCOSInstallArgs(cfg, "")
	// Neither ignition flag should appear.
	for _, a := range args {
		if a == "--ignition-file" || a == "--ignition-url" {
			t.Errorf("unexpected ignition arg %q when no ignition configured", a)
		}
	}
	// Disk is still the final positional arg.
	if args[len(args)-1] != "/dev/sda" {
		t.Errorf("disk arg = %q, want /dev/sda", args[len(args)-1])
	}
}

// --- FCOSInstaller.Install integration tests (using SpyRunner) ---

func TestFCOSInstall_Basic(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
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

	// Must call coreos-installer (not flatcar-install).
	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			installCall = &spy.Calls[i]
		}
		if spy.Calls[i].Name == "flatcar-install" {
			t.Error("flatcar-install must not be called by FCOSInstaller")
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}
}

func TestFCOSInstall_CorrectArgs(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
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

	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			installCall = &spy.Calls[i]
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	args := installCall.Args
	// Must start with "install".
	if len(args) < 1 || args[0] != "install" {
		t.Fatalf("first arg = %q, want \"install\"", args[0])
	}
	// Must contain --stream stable.
	assertArgsContain(t, args, "--stream", "stable")
	// Must contain --ignition-file pointing to a temp path.
	assertArgPresent(t, args, "--ignition-file")
	// Disk must be the last positional arg.
	if args[len(args)-1] != "/dev/sda" {
		t.Errorf("last arg = %q, want /dev/sda", args[len(args)-1])
	}
}

func TestFCOSInstall_NoWipefs(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-node",
		Disk: model.DiskInfo{
			DevPath: "/dev/sda",
			Path:    "/dev/disk/by-id/ata-test-disk",
		},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, call := range spy.Calls {
		if call.Name == "wipefs" {
			t.Error("wipefs must not be called by FCOSInstaller — coreos-installer handles disk prep")
		}
		if call.Name == "sfdisk" {
			t.Error("sfdisk must not be called by FCOSInstaller — coreos-installer handles GPT")
		}
	}
}

func TestFCOSInstall_ExternalURL(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:          model.OSFCOS,
		Channel:     "testing",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/fcos.ign",
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			installCall = &spy.Calls[i]
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}
	assertArgsContain(t, installCall.Args, "--ignition-url", "https://example.com/fcos.ign")
	for _, a := range installCall.Args {
		if a == "--ignition-file" {
			t.Error("--ignition-file must not appear when IgnitionURL is set")
		}
	}
}

func TestFCOSInstall_NilConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	err := installer.Install(context.Background(), nil, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if err.Error() != "install config cannot be nil" {
		t.Errorf("error = %q, want 'install config cannot be nil'", err.Error())
	}
}

func TestFCOSInstall_VersionWarningIgnored(t *testing.T) {
	// coreos-installer has no -V equivalent; Version is logged as warning and ignored.
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Version:  "44.20260510.3.1",
		Hostname: "fcos-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Version must not appear in coreos-installer args.
	for _, call := range spy.Calls {
		if call.Name == "coreos-installer" {
			for _, arg := range call.Args {
				if arg == cfg.Version {
					t.Errorf("FCOS version %q must not be passed to coreos-installer", cfg.Version)
				}
			}
		}
	}
}

func TestFCOSInstall_RunnerError(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.AllError = fmt.Errorf("disk not found")
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
}

func TestFCOSInstall_OnlyOneCoreosInstallerCall(t *testing.T) {
	// Ensure FCOSInstaller makes exactly one runner call: coreos-installer install.
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
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

	if len(spy.Calls) != 1 {
		t.Errorf("expected exactly 1 runner call, got %d: %v", len(spy.Calls), spy.Calls)
	}
	if spy.Calls[0].Name != "coreos-installer" {
		t.Errorf("call[0] = %q, want coreos-installer", spy.Calls[0].Name)
	}
}

// --- helpers ---

func assertArgsContain(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("args %v do not contain %s %s", args, flag, value)
}

func assertArgPresent(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			return
		}
	}
	t.Errorf("args %v do not contain %s", args, flag)
}
