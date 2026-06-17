package probe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NVIDIA/go-nvlib/pkg/nvpci"
)

// TestNvidiaGPUsFromClient_EmptyDeviceName covers probe.go:279-280 —
// when DeviceName is "" (not UnknownDeviceString), the fallback name is used.
func TestNvidiaGPUsFromClient_EmptyDeviceName(t *testing.T) {
	mock := &nvpci.InterfaceMock{
		GetGPUsFunc: func() ([]*nvpci.NvidiaPCIDevice, error) {
			return []*nvpci.NvidiaPCIDevice{
				{
					Address:    "0000:04:00.0",
					Class:      0x030200,
					Device:     0x1234,
					DeviceName: "", // empty string — triggers the "" branch
				},
			}, nil
		},
	}
	gpus := nvidiaGPUsFromClient(mock)
	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}
	if gpus[0].DeviceName == "" {
		t.Error("DeviceName must not remain empty — should fall back to device ID format")
	}
	if gpus[0].DeviceName == nvpci.UnknownDeviceString {
		t.Error("DeviceName should not be UNKNOWN_DEVICE string")
	}
}

// TestResolveByIDPathIn_RegularFile covers probe.go:232-234 —
// when a directory entry is a regular file (not a symlink), Readlink fails and
// the loop continues. With no valid symlinks, it falls through to the slog.Warn
// + return devPath branch (probe.go:248-250).
func TestResolveByIDPathIn_RegularFile(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file — os.Readlink on a non-symlink returns an error.
	if err := os.WriteFile(filepath.Join(dir, "disk-not-a-symlink"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	devPath := "/dev/sda"
	got := resolveByIDPathIn(devPath, dir)
	if got != devPath {
		t.Errorf("expected fallback to %q, got %q", devPath, got)
	}
}

// TestResolveByIDPathIn_NoMatchingSymlink covers probe.go:248-250 —
// when a symlink exists but points to a different device, no match is found
// and the function logs a warning and returns devPath.
func TestResolveByIDPathIn_NoMatchingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-device")
	if err := os.WriteFile(target, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// Symlink points to a real file, but not to devPath.
	linkPath := filepath.Join(dir, "disk-by-id-other")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	devPath := "/dev/sdb"
	got := resolveByIDPathIn(devPath, dir)
	if got != devPath {
		t.Errorf("expected fallback to %q when no symlink matches, got %q", devPath, got)
	}
}
