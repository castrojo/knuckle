package install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/castrojo/knuckle/internal/model"
	"github.com/castrojo/knuckle/internal/runner"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestInstallWithGeneratedConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubResponse("butane --strict", &runner.Result{
		Command:  "butane",
		Args:     []string{"--strict"},
		Stdout:   `{"ignition":{"version":"3.4.0"}}`,
		ExitCode: 0,
	})

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

	// Verify butane was called with input (Butane YAML as stdin)
	var butaneCall *runner.SpyCall
	for i := range spy.Calls {
		if spy.Calls[i].Name == "butane" {
			butaneCall = &spy.Calls[i]
			break
		}
	}
	if butaneCall == nil {
		t.Fatal("butane was not called")
	}
	if butaneCall.Input == "" {
		t.Error("butane was called without stdin input")
	}
	if len(butaneCall.Args) != 1 || butaneCall.Args[0] != "--strict" {
		t.Errorf("butane args = %v, want [--strict]", butaneCall.Args)
	}

	// Verify flatcar-install was called with correct args
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
	wantArgs := []string{"-d", "/dev/sda", "-C", "stable", "-i", "/tmp/knuckle-ignition.json"}
	if len(installCall.Args) != len(wantArgs) {
		t.Fatalf("flatcar-install args = %v, want %v", installCall.Args, wantArgs)
	}
	for i, arg := range wantArgs {
		if installCall.Args[i] != arg {
			t.Errorf("flatcar-install arg[%d] = %q, want %q", i, installCall.Args[i], arg)
		}
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

func TestInstallButaneFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubResponse("butane --strict", &runner.Result{
		Command:  "butane",
		Args:     []string{"--strict"},
		Stdout:   "",
		Stderr:   "error: invalid yaml at line 3",
		ExitCode: 1,
	})

	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "fail-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when butane fails")
	}
	if got := err.Error(); got != "butane compilation failed: error: invalid yaml at line 3" {
		t.Errorf("error = %q, want butane compilation failed message", got)
	}
}

func TestInstallFlatcarInstallFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubResponse("butane --strict", &runner.Result{
		Command:  "butane",
		Args:     []string{"--strict"},
		Stdout:   `{"ignition":{"version":"3.4.0"}}`,
		ExitCode: 0,
	})
	spy.StubResponse("flatcar-install -d /dev/sda -C stable -i /tmp/knuckle-ignition.json", &runner.Result{
		Command:  "flatcar-install",
		Args:     []string{"-d", "/dev/sda", "-C", "stable", "-i", "/tmp/knuckle-ignition.json"},
		Stderr:   "error: disk not found",
		ExitCode: 1,
	})
	spy.StubError("flatcar-install -d /dev/sda -C stable -i /tmp/knuckle-ignition.json",
		fmt.Errorf("command exited with code 1"))

	installer := NewFlatcarInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "fail-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when flatcar-install fails")
	}
	if got := err.Error(); got != "flatcar-install: command exited with code 1" {
		t.Errorf("error = %q", got)
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
			want:         []string{"-d", "/dev/sda", "-C", "stable", "-i", "/tmp/knuckle-ignition.json"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInstallArgs(tt.cfg, tt.ignitionJSON)
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
	spy.StubResponse("butane --strict", &runner.Result{
		Command:  "butane",
		Args:     []string{"--strict"},
		Stdout:   `{"ignition":{"version":"3.4.0"}}`,
		ExitCode: 0,
	})

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
		"Running flatcar-install...",
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
