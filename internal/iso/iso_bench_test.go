package iso

import (
	"testing"
)

func BenchmarkGenerateInstallerIgnition(b *testing.B) {
	sshKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFq admin@server"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateInstallerIgnition("flatcar", sshKey)
		if err != nil {
			b.Fatalf("GenerateInstallerIgnition() error: %v", err)
		}
	}
}

func BenchmarkGenerateInstallerIgnitionNoSSH(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateInstallerIgnition("flatcar", "")
		if err != nil {
			b.Fatalf("GenerateInstallerIgnition() error: %v", err)
		}
	}
}

func BenchmarkGenerateInstallerIgnitionFCOS(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateInstallerIgnition("fcos", "")
		if err != nil {
			b.Fatalf("GenerateInstallerIgnition() error: %v", err)
		}
	}
}
