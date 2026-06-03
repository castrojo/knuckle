package bakery

import (
	"context"
	"fmt"

	"github.com/projectbluefin/knuckle/internal/model"
)

// DispatchingClient delegates to the appropriate bakery Client based on OS.
// The wizard calls FetchCatalogForOS with the current cfg.OS so that dispatch
// happens at request time rather than at construction time.
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

// FetchCatalogForOS fetches the sysext catalog for the given OS and architecture.
// For Flatcar, it delegates to the Flatcar client. For FCOS, it delegates to the
// FCOS client (once implemented). This is the method the wizard should call at
// StepSysext with the current cfg.OS.
func (d *DispatchingClient) FetchCatalogForOS(ctx context.Context, arch, os string) ([]model.SysextEntry, error) {
	switch os {
	case model.OSFCOS:
		if d.FCOS == nil {
			return nil, fmt.Errorf("FCOS bakery client not configured")
		}
		return d.FCOS.FetchCatalogArch(ctx, arch)
	case model.OSFlatcar, "":
		if d.Flatcar == nil {
			return nil, fmt.Errorf("Flatcar bakery client not configured")
		}
		return d.Flatcar.FetchCatalogArch(ctx, arch)
	default:
		return nil, fmt.Errorf("unsupported OS %q", os)
	}
}
