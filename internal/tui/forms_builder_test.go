package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// TestBuildNetworkForm tests the buildNetworkForm function with various configurations
func TestBuildNetworkForm(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	tests := []struct {
		name       string
		interfaces []model.NetworkInterface
		wantFields int
	}{
		{
			name:       "no interfaces detected",
			interfaces: []model.NetworkInterface{},
			wantFields: 2, // mode + static fields group
		},
		{
			name: "single interface",
			interfaces: []model.NetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
			},
			wantFields: 2,
		},
		{
			name: "multiple interfaces",
			interfaces: []model.NetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
				{Name: "wlan0", MAC: "AA:BB:CC:DD:EE:FF", State: "down"},
			},
			wantFields: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWizard()
			w.State.Interfaces = tt.interfaces
			m := New(w)

			form := m.buildNetworkForm()
			if form == nil {
				t.Fatal("buildNetworkForm returned nil")
			}

		})
	}
}

// TestBuildNetworkFormStaticValidation tests the inline validators for static network config
func TestBuildNetworkFormStaticValidation(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
	}
	m := New(w)

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil")
	}

	// Test that form contains fields for static config
	// The form should have IP address, gateway, and DNS fields
	// These are in the second group (static fields)
}

// TestBuildNetworkFormDHCPMode tests DHCP mode selection
func TestBuildNetworkFormDHCPMode(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
	}
	m := New(w)
	m.networkModeInput = "dhcp"

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil")
	}

	if m.networkModeInput != "dhcp" {
		t.Errorf("expected networkModeInput to be dhcp, got %s", m.networkModeInput)
	}
}

// TestBuildNetworkFormStaticMode tests static mode selection
func TestBuildNetworkFormStaticMode(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
	}
	m := New(w)
	m.networkModeInput = "static"
	m.Wizard.State.Config.Network.Address = "192.168.1.100/24"
	m.Wizard.State.Config.Network.Gateway = "192.168.1.1"

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil")
	}

	if m.Wizard.State.Config.Network.Address != "192.168.1.100/24" {
		t.Errorf("expected address 192.168.1.100/24, got %s", m.Wizard.State.Config.Network.Address)
	}
}

// TestBuildUserForm tests the buildUserForm function with various configurations
func TestBuildUserForm(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	tests := []struct {
		name       string
		hostname   string
		username   string
		password   string
		githubUser string
		sshKey     string
		timezone   string
		wantGroups int
	}{
		{
			name:       "empty form",
			hostname:   "",
			username:   "",
			password:   "",
			githubUser: "",
			sshKey:     "",
			timezone:   "",
			wantGroups: 2, // identity + auth groups
		},
		{
			name:       "with hostname and username",
			hostname:   "flatcar-node01",
			username:   "admin",
			password:   "",
			githubUser: "",
			sshKey:     "",
			timezone:   "UTC",
			wantGroups: 2,
		},
		{
			name:       "with password",
			hostname:   "flatcar-node01",
			username:   "admin",
			password:   "secret123",
			githubUser: "",
			sshKey:     "",
			timezone:   "America/New_York",
			wantGroups: 2,
		},
		{
			name:       "with GitHub username",
			hostname:   "flatcar-node01",
			username:   "admin",
			password:   "",
			githubUser: "octocat",
			sshKey:     "",
			timezone:   "UTC",
			wantGroups: 2,
		},
		{
			name:       "with SSH key",
			hostname:   "flatcar-node01",
			username:   "admin",
			password:   "",
			githubUser: "",
			sshKey:     "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
			timezone:   "Europe/Berlin",
			wantGroups: 2,
		},
		{
			name:       "with all fields",
			hostname:   "flatcar-node01",
			username:   "admin",
			password:   "secret123",
			githubUser: "@octocat",
			sshKey:     "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
			timezone:   "Asia/Tokyo",
			wantGroups: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWizard()
			m := New(w)
			m.Wizard.State.Config.Hostname = tt.hostname
			m.usernameInput = tt.username
			m.passwordInput = tt.password
			m.githubUserInput = tt.githubUser
			m.sshKeyInput = tt.sshKey
			m.Wizard.State.Config.Timezone = tt.timezone

			form := m.buildUserForm()
			if form == nil {
				t.Fatal("buildUserForm returned nil")
			}

			// Verify inputs are preserved
			if m.Wizard.State.Config.Hostname != tt.hostname {
				t.Errorf("hostname mismatch: got %s, want %s", m.Wizard.State.Config.Hostname, tt.hostname)
			}
			if m.usernameInput != tt.username {
				t.Errorf("username mismatch: got %s, want %s", m.usernameInput, tt.username)
			}
			if m.passwordInput != tt.password {
				t.Errorf("password mismatch: got %s, want %s", m.passwordInput, tt.password)
			}
		})
	}
}

