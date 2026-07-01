package install

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func ddiLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// noopDDIInstaller returns a BluefinDDIInstaller with no-op filesystem calls
// so tests don't touch the real FS.
func noopDDIInstaller(spy *runner.SpyRunner) *BluefinDDIInstaller {
	i := NewBluefinDDIInstaller(spy, ddiLogger())
	i.mkdirAll = func(string, os.FileMode) error { return nil }
	i.writeFile = func(string, []byte, os.FileMode) error { return nil }
	i.readFile = func(string) ([]byte, error) { return []byte{}, nil }
	i.readDir = func(string) ([]os.DirEntry, error) { return nil, nil }
	return i
}

type mockInstaller struct {
	fn func(context.Context, *model.InstallConfig, func(string)) error
}

func (m *mockInstaller) Install(ctx context.Context, cfg *model.InstallConfig, p func(string)) error {
	return m.fn(ctx, cfg, p)
}

func stubOut(spy *runner.SpyRunner, cmd, stdout string) {
	spy.StubResponse(cmd, &runner.Result{Stdout: stdout, ExitCode: 0})
}

func stubFail(spy *runner.SpyRunner, cmd string) {
	spy.StubResponse(cmd, &runner.Result{ExitCode: 1, Stderr: "error"})
}

// ── happy path ────────────────────────────────────────────────────────────────

func TestBluefinDDIInstaller_DryRun(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota") // no local DDI
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b abc-222 EFI\n" +
		"/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 abc-333 bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	stubOut(spy, "mount /dev/sda1 /mnt/esp", "")
	stubOut(spy, "mount /dev/sda2 /mnt/root", "")
	stubOut(spy, "useradd --root /mnt/root --create-home --groups wheel,sudo --shell /bin/bash core", "")
	stubOut(spy, "systemd-firstboot --image-root=/mnt/root --hostname=test-node --timezone=UTC", "")
	stubOut(spy, "bootctl install --esp-path=/mnt/esp", "")
	stubOut(spy, "umount /mnt/root", "")
	stubOut(spy, "umount /mnt/esp", "")

	cfg := &model.InstallConfig{
		OS:       model.OSBluefinDDI,
		Hostname: "test-node",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Users:    []model.UserConfig{{Username: "core", Groups: []string{"wheel", "sudo"}}},
		DryRun:   true,
	}

	var steps []string
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(s string) { steps = append(steps, s) })
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if len(steps) == 0 || steps[len(steps)-1] != "Installation complete!" {
		t.Errorf("last step = %q, want 'Installation complete!'", steps)
	}
}

// ── no disk ───────────────────────────────────────────────────────────────────

func TestBluefinDDIInstaller_NoDisk(t *testing.T) {
	err := noopDDIInstaller(runner.NewSpyRunner()).Install(
		context.Background(), &model.InstallConfig{OS: model.OSBluefinDDI}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "no target disk") {
		t.Errorf("want 'no target disk' error, got %v", err)
	}
}

// ── repart failure bubbles up ─────────────────────────────────────────────────

func TestBluefinDDIInstaller_RepartFailure(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	spy.StubError("systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", os.ErrPermission)

	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "systemd-repart") {
		t.Errorf("want 'systemd-repart' error, got %v", err)
	}
}

// ── dispatch routes BluefinDDI ────────────────────────────────────────────────

func TestDispatchingInstaller_BluefinDDI(t *testing.T) {
	called := false
	d := &DispatchingInstaller{BluefinDDI: &mockInstaller{fn: func(_ context.Context, _ *model.InstallConfig, _ func(string)) error {
		called = true
		return nil
	}}}
	if err := d.Install(context.Background(), &model.InstallConfig{OS: model.OSBluefinDDI}, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("BluefinDDIInstaller was not called")
	}
}

func TestDispatchingInstaller_BluefinDDI_NotConfigured_DryRun(t *testing.T) {
	d := &DispatchingInstaller{}
	err := d.Install(context.Background(), &model.InstallConfig{OS: model.OSBluefinDDI, DryRun: true}, func(string) {})
	if err != nil {
		t.Errorf("dry-run with nil BluefinDDI should succeed, got: %v", err)
	}
}

// ── wizard_mergeKeys deduplication ───────────────────────────────────────────

func TestWizardMergeKeys(t *testing.T) {
	a := []string{"ssh-ed25519 AAA key1", "ssh-ed25519 BBB key2"}
	b := []string{"ssh-ed25519 BBB key2", "ssh-ed25519 CCC key3"}
	got := wizard_mergeKeys(a, b)
	if len(got) != 3 {
		t.Errorf("len = %d, want 3 (duplicate removed)", len(got))
	}
}
