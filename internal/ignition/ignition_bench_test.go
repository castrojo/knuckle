package ignition

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func BenchmarkGenerateButane(b *testing.B) {
	cfg := &model.InstallConfig{
		Hostname: "flatcar-benchmark",
		Timezone: "America/New_York",
		Channel:  "stable",
		Users: []model.UserConfig{
			{
				Username:     "admin",
				PasswordHash: "$6$rounds=4096$salt$hash",
				Groups:       []string{"sudo", "docker"},
				SSHKeys:      []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server"},
			},
		},
		SSHKeys: []string{
			"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7Z user@host",
		},
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "eth0",
			Address:   "192.168.1.100/24",
			Gateway:   "192.168.1.1",
			DNS:       []string{"8.8.8.8", "1.1.1.1"},
		},
		Sysexts: []model.SysextEntry{
			{Name: "docker", URL: "https://example.com/docker.raw", Selected: true, Sha256: "abc123"},
			{Name: "k3s", URL: "https://example.com/k3s.raw", Selected: true},
		},
		UpdateStrategy: model.UpdateStrategy{
			RebootStrategy: "reboot",
		},
		NvidiaDriverVersion: "570-open",
		Swap: model.SwapConfig{
			Enabled: true,
			SizeMB:  4096,
		},
		Tailscale: model.TailscaleConfig{
			AuthKey: "tskey-auth-k12345678901-abcdefghijklmnopqrstuvwxyz",
			Mode:    model.TailscaleModeExitNode,
		},
	}

	gen := NewGenerator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateButane(cfg)
		if err != nil {
			b.Fatalf("GenerateButane() error: %v", err)
		}
	}
}

func BenchmarkGenerateButaneMinimal(b *testing.B) {
	cfg := &model.InstallConfig{
		Hostname: "minimal",
		Channel:  "stable",
		SSHKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server"},
		Network: model.NetworkConfig{
			Mode: model.NetworkDHCP,
		},
	}

	gen := NewGenerator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateButane(cfg)
		if err != nil {
			b.Fatalf("GenerateButane() error: %v", err)
		}
	}
}

func BenchmarkGenerateButaneSubnetRouter(b *testing.B) {
	cfg := &model.InstallConfig{
		Hostname: "tailscale-router",
		Channel:  "stable",
		SSHKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server"},
		Network: model.NetworkConfig{
			Mode: model.NetworkDHCP,
		},
		Tailscale: model.TailscaleConfig{
			AuthKey: "tskey-auth-k12345678901-abcdefghijklmnopqrstuvwxyz",
			Mode:    model.TailscaleModeSubnetRouter,
			Routes:  "10.0.0.0/24,192.168.1.0/24,172.16.0.0/16",
		},
	}

	gen := NewGenerator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateButane(cfg)
		if err != nil {
			b.Fatalf("GenerateButane() error: %v", err)
		}
	}
}

func BenchmarkFilterSelected(b *testing.B) {
	sysexts := []model.SysextEntry{
		{Name: "docker", Selected: true},
		{Name: "k3s", Selected: false},
		{Name: "postgres", Selected: true},
		{Name: "redis", Selected: false},
		{Name: "tailscale", Selected: true},
		{Name: "nvidia", Selected: false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filterSelected(sysexts)
	}
}

func BenchmarkCompileToIgnition(b *testing.B) {
	butaneYAML := `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      mode: 0644
      overwrite: true
      contents:
        inline: "flatcar-bench"
    - path: /etc/flatcar/update.conf
      mode: 0644
      overwrite: true
      contents:
        inline: |
          REBOOT_STRATEGY=reboot
          GROUP=stable
systemd:
  units:
    - name: update-engine.service
      enabled: true
passwd:
  users:
    - name: "core"
      ssh_authorized_keys:
        - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server"
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CompileToIgnition(butaneYAML)
		if err != nil {
			b.Fatalf("CompileToIgnition() error: %v", err)
		}
	}
}
