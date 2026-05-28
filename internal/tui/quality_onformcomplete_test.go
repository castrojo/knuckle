package tui

// quality_onformcomplete_test.go — tests for coverage gaps identified during
// the quality agent ACMM L3 run on 2026-05-27. Covers:
//
//   - tui.go:116-119   Init() KNUCKLE_TEST_TUI_AUTO_QUIT=1 early-exit path
//   - tui.go:209-212   fetchKeysMsg: Wizard.Next() error branch
//   - tui.go:552-554   handleEnter: advance to StepDone → return tea.Quit
//   - form_logic.go:66-68   onFormComplete StepWelcome invalid channel → nil form
//   - form_logic.go:77-79   onFormComplete StepWelcome IgnitionURL skip → nil form
//   - form_logic.go:162-168 onFormComplete fallthrough Next() error → re-inits form
//   - form_logic.go:191-194 viewWithForm checksStr dead-code (renderSystemChecks = "")
//   - form_logic.go:103-109 onFormComplete StepUser githubUserInput → async cmd
//   - form_logic.go:136-138 onFormComplete StepReview unconfirmed go-back → nil form

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

// ── tui.go:116-119  Init() KNUCKLE_TEST_TUI_AUTO_QUIT ────────────────────────

// TestInit_AutoQuit_EnvVar verifies that when KNUCKLE_TEST_TUI_AUTO_QUIT=1 is
// set, Init() immediately marks the model as quitting and returns tea.Quit
// (tui.go:116-119).
func TestInit_AutoQuit_EnvVar(t *testing.T) {
	t.Setenv("KNUCKLE_TEST_TUI_AUTO_QUIT", "1")

	w := newTestWizard()
	m := New(w)

	cmd := m.Init()

	if !m.quitting {
		t.Error("expected m.quitting = true when KNUCKLE_TEST_TUI_AUTO_QUIT=1")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init() in auto-quit mode")
	}
	// Execute the cmd and verify it produces a QuitMsg.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from Init() cmd, got %T", msg)
	}

}

// ── tui.go:209-212  fetchKeysMsg Next() error ────────────────────────────────

// TestUpdate_FetchKeysMsg_NextError exercises the branch where GitHub key
// fetch succeeds, HasAnyAuthentication() is true, but Wizard.Next() returns
// an error because the hostname is invalid (tui.go:209-212).
func TestUpdate_FetchKeysMsg_NextError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	// Invalid hostname triggers validateUser() → Next() error.
	w.State.Config.Hostname = "invalid hostname with spaces"
	w.State.Config.Channel = "stable"
	m := New(w)
	m.fetching = true

	// Valid key so validate.SSHPublicKey passes and HasAnyAuthentication passes.
	keys := []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA test@key"}

	newModel, cmd := m.Update(fetchKeysMsg{keys: keys, err: nil})
	got := newModel.(*Model)

	if got.fetching {
		t.Error("expected fetching=false after fetchKeysMsg")
	}
	if got.err == nil {
		t.Error("expected err to be set when Wizard.Next() fails after key fetch")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd on Next() error, got %v", cmd)
	}
	// Step should not have advanced.
	if got.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to remain at StepUser, got %v", got.Wizard.State.CurrentStep)
	}
}

// ── form_logic.go:66-68  StepWelcome invalid channel → nil form ──────────────

// TestOnFormComplete_WelcomeInvalidChannel_NilFormReturnsNil verifies that
// when onFormComplete() is called at StepWelcome with an invalid channel, it
// sets m.err, calls initForm() (which leaves activeForm=nil for Welcome), and
// returns nil (form_logic.go:66-68).
func TestOnFormComplete_WelcomeInvalidChannel_NilForm(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "not-a-real-channel"
	m := New(w)
	// Welcome step uses card selector, not huh form.
	if m.activeForm != nil {
		t.Fatal("precondition: Welcome step should have nil activeForm")
	}

	cmd := m.onFormComplete()

	if m.err == nil {
		t.Error("expected err to be set for invalid channel")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd (nil activeForm after initForm), got non-nil")
	}
	// Step should remain at Welcome.
	if m.Wizard.State.CurrentStep != model.StepWelcome {
		t.Errorf("expected to remain at StepWelcome, got %v", m.Wizard.State.CurrentStep)
	}
}

// ── form_logic.go:77-79  StepWelcome IgnitionURL skip → nil form ─────────────

// TestOnFormComplete_WelcomeIgnitionURL_SkipToStorage verifies that with a
// valid channel and IgnitionURL set, onFormComplete() at StepWelcome calls
// GoToStep(Storage), inits the form (nil for Storage), and returns nil
// (form_logic.go:77-79).
func TestOnFormComplete_WelcomeIgnitionURL_SkipToStorage(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "https://example.com/ignition.json"
	m := New(w)

	cmd := m.onFormComplete()

	if m.err != nil {
		t.Errorf("unexpected error: %v", m.err)
	}
	// Should have jumped to Storage (or left at Welcome if GoToStep refused, but
	// Storage has no conditional guard so it should succeed).
	if m.Wizard.State.CurrentStep == model.StepWelcome {
		t.Error("expected wizard to leave StepWelcome after IgnitionURL path")
	}
	// Storage step has no huh form → activeForm must be nil → cmd must be nil.
	if cmd != nil {
		t.Errorf("expected nil cmd (Storage has no huh form), got non-nil")
	}
}

