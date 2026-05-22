// Package demo provides hardcoded mock implementations of the Prober, bakery.Client,
// and Installer interfaces for use with the --demo flag. Demo mode lets the TUI run
// on any machine without hardware, network access, or an actual disk — ideal for
// generating reproducible recordings with VHS or asciinema.
package demo

import (
	"context"
	"time"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// Prober returns hardcoded fake disks and network interfaces.
type Prober struct{}

func (p *Prober) ListDisks(_ context.Context) ([]model.DiskInfo, error) {
	return []model.DiskInfo{
		{
			Path:      "/dev/sda",
			DevPath:   "/dev/sda",
			Model:     "SAMSUNG 870 EVO 1TB",
			Serial:    "S3EWNX0M123456",
			Size:      1_000_204_886_016,
			SizeHuman: "1.0 TB",
		},
		{
			Path:      "/dev/nvme0n1",
			DevPath:   "/dev/nvme0n1",
			Model:     "WD Blue SN570 NVMe 500GB",
			Serial:    "WD-WX22AA123456",
			Size:      500_107_862_016,
			SizeHuman: "500 GB",
		},
	}, nil
}

func (p *Prober) ListNetworkInterfaces(_ context.Context) ([]model.NetworkInterface, error) {
	return []model.NetworkInterface{
		{Name: "enp3s0", MAC: "52:54:00:12:34:56", State: "up"},
		{Name: "eth0", MAC: "52:54:00:ab:cd:ef", State: "up"},
	}, nil
}

// Bakery returns a curated set of popular sysexts instantly, with no network call.
type Bakery struct{}

func (b *Bakery) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return b.FetchCatalogArch(ctx, "amd64")
}

func (b *Bakery) FetchCatalogArch(_ context.Context, _ string) ([]model.SysextEntry, error) {
	return []model.SysextEntry{
		{
			Name:        "docker",
			Description: "Docker container runtime",
			Version:     "28.0.4",
			URL:         "https://github.com/flatcar/sysext-bakery/releases/download/docker-v28.0.4/docker-28.0.4-x86-64.raw",
			Category:    "container",
			SupportTier: bakery.TierIntegrated,
		},
		{
			Name:        "containerd",
			Description: "containerd container runtime",
			Version:     "2.1.5",
			URL:         "https://github.com/flatcar/sysext-bakery/releases/download/containerd-v2.1.5/containerd-2.1.5-x86-64.raw",
			Category:    "container",
			SupportTier: bakery.TierIntegrated,
		},
		{
			Name:        "kubernetes",
			Description: "Kubernetes node components (kubelet, kubeadm, kubectl)",
			Version:     "1.30.0",
			URL:         "https://github.com/flatcar/sysext-bakery/releases/download/kubernetes-v1.30.0/kubernetes-1.30.0-x86-64.raw",
			Category:    "orchestration",
			SupportTier: bakery.TierMaintained,
		},
		{
			Name:        "tailscale",
			Description: "Tailscale mesh VPN",
			Version:     "1.56.1",
			URL:         "https://github.com/flatcar/sysext-bakery/releases/download/tailscale-v1.56.1/tailscale-1.56.1-x86-64.raw",
			Category:    "networking",
			SupportTier: bakery.TierMaintained,
		},
		{
			Name:        "wasmcloud",
			Description: "wasmCloud WebAssembly runtime",
			Version:     "0.82.0",
			URL:         "https://github.com/flatcar/sysext-bakery/releases/download/wasmcloud-v0.82.0/wasmcloud-0.82.0-x86-64.raw",
			Category:    "runtime",
			SupportTier: bakery.TierMaintained,
		},
	}, nil
}

// Installer simulates an installation with realistic progress messages and timing.
type Installer struct{}

func (i *Installer) Install(ctx context.Context, _ *model.InstallConfig, progress func(string)) error {
	steps := []struct {
		msg   string
		delay time.Duration
	}{
		{"Generating Ignition configuration", 200 * time.Millisecond},
		{"Writing Ignition to temp file", 100 * time.Millisecond},
		{"Wiping disk metadata (wipefs)", 300 * time.Millisecond},
		{"Running flatcar-install", 1200 * time.Millisecond},
		{"Repairing GPT (sgdisk)", 200 * time.Millisecond},
		{"Applying Ignition config", 300 * time.Millisecond},
		{"Cleaning up temp files", 100 * time.Millisecond},
	}
	for _, s := range steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.delay):
			progress(s.msg)
		}
	}
	return nil
}

// Channels returns hardcoded channel info so the Welcome screen renders
// version cards without any network call.
func Channels() []bakery.ChannelInfo {
	return []bakery.ChannelInfo{
		{Channel: "stable", Version: "4081.2.0", BuildDate: "2026-05-01", Kernel: "6.12.25", Systemd: "255.13", Docker: "28.0.4", Containerd: "2.0.4"},
		{Channel: "lts", Version: "3815.2.5", BuildDate: "2026-04-15", Kernel: "6.6.87", Systemd: "255.13", Docker: "27.5.1", Containerd: "1.7.25"},
		{Channel: "beta", Version: "4091.0.0", BuildDate: "2026-05-10", Kernel: "6.12.28", Systemd: "256.5", Docker: "28.1.0", Containerd: "2.1.0"},
		{Channel: "alpha", Version: "4099.0.0", BuildDate: "2026-05-18", Kernel: "6.14.3", Systemd: "257.2", Docker: "28.1.1", Containerd: "2.1.2"},
	}
}
