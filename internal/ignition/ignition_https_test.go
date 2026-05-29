package ignition

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// TestGenerateButane_NonHTTPSSysextURL verifies that GenerateButane rejects
// sysext download URLs that do not use HTTPS. This enforces the security
// invariant at ignition.go:79 — non-HTTPS sysext sources must never appear
// in generated Ignition configs.
func TestGenerateButane_NonHTTPSSysextURL_ReturnsError(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "test-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			{Name: "docker", Version: "24.0.7", URL: "http://example.com/docker.raw", Selected: true},
		},
	}

	_, err := g.GenerateButane(cfg)
	if err == nil {
		t.Fatal("expected error for non-HTTPS sysext URL, got nil")
	}
	if !strings.Contains(err.Error(), "non-HTTPS") {
		t.Errorf("expected error to mention non-HTTPS, got: %v", err)
	}
	if !strings.Contains(err.Error(), "docker") {
		t.Errorf("expected error to mention sysext name 'docker', got: %v", err)
	}
}

// TestGenerateButane_NonHTTPSSysextURL_UnselectedIgnored verifies that a
// non-HTTPS URL on an unselected sysext does NOT trigger the error — only
// selected sysexts are included in the generated config.
func TestGenerateButane_NonHTTPSSysextURL_UnselectedIgnored(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "test-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			// Selected=false — not included in filterSelected output
			{Name: "bad-sysext", Version: "1.0", URL: "http://example.com/bad.raw", Selected: false},
			// Selected=true with HTTPS — valid
			{Name: "good-sysext", Version: "2.0", URL: "https://example.com/good.raw", Selected: true},
		},
	}

	_, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unselected non-HTTPS sysext should not cause error; got: %v", err)
	}
}
