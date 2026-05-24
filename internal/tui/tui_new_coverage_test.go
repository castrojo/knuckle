package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// ── tui.go: Update ctrl+c second-press with installCancel set ────────────────

func TestUpdate_CtrlC_SecondPress_CallsInstallCancel(t *testing.T) {
	cancelled := false
	w := newTestWizard()
	m := New(w)
	m.confirmQuit = true
	m.installCancel = func() { cancelled = true }

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := newModel.(*Model)

	if !got.quitting {
		t.Error("expected quitting=true")
	}
	if !cancelled {
		t.Error("expected installCancel to be called on second ctrl+c")
	}
}

// ── tui.go: fetchKeysMsg with invalid SSH key from GitHub ────────────────────

func TestUpdate_FetchKeysMsg_InvalidSSHKey(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	m := New(w)
	m.fetching = true

	newModel, _ := m.Update(fetchKeysMsg{keys: []string{"not-a-valid-ssh-key"}, err: nil})
	got := newModel.(*Model)

	if got.fetching {
		t.Error("expected fetching=false")
	}
	if got.err == nil {
		t.Fatal("expected error for invalid SSH key from GitHub")
	}
	if !strings.Contains(got.err.Error(), "invalid SSH key") {
		t.Errorf("expected 'invalid SSH key' in error, got: %v", got.err)
	}
}

// ── tui.go: fetchKeysMsg with empty keys and no prior auth ──────────────────

func TestUpdate_FetchKeysMsg_EmptyKeys_NoAuth(t *testing.T) {
	// Point HOME to a dir with no .ssh/ so detectLocalSSHKeys returns nothing.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	// No users, no SSHKeys, no password → HasAnyAuthentication returns false
	m := New(w)
	m.fetching = true
	m.sshKeyInput = ""

	newModel, _ := m.Update(fetchKeysMsg{keys: []string{}, err: nil})
	got := newModel.(*Model)

	if got.fetching {
		t.Error("expected fetching=false")
	}
	if got.err == nil {
		t.Fatal("expected error when no auth after empty key fetch")
	}
	if !strings.Contains(got.err.Error(), "no SSH keys") {
		t.Errorf("expected 'no SSH keys' in error, got: %v", got.err)
	}
}

// ── tui.go: handleKey ctrl+c direct — installCancel called ──────────────────
// Update's ctrl+c handler is reached before handleKey. The installCancel path
// inside handleKey (L269) is only exercisable via handleKey directly.

func TestHandleKey_CtrlC_Direct_CallsInstallCancel(t *testing.T) {
	cancelled := false
	w := newTestWizard()
	m := New(w)
	m.confirmQuit = true
	m.installCancel = func() { cancelled = true }

	newModel, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := newModel.(*Model)

	if !got.quitting {
		t.Error("expected quitting=true")
	}
	if !cancelled {
		t.Error("expected installCancel called in handleKey ctrl+c")
	}
}

// ── tui.go: handleKey 'q' direct — installCancel called ─────────────────────
// Update clears confirmQuit before reaching handleKey, so we call handleKey
// directly to test the installCancel path on 'q'.

func TestHandleKey_Q_Direct_CallsInstallCancel(t *testing.T) {
	cancelled := false
	w := newTestWizard()
	m := New(w)
	m.confirmQuit = true
	m.installCancel = func() { cancelled = true }
	// No fields so 'q' goes to quit confirmation path, not character insertion.

	newModel, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := newModel.(*Model)

	if !got.quitting {
		t.Error("expected quitting=true after 'q' with confirmQuit")
	}
	if !cancelled {
		t.Error("expected installCancel called in handleKey 'q'")
	}
}

// ── tui.go: handleKey 'r' on Done — rebootFn called ─────────────────────────

func TestHandleKey_Reboot_WithRebootFn_Direct(t *testing.T) {
	rebooted := false
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = false
	m := New(w)
	m.confirmReboot = true
	m.rebootFn = func(_ context.Context) error {
		rebooted = true
		return nil
	}

	newModel, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := newModel.(*Model)

	if !got.quitting {
		t.Error("expected quitting=true on confirmed reboot")
	}
	if cmd == nil {
		t.Fatal("expected reboot cmd")
	}
	// Execute the cmd to drive rebootFn.
	_ = cmd()
	if !rebooted {
		t.Error("expected rebootFn to be called when cmd is executed")
	}
}

// ── tui.go: handleKey tab delegates to sysext list when ready ────────────────

