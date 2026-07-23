// Package install — BluefinDDIInstaller provisions a Bluefin Server DDI image.
//
// Flow:
//  1. Detect local DDI source: embedded installer-media partition, local device, or sysupdate.
//  2. systemd-repart partitions the target disk and copies the DDI payload.
//  3. Mount ESP + root; provision user with useradd/chpasswd/authorized_keys.
//  4. systemd-firstboot sets hostname and timezone on the installed rootfs.
//  5. bootctl installs the EFI boot manager; a Type 1 loader entry is written
//     using the root partition's PARTUUID for stable identification.
package install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// BluefinDDIInstaller provisions a Bluefin Server DDI image onto the target
// disk using systemd-repart + native Linux user-space tools. No Ignition.
type BluefinDDIInstaller struct {
	Runner    runner.Runner
	Logger    *slog.Logger
	mkdirAll  func(path string, perm os.FileMode) error // injectable for tests
	writeFile func(name string, data []byte, perm os.FileMode) error
	readFile  func(name string) ([]byte, error)
	readDir   func(name string) ([]os.DirEntry, error)
	mkdirTemp func(dir, pattern string) (string, error)
	statFile  func(name string) (os.FileInfo, error)
}

// NewBluefinDDIInstaller creates a BluefinDDIInstaller with the given runner and logger.
func NewBluefinDDIInstaller(r runner.Runner, logger *slog.Logger) *BluefinDDIInstaller {
	return &BluefinDDIInstaller{
		Runner:    r,
		Logger:    logger,
		mkdirAll:  os.MkdirAll,
		writeFile: os.WriteFile,
		readFile:  os.ReadFile,
		readDir:   os.ReadDir,
		mkdirTemp: os.MkdirTemp,
		statFile:  os.Stat,
	}
}

func (i *BluefinDDIInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(string)) error {
	disk := cfg.Disk.DevPath
	if cfg.Disk.Path != "" {
		disk = cfg.Disk.Path
	}
	if disk == "" {
		return fmt.Errorf("no target disk selected")
	}

	// ── 1. DDI source ───────────────────────────────────────────────────────
	repartDefs, cleanup, err := i.prepareDDI(ctx, progress)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// ── 2. Partition target disk ─────────────────────────────────────────────
	progress("Partitioning disk with systemd-repart...")
	i.Logger.Info("running systemd-repart", "disk", disk, "defs", repartDefs)
	if _, err := i.Runner.Run(ctx, "systemd-repart",
		"--dry-run=no",
		"--empty=force",
		"--definitions="+repartDefs,
		disk,
	); err != nil {
		return fmt.Errorf("systemd-repart: %w", err)
	}

	// ── 3. Detect partitions ─────────────────────────────────────────────────
	progress("Detecting installed partitions...")
	espDev, rootDev, rootPartuuid, err := i.detectPartitions(ctx, disk)
	if err != nil {
		return fmt.Errorf("detecting partitions: %w", err)
	}
	i.Logger.Info("partitions detected", "esp", espDev, "root", rootDev, "partuuid", rootPartuuid)

	// ── 4. Mount ─────────────────────────────────────────────────────────────
	if err := i.mkdirAll("/mnt/esp", 0755); err != nil {
		return fmt.Errorf("mkdir /mnt/esp: %w", err)
	}
	if err := i.mkdirAll("/mnt/root", 0755); err != nil {
		return fmt.Errorf("mkdir /mnt/root: %w", err)
	}
	// Best-effort unmount on exit.
	defer func() {
		_, _ = i.Runner.Run(context.Background(), "umount", "/mnt/root")
		_, _ = i.Runner.Run(context.Background(), "umount", "/mnt/esp")
	}()
	if _, err := i.Runner.Run(ctx, "mount", espDev, "/mnt/esp"); err != nil {
		return fmt.Errorf("mount ESP: %w", err)
	}
	if _, err := i.Runner.Run(ctx, "mount", rootDev, "/mnt/root"); err != nil {
		return fmt.Errorf("mount root: %w", err)
	}

	// ── 5. User provisioning ─────────────────────────────────────────────────
	if len(cfg.Users) > 0 && cfg.Users[0].Username != "" {
		progress(fmt.Sprintf("Creating user %q...", cfg.Users[0].Username))
		keys := wizard_mergeKeys(cfg.Users[0].SSHKeys, cfg.SSHKeys)
		if err := i.provisionUser(ctx, cfg.Users[0], keys); err != nil {
			return fmt.Errorf("provisioning user: %w", err)
		}
	}

	// ── 6. systemd-firstboot (hostname, timezone) ────────────────────────────
	progress("Configuring hostname and timezone...")
	fbArgs := []string{"--image-root=/mnt/root"}
	if cfg.Hostname != "" {
		fbArgs = append(fbArgs, "--hostname="+cfg.Hostname)
	}
	tz := cfg.Timezone
	if tz == "" {
		tz = "UTC"
	}
	fbArgs = append(fbArgs, "--timezone="+tz)
	if _, err := i.Runner.Run(ctx, "systemd-firstboot", fbArgs...); err != nil {
		// Non-fatal: firstboot may fail on already-configured images.
		i.Logger.Warn("systemd-firstboot failed (non-fatal)", "error", err)
	}

	// ── 7. Bootloader ────────────────────────────────────────────────────────
	progress("Installing systemd-boot...")
	if _, err := i.Runner.Run(ctx, "bootctl", "install", "--esp-path=/mnt/esp"); err != nil {
		i.Logger.Warn("bootctl install failed (non-fatal)", "error", err)
	}

	// ── 8. Boot entry ────────────────────────────────────────────────────────
	progress("Writing boot entry...")
	if err := i.writeBootEntry(rootPartuuid); err != nil {
		i.Logger.Warn("boot entry failed (non-fatal)", "error", err)
	}

	progress("Installation complete!")
	return nil
}

