package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// ── buildNetworkForm — additional interface coverage ──────────────────────────

func TestBuildNetworkForm_MultipleInterfaces(t *testing.T) {
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
		{Name: "eth1", MAC: "aa:bb:cc:dd:ee:ff", State: "down"},
		{Name: "wlan0", MAC: "66:77:88:99:aa:bb", State: "up"},
	}
	m := New(w)
	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil with multiple interfaces")
	}
}

func TestBuildNetworkForm_NoInterfaces(t *testing.T) {
	w := newTestWizard()
	w.State.Interfaces = nil
	m := New(w)
	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil with nil interfaces")
	}
}

// ── buildUserForm — additional state coverage ─────────────────────────────────

func TestBuildUserForm_PrefilledUsername(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Users = []model.UserConfig{
		{Username: "admin"},
	}
	m := New(w)
	m.usernameInput = "admin"
	m.passwordInput = "s3cret"
	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil with prefilled user")
	}
}

func TestBuildUserForm_WithGitHubAndPastedKey(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.githubUserInput = "octocat"
	m.sshKeyInput = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA test@machine;ssh-rsa AAAA second"
	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil with SSH data")
	}
}

// ── buildTailscaleForm — mode variants ────────────────────────────────────────

func TestBuildTailscaleForm_ExitNodeMode(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.tailscaleAuthKeyIn = "tskey-auth-abc123"
	m.tailscaleModeIn = model.TailscaleModeExitNode
	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil for exit-node mode")
	}
}

func TestBuildTailscaleForm_SubnetRouterWithRoutes(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.tailscaleModeIn = model.TailscaleModeSubnetRouter
	m.tailscaleRoutesIn = "10.0.0.0/24,192.168.1.0/24"
	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil for subnet-router mode")
	}
}

// ── buildReviewForm ───────────────────────────────────────────────────────────

func TestBuildReviewForm_FullConfig(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Disk.DevPath = "/dev/nvme0n1"
	w.State.Config.Disk.Model = "WD Black"
	w.State.Config.Hostname = "prod-01"
	w.State.Config.Users = []model.UserConfig{{Username: "ops"}}
	m := New(w)
	form := m.buildReviewForm()
	if form == nil {
		t.Fatal("buildReviewForm returned nil with full config")
	}
}

// ── reviewSummary — additional branch coverage ────────────────────────────────

func TestReviewSummary_MinimalDHCP(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Hostname = "node01"
	w.State.Config.Network.Mode = model.NetworkDHCP
	m := New(w)

	out := m.reviewSummary()
	if !strings.Contains(out, "stable") {
		t.Errorf("reviewSummary missing channel, got: %q", out)
	}
	if !strings.Contains(out, "/dev/sda") {
		t.Errorf("reviewSummary missing disk, got: %q", out)
	}
	if !strings.Contains(out, "node01") {
		t.Errorf("reviewSummary missing hostname, got: %q", out)
	}
	// DHCP should not show static address fields
	if strings.Contains(out, "via") {
		t.Errorf("reviewSummary should not show 'via' for DHCP, got: %q", out)
	}
}

func TestReviewSummary_WithMultipleSSHKeys(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Hostname = "test"
	w.State.Config.SSHKeys = []string{
		"ssh-ed25519 AAAA key1",
		"ssh-rsa AAAA key2",
		"ecdsa-sha2-nistp256 AAAA key3",
	}
	m := New(w)

	out := m.reviewSummary()
	if !strings.Contains(out, "3 key(s)") {
		t.Errorf("reviewSummary missing SSH key count, got: %q", out)
	}
}

func TestReviewSummary_SwapEnabledDefaultSize(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Hostname = "test"
	w.State.Config.Swap.Enabled = true
	w.State.Config.Swap.SizeMB = 0 // triggers default 4096
	m := New(w)

	out := m.reviewSummary()
	if !strings.Contains(out, "4096 MiB") {
		t.Errorf("reviewSummary should use default swap size (4096), got: %q", out)
	}
}

func TestReviewSummary_SwapCustomSize(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Hostname = "test"
	w.State.Config.Swap.Enabled = true
	w.State.Config.Swap.SizeMB = 8192
	m := New(w)

	out := m.reviewSummary()
	if !strings.Contains(out, "8192 MiB") {
		t.Errorf("reviewSummary missing custom swap size, got: %q", out)
	}
}

func TestReviewSummary_TailscaleEmptyModeDefaults(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Hostname = "test"
	w.State.Config.Tailscale.AuthKey = "tskey-auth-xxx"
	w.State.Config.Tailscale.Mode = "" // should default to "connect"
	m := New(w)

	out := m.reviewSummary()
	if !strings.Contains(out, "mode=connect") {
		t.Errorf("reviewSummary should default to connect mode, got: %q", out)
	}
}

func TestReviewSummary_WithUsers(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Hostname = "test"
	w.State.Config.Users = []model.UserConfig{
		{Username: "admin"},
	}
	m := New(w)

	out := m.reviewSummary()
	if !strings.Contains(out, "User: admin") {
		t.Errorf("reviewSummary missing user, got: %q", out)
	}
}

// ── localKeysSummary ──────────────────────────────────────────────────────────

func TestLocalKeysSummary_OutputFormat(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	out := m.localKeysSummary()
	// Must contain one of the two known outputs
	if !strings.Contains(out, "No local SSH keys") && !strings.Contains(out, "local key(s)") {
		t.Errorf("localKeysSummary returned unexpected format: %q", out)
	}
}
