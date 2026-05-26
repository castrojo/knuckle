package tui

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

func BenchmarkReviewSummary(b *testing.B) {
	m := &Model{
		Wizard: &wizard.Wizard{
			State: &wizard.State{
				Config: model.InstallConfig{
					Hostname: "flatcar-benchmark",
					Channel:  "stable",
					Version:  "3815.2.0",
					Disk: model.DiskInfo{
						DevPath:   "/dev/sda",
						Model:     "Samsung SSD 980 PRO",
						SizeHuman: "1.0 TB",
					},
					Network: model.NetworkConfig{
						Mode:    model.NetworkStatic,
						Address: "192.168.1.100/24",
						Gateway: "192.168.1.1",
					},
					Users: []model.UserConfig{
						{Username: "admin"},
					},
					SSHKeys: []string{
						"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server",
						"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7Z user@host",
					},
					Sysexts: []model.SysextEntry{
						{Name: "docker", Selected: true},
						{Name: "k3s", Selected: true},
						{Name: "tailscale", Selected: true},
					},
					Swap: model.SwapConfig{
						Enabled: true,
						SizeMB:  4096,
					},
					Tailscale: model.TailscaleConfig{
						AuthKey: "tskey-auth-k12345678901-abcdefghijklmnopqrstuvwxyz",
						Mode:    model.TailscaleModeSubnetRouter,
						Routes:  "10.0.0.0/24,192.168.1.0/24",
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.reviewSummary()
	}
}

func BenchmarkRenderZenChrome(b *testing.B) {
	m := &Model{
		Wizard: &wizard.Wizard{
			State: &wizard.State{
				CurrentStep: model.StepNetwork,
				Config: model.InstallConfig{
					Channel: "stable",
				},
				Channels: []bakery.ChannelInfo{
					{
						Channel: "stable",
						Version: "3815.2.0",
						Kernel:  "6.6.21",
						Systemd: "255.4",
						Docker:  "26.0.0",
					},
				},
				SystemChecks: []wizard.SystemCheck{
					{Name: "disk", Status: "ok"},
					{Name: "network", Status: "ok"},
					{Name: "memory", Status: "warn"},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderZenChrome()
	}
}

func BenchmarkGetChannelMeta(b *testing.B) {
	m := &Model{
		Wizard: &wizard.Wizard{
			State: &wizard.State{
				Channels: []bakery.ChannelInfo{
					{Channel: "stable", Version: "3815.2.0", Kernel: "6.6.21", Systemd: "255.4", Docker: "26.0.0"},
					{Channel: "lts", Version: "3760.2.1", Kernel: "6.1.96", Systemd: "252.22", Docker: "24.0.9"},
					{Channel: "beta", Version: "3850.0.0", Kernel: "6.7.5", Systemd: "255.4", Docker: "26.0.1"},
					{Channel: "alpha", Version: "3900.0.0", Kernel: "6.8.0", Systemd: "256.0", Docker: "26.1.0"},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.getChannelMeta()
	}
}

func BenchmarkViewChannelCards(b *testing.B) {
	m := &Model{
		cursor: 0,
		Wizard: &wizard.Wizard{
			State: &wizard.State{
				Channels: []bakery.ChannelInfo{
					{Channel: "stable", Version: "3815.2.0", Kernel: "6.6.21", Systemd: "255.4", Docker: "26.0.0"},
					{Channel: "lts", Version: "3760.2.1", Kernel: "6.1.96", Systemd: "252.22", Docker: "24.0.9"},
					{Channel: "beta", Version: "3850.0.0", Kernel: "6.7.5", Systemd: "255.4", Docker: "26.0.1"},
					{Channel: "alpha", Version: "3900.0.0", Kernel: "6.8.0", Systemd: "256.0", Docker: "26.1.0"},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.viewChannelCards()
	}
}

func BenchmarkLocalKeysSummary(b *testing.B) {
	m := &Model{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.localKeysSummary()
	}
}