// prepareDDI returns the repart.d definitions path to use and an optional
// cleanup function. Detection order:
//  1. Embedded DDI on installer media (GPT partition label "bluefin-installer-data")
//  2. Local DDI device by filesystem label (QEMU show-me-the-future attach)
//  3. Network download via systemd-sysupdate (production OTA)
func (i *BluefinDDIInstaller) prepareDDI(ctx context.Context, progress func(string)) (defs string, cleanup func(), err error) {
	// ── 1. Embedded DDI on offline installer media (USB stick / ISO) ──────
	// The installer disk has a GPT partition labelled "bluefin-installer-data"
	// containing the DDI filesystem image as a raw block copy. systemd-repart's
	// CopyBlocks= can copy directly from the device without mounting it.
	const installerDataPartlabel = "/dev/disk/by-partlabel/bluefin-installer-data"
	if _, statErr := i.statFile(installerDataPartlabel); statErr == nil {
		progress("Embedded DDI detected on installer media")
		i.Logger.Info("embedded DDI source", "device", installerDataPartlabel)

		tmpDir, err := i.mkdirTemp("", "bluefin-repart-*")
		if err != nil {
			return "", nil, fmt.Errorf("creating temp repart.d: %w", err)
		}
		if err := i.writeLocalRepartDefs(tmpDir, installerDataPartlabel); err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", nil, err
		}
		return tmpDir, func() { _ = os.RemoveAll(tmpDir) }, nil
	}

	// ── 2. Local DDI device (e.g. /dev/vdc in show-me-the-future QEMU) ───
	// Check by XFS filesystem label "bluefin-root" (12 chars, XFS max).
	for _, label := range []string{"bluefin-root", "bluefin-roota"} {
		result, blkErr := i.Runner.Run(ctx, "blkid", "-L", label)
		if blkErr == nil && result != nil && strings.TrimSpace(result.Stdout) != "" {
			src := strings.TrimSpace(result.Stdout)
			progress(fmt.Sprintf("Local DDI detected (%s) — skipping network download", src))
			i.Logger.Info("local DDI source", "device", src, "label", label)

			tmpDir, err := i.mkdirTemp("", "bluefin-repart-*")
			if err != nil {
				return "", nil, fmt.Errorf("creating temp repart.d: %w", err)
			}
			if err := i.writeLocalRepartDefs(tmpDir, src); err != nil {
				_ = os.RemoveAll(tmpDir)
				return "", nil, err
			}
			return tmpDir, func() { _ = os.RemoveAll(tmpDir) }, nil
		}
	}

	// ── 3. No local source — pull via sysupdate ──────────────────────────
	progress("Waiting for network...")
	if _, err := i.Runner.Run(ctx, "systemctl", "start", "systemd-networkd-wait-online.service"); err != nil {
		return "", nil, fmt.Errorf("network wait: %w", err)
	}
	progress("Downloading OS DDI via systemd-sysupdate...")
	if _, err := i.Runner.Run(ctx, "/usr/lib/systemd/systemd-sysupdate",
		"--definitions=/usr/lib/sysupdate.d", "update"); err != nil {
		return "", nil, fmt.Errorf("systemd-sysupdate: %w", err)
	}
	return "/usr/lib/repart.d", nil, nil
}

