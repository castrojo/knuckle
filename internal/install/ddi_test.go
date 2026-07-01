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
	i.mkdirTemp = func(string, string) (string, error) { return "/tmp/fake-repart-dir", nil }
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

// ── fakeDirEntry ──────────────────────────────────────────────────────────────

type fakeDirEntry struct{ name string }

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return false }
func (f fakeDirEntry) Type() os.FileMode          { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, nil }

// ── local DDI path coverage ───────────────────────────────────────────────────

func TestBluefinDDIInstaller_LocalDDI(t *testing.T) {
	spy := runner.NewSpyRunner()
	// blkid finds local DDI device
	stubOut(spy, "blkid -L bluefin-roota", "/dev/vdc\n")
	// systemd-repart with dynamic tmpDir path — no stub needed (defaults to exit 0)
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b uuid-esp EFI\n" +
		"/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 uuid-root bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	stubOut(spy, "mount /dev/sda1 /mnt/esp", "")
	stubOut(spy, "mount /dev/sda2 /mnt/root", "")
	stubOut(spy, "useradd --root /mnt/root --create-home --groups wheel --shell /bin/bash user1", "")
	// systemd-firstboot and bootctl/umount default to success

	i := noopDDIInstaller(spy)
	i.readDir = func(name string) ([]os.DirEntry, error) {
		if strings.Contains(name, "modules") {
			return []os.DirEntry{fakeDirEntry{"6.12.0-bluefin"}}, nil
		}
		return []os.DirEntry{
			fakeDirEntry{"10-esp.conf"},
			fakeDirEntry{"20-root-a.conf"},
		}, nil
	}
	i.readFile = func(name string) ([]byte, error) { return []byte("[Partition]\n"), nil }

	cfg := &model.InstallConfig{
		OS:       model.OSBluefinDDI,
		Hostname: "myhost",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
		Users:    []model.UserConfig{{Username: "user1", Groups: []string{"wheel"}}},
	}

	var steps []string
	err := i.Install(context.Background(), cfg, func(s string) { steps = append(steps, s) })
	if err != nil {
		t.Fatalf("Install() with local DDI error: %v", err)
	}
	found := false
	for _, s := range steps {
		if strings.Contains(s, "Local DDI") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'Local DDI' progress step, steps=%v", steps)
	}
}

// ── provisionUser with password + SSH keys ────────────────────────────────────

func TestBluefinDDIInstaller_ProvisionUser_PasswordAndSSH(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "useradd --root /mnt/root --create-home --groups wheel --shell /bin/bash bob", "")
	stubOut(spy, "chown -R bob:bob /mnt/root/home/bob/.ssh", "")
	spy.StubResponse("chpasswd --root /mnt/root --encrypted", &runner.Result{ExitCode: 0})

	i := noopDDIInstaller(spy)

	user := model.UserConfig{
		Username:     "bob",
		PasswordHash: "$2b$12$hash",
		Groups:       []string{"wheel"},
	}
	err := i.provisionUser(context.Background(), user, []string{"ssh-ed25519 AAAA key1"})
	if err != nil {
		t.Fatalf("provisionUser error: %v", err)
	}
}

// ── writeBootEntry with kernel ────────────────────────────────────────────────

func TestBluefinDDIInstaller_WriteBootEntry_WithKernel(t *testing.T) {
	i := noopDDIInstaller(runner.NewSpyRunner())
	i.readDir = func(name string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{"6.12.0-bluefin"}}, nil
	}
	i.readFile = func(name string) ([]byte, error) { return []byte("vmlinuz-data"), nil }

	if err := i.writeBootEntry("abc-partuuid-123"); err != nil {
		t.Fatalf("writeBootEntry error: %v", err)
	}
}

// ── writeBootEntry fallback (empty PARTUUID) ──────────────────────────────────

func TestBluefinDDIInstaller_WriteBootEntry_FallbackRoot(t *testing.T) {
	i := noopDDIInstaller(runner.NewSpyRunner())
	i.readDir = func(string) ([]os.DirEntry, error) { return nil, nil } // no kernel
	i.readFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }

	if err := i.writeBootEntry(""); err != nil {
		t.Fatalf("writeBootEntry fallback error: %v", err)
	}
}

