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

// FCOSInstaller runs coreos-installer via the runner for Fedora CoreOS targets.
// Unlike FlatcarInstaller, it does not run wipefs or sfdisk — coreos-installer
// handles disk preparation internally.
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
// 1. Generate Butane YAML (or use external IgnitionURL)
// 2. Compile Butane → Ignition JSON
// 3. Run coreos-installer with --stream, --ignition-file, and target disk
//
// Unlike Flatcar, FCOS does not require wipefs or sfdisk — coreos-installer
// manages the target device directly.
func (i *FCOSInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	diskPath := installDiskPath(cfg)

	args := []string{"install", "--stream", cfg.Channel}

	if cfg.IgnitionURL != "" {
		progress("Using external Ignition config...")
		args = append(args, "--ignition-url", cfg.IgnitionURL)
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
		ignPath, err := i.writeIgnitionFile(ignitionJSON)
		if err != nil {
			return fmt.Errorf("writing ignition file: %w", err)
		}
		i.ignitionPath = ignPath
		defer i.cleanupIgnitionFile()

		args = append(args, "--ignition-file", ignPath)
	}

	args = append(args, diskPath)

	progress("Running coreos-installer...")
	i.Logger.Info("executing coreos-installer", "args", args)
	result, err := i.Runner.Run(ctx, "coreos-installer", args...)
	if err != nil || (result != nil && result.ExitCode != 0) {
		return formatCommandError("coreos-installer failed", result, err)
	}

	progress("Installation complete!")
	return nil
}

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

func (i *FCOSInstaller) cleanupIgnitionFile() {
	if i.ignitionPath == "" {
		return
	}
	if err := removeIgnitionFile(i.ignitionPath); err != nil && !os.IsNotExist(err) {
		i.Logger.Warn("failed to clean up ignition file", "path", i.ignitionPath, "error", err)
	}
	i.ignitionPath = ""
}

// compile-time assertion: FCOSInstaller must implement Installer.
var _ Installer = (*FCOSInstaller)(nil)
