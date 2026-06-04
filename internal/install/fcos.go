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

// FCOSInstaller runs coreos-installer for Fedora CoreOS installations.
type FCOSInstaller struct {
	Runner       runner.Runner
	Generator    *ignition.Generator
	Logger       *slog.Logger
	ignitionPath string
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
// 1. Generate FCOS Butane YAML (or use external IgnitionURL)
// 2. Compile Butane → Ignition JSON
// 3. Run coreos-installer install with --stream, --ignition-file, and target disk
func (i *FCOSInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	if cfg.Version != "" {
		i.Logger.Warn("version pinning is not supported for FCOS; using stream latest", "version", cfg.Version)
	}

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

// WriteIgnitionFile writes the Ignition JSON to a secure temp file.
func (i *FCOSInstaller) WriteIgnitionFile(ignitionJSON string) (string, error) {
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

func (i *FCOSInstaller) cleanupIgnitionFile() {
	if i.ignitionPath == "" {
		return
	}
	if err := removeIgnitionFile(i.ignitionPath); err != nil && !os.IsNotExist(err) {
		i.Logger.Warn("failed to clean up ignition file", "path", i.ignitionPath, "error", err)
	}
	i.ignitionPath = ""
}
