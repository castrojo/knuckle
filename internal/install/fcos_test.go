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

func TestFCOSInstall_BasicDHCP(t *testing.T) {
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
		if spy.Calls[i].Name == "flatcar-install" {
			t.Error("flatcar-install should not be called by FCOSInstaller")
		}
	}
	if installCall == nil {
		t.Fatal("coreos-installer was not called")
	}

	if installCall.Args[0] != "install" {
		t.Errorf("first arg = %q, want %q", installCall.Args[0], "install")
	}
	if installCall.Args[1] != "--stream" || installCall.Args[2] != "stable" {
		t.Errorf("stream args = %v, want --stream stable", installCall.Args[1:3])
	}
	if installCall.Args[3] != "/dev/sda" {
		t.Errorf("disk arg = %q, want /dev/sda", installCall.Args[3])
	}
	if installCall.Args[4] != "--ignition-file" {
		t.Errorf("ignition flag = %q, want --ignition-file", installCall.Args[4])
	}
}

func TestFCOSInstall_NoWipefsOrSfdisk(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-no-wipe",
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
			t.Error("wipefs should NOT be called by FCOSInstaller")
		}
		if call.Name == "sfdisk" {
			t.Error("sfdisk should NOT be called by FCOSInstaller")
		}
	}
}

func TestFCOSInstall_ExternalIgnitionURL(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:          model.OSFCOS,
		Channel:     "testing",
		Hostname:    "fcos-ext",
		Disk:        model.DiskInfo{DevPath: "/dev/vda"},
		IgnitionURL: "https://example.com/config.ign",
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

	wantArgs := []string{"install", "--stream", "testing", "/dev/vda", "--ignition-url", "https://example.com/config.ign"}
	if len(installCall.Args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", installCall.Args, wantArgs)
	}
	for i, arg := range wantArgs {
		if installCall.Args[i] != arg {
			t.Errorf("arg[%d] = %q, want %q", i, installCall.Args[i], arg)
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
	if got := err.Error(); got != "install config cannot be nil" {
		t.Errorf("error = %q, want %q", got, "install config cannot be nil")
	}
}

func TestFCOSInstall_VersionPinWarning(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Version:  "38.20230514.3.0",
		Hostname: "fcos-version",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			for _, arg := range spy.Calls[i].Args {
				if arg == "38.20230514.3.0" || arg == "-V" || arg == "--version" {
					t.Errorf("version pinning should not be passed to coreos-installer, got arg %q", arg)
				}
			}
		}
	}
}

func TestFCOSInstall_PrefersByIDPath(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:      model.OSFCOS,
		Channel: "stable",
		Disk: model.DiskInfo{
			DevPath: "/dev/sda",
			Path:    "/dev/disk/by-id/ata-Samsung_SSD",
		},
		Hostname: "fcos-byid",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := range spy.Calls {
		if spy.Calls[i].Name == "coreos-installer" {
			if spy.Calls[i].Args[3] != "/dev/disk/by-id/ata-Samsung_SSD" {
				t.Errorf("disk path = %q, want /dev/disk/by-id/ata-Samsung_SSD", spy.Calls[i].Args[3])
			}
		}
	}
}

func TestFCOSInstall_Failure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.AllError = fmt.Errorf("command exited with code 1")

	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-fail",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when coreos-installer fails")
	}
}

func TestFCOSInstall_ProgressSteps(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-progress",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
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

func TestFCOSInstall_GenerateButaneError(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-butane-err",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Sysexts: []model.SysextEntry{
			{Name: "bad", URL: "http://insecure.example.com/bad.raw", Selected: true},
		},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from FCOS butane generation")
	}
	if !strings.Contains(err.Error(), "generating butane config") {
		t.Errorf("error = %q, want 'generating butane config' prefix", err.Error())
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
		t.Fatal("expected error from compileToIgnitionFunc")
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
		t.Fatal("expected error from writeIgnitionFile")
	}
	if !strings.Contains(err.Error(), "writing ignition file") {
		t.Errorf("error = %q, want 'writing ignition file' prefix", err.Error())
	}
}