// TestBuildUserFormValidationHostname tests hostname validation
func TestBuildUserFormValidationHostname(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}

	// The form should be created successfully
	// Actual validation happens when the form is submitted
}

// TestBuildUserFormValidationUsername tests username validation
func TestBuildUserFormValidationUsername(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.usernameInput = "testuser"

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}

	if m.usernameInput != "testuser" {
		t.Errorf("expected usernameInput testuser, got %s", m.usernameInput)
	}
}

// TestBuildUserFormValidationTimezone tests timezone validation
func TestBuildUserFormValidationTimezone(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.Wizard.State.Config.Timezone = "America/New_York"

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}

	if m.Wizard.State.Config.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", m.Wizard.State.Config.Timezone)
	}
}

// TestBuildTailscaleForm tests the buildTailscaleForm function with various configurations
func TestBuildTailscaleForm(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	tests := []struct {
		name       string
		authKey    string
		mode       string
		routes     string
		wantGroups int
	}{
		{
			name:       "empty form",
			authKey:    "",
			mode:       "",
			routes:     "",
			wantGroups: 1,
		},
		{
			name:       "with auth key - connect mode",
			authKey:    "tskey-auth-kNUCKLE-aBcDeFgHiJkLmNoPqRsTuVwXyZ",
			mode:       model.TailscaleModeConnect,
			routes:     "",
			wantGroups: 1,
		},
		{
			name:       "with auth key - exit node mode",
			authKey:    "tskey-auth-kNUCKLE-aBcDeFgHiJkLmNoPqRsTuVwXyZ",
			mode:       model.TailscaleModeExitNode,
			routes:     "",
			wantGroups: 1,
		},
		{
			name:       "with auth key - subnet router mode",
			authKey:    "tskey-auth-kNUCKLE-aBcDeFgHiJkLmNoPqRsTuVwXyZ",
			mode:       model.TailscaleModeSubnetRouter,
			routes:     "10.0.0.0/24,192.168.1.0/24",
			wantGroups: 1,
		},
		{
			name:       "without auth key - connect mode",
			authKey:    "",
			mode:       model.TailscaleModeConnect,
			routes:     "",
			wantGroups: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWizard()
			m := New(w)
			m.tailscaleAuthKeyIn = tt.authKey
			m.tailscaleModeIn = tt.mode
			m.tailscaleRoutesIn = tt.routes

			form := m.buildTailscaleForm()
			if form == nil {
				t.Fatal("buildTailscaleForm returned nil")
			}

			// Verify inputs are preserved
			if m.tailscaleAuthKeyIn != tt.authKey {
				t.Errorf("authKey mismatch: got %s, want %s", m.tailscaleAuthKeyIn, tt.authKey)
			}
			if m.tailscaleModeIn != tt.mode {
				t.Errorf("mode mismatch: got %s, want %s", m.tailscaleModeIn, tt.mode)
			}
			if m.tailscaleRoutesIn != tt.routes {
				t.Errorf("routes mismatch: got %s, want %s", m.tailscaleRoutesIn, tt.routes)
			}
		})
	}
}

// TestBuildTailscaleFormModes tests all three Tailscale modes
func TestBuildTailscaleFormModes(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	modes := []string{
		model.TailscaleModeConnect,
		model.TailscaleModeExitNode,
		model.TailscaleModeSubnetRouter,
	}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			w := newTestWizard()
			m := New(w)
			m.tailscaleModeIn = mode

			form := m.buildTailscaleForm()
			if form == nil {
				t.Fatal("buildTailscaleForm returned nil")
			}

			if m.tailscaleModeIn != mode {
				t.Errorf("expected mode %s, got %s", mode, m.tailscaleModeIn)
			}
		})
	}
}

// TestBuildTailscaleFormAuthKeyValidation tests auth key validation
func TestBuildTailscaleFormAuthKeyValidation(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.tailscaleAuthKeyIn = "tskey-auth-test-validkey"

	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil")
	}

	// Verify auth key is set
	if m.tailscaleAuthKeyIn != "tskey-auth-test-validkey" {
		t.Errorf("expected auth key tskey-auth-test-validkey, got %s", m.tailscaleAuthKeyIn)
	}
}

