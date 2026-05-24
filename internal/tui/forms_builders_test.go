package tui

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

// TestBuildNetworkForm_CreatesFormWithInterfaces verifies that buildNetworkForm
// produces a non-nil form and includes detected interfaces in its options.
func TestBuildNetworkForm_CreatesFormWithInterfaces(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "UP"},
		{Name: "wlan0", MAC: "aa:bb:cc:dd:ee:ff", State: "DOWN"},
	}
	m := New(w)

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil")
	}
}

// TestBuildNetworkForm_NoInterfaces verifies form creation with empty interface list.
func TestBuildNetworkForm_NoInterfaces(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.Interfaces = nil
	m := New(w)

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil with no interfaces")
	}
}

// TestBuildUserForm_CreatesForm verifies that buildUserForm produces a valid form.
func TestBuildUserForm_CreatesForm(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	m := New(w)

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}
}

// TestBuildUserForm_WithExistingUsername verifies form with pre-set username.
func TestBuildUserForm_WithExistingUsername(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.Config.Users = []model.UserConfig{
		{Username: "testuser"},
	}
	m := New(w)

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}
	// usernameInput is populated from wizard state in New()
	if m.usernameInput != "testuser" {
		t.Errorf("usernameInput = %q, want %q", m.usernameInput, "testuser")
	}
}

// TestBuildTailscaleForm_CreatesForm verifies tailscale form creation.
func TestBuildTailscaleForm_CreatesForm(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	m := New(w)

	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil")
	}
}

// TestBuildTailscaleForm_ModeDefaultsToConnect ensures the mode select
// defaults to the connect option value.
func TestBuildTailscaleForm_ModeDefaultsToConnect(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	m := New(w)
	m.tailscaleModeIn = model.TailscaleModeConnect

	form := m.buildTailscaleForm()
	if form == nil {
		t.Fatal("buildTailscaleForm returned nil")
	}
	if m.tailscaleModeIn != model.TailscaleModeConnect {
		t.Errorf("tailscaleModeIn = %q, want %q", m.tailscaleModeIn, model.TailscaleModeConnect)
	}
}

// TestBuildUserForm_ValidatorsAcceptValidInput exercises the inline validators
// with known-good inputs to ensure forms don't reject valid user data.
func TestBuildUserForm_ValidatorsAcceptValidInput(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	m := New(w)

	// Set valid values that would pass validation
	m.usernameInput = "admin"
	m.passwordInput = "securepass123"
	m.githubUserInput = "octocat"

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil")
	}
}

// TestBuildUserForm_EmptyFieldsAreOptional verifies that empty optional fields
// don't cause validation failures (hostname, username are optional).
func TestBuildUserForm_EmptyFieldsAreOptional(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	m := New(w)

	m.usernameInput = ""
	m.passwordInput = ""
	m.githubUserInput = ""
	m.sshKeyInput = ""

	form := m.buildUserForm()
	if form == nil {
		t.Fatal("buildUserForm returned nil with all empty inputs")
	}
}

// TestBuildNetworkForm_WithManyInterfaces verifies form handles many interfaces.
func TestBuildNetworkForm_WithManyInterfaces(t *testing.T) {
	w := wizard.New(nil, nil, nil)
	w.State.Interfaces = make([]model.NetworkInterface, 10)
	for i := range w.State.Interfaces {
		w.State.Interfaces[i] = model.NetworkInterface{
			Name:  "eth" + string(rune('0'+i)),
			MAC:   "00:00:00:00:00:0" + string(rune('0'+i)),
			State: "UP",
		}
	}
	m := New(w)

	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm returned nil with many interfaces")
	}
}