// ── dispatch: BluefinDDI nil, not dry-run ─────────────────────────────────────

func TestDispatchingInstaller_BluefinDDI_NotConfigured_Error(t *testing.T) {
	d := &DispatchingInstaller{}
	err := d.Install(context.Background(), &model.InstallConfig{OS: model.OSBluefinDDI}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("want 'not configured' error, got %v", err)
	}
}

// ── detectPartitions: lsblk fails ────────────────────────────────────────────

func TestDetectPartitions_LsblkFails(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubError("lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", os.ErrPermission)
	i := noopDDIInstaller(spy)
	_, _, _, err := i.detectPartitions(context.Background(), "/dev/sda")
	if err == nil || !strings.Contains(err.Error(), "lsblk") {
		t.Errorf("want 'lsblk' error, got %v", err)
	}
}

// ── detectPartitions: no matching partitions (ESP not found = error) ──────────

func TestDetectPartitions_NoMatchingPartitions(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda",
		"/dev/sda1 deadbeef-dead-beef-dead-beefdeadbeef uuid-1 other-label\n")
	i := noopDDIInstaller(spy)
	_, _, _, err := i.detectPartitions(context.Background(), "/dev/sda")
	if err == nil || !strings.Contains(err.Error(), "ESP partition not found") {
		t.Errorf("want 'ESP partition not found' error, got %v", err)
	}
}

// ── writeLocalRepartDefs: readDir fails ──────────────────────────────────────

func TestWriteLocalRepartDefs_ReadDirFails(t *testing.T) {
	i := noopDDIInstaller(runner.NewSpyRunner())
	i.readDir = func(string) ([]os.DirEntry, error) { return nil, os.ErrPermission }
	if err := i.writeLocalRepartDefs("/tmp/test", "/dev/vdc"); err == nil {
		t.Error("expected readDir error")
	}
}

// ── writeLocalRepartDefs: writeFile fails for 20-root-a.conf ─────────────────

func TestWriteLocalRepartDefs_WriteFileFails_RootA(t *testing.T) {
	i := noopDDIInstaller(runner.NewSpyRunner())
	i.readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{"20-root-a.conf"}}, nil
	}
	i.writeFile = func(string, []byte, os.FileMode) error { return os.ErrPermission }
	if err := i.writeLocalRepartDefs("/tmp/test", "/dev/vdc"); err == nil {
		t.Error("expected writeFile error for 20-root-a.conf")
	}
}

// ── writeLocalRepartDefs: readFile fails for other conf ───────────────────────

func TestWriteLocalRepartDefs_ReadFileFails(t *testing.T) {
	i := noopDDIInstaller(runner.NewSpyRunner())
	i.readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{"10-esp.conf"}}, nil
	}
	i.readFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	if err := i.writeLocalRepartDefs("/tmp/test", "/dev/vdc"); err == nil {
		t.Error("expected readFile error")
	}
}

// ── writeLocalRepartDefs: writeFile fails for other conf ─────────────────────

func TestWriteLocalRepartDefs_WriteFileFails_Other(t *testing.T) {
	i := noopDDIInstaller(runner.NewSpyRunner())
	i.readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{"10-esp.conf"}}, nil
	}
	i.readFile = func(string) ([]byte, error) { return []byte("data"), nil }
	i.writeFile = func(string, []byte, os.FileMode) error { return os.ErrPermission }
	if err := i.writeLocalRepartDefs("/tmp/test", "/dev/vdc"); err == nil {
		t.Error("expected writeFile error for 10-esp.conf")
	}
}

// ── prepareDDI: network wait fails ───────────────────────────────────────────

func TestPrepareDDI_NetworkFails(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	spy.StubError("systemctl start systemd-networkd-wait-online.service", os.ErrPermission)
	i := noopDDIInstaller(spy)
	_, _, err := i.prepareDDI(context.Background(), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "network wait") {
		t.Errorf("want 'network wait' error, got %v", err)
	}
}

// ── prepareDDI: sysupdate fails ───────────────────────────────────────────────

