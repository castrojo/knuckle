package install

import (
	"context"
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
