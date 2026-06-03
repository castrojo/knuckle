package validate

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestCheckConsistency_FCOS_RejectNvidiaDriver(t *testing.T) {
	cfg := &model.InstallConfig{
		OS:                  model.OSFCOS,
		Channel:             "stable",
		Hostname:            "fcos-node",
		NvidiaDriverVersion: "570-open",
		Disk:                model.DiskInfo{DevPath: "/dev/vda"},
		Users:               []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
		SSHKeys:             []string{"ssh-ed25519 AAAA test"},
	}

	err := CheckConsistency(cfg)
	if err == nil {
		t.Fatal("expected error for FCOS with NvidiaDriverVersion")
	}
	if !strings.Contains(err.Error(), "nvidia_driver_version") {
		t.Errorf("error should mention nvidia_driver_version, got: %v", err)
	}
}

func TestCheckConsistency_FCOS_NoNvidiaOK(t *testing.T) {
	cfg := &model.InstallConfig{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-node",
		Disk:     model.DiskInfo{DevPath: "/dev/vda"},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
		SSHKeys:  []string{"ssh-ed25519 AAAA test"},
	}

	if err := CheckConsistency(cfg); err != nil {
		t.Errorf("expected no error for FCOS without NVIDIA, got: %v", err)
	}
}

func TestCheckConsistency_Flatcar_NvidiaOK(t *testing.T) {
	cfg := &model.InstallConfig{
		OS:                  model.OSFlatcar,
		Channel:             "stable",
		Hostname:            "flatcar-node",
		NvidiaDriverVersion: "570-open",
		Disk:                model.DiskInfo{DevPath: "/dev/vda"},
		Users:               []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
		SSHKeys:             []string{"ssh-ed25519 AAAA test"},
	}

	if err := CheckConsistency(cfg); err != nil {
		t.Errorf("expected no error for Flatcar with valid NVIDIA driver, got: %v", err)
	}
}
