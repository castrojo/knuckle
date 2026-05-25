package install

import (
	"errors"
	"os"
	"testing"

	"github.com/projectbluefin/knuckle/internal/runner"
)

type stubIgnitionTempFile struct {
	file        *os.File
	writeErr    error
	closeErr    error
	closeCalled bool
}

func (f *stubIgnitionTempFile) Name() string {
	return f.file.Name()
}

func (f *stubIgnitionTempFile) WriteString(s string) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.file.WriteString(s)
}

func (f *stubIgnitionTempFile) Close() error {
	f.closeCalled = true
	if err := f.file.Close(); err != nil {
		return err
	}
	return f.closeErr
}

func overrideIgnitionFileOps(t *testing.T, newFile ignitionTempFile, createErr error) {
	t.Helper()
	oldCreate := newIgnitionTempFile
	oldRemove := removeIgnitionFile
	newIgnitionTempFile = func() (ignitionTempFile, error) {
		if createErr != nil {
			return nil, createErr
		}
		return newFile, nil
	}
	removeIgnitionFile = os.Remove
	t.Cleanup(func() {
		newIgnitionTempFile = oldCreate
		removeIgnitionFile = oldRemove
	})
}

func TestWriteIgnitionFile_WriteErrorRemovesTempFile(t *testing.T) {
	installer := NewFlatcarInstaller(runner.NewSpyRunner(), testLogger())
	backingFile, err := os.CreateTemp(t.TempDir(), "ignition-write-error-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	writeErr := errors.New("simulated write failure")
	stubFile := &stubIgnitionTempFile{file: backingFile, writeErr: writeErr}
	overrideIgnitionFileOps(t, stubFile, nil)

	removedPath := ""
	removeIgnitionFile = func(path string) error {
		removedPath = path
		return os.Remove(path)
	}

	path, err := installer.WriteIgnitionFile(`{"passwd":{"users":[{"sshAuthorizedKeys":["ssh-ed25519 AAAA"]}]}}`)
	if path != "" {
		t.Fatalf("path = %q, want empty string", path)
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want wrapped write error", err)
	}
	if !stubFile.closeCalled {
		t.Fatal("expected write failure path to close temp file before cleanup")
	}
	if removedPath != backingFile.Name() {
		t.Fatalf("cleanup removed %q, want %q", removedPath, backingFile.Name())
	}
	if _, statErr := os.Stat(backingFile.Name()); !os.IsNotExist(statErr) {
		t.Fatalf("temp file should be removed after write failure, stat err = %v", statErr)
	}
}

func TestWriteIgnitionFile_CloseErrorRemovesTempFile(t *testing.T) {
	installer := NewFlatcarInstaller(runner.NewSpyRunner(), testLogger())
	backingFile, err := os.CreateTemp(t.TempDir(), "ignition-close-error-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	closeErr := errors.New("simulated close failure")
	stubFile := &stubIgnitionTempFile{file: backingFile, closeErr: closeErr}
	overrideIgnitionFileOps(t, stubFile, nil)

	removedPath := ""
	removeIgnitionFile = func(path string) error {
		removedPath = path
		return os.Remove(path)
	}

	path, err := installer.WriteIgnitionFile(`{"passwd":{"users":[{"sshAuthorizedKeys":["ssh-ed25519 AAAA"]}]}}`)
	if path != "" {
		t.Fatalf("path = %q, want empty string", path)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want wrapped close error", err)
	}
	if removedPath != backingFile.Name() {
		t.Fatalf("cleanup removed %q, want %q", removedPath, backingFile.Name())
	}
	if _, statErr := os.Stat(backingFile.Name()); !os.IsNotExist(statErr) {
		t.Fatalf("temp file should be removed after close failure, stat err = %v", statErr)
	}
}

func TestCleanupIgnitionFile_RemovesSensitiveFile(t *testing.T) {
	installer := NewFlatcarInstaller(runner.NewSpyRunner(), testLogger())
	backingFile, err := os.CreateTemp(t.TempDir(), "ignition-cleanup-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := backingFile.WriteString(`{"passwd":{"users":[{"sshAuthorizedKeys":["ssh-ed25519 AAAA sensitive"]}]}}`); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := backingFile.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	installer.ignitionPath = backingFile.Name()
	installer.cleanupIgnitionFile()

	if installer.ignitionPath != "" {
		t.Fatal("ignitionPath should be cleared after cleanup")
	}
	if _, statErr := os.Stat(backingFile.Name()); !os.IsNotExist(statErr) {
		t.Fatalf("cleanup should remove ignition file, stat err = %v", statErr)
	}
}
