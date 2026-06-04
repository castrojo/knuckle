package install

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestFCOSInstallWithGeneratedConfig(t *testing.T) {
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

	// coreos-installer must be called, not flatcar-install
	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			installCall = &spy.Calls[i]
			break
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	// Verify args: install --stream stable <disk> --ignition-file <path>
	if installCall.Args[0] != "install" {
		t.Errorf("expected first arg 'install', got %q", installCall.Args[0])
	}
	if installCall.Args[1] != "--stream" || installCall.Args[2] != "stable" {
		t.Errorf("expected --stream stable, got %v", installCall.Args[1:3])
	}
	if installCall.Args[3] != "/dev/sda" {
		t.Errorf("expected disk /dev/sda, got %q", installCall.Args[3])
	}
	if installCall.Args[4] != "--ignition-file" {
		t.Errorf("expected --ignition-file, got %q", installCall.Args[4])
	}

	for i := range spy.Calls {
		if spy.Calls[i].Name == "flatcar-install" {
			t.Error("flatcar-install should not be called by FCOSInstaller")
		}
	}
}

func TestFCOSInstall_NoWipefs(t *testing.T) {
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

	for _, call := range spy.Calls {
		if call.Name == "wipefs" {
			t.Error("wipefs should not be called by FCOSInstaller")
		}
	}
}

func TestFCOSInstall_NoSfdisk(t *testing.T) {
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

	for _, call := range spy.Calls {
		if call.Name == "sfdisk" {
			t.Error("sfdisk should not be called by FCOSInstaller")
		}
	}
}

func TestFCOSInstallWithExternalURL(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:          model.OSFCOS,
		Channel:     "testing",
		Hostname:    "fcos-ext",
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
			break
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	wantArgs := []string{"install", "--stream", "testing", "/dev/vda", "--ignition-url", "https://example.com/fcos.ign"}
	if len(installCall.Args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", installCall.Args, wantArgs)
	}
	for i, arg := range wantArgs {
		if installCall.Args[i] != arg {
			t.Errorf("arg[%d] = %q, want %q", i, installCall.Args[i], arg)
		}
	}
}

func TestFCOSInstallNilConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())

	err := installer.Install(context.Background(), nil, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if got := err.Error(); got != "install config cannot be nil" {
		t.Errorf("error = %q, want %q", got, "install config cannot be nil")
	}
}

func TestFCOSInstall_CoreosInstallerFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.AllError = fmt.Errorf("command exited with code 1")

	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fail-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when coreos-installer fails")
	}
	if !strings.Contains(err.Error(), "coreos-installer failed") {
		t.Errorf("error = %q, want coreos-installer context", err.Error())
	}
}

func TestFCOSInstall_ProgressSteps(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
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
		"Running coreos-installer...",
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

func TestFCOSInstall_PrefersByIDPath(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "byid-node",
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

	var installCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			installCall = &spy.Calls[i]
			break
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	if installCall.Args[3] != "/dev/disk/by-id/ata-test-disk" {
		t.Errorf("disk arg = %q, want by-id path", installCall.Args[3])
	}
}

func TestBuildFCOSInstallArgs(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *model.InstallConfig
		ignitionPath string
		want         []string
	}{
		{
			name: "basic with ignition file",
			cfg: &model.InstallConfig{
				Channel: "stable",
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
			},
			ignitionPath: "/tmp/test-ign.json",
			want:         []string{"install", "--stream", "stable", "/dev/sda", "--ignition-file", "/tmp/test-ign.json"},
		},
		{
			name: "external URL",
			cfg: &model.InstallConfig{
				Channel:     "next",
				Disk:        model.DiskInfo{DevPath: "/dev/nvme0n1"},
				IgnitionURL: "https://example.com/ign.json",
			},
			ignitionPath: "",
			want:         []string{"install", "--stream", "next", "/dev/nvme0n1", "--ignition-url", "https://example.com/ign.json"},
		},
		{
			name: "no ignition",
			cfg: &model.InstallConfig{
				Channel: "testing",
				Disk:    model.DiskInfo{DevPath: "/dev/vda"},
			},
			ignitionPath: "",
			want:         []string{"install", "--stream", "testing", "/dev/vda"},
		},
		{
			name: "prefers by-id path",
			cfg: &model.InstallConfig{
				Channel: "stable",
				Disk: model.DiskInfo{
					DevPath: "/dev/sda",
					Path:    "/dev/disk/by-id/ata-Samsung_SSD",
				},
			},
			ignitionPath: "/tmp/test-ign.json",
			want:         []string{"install", "--stream", "stable", "/dev/disk/by-id/ata-Samsung_SSD", "--ignition-file", "/tmp/test-ign.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFCOSInstallArgs(tt.cfg, tt.ignitionPath)
			if len(got) != len(tt.want) {
				t.Fatalf("buildFCOSInstallArgs() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFCOSInstall_GenerateButaneError(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-err-test",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Sysexts: []model.SysextEntry{
			{Name: "bad", URL: "http://insecure.example.com/bad.raw", Selected: true},
		},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from FCOS butane generation, got nil")
	}
	if !strings.Contains(err.Error(), "generating butane config") {
		t.Errorf("error should mention generating butane config, got: %v", err)
	}
}

func TestFCOSInstall_CompileToIgnitionError(t *testing.T) {
	prev := compileToIgnitionFunc
	t.Cleanup(func() { compileToIgnitionFunc = prev })
	compileToIgnitionFunc = func(string) (string, error) {
		return "", fmt.Errorf("injected compilation error")
	}

	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "compile-err",
		Disk:     model.DiskInfo{DevPath: "/dev/vda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from compileToIgnitionFunc, got nil")
	}
	if !strings.Contains(err.Error(), "compiling butane") {
		t.Errorf("error = %q, want 'compiling butane' prefix", err.Error())
	}
}

func TestFCOSInstall_OnlyOneCommand(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "single-cmd",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(spy.Calls) != 1 {
		t.Errorf("expected exactly 1 command call, got %d: %v", len(spy.Calls), spy.Calls)
	}
	if spy.Calls[0].Name != "coreos-installer" {
		t.Errorf("expected coreos-installer, got %q", spy.Calls[0].Name)
	}
}