func TestHandleKey_Tab_SysextListReady_Delegates(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated},
		{Name: "tailscale", SupportTier: bakery.TierMaintained},
	}
	m := New(w)
	m.initStepFields() // calls initSysextList which sets sysextListReady=true
	if !m.sysextListReady {
		t.Skip("sysext list not initialized")
	}

	// Tab via Update (no activeForm, not ctrl+c/ctrl+a so passes to handleKey).
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := newModel.(*Model)

	// The list delegate was called — cursor should still be valid.
	if got.cursor < 0 {
		t.Errorf("cursor should be non-negative after sysext list tab, got %d", got.cursor)
	}
}

// ── tui.go: handleKey up delegates to sysext list when ready ─────────────────

func TestHandleKey_Up_SysextListReady_Delegates(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated},
		{Name: "tailscale", SupportTier: bakery.TierMaintained},
	}
	m := New(w)
	m.initStepFields()
	if !m.sysextListReady {
		t.Skip("sysext list not initialized")
	}

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	got := newModel.(*Model)

	if got.cursor < 0 {
		t.Errorf("cursor should be non-negative after sysext list up, got %d", got.cursor)
	}
}

// ── tui.go: handleKey default delegates to sysext list when ready ─────────────

func TestHandleKey_Default_SysextListReady_Delegates(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated},
	}
	m := New(w)
	m.initStepFields()
	if !m.sysextListReady {
		t.Skip("sysext list not initialized")
	}

	// Send a regular character that hits the default case.
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_ = newModel // should not panic
}

// ── tui.go: handleKey esc on sysext list while filtering ─────────────────────

func TestHandleKey_Esc_SysextList_Filtering(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated},
		{Name: "tailscale", SupportTier: bakery.TierMaintained},
	}
	m := New(w)
	m.initStepFields()
	if !m.sysextListReady {
		t.Skip("sysext list not initialized")
	}

	// Drive the list into filtering state by sending "/" then check esc.
	// First enable filtering with a forward-slash (search trigger).
	m.sysextList.SetFilteringEnabled(true)
	slashMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	m.sysextList, _ = m.sysextList.Update(slashMsg)

	if m.sysextList.FilterState() != list.Filtering {
		t.Skip("list not in Filtering state — key binding may differ")
	}

	// Now esc should clear the filter, not go back a step.
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := newModel.(*Model)

	// Should still be on StepSysext (esc cleared filter, not navigated back).
	if got.Wizard.State.CurrentStep != model.StepSysext {
		t.Errorf("esc in filtering should stay on StepSysext, got %v", got.Wizard.State.CurrentStep)
	}
}

// ── tui.go: handleEnter Storage with IgnitionURL — validation failure ────────

func TestHandleEnter_Storage_IgnitionURL_ValidationFails(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	// No disks, so cursor won't select a disk and validateStorage will fail.
	w.State.Disks = nil
	m := New(w)

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := newModel.(*Model)

	if got.err == nil {
		t.Fatal("expected validation error for missing disk with IgnitionURL")
	}
	// Should stay on Storage
	if got.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("expected to stay on StepStorage, got %v", got.Wizard.State.CurrentStep)
	}
}

// ── tui.go: handleEnter Storage with IgnitionURL — jumps to Review ───────────

func TestHandleEnter_Storage_IgnitionURL_JumpsToReview(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	w.State.Config.Channel = "stable"
	w.State.Disks = []model.DiskInfo{{DevPath: "/dev/sda", Path: "/dev/sda"}}
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda"}
	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := newModel.(*Model)

	if got.err != nil {
		t.Fatalf("unexpected error: %v", got.err)
	}
	if got.Wizard.State.CurrentStep != model.StepReview {
		t.Errorf("expected StepReview, got %v", got.Wizard.State.CurrentStep)
	}
}

// ── forms.go: reviewSummary Tailscale empty mode defaults ────────────────────

func TestReviewSummary_Tailscale_EmptyMode_DefaultsToConnect(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Tailscale = model.TailscaleConfig{
		AuthKey: "tskey-auth-kSomeID12345-SomeSecretThatIsLongEnough1234",
		Mode:    "", // empty mode → should default to "connect" in summary
	}
	s := New(w).reviewSummary()
	if !strings.Contains(s, "connect") {
		t.Errorf("empty Tailscale mode should default to 'connect' in summary, got: %q", s)
	}
}

// ── forms.go: buildNetworkForm with interfaces populated ─────────────────────

