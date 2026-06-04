package bakery

import (
	"context"
	"fmt"

	"github.com/projectbluefin/knuckle/internal/model"
)

// FCOSCatalogClient extends Client with FCOS-specific Fedora-version filtering.
// FetchCatalogFCOS filters assets by fedoraVersion, which is obtained from the
// FCOS stream metadata (see fcos.FetchStreamFedoraVersion).
type FCOSCatalogClient interface {
	Client
	FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error)
}

// DispatchingClient delegates to an OS-specific bakery Client based on
// the os parameter passed to FetchCatalogForOS. The wizard calls this
// method with the current cfg.OS at StepSysext time, when the user's
// OS choice is known.
type DispatchingClient struct {
	Flatcar Client
	FCOS    FCOSCatalogClient
}

func (d *DispatchingClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return d.Flatcar.FetchCatalog(ctx)
}

func (d *DispatchingClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	return d.Flatcar.FetchCatalogArch(ctx, arch)
}

// FetchCatalogForOS fetches the sysext catalog for the given OS and architecture.
// For FCOS, fedoraVersion is required to filter assets by Fedora major version;
// obtain it via fcos.FetchStreamFedoraVersion before calling this method.
// For Flatcar, fedoraVersion is unused.
func (d *DispatchingClient) FetchCatalogForOS(ctx context.Context, arch, os string, fedoraVersion int) ([]model.SysextEntry, error) {
	switch os {
	case model.OSFCOS:
		if d.FCOS == nil {
			return nil, fmt.Errorf("fcos bakery client not configured")
		}
		if fedoraVersion <= 0 {
			return nil, fmt.Errorf("invalid FCOS Fedora version %d: stream version must be resolved before fetching the FCOS catalog", fedoraVersion)
		}
		return d.FCOS.FetchCatalogFCOS(ctx, arch, fedoraVersion)
	case model.OSFlatcar, "":
		return d.Flatcar.FetchCatalogArch(ctx, arch)
	default:
		return nil, fmt.Errorf("unsupported OS %q for sysext catalog", os)
	}
}
