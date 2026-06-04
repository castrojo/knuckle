package install

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestFCOSInstaller_WriteIgnitionFile_CreateError(t *testing.T) {
	createErr := errors.New("disk full")
	oldCreate := newIgnitionTempFile
	t.Cleanup(func() { newIgnitionTempFile = oldCreate })
	newIgnitionTempFile = func() (ignitionTempFile, error) { return nil, createErr }

	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	_, err := installer.writeIgnitionFile(`{}`)
	if !errors.Is(err, createErr) {
		t.Fatalf("expected createErr, got: %v", err)
	}
}

func TestFCOSInstaller_WriteIgnitionFile_WriteError(t *testing.T) {
	backingFile, err := os.CreateTemp(t.TempDir(), "fcos-write-err-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	writeErr := errors.New("write failure")
	stub := &stubIgnitionTempFile{file: backingFile, writeErr: writeErr}

	overrideIgnitionFileOps(t, stub, nil)

	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	_, err = installer.writeIgnitionFile(`{}`)
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected writeErr, got: %v", err)
	}
}

func TestFCOSInstaller_WriteIgnitionFile_CloseError(t *testing.T) {
	backingFile, err := os.CreateTemp(t.TempDir(), "fcos-close-err-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	closeErr := errors.New("close failure")
	stub := &stubIgnitionTempFile{file: backingFile, closeErr: closeErr}

	overrideIgnitionFileOps(t, stub, nil)

	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	_, err = installer.writeIgnitionFile(`{}`)
	if !errors.Is(err, closeErr) {
		t.Fatalf("expected closeErr, got: %v", err)
	}
}

func TestFCOSInstaller_CleanupIgnitionFile_EmptyPath(t *testing.T) {
	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	installer.cleanupIgnitionFile() // no-op — must not panic
}

func TestFCOSInstaller_CleanupIgnitionFile_AlreadyRemoved(t *testing.T) {
	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	installer.ignitionPath = "/tmp/no-such-fcos-ignition-file.json"
	installer.cleanupIgnitionFile() // os.IsNotExist → silent
	if installer.ignitionPath != "" {
		t.Error("ignitionPath should be cleared after cleanup")
	}
}

func TestFCOSInstaller_CleanupIgnitionFile_RemoveFailure(t *testing.T) {
	removeErr := errors.New("remove denied")
	oldRemove := removeIgnitionFile
	t.Cleanup(func() { removeIgnitionFile = oldRemove })
	removeIgnitionFile = func(string) error { return removeErr }

	installer := NewFCOSInstaller(runner.NewSpyRunner(), testLogger())
	installer.ignitionPath = "/dev/fcos-fake"
	installer.cleanupIgnitionFile() // should warn, not panic
	if installer.ignitionPath != "" {
		t.Error("ignitionPath should be cleared even after remove failure")
	}
}

func TestFCOSInstaller_CoreosInstallerFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.AllError = errors.New("coreos-installer: no such device")
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:      model.OSFCOS,
		Channel: "stable",
		Hostname: "fcos-node",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}
	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error from coreos-installer failure, got nil")
	}
}

func TestFCOSInstaller_CompileToIgnitionError(t *testing.T) {
	compileErr := errors.New("butane compile failure")
	orig := compileToIgnitionFunc
	t.Cleanup(func() { compileToIgnitionFunc = orig })
	compileToIgnitionFunc = func(string) (string, error) { return "", compileErr }

	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:      model.OSFCOS,
		Channel: "stable",
		Hostname: "fcos-node",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}
	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected compile error, got nil")
	}
	if !errors.Is(err, compileErr) {
		t.Errorf("expected compileErr wrapped, got: %v", err)
	}
}

func TestFCOSInstaller_TempFileCreateError(t *testing.T) {
	createErr := errors.New("no space left")
	oldCreate := newIgnitionTempFile
	t.Cleanup(func() { newIgnitionTempFile = oldCreate })
	newIgnitionTempFile = func() (ignitionTempFile, error) { return nil, createErr }

	spy := runner.NewSpyRunner()
	installer := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{
		OS:      model.OSFCOS,
		Channel: "stable",
		Hostname: "fcos-node",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:   []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
	}
	err := installer.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected temp file error, got nil")
	}
	if !errors.Is(err, createErr) {
		t.Errorf("expected createErr wrapped, got: %v", err)
	}
}