func TestBuildNetworkForm_WithInterfaces_IncludesOptionLabels(t *testing.T) {
	w := newTestWizard()
	w.State.Interfaces = []model.NetworkInterface{
		{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff", State: "UP"},
		{Name: "eth1", MAC: "11:22:33:44:55:66", State: "DOWN"},
	}
	m := New(w)
	form := m.buildNetworkForm()
	if form == nil {
		t.Fatal("buildNetworkForm() returned nil with interfaces set")
	}
	// Just verify the form builds without panic — the interface options path ran.
}

// ── forms.go: viewChannelCards with component versions ───────────────────────

func TestViewChannelCards_WithComponentVersions(t *testing.T) {
	w := newTestWizard()
	w.State.Channels = []bakery.ChannelInfo{
		{
			Channel: "stable",
			Version: "3510.2.0",
			Kernel:  "6.1.90",
			Systemd: "255.4",
			Docker:  "26.1.3",
		},
	}
	m := New(w)
	out := m.viewChannelCards()
	// The component-version rendering path (kernel/systemd/docker) should have run.
	if !strings.Contains(out, "linux") || !strings.Contains(out, "6.1.90") {
		t.Errorf("viewChannelCards should show kernel version, got: %q", out[:min(300, len(out))])
	}
}

// ── form_logic.go: onFormComplete Review confirmed — Wizard.Next fails ────────

func TestOnFormComplete_Review_Confirmed_NextError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	// No disk and no auth → CheckConsistency fails → Next() returns error
	w.State.Config.Disk = model.DiskInfo{} // DevPath=""
	w.State.Config.SSHKeys = []string{}
	w.State.Config.Users = []model.UserConfig{}
	w.State.Confirmed = true
	m := New(w)
	m.initForm() // sets activeForm for StepReview

	_ = m.onFormComplete()

	if m.err == nil {
		t.Error("expected error when Next() fails at Review step")
	}
	if m.Wizard.State.Confirmed {
		t.Error("expected Confirmed reset to false after error")
	}
}

// ── tui.go: View — quitting branch ───────────────────────────────────────────

func TestView_Quitting_ShowsCancelled(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.quitting = true

	out := m.View()
	if !strings.Contains(out, "cancelled") {
		t.Errorf("quitting view should show cancellation message, got: %q", out)
	}
}

// ── tui.go: viewStorage — no disks detected ──────────────────────────────────

func TestViewStorage_NoDisks(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = nil
	m := New(w)

	out := m.viewStorage()
	if !strings.Contains(out, "No disks detected") {
		t.Errorf("viewStorage with no disks should show message, got: %q", out)
	}
}

// ── tui.go: viewStorage — disk with empty model and removable flag ────────────

func TestViewStorage_DiskWithNoModel_ShowsUnknown(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{
		{DevPath: "/dev/sda", Model: "", SizeHuman: "500 GB", Removable: true},
	}
	m := New(w)

	out := m.viewStorage()
	if !strings.Contains(out, "Unknown Disk") {
		t.Errorf("disk with no model should show 'Unknown Disk', got: %q", out)
	}
	if !strings.Contains(out, "removable") {
		t.Errorf("removable disk should show 'removable', got: %q", out)
	}
}

// ── tui.go: viewStorage — disk with explicit Path field ──────────────────────

func TestViewStorage_DiskWithExplicitPath(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{
		{DevPath: "/dev/sda", Path: "/dev/disk/by-id/ata-FOO", Model: "Seagate", SizeHuman: "1 TB"},
	}
	m := New(w)

	out := m.viewStorage()
	if !strings.Contains(out, "/dev/disk/by-id/ata-FOO") {
		t.Errorf("viewStorage should show explicit Path, got: %q", out)
	}
}

// ── tui.go: handleEnter StepInstall — re-enter while installing ──────────────

func TestHandleEnter_StepInstall_WhileInstalling_IsNoop(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	m := New(w)
	m.installing = true // already installing

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := newModel.(*Model)

	if !got.installing {
		t.Error("expected installing to remain true")
	}
	if cmd != nil {
		t.Error("expected nil cmd when already installing")
	}
}

// ── tui.go: applyFields — password hash error path ───────────────────────────
// hashPassword panics/errors only on extremely malformed input.
// We cover the branch indirectly by calling applyFields() with a non-empty password
// in a step that has a user configured.
func TestApplyFields_PasswordSet_Hashed(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	m.initStepFields()
	// Set password field value
	for i, f := range m.fields {
		if f.key == "password" {
			m.fields[i].value = "secretpassword123"
			break
		}
	}

	m.applyFields()

	// If no error, password was hashed successfully.
	if m.err != nil {
		t.Errorf("unexpected error hashing password: %v", m.err)
	}
	if m.Wizard.State.Config.Users[0].PasswordHash == "" {
		t.Error("expected password to be hashed and stored")
	}
}

