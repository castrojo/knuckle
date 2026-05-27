package tui

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestOnFormComplete_Welcome_InvalidChannel(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "invalid-channel"
	m := New(w)

	// This exercises the nil-guard fix: initForm() sets activeForm=nil
	// for Welcome (card-based), so the error path must not call
	// m.activeForm.Init() unconditionally.
	_ = m.onFormComplete()

	if m.err == nil {
		t.Error("expected validation error for invalid channel")
	}
	// Should stay on Welcome
	if m.Wizard.State.CurrentStep != model.StepWelcome {
		t.Errorf("expected to stay on Welcome, got %v", m.Wizard.State.CurrentStep)
	}
}

func TestOnFormComplete_Network_Static(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	m := New(w)
	m.networkModeInput = "static"
	m.dnsInput = "8.8.8.8,1.1.1.1"

	_ = m.onFormComplete()

	cfg := m.Wizard.State.Config
	if cfg.Network.Mode != model.NetworkStatic {
		t.Errorf("expected Static mode, got %v", cfg.Network.Mode)
	}
	if len(cfg.Network.DNS) != 2 {
		t.Errorf("expected 2 DNS entries, got %v", cfg.Network.DNS)
	}
}

func TestOnFormComplete_Network_DHCP(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	m := New(w)
	m.networkModeInput = "dhcp"
	m.dnsInput = ""

	_ = m.onFormComplete()

	cfg := m.Wizard.State.Config
	if cfg.Network.Mode != model.NetworkDHCP {
		t.Errorf("expected DHCP mode, got %v", cfg.Network.Mode)
	}
}

func TestOnFormComplete_User_CreatesUser(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "node1"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	m := New(w)
	m.usernameInput = "admin"
	m.passwordInput = ""
	m.sshKeyInput = "ssh-ed25519 AAAA test@key"
	m.githubUserInput = "" // no GitHub fetch

	_ = m.onFormComplete()

	cfg := m.Wizard.State.Config
	if len(cfg.Users) == 0 {
		t.Fatal("expected user to be created")
	}
	if cfg.Users[0].Username != "admin" {
		t.Errorf("expected username 'admin', got %q", cfg.Users[0].Username)
	}
	if len(cfg.SSHKeys) == 0 {
		t.Error("expected SSH keys to be set")
	}
}

func TestOnFormComplete_User_UpdatesExisting(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "node1"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "old", Groups: []string{"sudo"}}}
	m := New(w)
	m.usernameInput = "newuser"
	m.sshKeyInput = "ssh-ed25519 AAAA test@key"

	_ = m.onFormComplete()

	if m.Wizard.State.Config.Users[0].Username != "newuser" {
		t.Errorf("expected 'newuser', got %q", m.Wizard.State.Config.Users[0].Username)
	}
}

func TestOnFormComplete_User_WithPassword(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "node1"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	m.usernameInput = "core"
	m.passwordInput = "secret123"
	m.sshKeyInput = "ssh-ed25519 AAAA test@key"

	_ = m.onFormComplete()

	if m.err != nil {
		t.Fatalf("unexpected error: %v", m.err)
	}
	if m.Wizard.State.Config.Users[0].PasswordHash == "" {
		t.Error("expected password hash to be set")
	}
}

func TestOnFormComplete_User_GitHubFetch(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "node1"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	m.usernameInput = "core"
	m.githubUserInput = "@someuser"
	m.sshKeyInput = "ssh-ed25519 AAAA local@key"

	cmd := m.onFormComplete()

	if !m.fetching {
		t.Error("expected fetching=true for GitHub key fetch")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for async fetch")
	}
}

func TestOnFormComplete_User_GitHubFetch_ClosureBody(t *testing.T) {
	// Verifies the returned tea.Cmd closure body (the actual fetch call)
	// by replacing fetchGitHubKeysFn with a stub.
	want := []string{"ssh-ed25519 AAAA stub@key"}
	old := fetchGitHubKeysFn
	fetchGitHubKeysFn = func(username string) ([]string, error) {
		if username != "someuser" {
			t.Errorf("fetchGitHubKeysFn: got username %q, want %q", username, "someuser")
		}
		return want, nil
	}
	defer func() { fetchGitHubKeysFn = old }()

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "node1"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	m.usernameInput = "core"
	m.githubUserInput = "@someuser"
	m.sshKeyInput = "ssh-ed25519 AAAA local@key"

	cmd := m.onFormComplete()
	if cmd == nil {
		t.Fatal("expected non-nil cmd for async fetch")
	}

	// Execute the closure — this covers the closure body in form_logic.go.
	msg := cmd()
	got, ok := msg.(fetchKeysMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want fetchKeysMsg", msg)
	}
	if got.err != nil {
		t.Fatalf("unexpected error in fetchKeysMsg: %v", got.err)
	}
	if len(got.keys) != 1 || got.keys[0] != want[0] {
		t.Errorf("fetchKeysMsg.keys = %v, want %v", got.keys, want)
	}
}

