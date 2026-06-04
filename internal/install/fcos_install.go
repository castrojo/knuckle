package install

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/projectbluefin/knuckle/internal/ignition"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// FCOSInstaller runs coreos-installer via the runner.
type FCOSInstaller struct {
	Runner    runner.Runner
	Generator *ignition.Generator
	Logger    *slog.Logger
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
// 1. Generate FCOS Butane → compile Ignition JSON
// 2. Write ignition to secure temp file
// 3. Run: coreos-installer install --stream <stream> [--ignition-file <path>] <disk>
func (i *FCOSInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	if cfg.Version != "" {
		i.Logger.Warn("version pinning is not supported for FCOS; coreos-installer uses stream latest", "version", cfg.Version)
	}

	var ignitionPath string

	if cfg.IgnitionURL != "" {
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
		ignPath, err := writeIgnitionFile(i.Logger, ignitionJSON)
		if err != nil {
			return fmt.Errorf("writing ignition file: %w", err)
		}
		ignitionPath = ignPath
		defer removeIgnitionTempFile(i.Logger, ignitionPath)
	}

	args := buildFCOSInstallArgs(cfg, ignitionPath)

	progress("Running coreos-installer...")
	i.Logger.Info("executing coreos-installer", "args", args)

	result, err := i.Runner.Run(ctx, "coreos-installer", args...)
	if err != nil || (result != nil && result.ExitCode != 0) {
		return formatCommandError("coreos-installer failed", result, err)
	}

	progress("Installation complete!")
	return nil
}

func buildFCOSInstallArgs(cfg *model.InstallConfig, ignitionPath string) []string {
	diskPath := installDiskPath(cfg)
	args := []string{
		"install",
		"--stream", cfg.Channel,
		diskPath,
	}

	if cfg.IgnitionURL != "" {
		args = append(args, "--ignition-url", cfg.IgnitionURL)
	} else if ignitionPath != "" {
		args = append(args, "--ignition-file", ignitionPath)
	}

	return args
}