// writeLocalRepartDefs copies /usr/lib/repart.d into tmpDir, replacing
// 20-root-a.conf with a version that uses CopyBlocks=<localDevice>.
func (i *BluefinDDIInstaller) writeLocalRepartDefs(tmpDir, localDevice string) error {
	entries, err := i.readDir("/usr/lib/repart.d")
	if err != nil {
		return fmt.Errorf("reading /usr/lib/repart.d: %w", err)
	}
	for _, e := range entries {
		src := filepath.Join("/usr/lib/repart.d", e.Name())
		dst := filepath.Join(tmpDir, e.Name())
		if e.Name() == "20-root-a.conf" {
			content := "[Partition]\n" +
				"Type=root\n" +
				"Label=bluefin-server-root-a\n" +
				"CopyBlocks=" + localDevice + "\n" +
				"SizeMinBytes=4G\n" +
				"SizeMaxBytes=16G\n"
			if err := i.writeFile(dst, []byte(content), 0644); err != nil {
				return fmt.Errorf("writing 20-root-a.conf: %w", err)
			}
		} else {
			data, err := i.readFile(src)
			if err != nil {
				return fmt.Errorf("reading %s: %w", src, err)
			}
			if err := i.writeFile(dst, data, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", dst, err)
			}
		}
	}
	return nil
}

// detectPartitions finds ESP and root partitions on disk, returning the root's PARTUUID.
func (i *BluefinDDIInstaller) detectPartitions(ctx context.Context, disk string) (espDev, rootDev, rootPartuuid string, err error) {
	// lsblk -rno PATH,PARTTYPE,PARTUUID,PARTLABEL prints one line per partition.
	result, err := i.Runner.Run(ctx, "lsblk", "-rno", "PATH,PARTTYPE,PARTUUID,PARTLABEL", disk)
	if err != nil {
		return "", "", "", fmt.Errorf("lsblk: %w", err)
	}
	const espTypeGUID = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		path, partType, partuuid, label := fields[0], strings.ToLower(fields[1]), fields[2], fields[3]
		if partType == espTypeGUID {
			espDev = path
		}
		if label == "bluefin-server-root-a" {
			rootDev = path
			rootPartuuid = partuuid
		}
	}
	if espDev == "" {
		return "", "", "", fmt.Errorf("ESP partition not found on %s", disk)
	}
	if rootDev == "" {
		return "", "", "", fmt.Errorf("root partition (bluefin-server-root-a) not found on %s", disk)
	}
	return espDev, rootDev, rootPartuuid, nil
}