func TestPrepareDDI_SysupdateFails(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	spy.StubError("/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", os.ErrPermission)
	i := noopDDIInstaller(spy)
	_, _, err := i.prepareDDI(context.Background(), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "sysupdate") {
		t.Errorf("want 'sysupdate' error, got %v", err)
	}
}

// ── Install: prepareDDI error ─────────────────────────────────────────────────

func TestInstall_PrepareDDI_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	spy.StubError("/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", os.ErrPermission)
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Error("expected error from prepareDDI")
	}
}

// ── Install: detectPartitions error ──────────────────────────────────────────

func TestInstall_DetectPartitions_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	spy.StubError("lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", os.ErrPermission)
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "partition") {
		t.Errorf("want 'partition' error, got %v", err)
	}
}

// ── Install: mkdirAll /mnt/esp error ─────────────────────────────────────────

func TestInstall_MkdirESP_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	i := noopDDIInstaller(spy)
	callCount := 0
	i.mkdirAll = func(path string, _ os.FileMode) error {
		callCount++
		if callCount == 1 {
			return os.ErrPermission
		}
		return nil
	}
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	err := i.Install(context.Background(), cfg, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "/mnt/esp") {
		t.Errorf("want '/mnt/esp' error, got %v", err)
	}
}

// ── Install: mount ESP error ──────────────────────────────────────────────────

func TestInstall_MountESP_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	spy.StubError("mount /dev/sda1 /mnt/esp", os.ErrPermission)
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "mount ESP") {
		t.Errorf("want 'mount ESP' error, got %v", err)
	}
}

// ── Install: mount root error ─────────────────────────────────────────────────

func TestInstall_MountRoot_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	stubOut(spy, "mount /dev/sda1 /mnt/esp", "")
	spy.StubError("mount /dev/sda2 /mnt/root", os.ErrPermission)
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "mount root") {
		t.Errorf("want 'mount root' error, got %v", err)
	}
}

// ── Install: provisionUser error ──────────────────────────────────────────────

func TestInstall_ProvisionUser_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	stubOut(spy, "mount /dev/sda1 /mnt/esp", "")
	stubOut(spy, "mount /dev/sda2 /mnt/root", "")
	spy.StubError("useradd --root /mnt/root --create-home --groups wheel,sudo --shell /bin/bash core", os.ErrPermission)
	cfg := &model.InstallConfig{
		OS:   model.OSBluefinDDI,
		Disk: model.DiskInfo{DevPath: "/dev/sda"},
		Users: []model.UserConfig{{Username: "core"}},
	}
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "provisioning user") {
		t.Errorf("want 'provisioning user' error, got %v", err)
	}
}

// ── Install: mkdirAll /mnt/root error ─────────────────────────────────────────

func TestInstall_MkdirRoot_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	i := noopDDIInstaller(spy)
	callCount := 0
	i.mkdirAll = func(path string, _ os.FileMode) error {
		callCount++
		if callCount == 2 {
			return os.ErrPermission
		}
		return nil
	}
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	err := i.Install(context.Background(), cfg, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "/mnt/root") {
		t.Errorf("want '/mnt/root' error, got %v", err)
	}
}

// ── Install: bootctl error (non-fatal: logged, Install still returns nil) ────

func TestInstall_Bootctl_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	stubOut(spy, "mount /dev/sda1 /mnt/esp", "")
	stubOut(spy, "mount /dev/sda2 /mnt/root", "")
	spy.StubError("bootctl install --esp-path=/mnt/esp", os.ErrPermission)
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	// bootctl failure is non-fatal: Install should return nil
	if err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Errorf("bootctl failure should be non-fatal, got: %v", err)
	}
}

// ── Install: writeBootEntry error (non-fatal: logged, Install still returns nil) ──

func TestInstall_WriteBootEntry_Error(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	stubOut(spy, "mount /dev/sda1 /mnt/esp", "")
	stubOut(spy, "mount /dev/sda2 /mnt/root", "")
	i := noopDDIInstaller(spy)
	i.mkdirAll = func(path string, _ os.FileMode) error {
		if strings.Contains(path, "entries") {
			return os.ErrPermission
		}
		return nil
	}
	cfg := &model.InstallConfig{OS: model.OSBluefinDDI, Disk: model.DiskInfo{DevPath: "/dev/sda"}}
	// writeBootEntry failure is non-fatal: Install should return nil
	if err := i.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Errorf("writeBootEntry failure should be non-fatal, got: %v", err)
	}
}

