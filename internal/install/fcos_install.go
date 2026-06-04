package install

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/projectbluefin/knuckle/internal/ignition"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// FCOSInstaller runs coreos-installer via the runner.
//
// Key differences from FlatcarInstaller:
//   - Uses "coreos-installer install <disk>" (subcommand + positional arg)
//   - Uses --stream and --ignition-file flags (not -C / -i)
//   - Does NOT call wipefs or sfdisk — coreos-installer handles disk prep internally
//
// Version pinning: coreos-installer has no -V equivalent for stream-based pinning.
// If cfg.Version is set for an FCOS install, it is logged as a warning and ignored.
// See docs/HEADLESS-CONFIG.md for details.
type FCOSInstaller struct {
	Runner       runner.Runner
	Generator    *ignition.Generator
	Logger       *slog.Logger
	ignitionPath string // dynamically set temp file path
}

// NewFCOSInstaller creates an FCOSInstaller with the given runner and logger.
func NewFCOSInstaller(r runner.Runner, logger *slog.Logger) *FCOSInstaller {
	return &FCOSInstaller{
		Runner:    r,
		Generator: ignition.NewGenerator(),
		Logger:    logger,
	}
}

// Install performs the FCOS installation:
//  1. Generate FCOS Butane YAML → compile to Ignition JSON (or use external IgnitionURL)
//  2. Write Ignition JSON to a secure temp file
//  3. Run: coreos-installer install --stream <stream> --ignition-file <path> <disk>
//  4. Done — no wipefs or sfdisk steps needed
func (i *FCOSInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	if cfg.Version != "" {
		i.Logger.Warn("version pinning is not supported for FCOS installs — ignoring cfg.Version",
			"version", cfg.Version)
	}

	if cfg.IgnitionURL != "" {
		// External ignition URL mode — pass directly to coreos-installer.
		progress("Using external Ignition config...")
	} else {
		// Generate FCOS Butane YAML and compile to Ignition JSON.
		progress("Generating Butane config...")
		butaneYAML, err := i.Generator.GenerateFCOSButane(cfg)
		if err != nil {
			return fmt.Errorf("generating butane config: %w", err)
		}

		progress("Compiling Ignition config...")
		ignitionJSON, err := compileToIgnitionFunc(butaneYAML)
		if err != nil {
			return fmt.Errorf("compiling butane: %w", err)
		}

		progress("Writing Ignition config...")
		ignPath, err := i.writeIgnitionFile(ignitionJSON)
		if err != nil {
			return fmt.Errorf("writing ignition file: %w", err)
		}
		i.ignitionPath = ignPath
		defer i.cleanupFCOSIgnitionFile()
	}

	args := buildFCOSInstallArgs(cfg, i.ignitionPath)

	progress("Running coreos-installer...")
	i.Logger.Info("executing coreos-installer", "args", args)

	result, err := i.Runner.Run(ctx, "coreos-installer", args...)
	if err != nil || (result != nil && result.ExitCode != 0) {
		return formatCommandError("coreos-installer install failed", result, err)
	}

	progress("Installation complete!")
	return nil
}

// buildFCOSInstallArgs constructs the coreos-installer CLI arguments.
//
// Format: coreos-installer install --stream <stream> [--ignition-file <path>|--ignition-url <url>] <disk>
//
// The disk path is the final positional argument, unlike flatcar-install which uses -d <disk>.
// Version pinning is not supported via stream — callers must handle that before calling Install.
func buildFCOSInstallArgs(cfg *model.InstallConfig, ignitionPath string) []string {
	diskPath := installDiskPath(cfg)
	args := []string{
		"install",
		"--stream", cfg.Channel,
	}

	if cfg.IgnitionURL != "" {
		args = append(args, "--ignition-url", cfg.IgnitionURL)
	} else if ignitionPath != "" {
		args = append(args, "--ignition-file", ignitionPath)
	}

	// Disk is the final positional argument for coreos-installer.
	args = append(args, diskPath)

	return args
}

// writeIgnitionFile writes Ignition JSON to a secure temp file and returns its path.
// Reuses the package-level newIgnitionTempFile and removeIgnitionFile seams so that
// write-error injection tests cover both FlatcarInstaller and FCOSInstaller paths.
func (i *FCOSInstaller) writeIgnitionFile(ignitionJSON string) (string, error) {
	f, err := newIgnitionTempFile()
	if err != nil {
		return "", fmt.Errorf("creating temp ignition file: %w", err)
	}
	path := f.Name()

	if _, err := f.WriteString(ignitionJSON); err != nil {
		_ = f.Close()
		_ = removeIgnitionFile(path)
		return "", fmt.Errorf("writing ignition content: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = removeIgnitionFile(path)
		return "", fmt.Errorf("closing ignition file: %w", err)
	}

	i.Logger.Info("ignition file written", "path", path)
	return path, nil
}

// cleanupFCOSIgnitionFile removes the temp ignition file (contains SSH keys).
func (i *FCOSInstaller) cleanupFCOSIgnitionFile() {
	if i.ignitionPath == "" {
		return
	}
	if err := removeIgnitionFile(i.ignitionPath); err != nil && !os.IsNotExist(err) {
		i.Logger.Warn("failed to clean up ignition file", "path", i.ignitionPath, "error", err)
	}
	i.ignitionPath = ""
}
