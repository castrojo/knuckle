package install

import (
	"context"
	"fmt"

	"github.com/projectbluefin/knuckle/internal/model"
)

// DispatchingInstaller delegates to FlatcarInstaller or a future FCOSInstaller
// based on cfg.OS at Install() call time. This allows construction before the
// user selects an OS (in the wizard or headless config).
type DispatchingInstaller struct {
	Flatcar Installer
	FCOS    Installer
}

// Install delegates to the appropriate installer based on cfg.OS.
func (d *DispatchingInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(string)) error {
	if cfg == nil {
		return fmt.Errorf("install config cannot be nil")
	}

	switch cfg.OS {
	case model.OSFCOS:
		if d.FCOS == nil {
			return fmt.Errorf("FCOS installer not configured")
		}
		return d.FCOS.Install(ctx, cfg, progress)
	default:
		if d.Flatcar == nil {
			return fmt.Errorf("Flatcar installer not configured")
		}
		return d.Flatcar.Install(ctx, cfg, progress)
	}
}
