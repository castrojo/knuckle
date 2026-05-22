package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// --- fetchKeysMsg: error path ---

// TestFetchKeysError_ShowsRetryHint verifies that when the async GitHub key
// fetch returns an error, the model surfaces a human-readable message that
// includes the retry instruction and stays on StepUser.
func TestFetchKeysError_ShowsRetryHint(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}

	m := New(w)
	newModel, _ := m.Update(fetchKeysMsg{keys: nil, err: fmt.Errorf("user not found")})
	got := newModel.(*Model)

	if got.err == nil {
		t.Fatal("expected err to be set after fetch error, got nil")
	}
	if !strings.Contains(got.err.Error(), "edit the username and press Enter to retry") {
		t.Errorf("expected retry hint in error message, got: %v", got.err)
	}
	if got.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to remain on StepUser after fetch error, got %v",
			got.Wizard.State.CurrentStep)
	}
}

// --- fetchKeysMsg: empty keys, no other auth ---

// TestFetchKeysEmpty_NoOtherAuth_ShowsError verifies that when GitHub returns
// zero keys and the user has no password and no manually-entered SSH keys, the
// model sets a "no SSH keys found" error and stays on StepUser.
func TestFetchKeysEmpty_NoOtherAuth_ShowsError(t *testing.T) {
	// Point HOME at an empty temp dir so detectLocalSSHKeys() returns nothing.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	// No PasswordHash, no pre-existing SSHKeys.

	m := New(w)
	m.sshKeyInput = "" // no manual key input

	newModel, _ := m.Update(fetchKeysMsg{keys: nil, err: nil})
	got := newModel.(*Model)

	if got.err == nil {
		t.Fatal("expected err to be set when no SSH keys and no password, got nil")
	}
	if !strings.Contains(got.err.Error(), "no SSH keys found") {
		t.Errorf("expected 'no SSH keys found' in error message, got: %v", got.err)
	}
	if got.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to remain on StepUser when no auth available, got %v",
			got.Wizard.State.CurrentStep)
	}
}

// --- fetchKeysMsg: empty keys, but password is set ---

// TestFetchKeysEmpty_WithPassword_Advances verifies that when GitHub returns
// zero keys but the user already has a PasswordHash set, the model treats that
// as sufficient authentication and advances past StepUser.
func TestFetchKeysEmpty_WithPassword_Advances(t *testing.T) {
	// Point HOME at an empty temp dir so detectLocalSSHKeys() returns nothing.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	// Bcrypt hash for "testpass" — value just needs to look like a hash; validation
	// only checks the presence of a non-empty string here.
	w.State.Config.Users = []model.UserConfig{{
		Username:     "core",
		PasswordHash: "$2a$10$aaaabbbbccccddddeeeeff0000111122223333445",
	}}

	m := New(w)
	m.sshKeyInput = "" // no manual key input

	newModel, _ := m.Update(fetchKeysMsg{keys: nil, err: nil})
	got := newModel.(*Model)

	if got.Wizard.State.CurrentStep == model.StepUser {
		t.Errorf("expected to advance past StepUser when password is set, still on StepUser (err=%v)",
			got.err)
	}
	if got.err != nil {
		t.Errorf("unexpected error when password hash is present: %v", got.err)
	}
}

// --- handleEnter: StepUser, no auth, no GitHub field ---

