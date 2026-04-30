// Package install orchestrates the flatcar-install command through the runner abstraction.
package install

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/castrojo/knuckle/internal/ignition"
	"github.com/castrojo/knuckle/internal/model"
	"github.com/castrojo/knuckle/internal/runner"
)

// Installer orchestrates the Flatcar installation process.
type Installer interface {
	Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error
}

// FlatcarInstaller runs flatcar-install via the runner.
type FlatcarInstaller struct {
	Runner    runner.Runner
	Generator *ignition.Generator
	Logger    *slog.Logger
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
// 2. Compile Butane → Ignition JSON via butane CLI
// 3. Run flatcar-install with the target disk, channel, and ignition config
func (i *FlatcarInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	var ignitionJSON string

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

		// Compile to Ignition JSON via butane CLI
		progress("Compiling Ignition config...")
		result, err := i.Runner.RunWithInput(ctx, butaneYAML, "butane", "--strict")
		if err != nil {
			return fmt.Errorf("compiling butane: %w", err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("butane compilation failed: %s", result.Stderr)
		}
		ignitionJSON = result.Stdout

		// Write ignition JSON to temp file for flatcar-install
		progress("Writing Ignition config...")
		if err := i.WriteIgnitionFile(ctx, ignitionJSON); err != nil {
			return fmt.Errorf("writing ignition file: %w", err)
		}
		// Clean up temp file after install (contains SSH keys)
		defer i.cleanupIgnitionFile(ctx)
	}

	// Build flatcar-install command args
	args := buildInstallArgs(cfg, ignitionJSON)

	// Run flatcar-install
	progress("Running flatcar-install...")
	i.Logger.Info("executing flatcar-install", "args", args)

	result, err := i.Runner.Run(ctx, "flatcar-install", args...)
	if err != nil {
		return fmt.Errorf("flatcar-install: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("flatcar-install failed (exit %d): %s", result.ExitCode, result.Stderr)
	}

	progress("Installation complete!")
	return nil
}

func buildInstallArgs(cfg *model.InstallConfig, ignitionJSON string) []string {
	args := []string{
		"-d", cfg.Disk.DevPath,
		"-C", cfg.Channel,
	}

	if cfg.IgnitionURL != "" {
		args = append(args, "-I", cfg.IgnitionURL)
	} else if ignitionJSON != "" {
		args = append(args, "-i", "/tmp/knuckle-ignition.json")
	}

	return args
}

// WriteIgnitionFile writes the Ignition JSON to a temp file for flatcar-install.
// In dry-run mode this is a no-op handled by the runner.
func (i *FlatcarInstaller) WriteIgnitionFile(ctx context.Context, ignitionJSON string) error {
	_, err := i.Runner.RunWithInput(ctx, ignitionJSON, "tee", "/tmp/knuckle-ignition.json")
	return err
}

// cleanupIgnitionFile removes the temp ignition file (contains SSH keys).
func (i *FlatcarInstaller) cleanupIgnitionFile(ctx context.Context) {
	_, err := i.Runner.Run(ctx, "rm", "-f", "/tmp/knuckle-ignition.json")
	if err != nil {
		i.Logger.Warn("failed to clean up ignition file", "error", err)
	}
}