func TestFCOSInstall_IgnitionFileCleanup(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-cleanup",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After install completes, ignitionPath should be cleared by cleanup
	if installer.ignitionPath != "" {
		t.Error("ignitionPath should be cleared after install")
	}
}

func TestFCOSInstall_CorEOSInstallerStderrIncluded(t *testing.T) {
	failRunner := &fcosStderrFailRunner{}
	installer := NewFCOSInstaller(failRunner, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-stderr",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when coreos-installer fails")
	}
	if !strings.Contains(err.Error(), "disk is in use") {
		t.Errorf("error = %q, want stderr from coreos-installer", err.Error())
	}
}

type fcosStderrFailRunner struct{}

func (fcosStderrFailRunner) Run(_ context.Context, name string, args ...string) (*runner.Result, error) {
	if name != "coreos-installer" {
		return &runner.Result{Command: name, Args: args, ExitCode: 0}, nil
	}
	return &runner.Result{
		Command:  name,
		Args:     args,
		Stderr:   "error: disk is in use",
		ExitCode: 1,
	}, fmt.Errorf("command %q exited with code 1", name)
}

func (fcosStderrFailRunner) RunWithInput(_ context.Context, _ string, _ string, _ ...string) (*runner.Result, error) {
	return nil, fmt.Errorf("unexpected RunWithInput call")
}

func TestFCOSInstall_ExternalURLProgressSteps(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:          model.OSFCOS,
		Channel:     "stable",
		Hostname:    "fcos-ext-progress",
		Disk:        model.DiskInfo{DevPath: "/dev/sda"},
		IgnitionURL: "https://example.com/config.ign",
	}

	var steps []string
	err := installer.Install(context.Background(), cfg, func(step string) {
		steps = append(steps, step)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSteps := []string{
		"Using external Ignition config...",
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

func TestFCOSInstall_NonZeroExitCode(t *testing.T) {
	failRunner := &fcosNonZeroExitRunner{}
	installer := NewFCOSInstaller(failRunner, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-nonzero",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "coreos-installer failed") {
		t.Errorf("error = %q, want coreos-installer failed prefix", err.Error())
	}
}

type fcosNonZeroExitRunner struct{}

func (fcosNonZeroExitRunner) Run(_ context.Context, name string, args ...string) (*runner.Result, error) {
	if name != "coreos-installer" {
		return &runner.Result{Command: name, Args: args, ExitCode: 0}, nil
	}
	return &runner.Result{Command: name, Args: args, ExitCode: 1, Stderr: ""}, nil
}

func (fcosNonZeroExitRunner) RunWithInput(_ context.Context, _ string, _ string, _ ...string) (*runner.Result, error) {
	return nil, fmt.Errorf("unexpected RunWithInput call")
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

func TestFCOSInstall_IgnitionFileCleanupOnFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubError("coreos-installer install --stream stable /dev/sda --ignition-file *", fmt.Errorf("install failed"))
	spy.AllError = fmt.Errorf("install failed")

	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-cleanup-fail",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	_ = installer.Install(context.Background(), cfg, func(string) {})

	// Even on failure, ignitionPath should be cleared by cleanup
	if installer.ignitionPath != "" {
		t.Error("ignitionPath should be cleared after failed install")
	}
}

func TestFCOSInstall_IgnitionFileCreatedAndRemoved(t *testing.T) {
	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-tempfile",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}

	// Capture the ignition path during install
	var capturedPath string
	origRemove := removeIgnitionFile
	t.Cleanup(func() { removeIgnitionFile = origRemove })
	removeIgnitionFile = func(path string) error {
		capturedPath = path
		return origRemove(path)
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath == "" {
		t.Fatal("ignition file was never removed (cleanup did not run)")
	}
	// File should no longer exist
	if _, err := os.Stat(capturedPath); !os.IsNotExist(err) {
		t.Errorf("ignition file %q should have been removed", capturedPath)
	}
}