// TestEnterUserStep_NoAuth_NoGithub_ShowsError verifies that pressing Enter on
// StepUser without any authentication configured (no SSH keys, no password, no
// GitHub username) sets a "no authentication configured" error and keeps the
// wizard on StepUser.
func TestEnterUserStep_NoAuth_NoGithub_ShowsError(t *testing.T) {
	// Point HOME at an empty temp dir so detectLocalSSHKeys() returns nothing.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	// No PasswordHash, no SSHKeys.

	m := New(w)
	// Bypass the huh form so handleEnter() is reached directly.
	m.activeForm = nil
	// Ensure no github_user field carries a value.
	for i := range m.fields {
		if m.fields[i].key == "github_user" {
			m.fields[i].value = ""
		}
	}

	_, _ = m.handleEnter()

	if m.err == nil {
		t.Fatal("expected err to be set when no auth configured, got nil")
	}
	if !strings.Contains(m.err.Error(), "no authentication configured") {
		t.Errorf("expected 'no authentication configured' in error message, got: %v", m.err)
	}
	if m.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to remain on StepUser when no auth configured, got %v",
			m.Wizard.State.CurrentStep)
	}
}

// --- handleEnter: StepUser, local SSH keys available ---

// --- fetchKeysMsg: GitHub user WITH keys ---

// TestFetchKeysWithKeys_Advances verifies that when the async GitHub key fetch
// returns one or more keys, the model merges them into cfg.SSHKeys, clears
// any error, and advances past StepUser.
func TestFetchKeysWithKeys_Advances(t *testing.T) {
	// Empty HOME so detectLocalSSHKeys() contributes nothing.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}

	m := New(w)
	m.sshKeyInput = ""

	githubKeys := []string{
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 github1@user",
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDjTid/Xxpik4yiFwhLdPiIkG28XBLeIqvb0/nAwjyYJU8KU7qy91tGCFf/09D3VTnJbp3jfrOwboxb4iL+BiowC5bhbdJtHkQ89tx/xDw8ljrOx025UWp6EvOrD+rk7Aw4kYnLJ0CA5MvzdgVOal0brgHIpw34hbrP/yPNdv/H8VMsZBT+pXDQP0JcGe0K8HRM54cn/xIrSYnUvEZBb+kpscPXJtUGFNDSFxFp7fPhlViYLxDuNQtRgc7u3mAMuLMbxI6JxkIsvZ14PxxFTQ4Vq+BnJEazHgFn3wz86dHqanwx/sE9bBWsk7fhV2rfWpI1WI4KaTVfgeFaJ404VRkP github2@user",
	}
	newModel, _ := m.Update(fetchKeysMsg{keys: githubKeys, err: nil})
	got := newModel.(*Model)

	if got.err != nil {
		t.Fatalf("expected no error when GitHub returns keys, got: %v", got.err)
	}
	if got.Wizard.State.CurrentStep == model.StepUser {
		t.Error("expected to advance past StepUser when GitHub keys are returned")
	}
	if len(got.Wizard.State.Config.SSHKeys) != 2 {
		t.Errorf("expected 2 SSH keys in config, got %d: %v",
			len(got.Wizard.State.Config.SSHKeys), got.Wizard.State.Config.SSHKeys)
	}
	if got.Wizard.State.Config.SSHKeys[0] != githubKeys[0] {
		t.Errorf("expected SSHKeys[0] = %q, got %q",
			githubKeys[0], got.Wizard.State.Config.SSHKeys[0])
	}
}

// --- fetchKeysMsg: 404 not found ---

// TestFetchKeysError_404_ShowsRetryHint verifies that a GitHub 404 error
// ("user not found") surfaces the retry instruction and stays on StepUser.
func TestFetchKeysError_404_ShowsRetryHint(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "nonexistent-user-xyz"}}

	m := New(w)
	// This is exactly the error github.Client returns for a 404 response.
	notFoundErr := fmt.Errorf("GitHub user %q not found", "nonexistent-user-xyz")
	newModel, _ := m.Update(fetchKeysMsg{keys: nil, err: notFoundErr})
	got := newModel.(*Model)

	if got.err == nil {
		t.Fatal("expected err to be set after 404, got nil")
	}
	if !strings.Contains(got.err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got: %v", got.err)
	}
	if !strings.Contains(got.err.Error(), "edit the username and press Enter to retry") {
		t.Errorf("expected retry hint in 404 error message, got: %v", got.err)
	}
	if got.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to remain on StepUser after 404, got %v",
			got.Wizard.State.CurrentStep)
	}
}

