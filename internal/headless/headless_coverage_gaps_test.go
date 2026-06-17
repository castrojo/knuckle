package headless

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// validTailscaleAuthKey is a key that passes validate.TailscaleAuthKey regex.
const validTailscaleAuthKey = "tskey-auth-ABCDEFGHIJ-ABCDEFGHIJKLMNOPQRST"

// TestToInstallConfig_TailscaleDefaultMode covers headless.go:149-151 —
// when AuthKey is non-empty and Mode is "", ToInstallConfig defaults Mode to "connect".
func TestToInstallConfig_TailscaleDefaultMode(t *testing.T) {
	cfg := &Config{
		Channel:  "stable",
		Hostname: "test-node",
		Disk:     "/dev/sda",
		Network:  NetworkConfig{Mode: "dhcp"},
		Users:    []UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test"}}},
		Tailscale: TailscaleConfig{
			AuthKey: validTailscaleAuthKey,
			Mode:    "", // empty → should default to "connect"
		},
	}

	result := cfg.ToInstallConfig()
	if result.Tailscale.Mode != model.TailscaleModeConnect {
		t.Errorf("expected tailscale mode=%q when empty, got %q", model.TailscaleModeConnect, result.Tailscale.Mode)
	}
	if result.Tailscale.AuthKey != validTailscaleAuthKey {
		t.Errorf("expected auth key to be preserved, got %q", result.Tailscale.AuthKey)
	}
}