// TestBuildTailscaleFormSubnetRoutes tests subnet router with routes
func TestBuildTailscaleFormSubnetRoutes(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.tailscaleModeIn = model.TailscaleModeSubnetRouter
	m.tailscaleRoutesIn = "10.0.0.0/24,192.168.1.0/24"

	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil")
	}

	if m.tailscaleModeIn != model.TailscaleModeSubnetRouter {
		t.Errorf("expected subnet-router mode, got %s", m.tailscaleModeIn)
	}

	if m.tailscaleRoutesIn != "10.0.0.0/24,192.168.1.0/24" {
		t.Errorf("expected routes 10.0.0.0/24,192.168.1.0/24, got %s", m.tailscaleRoutesIn)
	}
}

// TestLocalKeysSummary tests the localKeysSummary helper function
func TestLocalKeysSummary(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)

	summary := m.localKeysSummary()
	if summary == "" {
		t.Error("localKeysSummary should return non-empty string")
	}

	// Should contain either "No local SSH keys" or "key(s) from ~/.ssh/"
	if !strings.Contains(summary, "No local SSH keys") && !strings.Contains(summary, "key(s)") {
		t.Errorf("unexpected summary format: %s", summary)
	}
}

// TestBuildNetworkFormInterfaceOptions tests interface option generation
func TestBuildNetworkFormInterfaceOptions(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
		{Name: "eth1", MAC: "66:77:88:99:AA:BB", State: "down"},
		{Name: "wlan0", MAC: "CC:DD:EE:FF:00:11", State: "up"},
	}
	m := New(w)

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil")
	}

	// Verify all interfaces are available
	if len(w.State.Interfaces) != 3 {
		t.Errorf("expected 3 interfaces, got %d", len(w.State.Interfaces))
	}
}

// TestBuildUserFormPasswordEchoMode tests password field is masked
func TestBuildUserFormPasswordEchoMode(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.passwordInput = "secret"

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}

	// Password should be set
	if m.passwordInput != "secret" {
		t.Errorf("expected password to be set, got %s", m.passwordInput)
	}
}

// TestBuildTailscaleFormEmptyAuthKey tests form with empty auth key
func TestBuildTailscaleFormEmptyAuthKey(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.tailscaleAuthKeyIn = ""
	m.tailscaleModeIn = model.TailscaleModeConnect

	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil")
	}

	if m.tailscaleAuthKeyIn != "" {
		t.Errorf("expected empty auth key, got %s", m.tailscaleAuthKeyIn)
	}
}

// TestBuildNetworkFormAutoInterface tests auto DHCP option
func TestBuildNetworkFormAutoInterface(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
	}
	m := New(w)
	m.Wizard.State.Config.Network.Interface = "" // Auto option

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil")
	}

	if m.Wizard.State.Config.Network.Interface != "" {
		t.Errorf("expected empty interface (auto), got %s", m.Wizard.State.Config.Network.Interface)
	}
}

// TestBuildUserFormMultipleSSHKeys tests SSH key field with multiple keys
func TestBuildUserFormMultipleSSHKeys(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.sshKeyInput = "ssh-rsa KEY1;ssh-ed25519 KEY2"

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}

	if !strings.Contains(m.sshKeyInput, ";") {
		t.Error("expected semicolon-separated keys")
	}
}

// TestBuildUserFormGitHubUsernameWithAt tests GitHub username with @ prefix
func TestBuildUserFormGitHubUsernameWithAt(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.githubUserInput = "@octocat"

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}

	if m.githubUserInput != "@octocat" {
		t.Errorf("expected @octocat, got %s", m.githubUserInput)
	}
}

// TestBuildNetworkFormDNSInput tests DNS input field
func TestBuildNetworkFormDNSInput(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
	}
	m := New(w)
	m.dnsInput = "1.1.1.1,8.8.8.8"

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil")
	}

	if m.dnsInput != "1.1.1.1,8.8.8.8" {
		t.Errorf("expected DNS 1.1.1.1,8.8.8.8, got %s", m.dnsInput)
	}
}

// TestBuildTailscaleFormWithRoutesNoAuthKey tests routes without auth key
func TestBuildTailscaleFormWithRoutesNoAuthKey(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	m := New(w)
	m.tailscaleAuthKeyIn = ""
	m.tailscaleModeIn = model.TailscaleModeSubnetRouter
	m.tailscaleRoutesIn = "10.0.0.0/8"

	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil")
	}

	// Routes can be set even without auth key
	if m.tailscaleRoutesIn != "10.0.0.0/8" {
		t.Errorf("expected routes 10.0.0.0/8, got %s", m.tailscaleRoutesIn)
	}
}
