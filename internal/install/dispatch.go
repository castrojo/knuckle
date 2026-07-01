package install

import (
	"context"
	"fmt"

	"github.com/projectbluefin/knuckle/internal/model"
)

// DispatchingInstaller delegates to FlatcarInstaller, FCOSInstaller, or
// BluefinDDIInstaller based on cfg.OS at Install() call time.
type DispatchingInstaller struct {
	Flatcar    Installer
	FCOS       Installer
	BluefinDDI Installer
}

func (d *DispatchingInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	switch cfg.OS {
	case model.OSFCOS:
		if d.FCOS == nil {
			if cfg.DryRun {
				return nil
			}
			return fmt.Errorf("fcos installer not configured")
		}
		return d.FCOS.Install(ctx, cfg, progress)
	case model.OSBluefinDDI:
		if d.BluefinDDI == nil {
			if cfg.DryRun {
				return nil
			}
			return fmt.Errorf("bluefin-ddi installer not configured")
		}
		return d.BluefinDDI.Install(ctx, cfg, progress)
	case model.OSFlatcar, "":
		if d.Flatcar == nil {
			return fmt.Errorf("flatcar installer not configured")
		}
		return d.Flatcar.Install(ctx, cfg, progress)
	default:
		return fmt.Errorf("unsupported OS %q", cfg.OS)
	}
}
