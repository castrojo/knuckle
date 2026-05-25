package wizard

// Tests for wizard.Next() error paths (#388).
// Each test verifies that when ValidateCurrentStep returns an error,
// Next() propagates it AND does not advance CurrentStep.

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// Next() from StepNetwork with an invalid static configuration must return
// an error and leave CurrentStep at StepNetwork.
func TestNext_NetworkValidationError_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:      model.NetworkStatic,
		Interface: "eth0",
		Address:   "not-a-cidr", // invalid — triggers validateNetwork error
		Gateway:   "192.168.1.1",
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepNetwork with invalid CIDR, got nil")
	}
	if !strings.Contains(err.Error(), "network address") {
		t.Errorf("expected 'network address' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepNetwork {
		t.Errorf("Next() should not advance on validation error: got %v, want StepNetwork", w.State.CurrentStep)
	}
}

// Next() from StepNetwork with a missing interface must return an error and
// leave CurrentStep at StepNetwork.
func TestNext_NetworkMissingInterface_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:    model.NetworkStatic,
		Address: "10.0.0.5/24",
		Gateway: "10.0.0.1",
		// Interface intentionally empty
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepNetwork with empty interface, got nil")
	}
	if !strings.Contains(err.Error(), "interface") {
		t.Errorf("expected 'interface' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepNetwork {
		t.Errorf("Next() should not advance on validation error: got %v, want StepNetwork", w.State.CurrentStep)
	}
}

// Next() from StepUser when no users and no SSH keys are configured must
// return an error and leave CurrentStep at StepUser.
func TestNext_UserNoUsersNoKeys_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = nil
	w.State.Config.SSHKeys = nil

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepUser with no users/keys, got nil")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Errorf("expected 'at least one' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepUser {
		t.Errorf("Next() should not advance on validation error: got %v, want StepUser", w.State.CurrentStep)
	}
}

// Next() from StepUser when a user has an invalid SSH key must return an
// error and leave CurrentStep at StepUser.
func TestNext_UserInvalidSSHKey_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"not-a-valid-ssh-key"}},
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepUser with invalid SSH key, got nil")
	}
	if w.State.CurrentStep != model.StepUser {
		t.Errorf("Next() should not advance on validation error: got %v, want StepUser", w.State.CurrentStep)
	}
}

// Next() from StepUser when a hostname is invalid must return an error and
// leave CurrentStep at StepUser.
func TestNext_UserInvalidHostname_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Hostname = "-bad" // leading hyphen is invalid
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test"}},
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepUser with invalid hostname, got nil")
	}
	if w.State.CurrentStep != model.StepUser {
		t.Errorf("Next() should not advance on validation error: got %v, want StepUser", w.State.CurrentStep)
	}
}

// Next() from StepTailscale when the auth key is present but invalid must
// return an error and leave CurrentStep at StepTailscale.
func TestNext_TailscaleInvalidAuthKey_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{{Name: "tailscale", Selected: true}}
	w.State.CurrentStep = model.StepTailscale
	w.State.Config.Tailscale.AuthKey = "not-a-real-tskey"

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepTailscale with invalid auth key, got nil")
	}
	if !strings.Contains(err.Error(), "tailscale auth key") {
		t.Errorf("expected 'tailscale auth key' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepTailscale {
		t.Errorf("Next() should not advance on validation error: got %v, want StepTailscale", w.State.CurrentStep)
	}
}

// Next() from StepTailscale in subnet-router mode with no routes must return
// an error and leave CurrentStep at StepTailscale.
func TestNext_TailscaleSubnetRouterNoRoutes_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{{Name: "tailscale", Selected: true}}
	w.State.CurrentStep = model.StepTailscale
	w.State.Config.Tailscale.AuthKey = "tskey-auth-kExampleKeyID1-ExampleSecretThatIsLongEnough123"
	w.State.Config.Tailscale.Mode = model.TailscaleModeSubnetRouter
	w.State.Config.Tailscale.Routes = "" // required for subnet-router mode

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepTailscale (subnet-router, no routes), got nil")
	}
	if !strings.Contains(err.Error(), "tailscale routes") {
		t.Errorf("expected 'tailscale routes' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepTailscale {
		t.Errorf("Next() should not advance on validation error: got %v, want StepTailscale", w.State.CurrentStep)
	}
}

// Next() from StepWelcome when an IgnitionURL is present but malformed must
// return an error and leave CurrentStep at StepWelcome.
func TestNext_WelcomeInvalidIgnitionURL_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "not-a-url" // invalid

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepWelcome with invalid IgnitionURL, got nil")
	}
	if !strings.Contains(err.Error(), "ignition URL") {
		t.Errorf("expected 'ignition URL' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepWelcome {
		t.Errorf("Next() should not advance on validation error: got %v, want StepWelcome", w.State.CurrentStep)
	}
}

// Next() from StepReview when disk is set but no auth method is provided must
// return an error and leave CurrentStep at StepReview.
func TestNext_ReviewNoAuthMethod_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config = model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		// No users, no SSH keys, no IgnitionURL — CheckConsistency must reject this.
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepReview with no auth method, got nil")
	}
	if !strings.Contains(err.Error(), "authentication") {
		t.Errorf("expected 'authentication' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepReview {
		t.Errorf("Next() should not advance on validation error: got %v, want StepReview", w.State.CurrentStep)
	}
}

// Next() from StepReview when channel is empty must return an error and leave
// CurrentStep at StepReview.
func TestNext_ReviewEmptyChannel_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config = model.InstallConfig{
		Channel: "", // missing — CheckConsistency must reject
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		SSHKeys: []string{"ssh-ed25519 AAAA test"},
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepReview with empty channel, got nil")
	}
	if !strings.Contains(err.Error(), "channel") {
		t.Errorf("expected 'channel' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepReview {
		t.Errorf("Next() should not advance on validation error: got %v, want StepReview", w.State.CurrentStep)
	}
}

// Next() from StepNetwork with an invalid gateway must return an error and
// leave CurrentStep at StepNetwork.
func TestNext_NetworkInvalidGateway_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:      model.NetworkStatic,
		Interface: "eth0",
		Address:   "192.168.1.10/24",
		Gateway:   "not-an-ip",
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepNetwork with invalid gateway, got nil")
	}
	if !strings.Contains(err.Error(), "gateway") {
		t.Errorf("expected 'gateway' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepNetwork {
		t.Errorf("Next() should not advance on validation error: got %v, want StepNetwork", w.State.CurrentStep)
	}
}

// Next() from StepNetwork with an invalid DNS server must return an error and
// leave CurrentStep at StepNetwork.
func TestNext_NetworkInvalidDNS_DoesNotAdvance(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:      model.NetworkStatic,
		Interface: "eth0",
		Address:   "192.168.1.10/24",
		Gateway:   "192.168.1.1",
		DNS:       []string{"not-a-dns"},
	}

	err := w.Next()
	if err == nil {
		t.Fatal("expected error from Next() at StepNetwork with invalid DNS, got nil")
	}
	if !strings.Contains(err.Error(), "DNS") {
		t.Errorf("expected 'DNS' in error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepNetwork {
		t.Errorf("Next() should not advance on validation error: got %v, want StepNetwork", w.State.CurrentStep)
	}
}
