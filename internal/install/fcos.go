package install

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/projectbluefin/knuckle/internal/ignition"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

// FCOSInstaller runs coreos-installer for Fedora CoreOS targets.
// It shares the Butane→Ignition compilation pipeline with FlatcarInstaller
// but uses coreos-installer instead of flatcar-install and generates
// FCOS-variant Butane via GenerateFCOSButane.
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

// Install performs the FCOS installation. This is a stub that will be fully
// implemented in #639; for now it returns an error indicating FCOS support
// is not yet available.
func (i *FCOSInstaller) Install(_ context.Context, cfg *model.InstallConfig, _ func(string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}
	return fmt.Errorf("FCOS installation is not yet implemented (see #639)")
}