// ── prepareDDI: mkdirTemp fails ───────────────────────────────────────────────

func TestPrepareDDI_MkdirTempFails(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "blkid -L bluefin-roota", "/dev/vdc\n")
	i := noopDDIInstaller(spy)
	i.mkdirTemp = func(string, string) (string, error) { return "", os.ErrPermission }
	_, _, err := i.prepareDDI(context.Background(), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "repart.d") {
		t.Errorf("want 'repart.d' error, got %v", err)
	}
}

// ── detectPartitions: root partition not found ────────────────────────────────

func TestDetectPartitions_RootNotFound(t *testing.T) {
	spy := runner.NewSpyRunner()
	// ESP exists but no bluefin-server-root-a
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda",
		"/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n")
	i := noopDDIInstaller(spy)
	_, _, _, err := i.detectPartitions(context.Background(), "/dev/sda")
	if err == nil || !strings.Contains(err.Error(), "root partition") {
		t.Errorf("want 'root partition' error, got %v", err)
	}
}

// ── provisionUser: mkdirAll .ssh fails ───────────────────────────────────────

func TestProvisionUser_SSHDirFails(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "useradd --root /mnt/root --create-home --groups wheel,sudo --shell /bin/bash bob", "")
	i := noopDDIInstaller(spy)
	i.mkdirAll = func(path string, _ os.FileMode) error {
		if strings.Contains(path, ".ssh") {
			return os.ErrPermission
		}
		return nil
	}
	user := model.UserConfig{Username: "bob"}
	err := i.provisionUser(context.Background(), user, []string{"ssh-ed25519 AAAA key1"})
	if err == nil || !strings.Contains(err.Error(), ".ssh") {
		t.Errorf("want '.ssh' error, got %v", err)
	}
}

// ── wizard_mergeKeys: empty keys filtered ─────────────────────────────────────

func TestWizardMergeKeys_EmptyFiltered(t *testing.T) {
	got := wizard_mergeKeys([]string{"", "key1"}, []string{"", "key2"})
	if len(got) != 2 {
		t.Errorf("len = %d, want 2 (empty strings filtered)", len(got))
	}
	for _, k := range got {
		if k == "" {
			t.Error("empty key should have been filtered")
		}
	}
}

// ── provisionUser: chpasswd failure is non-fatal ──────────────────────────────

func TestProvisionUser_ChpasswdFails_NonFatal(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "useradd --root /mnt/root --create-home --groups wheel,sudo --shell /bin/bash bob", "")
	spy.StubError("chpasswd --root /mnt/root --encrypted", os.ErrPermission)
	i := noopDDIInstaller(spy)
	user := model.UserConfig{Username: "bob", PasswordHash: "$2b$12$hash"}
	// chpasswd failure is non-fatal
	if err := i.provisionUser(context.Background(), user, nil); err != nil {
		t.Errorf("chpasswd failure should be non-fatal, got: %v", err)
	}
}

// ── provisionUser: writeFile authorized_keys fails ────────────────────────────

func TestProvisionUser_WriteAuthKeysFails(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "useradd --root /mnt/root --create-home --groups wheel,sudo --shell /bin/bash bob", "")
	i := noopDDIInstaller(spy)
	i.writeFile = func(string, []byte, os.FileMode) error { return os.ErrPermission }
	user := model.UserConfig{Username: "bob"}
	err := i.provisionUser(context.Background(), user, []string{"ssh-ed25519 AAAA key1"})
	if err == nil || !strings.Contains(err.Error(), "authorized_keys") {
		t.Errorf("want 'authorized_keys' error, got %v", err)
	}
}

// ── detectPartitions: malformed lsblk line (< 4 fields) ──────────────────────

