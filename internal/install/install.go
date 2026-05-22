// Package install orchestrates the flatcar-install command through the runner abstraction.
package install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/projectbluefin/knuckle/internal/ignition"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// Installer orchestrates the Flatcar installation process.
type Installer interface {
	Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error
}

// FlatcarInstaller runs flatcar-install via the runner.
type FlatcarInstaller struct {
	Runner       runner.Runner
	Generator    *ignition.Generator
	Logger       *slog.Logger
	ignitionPath string // dynamically set temp file path
}

// NewFlatcarInstaller creates a FlatcarInstaller with the given runner and logger.
func NewFlatcarInstaller(r runner.Runner, logger *slog.Logger) *FlatcarInstaller {
	return &FlatcarInstaller{
		Runner:    r,
		Generator: ignition.NewGenerator(),
		Logger:    logger,
	}
}

// Install performs the Flatcar installation:
// 1. Generate Butane YAML (or use external IgnitionURL)
// 2. Compile Butane → Ignition JSON via coreos/butane Go library
// 3. Wipe stale filesystem/partition signatures from the target disk
// 4. Run flatcar-install with the target disk, channel, and ignition config
// 5. Relocate the backup GPT header to the end of the target disk
func (i *FlatcarInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	if cfg.IgnitionURL != "" {
		// External ignition URL mode — pass directly to flatcar-install
		progress("Using external Ignition config...")
	} else {
		// Generate Butane YAML
		progress("Generating Butane config...")
		butaneYAML, err := i.Generator.GenerateButane(cfg)
		if err != nil {
			return fmt.Errorf("generating butane config: %w", err)
		}

		// Compile to Ignition JSON via coreos/butane Go library
		// (butane CLI is not available on Flatcar Container Linux)
		progress("Compiling Ignition config...")
		ignitionJSON, err := ignition.CompileToIgnition(butaneYAML)
		if err != nil {
			return fmt.Errorf("compiling butane: %w", err)
		}

		// Write ignition JSON to secure temp file for flatcar-install
		progress("Writing Ignition config...")
		ignPath, err := i.WriteIgnitionFile(ignitionJSON)
		if err != nil {
			return fmt.Errorf("writing ignition file: %w", err)
		}
		i.ignitionPath = ignPath
		// Clean up temp file after install (contains SSH keys)
		defer i.cleanupIgnitionFile()
	}

	diskPath := installDiskPath(cfg)
	progress("Wiping target disk signatures...")
	i.Logger.Info("wiping target disk signatures", "disk", diskPath)
	wipeResult, err := i.Runner.Run(ctx, "wipefs", "--all", "--force", diskPath)
	if err != nil || (wipeResult != nil && wipeResult.ExitCode != 0) {
		return formatCommandError("wiping target disk", wipeResult, err)
	}

	// Build flatcar-install command args
	args := buildInstallArgs(cfg, i.ignitionPath)

	// Run flatcar-install
	progress("Running flatcar-install...")
	i.Logger.Info("executing flatcar-install", "args", args)

	result, err := i.Runner.Run(ctx, "flatcar-install", args...)
	if err != nil || (result != nil && result.ExitCode != 0) {
		return formatFlatcarInstallError(result, err)
	}

	progress("Repairing GPT backup header...")
	i.Logger.Info("relocating backup gpt header", "disk", diskPath)
	// sfdisk --relocate gpt-bak-std requires util-linux >= 2.29.1 (2017).
	// Flatcar Container Linux ships util-linux 2.41+ — verified on Flatcar 4593.2.1.
	repairResult, err := i.Runner.Run(ctx, "sfdisk", "--relocate", "gpt-bak-std", diskPath)
	if err != nil || (repairResult != nil && repairResult.ExitCode != 0) {
		return formatCommandError("repairing GPT backup header", repairResult, err)
	}

	progress("Installation complete!")
	return nil
}

func formatCommandError(action string, result *runner.Result, err error) error {
	if result != nil {
		stderr := strings.TrimSpace(result.Stderr)
		if stderr != "" {
			return fmt.Errorf("%s (exit %d): %s", action, result.ExitCode, stderr)
		}
		if result.ExitCode != 0 {
			if err != nil {
				return fmt.Errorf("%s (exit %d): %w", action, result.ExitCode, err)
			}
			return fmt.Errorf("%s (exit %d)", action, result.ExitCode)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s failed", action)
}

func formatFlatcarInstallError(result *runner.Result, err error) error {
	return formatCommandError("flatcar-install failed", result, err)
}

func buildInstallArgs(cfg *model.InstallConfig, ignitionPath string) []string {
	diskPath := installDiskPath(cfg)
	args := []string{
		"-d", diskPath,
		"-C", cfg.Channel,
	}

	// Architecture: flatcar-install auto-detects from the running system's uname -m
	// (the ISO binary is compiled for the target arch, so uname -m will be correct).
	// No -B flag is needed; the installer derives the correct image URL automatically.

	if cfg.Version != "" {
		args = append(args, "-V", cfg.Version)
	}

	if cfg.IgnitionURL != "" {
		args = append(args, "-I", cfg.IgnitionURL)
	} else if ignitionPath != "" {
		args = append(args, "-i", ignitionPath)
	}

	return args
}

func installDiskPath(cfg *model.InstallConfig) string {
	// Prefer /dev/disk/by-id path for stable identification
	if cfg.Disk.Path != "" {
		return cfg.Disk.Path
	}
	return cfg.Disk.DevPath
}

// WriteIgnitionFile writes the Ignition JSON to a secure temp file.
// Go 1.16+ os.CreateTemp uses O_RDWR|O_CREATE|O_EXCL with 0600 permissions atomically —
// the file is never readable by other users, even between creation and content write.
// Returns the path to the created temp file.
func (i *FlatcarInstaller) WriteIgnitionFile(ignitionJSON string) (string, error) {
	f, err := os.CreateTemp("", "knuckle-ignition-*.json")
	if err != nil {
		return "", fmt.Errorf("creating temp ignition file: %w", err)
	}
	path := f.Name()

	if _, err := f.WriteString(ignitionJSON); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("writing ignition content: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("closing ignition file: %w", err)
	}

	i.Logger.Info("ignition file written", "path", path)
	return path, nil
}

// cleanupIgnitionFile removes the temp ignition file (contains SSH keys).
func (i *FlatcarInstaller) cleanupIgnitionFile() {
	if i.ignitionPath == "" {
		return
	}
	if err := os.Remove(i.ignitionPath); err != nil && !os.IsNotExist(err) {
		i.Logger.Warn("failed to clean up ignition file", "path", i.ignitionPath, "error", err)
	}
	i.ignitionPath = ""
}