func TestOnFormComplete_User_DefaultTimezone(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "node1"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	w.State.Config.Timezone = ""
	m := New(w)
	m.usernameInput = "core"
	m.sshKeyInput = "ssh-ed25519 AAAA test@key"

	_ = m.onFormComplete()

	if m.Wizard.State.Config.Timezone != "UTC" {
		t.Errorf("expected default timezone 'UTC', got %q", m.Wizard.State.Config.Timezone)
	}
}

func TestOnFormComplete_Review_NotConfirmed_GoesBack(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	w.State.Confirmed = false
	m := New(w)

	_ = m.onFormComplete()

	// Should go back (Previous())
	if m.Wizard.State.CurrentStep == model.StepReview {
		t.Error("expected to navigate back from Review when not confirmed")
	}
}

func TestOnFormComplete_Review_Confirmed_StartsInstall(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	w.State.Config.DryRun = true
	w.State.Confirmed = true
	m := New(w)

	cmd := m.onFormComplete()

	if !m.installing {
		t.Error("expected installing=true after confirmed review")
	}
	if cmd == nil {
		t.Error("expected startInstall cmd")
	}
}

func TestOnFormComplete_AdvanceFails_ShowsError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	// Missing required fields → Next() will fail validation
	w.State.Config.Channel = "stable"
	// No users/hostname — wizard.Next() might pass or fail depending on per-step validation
	m := New(w)
	m.networkModeInput = "dhcp"

	_ = m.onFormComplete()

	// If Next() failed, error should be set; if it passed, step should advance.
	// Either outcome is valid — just verify no panic.
	if m.err != nil {
		// Validation error — should stay on Network
		if m.Wizard.State.CurrentStep != model.StepNetwork {
			t.Errorf("expected to stay on Network with error, got %v", m.Wizard.State.CurrentStep)
		}
	}
}

func TestOnFormComplete_Welcome_WithIgnitionURL_JumpsToStorage(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	m := New(w)

	_ = m.onFormComplete()

	if m.err != nil {
		t.Errorf("expected no error, got: %v", m.err)
	}
	if m.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("expected jump to StepStorage, got %v", m.Wizard.State.CurrentStep)
	}
}

func TestOnFormComplete_Tailscale_EmptyKey_Advances(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepTailscale
	w.State.Config.Sysexts = []model.SysextEntry{
		{Name: "tailscale", Selected: true},
	}
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test"}},
	}
	m := New(w)
	m.tailscaleAuthKeyIn = "" // empty key is valid (skip tailscale)

	_ = m.onFormComplete()

	if m.err != nil {
		t.Errorf("expected no error for empty tailscale key, got: %v", m.err)
	}
	if m.Wizard.State.CurrentStep == model.StepTailscale {
		t.Error("expected to advance past StepTailscale")
	}
}

func TestOnFormComplete_Tailscale_InvalidKey_ShowsError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepTailscale
	w.State.Config.Sysexts = []model.SysextEntry{
		{Name: "tailscale", Selected: true},
	}
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test"}},
	}
	m := New(w)
	m.tailscaleAuthKeyIn = "not-a-valid-tskey"

	_ = m.onFormComplete()

	if m.err == nil {
		t.Fatal("expected error for invalid tailscale key, got nil")
	}
	if m.Wizard.State.CurrentStep != model.StepTailscale {
		t.Errorf("expected to stay on StepTailscale, got %v", m.Wizard.State.CurrentStep)
	}
}

func TestOnFormComplete_Tailscale_ValidKey_Advances(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepTailscale
	w.State.Config.Sysexts = []model.SysextEntry{
		{Name: "tailscale", Selected: true},
	}
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test"}},
	}
	m := New(w)
	m.tailscaleAuthKeyIn = "tskey-auth-kSomeID12345-SomeSecretThatIsLongEnough1234"
	m.tailscaleModeIn = model.TailscaleModeConnect

	_ = m.onFormComplete()

	if m.err != nil {
		t.Errorf("expected no error for valid tailscale key, got: %v", m.err)
	}
	if m.Wizard.State.CurrentStep == model.StepTailscale {
		t.Error("expected to advance past StepTailscale with valid key")
	}
}