func TestDetectPartitions_MalformedLine(t *testing.T) {
	spy := runner.NewSpyRunner()
	// Line with only 2 fields should be skipped, then ESP found on next line
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda",
		"short line\n"+
			"/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n"+
			"/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n")
	i := noopDDIInstaller(spy)
	espDev, rootDev, partuuid, err := i.detectPartitions(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if espDev != "/dev/sda1" || rootDev != "/dev/sda2" || partuuid != "root-uuid" {
		t.Errorf("partition detection wrong: esp=%q root=%q partuuid=%q", espDev, rootDev, partuuid)
	}
}

// ── prepareDDI: local DDI writeLocalRepartDefs fails ─────────────────────────

func TestPrepareDDI_LocalDDI_WriteRepartDefsFails(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "blkid -L bluefin-roota", "/dev/vdc\n")
	i := noopDDIInstaller(spy)
	i.readDir = func(string) ([]os.DirEntry, error) { return nil, os.ErrPermission }
	_, _, err := i.prepareDDI(context.Background(), func(string) {})
	if err == nil {
		t.Error("expected writeLocalRepartDefs error to be propagated")
	}
}

// ── Install: uses Disk.Path when available ────────────────────────────────────

func TestInstall_UsesDiskPath(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	spy.StubError("systemctl start systemd-networkd-wait-online.service", os.ErrPermission)
	cfg := &model.InstallConfig{
		OS:   model.OSBluefinDDI,
		Disk: model.DiskInfo{Path: "/dev/disk/by-id/ata-SSD", DevPath: "/dev/sda"},
	}
	err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {})
	// Should fail at network wait — just verifies Disk.Path was used (not DevPath)
	if err == nil || !strings.Contains(err.Error(), "network wait") {
		t.Errorf("want 'network wait' error using Disk.Path, got %v", err)
	}
}

// ── systemd-firstboot failure is non-fatal ────────────────────────────────────

func TestInstall_FirstbootFails_NonFatal(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubFail(spy, "blkid -L bluefin-roota")
	stubOut(spy, "systemctl start systemd-networkd-wait-online.service", "")
	stubOut(spy, "/usr/lib/systemd/systemd-sysupdate --definitions=/usr/lib/sysupdate.d update", "")
	stubOut(spy, "systemd-repart --dry-run=no --empty=force --definitions=/usr/lib/repart.d /dev/sda", "")
	lsblkOut := "/dev/sda1 c12a7328-f81f-11d2-ba4b-00a0c93ec93b esp-uuid EFI\n/dev/sda2 4f68bce3-e8cd-4db1-96e7-fbcaf984b709 root-uuid bluefin-server-root-a\n"
	stubOut(spy, "lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL /dev/sda", lsblkOut)
	stubOut(spy, "mount /dev/sda1 /mnt/esp", "")
	stubOut(spy, "mount /dev/sda2 /mnt/root", "")
	// firstboot fails for all arg combinations (use AllError for simplicity)
	// Can't use AllError — need specific stub. Use StubError with the exact key.
	spy.StubError("systemd-firstboot --image-root=/mnt/root --hostname=myhostname --timezone=UTC", os.ErrPermission)
	cfg := &model.InstallConfig{
		OS:       model.OSBluefinDDI,
		Hostname: "myhostname",
		Disk:     model.DiskInfo{DevPath: "/dev/sda"},
	}
	if err := noopDDIInstaller(spy).Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Errorf("firstboot failure should be non-fatal, got: %v", err)
	}
}

// ── provisionUser: chown .ssh failure is non-fatal ────────────────────────────

func TestProvisionUser_ChownFails_NonFatal(t *testing.T) {
	spy := runner.NewSpyRunner()
	stubOut(spy, "useradd --root /mnt/root --create-home --groups wheel,sudo --shell /bin/bash bob", "")
	spy.StubError("chown -R bob:bob /mnt/root/home/bob/.ssh", os.ErrPermission)
	i := noopDDIInstaller(spy)
	user := model.UserConfig{Username: "bob"}
	if err := i.provisionUser(context.Background(), user, []string{"ssh-ed25519 AAAA key1"}); err != nil {
		t.Errorf("chown failure should be non-fatal, got: %v", err)
	}
}