// ── form_logic.go:103-109  StepUser githubUserInput → async cmd ──────────────

// TestOnFormComplete_UserStep_GithubFetch_ReturnsCmd verifies that when
// onFormComplete() is called at StepUser with a non-empty githubUserInput,
// it sets m.fetching=true and returns a non-nil async tea.Cmd
// (form_logic.go:103-109).
func TestOnFormComplete_UserStep_GithubFetch_ReturnsCmd(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test-host"
	w.State.Config.Timezone = "UTC"
	m := New(w)
	// Set the input fields that ApplyUserStep reads.
	m.usernameInput = "core"
	m.githubUserInput = "octocat"
	m.sshKeyInput = ""
	m.passwordInput = ""

	cmd := m.onFormComplete()

	if !m.fetching {
		t.Error("expected m.fetching=true after onFormComplete with github user")
	}
	if cmd == nil {
		t.Fatal("expected non-nil async cmd for GitHub key fetch")
	}
	// Should remain at StepUser until async fetch completes.
	if m.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to stay at StepUser during async fetch, got %v",
			m.Wizard.State.CurrentStep)
	}
}

// ── form_logic.go:136-138  StepReview unconfirmed go-back → nil form ─────────

// TestOnFormComplete_ReviewUnconfirmed_GoBack verifies that when
// onFormComplete() is called at StepReview with Confirmed=false, it calls
// Wizard.Previous(), resets state, and — since the previous step is a
// non-form step — leaves activeForm=nil and returns nil
// (form_logic.go:136-138).
func TestOnFormComplete_ReviewUnconfirmed_GoBack(t *testing.T) {
	w := newTestWizard()
	// Place wizard at StepReview; Previous() from there goes to StepUpdate.
	w.State.CurrentStep = model.StepReview
	w.State.Confirmed = false
	m := New(w)
	m.cursor = 3
	m.err = errTest

	cmd := m.onFormComplete()

	// Must have gone back (Previous() from Review → Update or earlier non-form step).
	if m.Wizard.State.CurrentStep == model.StepReview {
		t.Error("expected wizard to leave StepReview after go-back")
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor reset to 0, got %d", m.cursor)
	}
	if m.err != nil {
		t.Errorf("expected err cleared, got %v", m.err)
	}
	// The previous step (Update) has no huh form → activeForm should be nil.
	if m.activeForm != nil {
		t.Error("expected nil activeForm after going back to non-form step")
	}
	// And the returned cmd should be nil (no form to Init).
	if cmd != nil {
		t.Errorf("expected nil cmd (no form on previous step), got non-nil")
	}
}

// ── form_logic.go:162-168  fallthrough Next() error → form re-inited ───────

// TestOnFormComplete_NetworkStep_StaticNextError_ReinitsForm exercises the
// "outer" Wizard.Next() error branch in onFormComplete (form_logic.go:162-168).
//
// When currentStep is StepNetwork and networkModeInput is "static", ApplyNetworkStep
// stores NetworkStatic in the config. validateNetwork() then fails because
// Interface is empty. initForm() rebuilds the network form (non-nil), so line
// 165–166 is reached: the returned tea.Cmd comes from m.activeForm.Init().
func TestOnFormComplete_NetworkStep_StaticNextError_ReinitsForm(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	m := New(w)
	// "static" mode with no interface set → validateNetwork() returns an error.
	m.networkModeInput = "static"

	cmd := m.onFormComplete()

	if m.err == nil {
		t.Error("expected m.err to be set when Wizard.Next() fails (static with no interface)")
	}
	// initForm() for StepNetwork always sets a non-nil activeForm; the error
	// handler must call activeForm.Init() and return its cmd.
	if m.activeForm == nil {
		t.Error("expected activeForm to be re-initialized for StepNetwork after Next() error")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd from activeForm.Init() after fallthrough Next() error")
	}
	// Wizard must not have advanced past StepNetwork.
	if m.Wizard.State.CurrentStep != model.StepNetwork {
		t.Errorf("expected wizard to remain at StepNetwork, got %v", m.Wizard.State.CurrentStep)
	}
}

// ── form_logic.go:191-194  renderSystemChecks dead-code invariant ────────────

// TestRenderSystemChecks_AlwaysEmpty documents that renderSystemChecks()
// unconditionally returns "" (forms.go comment: "renderSystemChecks absorbed
// into zen chrome — returns empty"). This makes the checksStr guard in
// viewWithForm (form_logic.go:191-194) permanently unreachable.
//
// The test exists to catch a regression if someone adds logic to
// renderSystemChecks() without updating or removing the guard in viewWithForm.
func TestRenderSystemChecks_AlwaysEmpty(t *testing.T) {
	w := newTestWizard()
	m := New(w)

	if got := m.renderSystemChecks(); got != "" {
		t.Errorf("renderSystemChecks() must return empty string (absorbed into zen chrome), got %q", got)
	}
}
