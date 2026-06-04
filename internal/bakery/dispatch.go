package bakery

import (
	"context"
	"fmt"

	"github.com/projectbluefin/knuckle/internal/model"
)

// DispatchingClient delegates to an OS-specific bakery Client based on
// the os parameter passed to FetchCatalogForOS. The wizard calls this
// method with the current cfg.OS at StepSysext time, when the user's
// OS choice is known.
type DispatchingClient struct {
	Flatcar Client
	FCOS    Client
}

func (d *DispatchingClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return d.Flatcar.FetchCatalog(ctx)
}

func (d *DispatchingClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	return d.Flatcar.FetchCatalogArch(ctx, arch)
}

// FetchCatalogFCOS delegates to the FCOS client's FetchCatalogFCOS.
func (d *DispatchingClient) FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error) {
	if d.FCOS == nil {
		return nil, fmt.Errorf("fcos bakery client not configured")
	}
	return d.FCOS.FetchCatalogFCOS(ctx, arch, fedoraVersion)
}

// FetchCatalogForOS fetches the sysext catalog for the given OS and architecture.
func (d *DispatchingClient) FetchCatalogForOS(ctx context.Context, arch, os string) ([]model.SysextEntry, error) {
	switch os {
	case model.OSFCOS:
		if d.FCOS == nil {
			return nil, fmt.Errorf("fcos bakery client not configured")
		}
		return d.FCOS.FetchCatalogArch(ctx, arch)
	case model.OSFlatcar, "":
		return d.Flatcar.FetchCatalogArch(ctx, arch)
	default:
		return nil, fmt.Errorf("unsupported OS %q for sysext catalog", os)
	}
}