// --- fetchKeysMsg: network timeout ---

// TestFetchKeysError_Timeout_ShowsRetryHint verifies that a network timeout
// (context.DeadlineExceeded wrapped inside the fetch error) surfaces the retry
// instruction and stays on StepUser — not a raw Go error string.
func TestFetchKeysError_Timeout_ShowsRetryHint(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}

	m := New(w)
	// Matches what github.Client returns when the HTTP client times out.
	timeoutErr := fmt.Errorf("failed to fetch keys: %w", context.DeadlineExceeded)
	newModel, _ := m.Update(fetchKeysMsg{keys: nil, err: timeoutErr})
	got := newModel.(*Model)

	if got.err == nil {
		t.Fatal("expected err to be set after timeout, got nil")
	}
	if !strings.Contains(got.err.Error(), "context deadline exceeded") {
		t.Errorf("expected 'context deadline exceeded' in timeout error, got: %v", got.err)
	}
	if !strings.Contains(got.err.Error(), "edit the username and press Enter to retry") {
		t.Errorf("expected retry hint in timeout error message, got: %v", got.err)
	}
	if got.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to remain on StepUser after timeout, got %v",
			got.Wizard.State.CurrentStep)
	}
}

// --- handleEnter: manual SSH key only (no GitHub, no password, no local keys) ---

// TestEnterUserStep_ManualSSHKeyOnly_Advances verifies that a user who pastes
// a public key into the SSH key field (no GitHub username, no password, no local
// .pub files) can advance past StepUser, and that the key lands in cfg.SSHKeys.
func TestEnterUserStep_ManualSSHKeyOnly_Advances(t *testing.T) {
	// Empty HOME: no local keys.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}

	m := New(w)
	m.activeForm = nil

	// Set the manual SSH key field; clear GitHub so no async fetch fires.
	const manualKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 manualkey@host"
	for i := range m.fields {
		switch m.fields[i].key {
		case "github_user":
			m.fields[i].value = ""
		case "ssh_key":
			m.fields[i].value = manualKey
		}
	}

	_, _ = m.handleEnter()

	if m.err != nil {
		t.Errorf("unexpected error with manual SSH key: %v", m.err)
	}
	if m.Wizard.State.CurrentStep == model.StepUser {
		t.Error("expected to advance past StepUser with manual SSH key")
	}
	found := false
	for _, k := range m.Wizard.State.Config.SSHKeys {
		if strings.Contains(k, "manualkey@host") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("manual SSH key not found in cfg.SSHKeys: %v",
			m.Wizard.State.Config.SSHKeys)
	}
}

// --- handleEnter: password only (no keys, no GitHub) ---

// TestEnterUserStep_PasswordOnly_Advances verifies that when a user configures
// only a hashed password (no SSH keys, no GitHub username, no local .pub files),
// the model treats it as sufficient authentication and advances past StepUser.
func TestEnterUserStep_PasswordOnly_Advances(t *testing.T) {
	// Empty HOME: no local keys.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	// Pre-set a bcrypt hash so no hashing is needed in the test and no SSH
	// keys are present. applyFields() will not overwrite PasswordHash if the
	// password field value is empty (which it is after initStepFields).
	w.State.Config.Users = []model.UserConfig{{
		Username:     "core",
		PasswordHash: "$2a$10$aaaabbbbccccddddeeeeff0000111122223333445",
	}}

	m := New(w)
	m.activeForm = nil

	// Clear both SSH key and GitHub fields so no async fetch fires.
	for i := range m.fields {
		switch m.fields[i].key {
		case "github_user", "ssh_key":
			m.fields[i].value = ""
		}
	}

	_, _ = m.handleEnter()

	if m.err != nil {
		t.Errorf("unexpected error with password-only auth: %v", m.err)
	}
	if m.Wizard.State.CurrentStep == model.StepUser {
		t.Error("expected to advance past StepUser with password-only auth")
	}
	// No SSH keys should have been added.
	if len(m.Wizard.State.Config.SSHKeys) != 0 {
		t.Errorf("expected no SSH keys in password-only config, got: %v",
			m.Wizard.State.Config.SSHKeys)
	}
}

