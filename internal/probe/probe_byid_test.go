package probe

import (
	"os"
	"path/filepath"
	"testing"
)

// Tests for the resolveByIDPathIn success path — covers L225-236 in probe.go.
// Uses a temp directory with symlinks to simulate /dev/disk/by-id/ layout.

func TestResolveByIDPathIn_MatchFound(t *testing.T) {
	// Create a fake by-id directory with a symlink pointing to our target.
	byIDDir := t.TempDir()
	targetPath := "/dev/sda"

	// Create a relative symlink: by-id/ata-VBOX_HARDDISK_VB123 -> ../../sda
	// We need the symlink target to resolve to targetPath when joined with byIDDir.
	// Use an absolute symlink for simplicity in testing.
	linkName := "ata-VBOX_HARDDISK_VB12345678-9abcdef0"
	linkPath := filepath.Join(byIDDir, linkName)
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	got := resolveByIDPathIn(targetPath, byIDDir)
	want := filepath.Join(byIDDir, linkName)
	if got != want {
		t.Errorf("resolveByIDPathIn() = %q, want %q", got, want)
	}
}

func TestResolveByIDPathIn_RelativeSymlink(t *testing.T) {
	// Simulate the real /dev/disk/by-id layout where symlinks are relative:
	//   /dev/disk/by-id/ata-DISK -> ../../sda
	byIDDir := t.TempDir()

	// Create the directory structure that the relative symlink resolves through.
	// byIDDir is e.g. /tmp/test123, so "../../sda" resolves to /tmp/sda when
	// filepath.Join(byIDDir, "../../sda") is called then Abs'd.
	// We compute what the absolute target would be:
	relTarget := "../../sda"
	absTarget := filepath.Join(byIDDir, relTarget)
	absTarget, _ = filepath.Abs(absTarget)

	linkName := "scsi-0QEMU_QEMU_HARDDISK_drive-scsi0"
	linkPath := filepath.Join(byIDDir, linkName)
	if err := os.Symlink(relTarget, linkPath); err != nil {
		t.Fatalf("creating relative symlink: %v", err)
	}

	// Query with the absolute resolved path
	got := resolveByIDPathIn(absTarget, byIDDir)
	want := filepath.Join(byIDDir, linkName)
	if got != want {
		t.Errorf("resolveByIDPathIn() = %q, want %q", got, want)
	}
}

func TestResolveByIDPathIn_MultipleEntries_PicksCorrect(t *testing.T) {
	// Directory has multiple symlinks — only the matching one should be returned.
	byIDDir := t.TempDir()

	// Create symlinks for different disks
	targets := map[string]string{
		"ata-VBOX_HARDDISK_sda": "/dev/sda",
		"ata-VBOX_HARDDISK_sdb": "/dev/sdb",
		"ata-VBOX_HARDDISK_sdc": "/dev/sdc",
	}
	for name, target := range targets {
		if err := os.Symlink(target, filepath.Join(byIDDir, name)); err != nil {
			t.Fatalf("creating symlink %s: %v", name, err)
		}
	}

	// Look for /dev/sdb
	got := resolveByIDPathIn("/dev/sdb", byIDDir)
	want := filepath.Join(byIDDir, "ata-VBOX_HARDDISK_sdb")
	if got != want {
		t.Errorf("resolveByIDPathIn(/dev/sdb) = %q, want %q", got, want)
	}
}

func TestResolveByIDPathIn_NoMatch_FallsBack(t *testing.T) {
	// Directory exists but no symlink points to our device.
	byIDDir := t.TempDir()

	// Create a symlink to a different device
	if err := os.Symlink("/dev/sda", filepath.Join(byIDDir, "ata-DISK_A")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	got := resolveByIDPathIn("/dev/nvme0n1", byIDDir)
	if got != "/dev/nvme0n1" {
		t.Errorf("resolveByIDPathIn() = %q, want fallback to /dev/nvme0n1", got)
	}
}

func TestResolveByIDPathIn_UnreadableDir_FallsBack(t *testing.T) {
	got := resolveByIDPathIn("/dev/sda", "/nonexistent/path/by-id")
	if got != "/dev/sda" {
		t.Errorf("resolveByIDPathIn() = %q, want fallback to /dev/sda", got)
	}
}

func TestResolveByIDPathIn_BrokenSymlink_Skipped(t *testing.T) {
	// A broken symlink (target doesn't exist) should be skipped, not crash.
	byIDDir := t.TempDir()

	// Create a broken symlink and a valid one
	if err := os.Symlink("/dev/nonexistent", filepath.Join(byIDDir, "broken-link")); err != nil {
		t.Fatalf("creating broken symlink: %v", err)
	}
	if err := os.Symlink("/dev/sda", filepath.Join(byIDDir, "valid-link")); err != nil {
		t.Fatalf("creating valid symlink: %v", err)
	}

	// Should still find /dev/sda via the valid link
	got := resolveByIDPathIn("/dev/sda", byIDDir)
	want := filepath.Join(byIDDir, "valid-link")
	if got != want {
		t.Errorf("resolveByIDPathIn() = %q, want %q (should skip broken symlink)", got, want)
	}
}

func TestResolveByIDPathIn_RegularFile_Skipped(t *testing.T) {
	// Non-symlink entries in the directory should be harmlessly skipped.
	byIDDir := t.TempDir()

	// Create a regular file (not a symlink)
	if err := os.WriteFile(filepath.Join(byIDDir, "not-a-symlink"), []byte("x"), 0o644); err != nil {
		t.Fatalf("creating regular file: %v", err)
	}
	// Create valid symlink
	if err := os.Symlink("/dev/sda", filepath.Join(byIDDir, "ata-DISK")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	got := resolveByIDPathIn("/dev/sda", byIDDir)
	want := filepath.Join(byIDDir, "ata-DISK")
	if got != want {
		t.Errorf("resolveByIDPathIn() = %q, want %q", got, want)
	}
}

// TestResolveByIDPathIn_ReadlinkError_Continue covers probe.go:233-234 —
// the `if err != nil { continue }` branch inside the directory scan loop.
// The regular file "aaa-regular" sorts before "zzz-link" so os.Readlink is
// called on the regular file first, triggering the error/continue path before
// the loop reaches the matching symlink.
func TestResolveByIDPathIn_ReadlinkError_Continue(t *testing.T) {
	byIDDir := t.TempDir()

	// Regular file sorts before the symlink (alphabetically "aaa" < "zzz").
	if err := os.WriteFile(filepath.Join(byIDDir, "aaa-regular"), []byte("x"), 0o644); err != nil {
		t.Fatalf("creating regular file: %v", err)
	}
	// Matching symlink comes second.
	if err := os.Symlink("/dev/sda", filepath.Join(byIDDir, "zzz-link")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	got := resolveByIDPathIn("/dev/sda", byIDDir)
	want := filepath.Join(byIDDir, "zzz-link")
	if got != want {
		t.Errorf("resolveByIDPathIn() = %q, want %q (should skip non-symlink and find zzz-link)", got, want)
	}
}
