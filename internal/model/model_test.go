package model

import "testing"

func TestWizardStepString(t *testing.T) {
	tests := []struct {
		step WizardStep
		want string
	}{
		{StepWelcome, "Welcome"},
		{StepNetwork, "Network"},
		{StepStorage, "Storage"},
		{StepUser, "User"},
		{StepSysext, "Sysext"},
		{StepReview, "Review"},
		{StepInstall, "Install"},
		{StepDone, "Done"},
		{WizardStep(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.step.String(); got != tt.want {
				t.Errorf("WizardStep(%d).String() = %q, want %q", tt.step, got, tt.want)
			}
		})
	}
}

func TestNetworkModeString(t *testing.T) {
	tests := []struct {
		mode NetworkMode
		want string
	}{
		{NetworkDHCP, "dhcp"},
		{NetworkStatic, "static"},
		{NetworkMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("NetworkMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestInstallConfigInit(t *testing.T) {
	cfg := InstallConfig{
		Channel:  "stable",
		Hostname: "node01",
		Network: NetworkConfig{
			Mode:      NetworkDHCP,
			Interface: "eth0",
		},
		Disk: DiskInfo{
			Path:      "/dev/disk/by-id/scsi-SATA_VBOX_HARDDISK_VB12345678-abcdefgh",
			DevPath:   "/dev/sda",
			Model:     "VBOX HARDDISK",
			Serial:    "VB12345678",
			Size:      500_000_000_000,
			SizeHuman: "500 GB",
			Transport: "sata",
			Partitions: []PartitionInfo{
				{Path: "/dev/sda1", Label: "EFI", FSType: "vfat", Size: 512_000_000},
			},
		},
		Users: []UserConfig{
			{
				Username:     "core",
				SSHKeys:      []string{"ssh-ed25519 AAAA... user@host"},
				PasswordHash: "",
				Groups:       []string{"sudo", "docker"},
			},
		},
		SSHKeys: []string{"ssh-ed25519 AAAA... user@host"},
		Sysexts: []SysextEntry{
			{
				Name:        "docker",
				Description: "Docker container runtime",
				Version:     "24.0.7",
				URL:         "https://bakery.flatcar.org/docker-24.0.7.raw",
				Selected:    true,
			},
		},
		IgnitionURL: "",
		DryRun:      true,
	}

	if cfg.Channel != "stable" {
		t.Errorf("Channel = %q, want %q", cfg.Channel, "stable")
	}
	if cfg.Hostname != "node01" {
		t.Errorf("Hostname = %q, want %q", cfg.Hostname, "node01")
	}
	if cfg.Network.Mode != NetworkDHCP {
		t.Errorf("Network.Mode = %v, want %v", cfg.Network.Mode, NetworkDHCP)
	}
	if cfg.Disk.Size != 500_000_000_000 {
		t.Errorf("Disk.Size = %d, want %d", cfg.Disk.Size, uint64(500_000_000_000))
	}
	if len(cfg.Users) != 1 {
		t.Fatalf("len(Users) = %d, want 1", len(cfg.Users))
	}
	if cfg.Users[0].Username != "core" {
		t.Errorf("Users[0].Username = %q, want %q", cfg.Users[0].Username, "core")
	}
	if len(cfg.Disk.Partitions) != 1 {
		t.Fatalf("len(Disk.Partitions) = %d, want 1", len(cfg.Disk.Partitions))
	}
	if cfg.Disk.Partitions[0].Label != "EFI" {
		t.Errorf("Partitions[0].Label = %q, want %q", cfg.Disk.Partitions[0].Label, "EFI")
	}
	if len(cfg.Sysexts) != 1 || !cfg.Sysexts[0].Selected {
		t.Errorf("Sysexts not configured correctly")
	}
	if !cfg.DryRun {
		t.Errorf("DryRun = false, want true")
	}
}