// --- fetchKeysMsg: GitHub keys + local keys both present ---

// TestFetchKeysGitHubPlusLocal_MergesAndAdvances verifies that when a GitHub
// fetch completes and local .pub files are also present, mergeKeys() deduplicates
// them and both appear in cfg.SSHKeys, and the wizard advances.
func TestFetchKeysGitHubPlusLocal_MergesAndAdvances(t *testing.T) {
	// Create a temp HOME with a local SSH key.
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("creating temp .ssh dir: %v", err)
	}
	const localKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 localkey@installer"
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"),
		[]byte(localKey+"\n"), 0600); err != nil {
		t.Fatalf("writing temp pubkey: %v", err)
	}
	t.Setenv("HOME", dir)

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}

	m := New(w)
	m.sshKeyInput = ""

	githubKeys := []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 githubkey@user"}
	newModel, _ := m.Update(fetchKeysMsg{keys: githubKeys, err: nil})
	got := newModel.(*Model)

	if got.err != nil {
		t.Fatalf("expected no error, got: %v", got.err)
	}
	if got.Wizard.State.CurrentStep == model.StepUser {
		t.Error("expected to advance past StepUser when both local and GitHub keys are present")
	}
	if len(got.Wizard.State.Config.SSHKeys) != 2 {
		t.Errorf("expected 2 merged keys (local + github), got %d: %v",
			len(got.Wizard.State.Config.SSHKeys), got.Wizard.State.Config.SSHKeys)
	}
	// Both keys must be present.
	foundLocal, foundGitHub := false, false
	for _, k := range got.Wizard.State.Config.SSHKeys {
		if strings.Contains(k, "localkey@installer") {
			foundLocal = true
		}
		if strings.Contains(k, "githubkey@user") {
			foundGitHub = true
		}
	}
	if !foundLocal {
		t.Errorf("local SSH key not found in merged SSHKeys: %v",
			got.Wizard.State.Config.SSHKeys)
	}
	if !foundGitHub {
		t.Errorf("GitHub SSH key not found in merged SSHKeys: %v",
			got.Wizard.State.Config.SSHKeys)
	}
}

// TestEnterUserStep_WithLocalKeys_Advances verifies that when ~/.ssh/*.pub
// contains a valid SSH key, handleEnter() on StepUser detects it via
// detectLocalSSHKeys(), treats it as sufficient authentication, and advances
// the wizard past StepUser.
func TestEnterUserStep_WithLocalKeys_Advances(t *testing.T) {
	// Create a temp HOME containing a valid SSH public key.
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("creating temp .ssh dir: %v", err)
	}
	// Key format only needs type + base64-ish blob to pass validate.SSHPublicKey.
	keyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@local\n"
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte(keyLine), 0600); err != nil {
		t.Fatalf("writing temp pubkey: %v", err)
	}
	t.Setenv("HOME", dir)

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}

	m := New(w)
	// Bypass the huh form so handleEnter() is reached directly.
	m.activeForm = nil
	// No GitHub username — no async fetch triggered.
	for i := range m.fields {
		if m.fields[i].key == "github_user" {
			m.fields[i].value = ""
		}
	}

	_, _ = m.handleEnter()

	if m.err != nil {
		t.Errorf("unexpected error with local SSH key present: %v", m.err)
	}
	if m.Wizard.State.CurrentStep == model.StepUser {
		t.Errorf("expected to advance past StepUser when local SSH key is present, still on StepUser")
	}
}
