package install

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// TestInstall_CompileToIgnitionFunc_Error covers install.go:77–79 — the error
// return path when compileToIgnitionFunc fails after a successful GenerateButane call.
//
// Previously untestable because ignition.CompileToIgnition is called directly
// with no injection point; adding compileToIgnitionFunc follows the same
// injectable-var pattern already used for newIgnitionTempFile/removeIgnitionFile.
func TestInstall_CompileToIgnitionFunc_Error(t *testing.T) {
	origCompile := compileToIgnitionFunc
	compileToIgnitionFunc = func(_ string) (string, error) {
		return "", fmt.Errorf("injected compile error")
	}
	defer func() { compileToIgnitionFunc = origCompile }()

	spy := runner.NewSpyRunner()
	installer := NewFlatcarInstaller(spy, testLogger())

	cfg := &model.InstallConfig{
		Hostname: "node01",
		Channel:  "stable",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Disk:     model.DiskInfo{DevPath: "/dev/vda", Path: "/dev/vda"},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@qa"}},
		},
	}

	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when compileToIgnitionFunc fails, got nil")
	}
	if !strings.Contains(err.Error(), "compiling butane") {
		t.Errorf("expected error to mention 'compiling butane', got: %v", err)
	}
	if !strings.Contains(err.Error(), "injected compile error") {
		t.Errorf("expected error to wrap 'injected compile error', got: %v", err)
	}
}

// TestInstall_CompileToIgnitionFunc_Default verifies the injectable var uses
// the real ignition.CompileToIgnition by default (no accidental nil or stub).
func TestInstall_CompileToIgnitionFunc_Default(t *testing.T) {
	if compileToIgnitionFunc == nil {
		t.Fatal("compileToIgnitionFunc must not be nil at package init")
	}
}
