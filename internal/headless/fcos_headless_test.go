package headless

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestValidate_FCOS_ValidConfig(t *testing.T) {
	cfg := &Config{
		OS:             model.OSFCOS,
		Channel:        "stable",
		Hostname:       "fcos-node",
		Network:        NetworkConfig{Mode: "dhcp"},
		Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:           "/dev/vda",
		UpdateStrategy: "immediate",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid FCOS config, got: %v", err)
	}
}

func TestValidate_FCOS_DisabledStrategy(t *testing.T) {
	cfg := &Config{
		OS:             model.OSFCOS,
		Channel:        "stable",
		Hostname:       "fcos-node",
		Network:        NetworkConfig{Mode: "dhcp"},
		Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:           "/dev/vda",
		UpdateStrategy: "disabled",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid FCOS config with disabled strategy, got: %v", err)
	}
}

func TestValidate_FCOS_RejectFlatcarStrategy(t *testing.T) {
	for _, strategy := range []string{"reboot", "off", "etcd-lock"} {
		t.Run(strategy, func(t *testing.T) {
			cfg := &Config{
				OS:             model.OSFCOS,
				Channel:        "stable",
				Hostname:       "fcos-node",
				Network:        NetworkConfig{Mode: "dhcp"},
				Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
				Disk:           "/dev/vda",
				UpdateStrategy: strategy,
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for FCOS with Flatcar strategy %q", strategy)
			}
			if !strings.Contains(err.Error(), "update_strategy") {
				t.Errorf("error should mention update_strategy, got: %v", err)
			}
		})
	}
}

func TestValidate_FCOS_RejectNvidiaDriver(t *testing.T) {
	cfg := &Config{
		OS:                  model.OSFCOS,
		Channel:             "stable",
		Hostname:            "fcos-node",
		Network:             NetworkConfig{Mode: "dhcp"},
		Users:               []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:                "/dev/vda",
		UpdateStrategy:      "immediate",
		NvidiaDriverVersion: "570-open",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for FCOS with nvidia_driver_version")
	}
	if !strings.Contains(err.Error(), "nvidia_driver_version") || !strings.Contains(err.Error(), "FCOS") {
		t.Errorf("error should mention nvidia_driver_version and FCOS, got: %v", err)
	}
}

func TestValidate_FCOS_VersionFieldIgnored(t *testing.T) {
	cfg := &Config{
		OS:             model.OSFCOS,
		Channel:        "stable",
		Version:        "38.20230514.3.0",
		Hostname:       "fcos-node",
		Network:        NetworkConfig{Mode: "dhcp"},
		Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:           "/dev/vda",
		UpdateStrategy: "immediate",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("FCOS version field should be accepted (ignored), got: %v", err)
	}
}

func TestValidate_FCOS_Streams(t *testing.T) {
	for _, stream := range []string{"stable", "testing", "next"} {
		t.Run(stream, func(t *testing.T) {
			cfg := &Config{
				OS:             model.OSFCOS,
				Channel:        stream,
				Hostname:       "fcos-node",
				Network:        NetworkConfig{Mode: "dhcp"},
				Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
				Disk:           "/dev/vda",
				UpdateStrategy: "immediate",
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf("expected valid FCOS stream %q, got: %v", stream, err)
			}
		})
	}
}

func TestValidate_FCOS_InvalidStream(t *testing.T) {
	cfg := &Config{
		OS:             model.OSFCOS,
		Channel:        "lts",
		Hostname:       "fcos-node",
		Network:        NetworkConfig{Mode: "dhcp"},
		Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:           "/dev/vda",
		UpdateStrategy: "immediate",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for FCOS with lts channel")
	}
}

func TestToInstallConfig_FCOS(t *testing.T) {
	cfg := &Config{
		OS:             model.OSFCOS,
		Channel:        "stable",
		Version:        "44.20260510.3.1",
		Hostname:       "fcos-node",
		Network:        NetworkConfig{Mode: "dhcp"},
		Users:          []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
		Disk:           "/dev/vda",
		UpdateStrategy: "immediate",
	}

	ic := cfg.ToInstallConfig()

	if ic.OS != model.OSFCOS {
		t.Errorf("OS = %q, want %q", ic.OS, model.OSFCOS)
	}
	if ic.Version != "" {
		t.Errorf("Version should be empty for FCOS, got %q", ic.Version)
	}
	if ic.UpdateStrategy.FCOSUpdateStrategy != model.FCOSStrategyImmediate {
		t.Errorf("FCOSUpdateStrategy = %q, want %q", ic.UpdateStrategy.FCOSUpdateStrategy, model.FCOSStrategyImmediate)
	}
	if ic.UpdateStrategy.RebootStrategy != "" {
		t.Errorf("RebootStrategy should be empty for FCOS, got %q", ic.UpdateStrategy.RebootStrategy)
	}
}

func TestToInstallConfig_FCOS_DefaultsToImmediate(t *testing.T) {
	cfg := &Config{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-node",
		Network:  NetworkConfig{Mode: "dhcp"},
		Users:    []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
		Disk:     "/dev/vda",
	}

	ic := cfg.ToInstallConfig()
	if ic.UpdateStrategy.FCOSUpdateStrategy != model.FCOSStrategyImmediate {
		t.Errorf("default FCOSUpdateStrategy = %q, want %q", ic.UpdateStrategy.FCOSUpdateStrategy, model.FCOSStrategyImmediate)
	}
}

func TestToInstallConfig_DefaultFlatcar(t *testing.T) {
	cfg := &Config{
		Channel:  "stable",
		Hostname: "flatcar-node",
		Network:  NetworkConfig{Mode: "dhcp"},
		Users:    []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}}},
		Disk:     "/dev/vda",
	}

	ic := cfg.ToInstallConfig()

	if ic.OS != model.OSFlatcar {
		t.Errorf("OS = %q, want %q (default)", ic.OS, model.OSFlatcar)
	}
	if ic.UpdateStrategy.RebootStrategy != "reboot" {
		t.Errorf("default RebootStrategy = %q, want %q", ic.UpdateStrategy.RebootStrategy, "reboot")
	}
}

func TestValidate_FCOS_EmptyStrategyAllowed(t *testing.T) {
	cfg := &Config{
		OS:       model.OSFCOS,
		Channel:  "stable",
		Hostname: "fcos-node",
		Network:  NetworkConfig{Mode: "dhcp"},
		Users:    []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3Nz test@test"}}},
		Disk:     "/dev/vda",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("FCOS with empty strategy should be valid, got: %v", err)
	}
}