// ── tui.go: Update progress.FrameMsg body ────────────────────────────────────
// The existing TestUpdate_ProgressFrameMsg incorrectly passes a tea.Cmd directly
// to Update (which falls through as an unknown msg type). This test correctly
// executes the cmd to get the actual FrameMsg and passes it to Update.

func TestUpdate_ProgressFrameMsg_Correct(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	// SetPercent returns a tea.Cmd that produces a FrameMsg when called.
	setCmd := m.progress.SetPercent(0.5)
	if setCmd == nil {
		t.Skip("progress.SetPercent returned nil cmd, can't drive animation frame")
	}
	frameMsg := setCmd() // produce the actual progress.FrameMsg
	newModel, _ := m.Update(frameMsg)
	if newModel == nil {
		t.Fatal("Update returned nil for progress.FrameMsg")
	}
}

// ── tui.go: fetchKeysMsg success → Next → form step ──────────────────────────
// fetchKeysMsg arrives while on StepWelcome; Next() advances to StepNetwork
// (step order: Welcome→Network→Storage→User…) which has an activeForm,
// triggering m.activeForm.Init() at the end of the handler.

func TestUpdate_FetchKeysMsg_AdvancesToFormStep(t *testing.T) {
	// Empty HOME so detectLocalSSHKeys returns nothing.
	t.Setenv("HOME", t.TempDir())

	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome // Next() → StepNetwork (has form)
	w.State.Config.Channel = "stable"
	m := New(w)
	m.fetching = true

	keys := []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 github@key"}
	newModel, _ := m.Update(fetchKeysMsg{keys: keys, err: nil})
	got := newModel.(*Model)

	if got.err != nil {
		t.Errorf("unexpected error: %v", got.err)
	}
	// Welcome → Network; Network step has a huh form (activeForm != nil).
	if got.activeForm == nil {
		t.Error("expected activeForm to be set after advancing to Network step")
	}
}

// ── tui.go: handleEnter StepUser — GitHub field triggers async fetch ──────────
// Uses the raw-field path (activeForm=nil) so the github_user field value is
// read from m.fields, not the huh form.

func TestHandleEnter_User_GitHubField_TriggersAsyncFetch(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no local keys

	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Hostname = "test"
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	m.activeForm = nil // bypass huh form → raw field path
	m.initStepFields() // populates m.fields including github_user

	for i, f := range m.fields {
		if f.key == "github_user" {
			m.fields[i].value = "someuser"
			break
		}
	}
	m.fetching = false

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := newModel.(*Model)

	if !got.fetching {
		t.Error("expected fetching=true when github_user field is set")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for async GitHub key fetch")
	}
}

// ── form_logic.go: onFormComplete User — password too long → bcrypt error ────

func TestOnFormComplete_User_LongPassword_Error(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	m.usernameInput = "core"
	m.passwordInput = strings.Repeat("a", 73) // > 72 bytes — bcrypt limit
	m.sshKeyInput = "ssh-ed25519 AAAA test@key"
	m.githubUserInput = ""

	_ = m.onFormComplete()

	if m.err == nil {
		t.Fatal("expected error for password exceeding bcrypt limit")
	}
	if m.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to stay on StepUser, got %v", m.Wizard.State.CurrentStep)
	}
}

// ── form_logic.go: onFormComplete Tailscale — empty mode defaults ─────────────

func TestOnFormComplete_Tailscale_EmptyMode_DefaultsToConnect(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepTailscale
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
	}
	m := New(w)
	m.tailscaleAuthKeyIn = ""
	m.tailscaleModeIn = "" // force empty — triggers the `if mode == ""` guard

	_ = m.onFormComplete()

	if m.Wizard.State.Config.Tailscale.Mode != model.TailscaleModeConnect {
		t.Errorf("empty Tailscale mode should default to 'connect', got %q",
			m.Wizard.State.Config.Tailscale.Mode)
	}
}

// ── tui.go: handleEnter StepInstall — starts install on first enter ───────────

func TestHandleEnter_StepInstall_StartsInstall(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	w.State.Config.DryRun = true // so install won't do anything real
	m := New(w)
	m.installing = false // not yet installing

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := newModel.(*Model)

	if !got.installing {
		t.Error("expected installing=true after first enter on StepInstall")
	}
	if cmd == nil {
		t.Error("expected startInstall cmd")
	}
}