// provisionUser creates the user account, sets a password, and installs SSH keys.
func (i *BluefinDDIInstaller) provisionUser(ctx context.Context, user model.UserConfig, allKeys []string) error {
	groups := strings.Join(user.Groups, ",")
	if groups == "" {
		groups = "wheel,sudo"
	}
	if _, err := i.Runner.Run(ctx, "useradd",
		"--root", "/mnt/root",
		"--create-home",
		"--groups", groups,
		"--shell", "/bin/bash",
		user.Username,
	); err != nil {
		return fmt.Errorf("useradd: %w", err)
	}

	if user.PasswordHash != "" {
		// chpasswd --root /mnt/root --encrypted reads "username:hash" from stdin.
		input := user.Username + ":" + user.PasswordHash
		if _, err := i.Runner.RunWithInput(ctx, input, "chpasswd",
			"--root", "/mnt/root", "--encrypted"); err != nil {
			i.Logger.Warn("chpasswd failed (non-fatal — password not set)", "error", err)
		}
	}

	if len(allKeys) > 0 {
		sshDir := filepath.Join("/mnt/root/home", user.Username, ".ssh")
		if err := i.mkdirAll(sshDir, 0700); err != nil {
			return fmt.Errorf("mkdir .ssh: %w", err)
		}
		authKeysPath := filepath.Join(sshDir, "authorized_keys")
		content := strings.Join(allKeys, "\n") + "\n"
		if err := i.writeFile(authKeysPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("writing authorized_keys: %w", err)
		}
		// Fix ownership — useradd --root doesn't chown the .ssh dir for us.
		if _, err := i.Runner.Run(ctx, "chown", "-R",
			user.Username+":"+user.Username, sshDir); err != nil {
			i.Logger.Warn("chown .ssh failed (non-fatal)", "error", err)
		}
	}
	return nil
}

// writeBootEntry writes a systemd-boot Type 1 loader entry referencing the
// root partition by PARTUUID, which is stable regardless of device enumeration
// order (safe for both QEMU and bare metal).
func (i *BluefinDDIInstaller) writeBootEntry(rootPartuuid string) error {
	entriesDir := "/mnt/esp/loader/entries"
	if err := i.mkdirAll(entriesDir, 0755); err != nil {
		return fmt.Errorf("mkdir loader/entries: %w", err)
	}

	// Find the installed kernel version from the mounted rootfs.
	kver := ""
	modules, _ := i.readDir("/mnt/root/usr/lib/modules")
	if len(modules) > 0 {
		kver = modules[0].Name()
	}

	// Copy vmlinuz to ESP so systemd-boot can find it.
	vmlinuzSrc := "/mnt/root/boot/vmlinuz"
	vmlinuzDst := ""
	if kver != "" {
		vmlinuzDst = "/mnt/esp/vmlinuz-" + kver
	} else {
		vmlinuzDst = "/mnt/esp/vmlinuz"
	}
	if data, err := i.readFile(vmlinuzSrc); err == nil {
		_ = i.writeFile(vmlinuzDst, data, 0644)
	}

	// Entry title on ESP uses the basename.
	vmlinuzEntry := "/" + filepath.Base(vmlinuzDst)
	rootOpt := "root=PARTUUID=" + rootPartuuid
	if rootPartuuid == "" {
		// Fallback: not ideal but prevents a blank entry.
		rootOpt = "root=/dev/vda2"
		i.Logger.Warn("root PARTUUID empty — using fallback root=/dev/vda2")
	}

	entry := fmt.Sprintf("title   Bluefin Server\nlinux   %s\noptions %s console=tty0 console=ttyS0,115200 rw\n",
		vmlinuzEntry, rootOpt)
	entryPath := filepath.Join(entriesDir, "bluefin-server.conf")
	return i.writeFile(entryPath, []byte(entry), 0644)
}

// wizard_mergeKeys deduplicates two key slices, first-seen order preserved.
// ponytail: local helper to avoid an import cycle with the wizard package.
func wizard_mergeKeys(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, k := range append(a, b...) {
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}
