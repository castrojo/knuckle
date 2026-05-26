package validate

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func BenchmarkHostname(b *testing.B) {
	validHostnames := []string{
		"flatcar-node01",
		"web-server-prod",
		"k8s-master-1",
		"db-replica-03",
		"edge-router-lab",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, h := range validHostnames {
			_ = Hostname(h)
		}
	}
}

func BenchmarkIPAddress(b *testing.B) {
	validIPs := []string{
		"192.168.1.1",
		"10.0.0.254",
		"172.16.0.1",
		"8.8.8.8",
		"1.1.1.1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ip := range validIPs {
			_ = IPAddress(ip)
		}
	}
}

func BenchmarkCIDR(b *testing.B) {
	validCIDRs := []string{
		"192.168.1.100/24",
		"10.0.0.50/16",
		"172.16.5.10/22",
		"203.0.113.42/32",
		"198.51.100.0/24",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cidr := range validCIDRs {
			_ = CIDR(cidr)
		}
	}
}

func BenchmarkSSHPublicKey(b *testing.B) {
	validKeys := []string{
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7Z user@host",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server",
		"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY dev@workstation",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range validKeys {
			_ = SSHPublicKey(key)
		}
	}
}

func BenchmarkUsername(b *testing.B) {
	validUsernames := []string{
		"admin",
		"deploy_user",
		"app-service",
		"backup_01",
		"_system",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, u := range validUsernames {
			_ = Username(u)
		}
	}
}

func BenchmarkDiskPath(b *testing.B) {
	validPaths := []string{
		"/dev/sda",
		"/dev/nvme0n1",
		"/dev/disk/by-id/ata-Samsung_SSD",
		"/dev/vda",
		"/dev/xvda",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range validPaths {
			_ = DiskPath(path)
		}
	}
}

func BenchmarkChannel(b *testing.B) {
	channels := []string{"stable", "beta", "alpha", "lts", "edge"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ch := range channels {
			_ = Channel(ch)
		}
	}
}

func BenchmarkGitHubUsername(b *testing.B) {
	validUsernames := []string{
		"torvalds",
		"github",
		"microsoft",
		"kubernetes",
		"projectbluefin",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, u := range validUsernames {
			_ = GitHubUsername(u)
		}
	}
}

func BenchmarkTailscaleAuthKey(b *testing.B) {
	validKeys := []string{
		"tskey-auth-k12345678901-abcdefghijklmnopqrstuvwxyz",
		"tskey-client-k98765432109-1234567890abcdefghijklmnop",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range validKeys {
			_ = TailscaleAuthKey(key)
		}
	}
}

func BenchmarkCheckConsistency(b *testing.B) {
	cfg := &model.InstallConfig{
		Hostname: "flatcar-test",
		Channel:  "stable",
		Disk: model.DiskInfo{
			DevPath: "/dev/sda",
		},
		Network: model.NetworkConfig{
			Mode: model.NetworkDHCP,
		},
		SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CheckConsistency(cfg)
	}
}

func BenchmarkCheckConsistencyStaticNetwork(b *testing.B) {
	cfg := &model.InstallConfig{
		Hostname: "flatcar-static",
		Channel:  "stable",
		Disk: model.DiskInfo{
			DevPath: "/dev/sda",
		},
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "eth0",
			Address:   "192.168.1.100/24",
			Gateway:   "192.168.1.1",
			DNS:       []string{"8.8.8.8", "1.1.1.1"},
		},
		Users: []model.UserConfig{
			{Username: "admin", PasswordHash: "$6$rounds=4096$salt$hash"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CheckConsistency(cfg)
	}
}
