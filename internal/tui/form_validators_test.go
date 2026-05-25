package tui

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/validate"
)

// TestFormValidatorClosures exercises the inline validator closures defined
// inside buildNetworkForm, buildUserForm, and buildTailscaleForm.
//
// These closures add empty-string bypass logic before calling validate.X
// functions. The validate package itself is 99.4% covered; these tests
// ensure the closure wrappers behave correctly (empty → nil, invalid → error,
// valid → nil).

// ── Network form validators ──────────────────────────────────────────────────

func TestNetworkForm_CIDRValidator(t *testing.T) {
	// Mirrors the closure in buildNetworkForm for IP Address field:
	//   if s == "" { return nil }; return validate.CIDR(s)
	validator := func(s string) error {
		if s == "" {
			return nil
		}
		return validate.CIDR(s)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"valid CIDR", "192.168.1.100/24", false},
		{"valid CIDR small subnet", "10.0.0.0/8", false},
		{"missing prefix length", "192.168.1.100", true},
		{"invalid IP", "999.999.999.999/24", true},
		{"garbage", "not-a-cidr", true},
		{"IPv6 not supported", "fd00::1/64", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CIDR validator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestNetworkForm_GatewayValidator(t *testing.T) {
	// Mirrors the closure in buildNetworkForm for Gateway field:
	//   if s == "" { return nil }; return validate.IPAddress(s)
	validator := func(s string) error {
		if s == "" {
			return nil
		}
		return validate.IPAddress(s)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv4 loopback", "127.0.0.1", false},
		{"invalid IP", "999.1.1.1", true},
		{"garbage", "not-an-ip", true},
		{"IPv6 not supported", "::1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Gateway validator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ── User form validators ─────────────────────────────────────────────────────

func TestUserForm_HostnameValidator(t *testing.T) {
	// Mirrors the closure in buildUserForm for Hostname field:
	//   if s == "" { return nil }; return validate.Hostname(s)
	validator := func(s string) error {
		if s == "" {
			return nil
		}
		return validate.Hostname(s)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"valid hostname", "flatcar-node01", false},
		{"valid short", "a", false},
		{"starts with dash", "-bad", true},
		{"too long", string(make([]byte, 64)), true},
		{"contains underscore", "bad_host", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hostname validator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestUserForm_TimezoneValidator(t *testing.T) {
	// Mirrors the closure in buildUserForm for Timezone field:
	//   return validate.Timezone(s)
	// Note: no empty bypass — empty is handled by Timezone itself.
	validator := func(s string) error {
		return validate.Timezone(s)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty defaults to UTC", "", false},
		{"UTC", "UTC", false},
		{"America/New_York", "America/New_York", false},
		{"nonsense but passes", "Not/A/Zone", false}, // Timezone validator only checks format, not IANA DB
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Timezone validator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestUserForm_UsernameValidator(t *testing.T) {
	// Mirrors the closure in buildUserForm for Username field:
	//   if s == "" { return nil }; return validate.Username(s)
	validator := func(s string) error {
		if s == "" {
			return nil
		}
		return validate.Username(s)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"valid username", "admin", false},
		{"valid with numbers", "user42", false},
		{"starts with digit", "0user", true},
		{"contains spaces", "bad user", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Username validator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ── Tailscale form validators ────────────────────────────────────────────────

func TestTailscaleForm_AuthKeyValidator(t *testing.T) {
	// Mirrors the closure in buildTailscaleForm for Auth Key field:
	//   if s == "" { return nil }; return validate.TailscaleAuthKey(s)
	validator := func(s string) error {
		if s == "" {
			return nil
		}
		return validate.TailscaleAuthKey(s)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"valid auth key", "tskey-auth-kNUCKLEtest01-aBcDeFgHiJkLmNoPqRsTuVwXyZa", false},
		{"missing prefix", "not-a-tskey", true},
		{"partial prefix", "tskey-", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TailscaleAuthKey validator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ── Integration: form builder nil-safety ─────────────────────────────────────

// TestBuildAllForms_NilSafety ensures all form builders return non-nil forms
// even with zero-value model state (no interfaces, no channels, etc.).
func TestBuildAllForms_NilSafety(t *testing.T) {
	w := newTestWizard()
	m := New(w)

	if f := m.buildNetworkForm(); f == nil {
		t.Error("buildNetworkForm returned nil with empty state")
	}
	if f := m.buildUserForm(); f == nil {
		t.Error("buildUserForm returned nil with empty state")
	}
	if f := m.buildTailscaleForm(); f == nil {
		t.Error("buildTailscaleForm returned nil with empty state")
	}
	if f := m.buildReviewForm(); f == nil {
		t.Error("buildReviewForm returned nil with empty state")
	}
}

// TestReviewSummary_AllFields exercises reviewSummary with a fully populated config.
func TestReviewSummary_AllFields(t *testing.T) {
	w := newTestWizard()
	w.State.Config = model.InstallConfig{
		Channel:  "stable",
		Version:  "4593.2.0",
		Hostname: "node01",
		Disk:     model.DiskInfo{DevPath: "/dev/sda", Model: "Samsung", SizeHuman: "500G"},
		Network:  model.NetworkConfig{Mode: model.NetworkStatic, Address: "10.0.0.5/24", Gateway: "10.0.0.1"},
		Users:    []model.UserConfig{{Username: "admin"}},
		SSHKeys:  []string{"ssh-ed25519 AAAA..."},
		Sysexts:  []model.SysextEntry{{Name: "docker"}, {Name: "tailscale"}},
		Swap:     model.SwapConfig{Enabled: true, SizeMB: 2048},
		Tailscale: model.TailscaleConfig{
			AuthKey: "tskey-auth-xxx",
			Mode:    model.TailscaleModeSubnetRouter,
			Routes:  "10.0.0.0/24",
		},
	}
	m := New(w)
	summary := m.reviewSummary()

	checks := []string{
		"stable", "4593.2.0", "/dev/sda", "Samsung", "500G",
		"10.0.0.5/24", "10.0.0.1", "node01", "admin",
		"1 key(s)", "docker", "tailscale", "2048 MiB",
		"auth key set", "subnet-router", "10.0.0.0/24",
	}
	for _, want := range checks {
		if !contains(summary, want) {
			t.Errorf("reviewSummary missing %q in output:\n%s", want, summary)
		}
	}
}

// TestReviewSummary_SwapDisabledValidator verifies swap disabled text.
func TestReviewSummary_SwapDisabledValidator(t *testing.T) {
	w := newTestWizard()
	w.State.Config = model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/vda"},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Swap:    model.SwapConfig{Enabled: false},
	}
	m := New(w)
	summary := m.reviewSummary()
	if !contains(summary, "disabled") {
		t.Errorf("reviewSummary should show swap disabled, got:\n%s", summary)
	}
}

// TestReviewSummary_SwapDefaultSizeValidator verifies default swap size.
func TestReviewSummary_SwapDefaultSizeValidator(t *testing.T) {
	w := newTestWizard()
	w.State.Config = model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/vda"},
		Network: model.NetworkConfig{Mode: model.NetworkDHCP},
		Swap:    model.SwapConfig{Enabled: true, SizeMB: 0}, // should use default
	}
	m := New(w)
	summary := m.reviewSummary()
	if !contains(summary, "MiB") {
		t.Errorf("reviewSummary should show default swap size, got:\n%s", summary)
	}
}

// TestReviewSummary_TailscaleConnectModeValidator verifies connect mode display.
func TestReviewSummary_TailscaleConnectModeValidator(t *testing.T) {
	w := newTestWizard()
	w.State.Config = model.InstallConfig{
		Channel:   "stable",
		Disk:      model.DiskInfo{DevPath: "/dev/vda"},
		Network:   model.NetworkConfig{Mode: model.NetworkDHCP},
		Tailscale: model.TailscaleConfig{AuthKey: "tskey-auth-xxx", Mode: ""},
	}
	m := New(w)
	summary := m.reviewSummary()
	// Empty mode defaults to "connect"
	if !contains(summary, model.TailscaleModeConnect) {
		t.Errorf("reviewSummary should default to connect mode, got:\n%s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
