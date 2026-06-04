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
// Unlike FlatcarInstaller, coreos-installer handles disk preparation internally —
// no wipefs or sfdisk calls are needed.
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
// 1. Generate FCOS Butane → compile Ignition (uses GenerateFCOSButane)
// 2. Write ignition to secure temp file
// 3. Run: coreos-installer install --stream <stream> --ignition-file <path> <disk>
func (i *FCOSInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	if cfg.Version != "" {
		i.Logger.Warn("FCOS version pinning via cfg.Version is not supported; coreos-installer uses stream default",
			"version", cfg.Version)
	}

	if cfg.IgnitionURL != "" {
		// External ignition URL mode — pass directly to coreos-installer
		progress("Using external Ignition config...")
	} else {
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
		ignPath, err := i.WriteIgnitionFile(ignitionJSON)
		if err != nil {
			return fmt.Errorf("writing ignition file: %w", err)
		}
		i.ignitionPath = ignPath
		defer i.cleanupIgnitionFile()
	}

	args := buildFCOSInstallArgs(cfg, i.ignitionPath)

	progress("Running coreos-installer...")
	i.Logger.Info("executing coreos-installer", "args", args)

	result, err := i.Runner.Run(ctx, "coreos-installer", args...)
	if err != nil || (result != nil && result.ExitCode != 0) {
		return formatCommandError("coreos-installer failed", result, err)
	}

	progress("Installation complete!")
	return nil
}

// buildFCOSInstallArgs constructs the coreos-installer argument list.
// Disk path is positional (after subcommand); stream and ignition flags are named.
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

	// disk path is the positional argument — must come last
	args = append(args, diskPath)
	return args
}

// WriteIgnitionFile writes the Ignition JSON to a secure temp file.
func (i *FCOSInstaller) WriteIgnitionFile(ignitionJSON string) (string, error) {
	return writeIgnitionFileToTemp(i.Logger, ignitionJSON)
}

// cleanupIgnitionFile removes the temp ignition file (contains SSH keys).
func (i *FCOSInstaller) cleanupIgnitionFile() {
	if i.ignitionPath == "" {
		return
	}
	if err := removeIgnitionFile(i.ignitionPath); err != nil && !os.IsNotExist(err) {
		i.Logger.Warn("failed to clean up ignition file", "path", i.ignitionPath, "error", err)
	}
	i.ignitionPath = ""
}
