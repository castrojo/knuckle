package install

import (
	"context"
	"fmt"
	"os"
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

	if installCall.Args[0] != "install" {
		t.Errorf("first arg = %q, want \"install\"", installCall.Args[0])
	}
	if installCall.Args[1] != "--stream" || installCall.Args[2] != "stable" {
		t.Errorf("expected --stream stable, got %v", installCall.Args[1:3])
	}
	if installCall.Args[3] != "/dev/sda" {
		t.Errorf("disk arg = %q, want /dev/sda", installCall.Args[3])
	}

	hasIgnitionFile := false
	for _, arg := range installCall.Args {
		if arg == "--ignition-file" {
			hasIgnitionFile = true
			break
		}
	}
	if !hasIgnitionFile {
		t.Error("expected --ignition-file flag in coreos-installer args")
	}
}

func TestFCOSInstallWithExternalURL(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:          model.OSFCOS,
		Channel:     "testing",
		Hostname:    "ext-fcos",
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

func TestFCOSInstallNoWipefs(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "no-wipe-node",
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
			t.Error("wipefs must NOT be called by FCOSInstaller")
		}
		if call.Name == "sfdisk" {
			t.Error("sfdisk must NOT be called by FCOSInstaller")
		}
		if call.Name == "flatcar-install" {
			t.Error("flatcar-install must NOT be called by FCOSInstaller")
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

func TestFCOSInstallFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.AllError = fmt.Errorf("command exited with code 1")

	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fail-fcos",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when coreos-installer fails")
	}
	if !strings.Contains(err.Error(), "coreos-installer failed") {
		t.Errorf("error = %q, want coreos-installer context", err.Error())
	}
}

func TestFCOSInstallProgressSteps(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "progress-fcos",
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

func TestFCOSInstallPrefersByIDPath(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "byid-fcos",
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
		t.Errorf("disk path = %q, want by-id path", installCall.Args[3])
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
			want: []string{"install", "--stream", "next", "/dev/nvme0n1", "--ignition-url", "https://example.com/ign.json"},
		},
		{
			name: "no ignition",
			cfg: &model.InstallConfig{
				Channel: "testing",
				Disk:    model.DiskInfo{DevPath: "/dev/vda"},
			},
			want: []string{"install", "--stream", "testing", "/dev/vda"},
		},
		{
			name: "prefers by-id path",
			cfg: &model.InstallConfig{
				Channel: "stable",
				Disk: model.DiskInfo{
					DevPath: "/dev/sda",
					Path:    "/dev/disk/by-id/ata-Samsung",
				},
			},
			ignitionPath: "/tmp/test-ign.json",
			want:         []string{"install", "--stream", "stable", "/dev/disk/by-id/ata-Samsung", "--ignition-file", "/tmp/test-ign.json"},
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

func TestFCOSInstallVersionWarning(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Version:  "38.20230514.3.0",
		Hostname: "version-fcos",
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
			break
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	for _, arg := range installCall.Args {
		if arg == "38.20230514.3.0" || arg == "-V" || arg == "--image-url" {
			t.Errorf("version pinning args should not be passed to coreos-installer, found %q", arg)
		}
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

func TestFCOSInstall_CompileError(t *testing.T) {
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
		Hostname: "fcos-compile-err",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
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

func TestFCOSInstall_WriteIgnitionError(t *testing.T) {
	prev := newIgnitionTempFile
	t.Cleanup(func() { newIgnitionTempFile = prev })
	newIgnitionTempFile = func() (ignitionTempFile, error) {
		return nil, fmt.Errorf("injected temp file error")
	}

	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-write-err",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from writeIgnitionFile, got nil")
	}
	if !strings.Contains(err.Error(), "writing ignition file") {
		t.Errorf("error = %q, want 'writing ignition file' prefix", err.Error())
	}
}

func TestRemoveIgnitionTempFile_EmptyPath(t *testing.T) {
	removeIgnitionTempFile(testLogger(), "")
}

func TestRemoveIgnitionTempFile_NonExistentPath(t *testing.T) {
	removeIgnitionTempFile(testLogger(), "/nonexistent-path-xyz")
}

func TestRemoveIgnitionTempFile_RemoveFailsNonNotExist(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — permission checks don't apply")
	}
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "ignition-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0755) }()

	removeIgnitionTempFile(testLogger(), path)
}

// Compile-time check that FCOSInstaller implements Installer.
var _ Installer = (*FCOSInstaller)(nil)
