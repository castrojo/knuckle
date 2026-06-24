package install

import (
	"context"
	"fmt"

	"github.com/projectbluefin/knuckle/internal/model"
)

// DispatchingInstaller delegates to FlatcarInstaller or FCOSInstaller
// based on cfg.OS at Install() call time.
type DispatchingInstaller struct {
	Flatcar Installer
	FCOS    Installer
}

func (d *DispatchingInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(step string)) error {
	switch cfg.OS {
	case model.OSFCOS:
		if d.FCOS == nil {
			if cfg.DryRun {
				// FCOS installer not wired yet; dry-run validates config only.
				return nil
			}
			return fmt.Errorf("fcos installer not configured")
		}
		return d.FCOS.Install(ctx, cfg, progress)
	case model.OSFlatcar, "":
		if d.Flatcar == nil {
			return fmt.Errorf("flatcar installer not configured")
		}
		return d.Flatcar.Install(ctx, cfg, progress)
	default:
		return fmt.Errorf("unsupported OS %q", cfg.OS)
	}
}
