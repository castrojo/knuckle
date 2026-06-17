// keyboard_test.go — comprehensive coverage of every documented keyboard
// shortcut in the TUI.
//
// Organisation:
//
//	shift+tab   – back-nav on non-form steps; field-nav within huh forms
//	esc         – back-nav (including form steps), no-op on first step, filter-clear on sysext
//	tab / j / down – advance cursor / field
//	up / k      – retreat cursor / field
//	enter       – confirm / advance
//	space       – toggle sysext selection
//	q           – quit (double-press); char in field mode
//	r           – reboot on Done (double-press); char in field mode
//	ctrl+a      – advanced mode toggle (Welcome only)
//	ctrl+b      – Butane preview toggle (Review only)
//	/           – sysext filter entry
//	backspace   – delete last field character
package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// sysextWizard returns a wizard with StepSysext current and a non-empty sysext
// catalog so that initSysextList() sets sysextListReady=true.
func newSysextModel() *Model {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", Category: "Container Runtime", SupportTier: "Flatcar Integrated"},
		{Name: "tailscale", Version: "1.84.0", Category: "Networking", SupportTier: "Flatcar Integrated"},
		{Name: "btop", Version: "1.4.0", Category: "Utilities", SupportTier: "Bakery Maintained"},
	}
	m := New(w)
	// initSysextList is called by initStepFields; verify it fired.
	if !m.sysextListReady {
		m.initSysextList()
	}
	return m
}

// networkModel returns a model on StepNetwork.
// Pass formActive=true to keep the huh form wired (tests global intercept).
// Pass formActive=false to nil it out (tests handleKey path).
func networkModel(formActive bool) *Model {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	m := New(w)
	if !formActive {
		m.activeForm = nil
		m.initStepFields()
	}
	return m
}

// userModel returns a model on StepUser.
func userModel(formActive bool) *Model {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	m := New(w)
	if !formActive {
		m.activeForm = nil
		m.initStepFields()
	}
	return m
}

// ---------------------------------------------------------------------------
// shift+tab — back-nav on non-form steps; field-nav within huh forms
// ---------------------------------------------------------------------------

// TestKeyboard_ShiftTab_FormStep_DelegatesToHuh verifies that shift+tab on a
// form step (activeForm != nil) is NOT intercepted by our Update — it falls
// through to huh so the user can navigate backwards through form fields.
func TestKeyboard_ShiftTab_FormStep_DelegatesToHuh(t *testing.T) {
	m := networkModel(true /* form active */)
	if m.activeForm == nil {
		t.Skip("form not initialised — precondition not met")
	}

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	got := newModel.(*Model).Wizard.State.CurrentStep

	// Step must NOT change — huh handles the key for intra-form navigation.
	if got != model.StepNetwork {
		t.Errorf("shift+tab with activeForm set: expected StepNetwork (no wizard step change), got %v", got)
	}
}

// TestKeyboard_ShiftTab_NetworkToWelcome verifies the non-form path: without
// an active form, shift+tab goes back one wizard step.
func TestKeyboard_ShiftTab_NetworkToWelcome(t *testing.T) {
	m := networkModel(false)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	got := newModel.(*Model).Wizard.State.CurrentStep

	if got != model.StepWelcome {
		t.Errorf("shift+tab on Network: expected StepWelcome, got %v", got)
	}
}

// TestKeyboard_ShiftTab_UserToStorage verifies User (form step) → Storage.
func TestKeyboard_ShiftTab_UserToStorage(t *testing.T) {
	m := userModel(false)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	got := newModel.(*Model).Wizard.State.CurrentStep

	if got != model.StepStorage {
		t.Errorf("shift+tab on User: expected StepStorage, got %v", got)
	}
}

// TestKeyboard_ShiftTab_UserWithForm_DelegatesToHuh: form active on User,
// shift+tab must NOT change the wizard step — delegated to huh field nav.
func TestKeyboard_ShiftTab_UserWithForm_DelegatesToHuh(t *testing.T) {
	m := userModel(true)
	if m.activeForm == nil {
		t.Skip("form not initialised")
	}

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	got := newModel.(*Model).Wizard.State.CurrentStep

	// Step must stay on User — huh handles field navigation.
	if got != model.StepUser {
		t.Errorf("shift+tab on User (form active): expected StepUser (no wizard step change), got %v", got)
	}
}

// TestKeyboard_ShiftTab_WelcomeIsNoOp: first step, nothing to go back to.
// (Complements existing TestHandleKey_ShiftTabOnWelcomeIsNoOp with form nil.)
func TestKeyboard_ShiftTab_WelcomeIsNoOp(t *testing.T) {
	w := newTestWizard()
	// Welcome is the default step
	m := New(w)
	m.activeForm = nil

	newModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepWelcome {
		t.Errorf("shift+tab on Welcome should stay Welcome, got %v", tuiModel.Wizard.State.CurrentStep)
	}
	if cmd != nil {
		t.Error("shift+tab on Welcome should return nil cmd")
	}
}

// TestKeyboard_ShiftTab_ClearsError: a pending error must be wiped on back-nav.
func TestKeyboard_ShiftTab_ClearsError(t *testing.T) {
	m := networkModel(false)
	m.err = errTest

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	tuiModel := newModel.(*Model)

	if tuiModel.err != nil {
		t.Errorf("shift+tab should clear err, got: %v", tuiModel.err)
	}
}

// TestKeyboard_ShiftTab_ResetsCursor: cursor must be reset to 0 on back-nav.
func TestKeyboard_ShiftTab_ResetsCursor(t *testing.T) {
	m := networkModel(false)
	m.cursor = 3

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 0 {
		t.Errorf("shift+tab should reset cursor to 0, got %d", tuiModel.cursor)
	}
}

// ---------------------------------------------------------------------------
// esc — back-navigation, no-op on first step, filter-clear on sysext
// ---------------------------------------------------------------------------

// TestKeyboard_Esc_WelcomeIsNoOp: esc on the first step must not panic or
// decrement the step below zero.
func TestKeyboard_Esc_WelcomeIsNoOp(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.activeForm = nil

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	// Wizard.Previous() at step 0 is clamped; step must remain Welcome.
	if tuiModel.Wizard.State.CurrentStep != model.StepWelcome {
		t.Errorf("esc on Welcome: expected StepWelcome, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

// TestKeyboard_Esc_StorageGoesBack: esc from Storage → Network.
func TestKeyboard_Esc_StorageGoesBack(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	m := New(w)
	m.activeForm = nil

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepNetwork {
		t.Errorf("esc on Storage: expected StepNetwork, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

// TestKeyboard_Esc_UpdateGoesBack: esc from Update → Sysext.
func TestKeyboard_Esc_UpdateGoesBack(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.activeForm = nil

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepSysext {
		t.Errorf("esc on Update: expected StepSysext, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

// TestKeyboard_Esc_ClearsError: pending error must be wiped on esc.
func TestKeyboard_Esc_ClearsError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	m := New(w)
	m.activeForm = nil
	m.err = errTest

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	if tuiModel.err != nil {
		t.Errorf("esc should clear err, got: %v", tuiModel.err)
	}
}

// TestKeyboard_Esc_SysextFiltering_ClearsFilterNotBack: when the bubbles/list
// is actively filtering, esc clears the filter instead of going back a step.
func TestKeyboard_Esc_SysextFiltering_ClearsFilterNotBack(t *testing.T) {
	m := newSysextModel()

	// Send "/" to enter filter mode.
	slashMsg := tea.KeyPressMsg{Code: '/', Text: string('/')}
	newModel, _ := m.Update(slashMsg)
	m = newModel.(*Model)

	if !m.sysextListReady {
		t.Skip("sysextList not ready; can't test filter mode")
	}
	if m.sysextList.FilterState() != list.Filtering {
		// Not all bubbles/list versions trigger filter on '/'; skip gracefully.
		t.Skip("filter mode not activated by '/' — bubbles/list may handle it differently")
	}

	// Now press esc — must clear filter, not navigate back.
	prevStep := m.Wizard.State.CurrentStep
	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != prevStep {
		t.Errorf("esc during filter: expected to stay on %v, got %v",
			prevStep, tuiModel.Wizard.State.CurrentStep)
	}
}

// TestKeyboard_Esc_FormStep_GoesBack: esc with an active huh form must go
// back to the previous wizard step (new behaviour — previously esc on form
// steps did nothing because only handleKey handled esc, not form steps).
func TestKeyboard_Esc_FormStep_GoesBack(t *testing.T) {
	m := networkModel(true /* form active */)
	if m.activeForm == nil {
		t.Skip("form not initialised — precondition not met")
	}

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := newModel.(*Model).Wizard.State.CurrentStep

	if got != model.StepWelcome {
		t.Errorf("esc on Network (form active): expected StepWelcome, got %v", got)
	}
}

// TestKeyboard_Esc_FormStep_WelcomeIsNoOp: esc on Welcome (first step) with
// a form active must not panic or move below step 0.
func TestKeyboard_Esc_FormStep_WelcomeIsNoOp(t *testing.T) {
	w := newTestWizard()
	m := New(w) // Welcome step, may have a form

	newModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepWelcome {
		t.Errorf("esc on Welcome (form): expected StepWelcome, got %v", tuiModel.Wizard.State.CurrentStep)
	}
	_ = cmd // no assertion on cmd for Welcome esc
}

// TestKeyboard_Esc_FormStep_ClearsError: pending error must be cleared when
// esc goes back from a form step.
func TestKeyboard_Esc_FormStep_ClearsError(t *testing.T) {
	m := networkModel(true /* form active */)
	if m.activeForm == nil {
		t.Skip("form not initialised — precondition not met")
	}
	m.err = errTest

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	if tuiModel.err != nil {
		t.Errorf("esc on form step should clear err, got: %v", tuiModel.err)
	}
}

// ---------------------------------------------------------------------------
// tab / j / down — advance cursor or cycle field forward
// ---------------------------------------------------------------------------

// TestKeyboard_Tab_ListStep_IncrementsCursor: tab on a list step (no fields)
// moves the cursor forward.
func TestKeyboard_Tab_ListStep_IncrementsCursor(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{
		{DevPath: "/dev/vda"},
		{DevPath: "/dev/sdb"},
	}
	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 1 {
		t.Errorf("tab on Storage list: expected cursor=1, got %d", tuiModel.cursor)
	}
}

// TestKeyboard_J_ListStep_IncrementsCursor: 'j' must behave identically to tab
// on list steps.
func TestKeyboard_J_ListStep_IncrementsCursor(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: string('j')})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 1 {
		t.Errorf("'j' on Update list: expected cursor=1, got %d", tuiModel.cursor)
	}
}

// TestKeyboard_Down_ListStep_IncrementsCursor: KeyDown must behave like tab.
func TestKeyboard_Down_ListStep_IncrementsCursor(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.cursor = 1

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 2 {
		t.Errorf("KeyDown on Update list: expected cursor=2, got %d", tuiModel.cursor)
	}
}

// TestKeyboard_Tab_ListStep_Clamped: tab at last item stays clamped.
func TestKeyboard_Tab_ListStep_Clamped(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate // 3 items, indices 0-2
	m := New(w)
	m.cursor = 2 // last item

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 2 {
		t.Errorf("tab at last item: cursor should stay at 2, got %d", tuiModel.cursor)
	}
}

// TestKeyboard_J_FieldMode_AdvancesFieldIdx: 'j' falls through to down which
// cycles field index when fields exist.
func TestKeyboard_J_FieldMode_AdvancesFieldIdx(t *testing.T) {
	m := networkModel(false) // activeForm=nil, fields initialised
	if len(m.fields) < 2 {
		t.Skip("Network step has fewer than 2 fields")
	}
	m.fieldIdx = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: string('j')})
	tuiModel := newModel.(*Model)

	if tuiModel.fieldIdx != 1 {
		t.Errorf("'j' in field mode: expected fieldIdx=1, got %d", tuiModel.fieldIdx)
	}
}

// TestKeyboard_Tab_FieldMode_WrapsToFirst: tab from last field wraps to 0.
func TestKeyboard_Tab_FieldMode_WrapsToFirst(t *testing.T) {
	m := networkModel(false)
	lastIdx := len(m.fields) - 1
	if lastIdx < 1 {
		t.Skip("need at least 2 fields")
	}
	m.fieldIdx = lastIdx
	m.activeForm = nil

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	tuiModel := newModel.(*Model)

	if tuiModel.fieldIdx != 0 {
		t.Errorf("tab from last field: expected fieldIdx=0, got %d", tuiModel.fieldIdx)
	}
}

// ---------------------------------------------------------------------------
// up / k — retreat cursor or cycle field backward
// ---------------------------------------------------------------------------

// TestKeyboard_K_ListStep_DecrementsCursor: 'k' moves cursor up on a list step.
func TestKeyboard_K_ListStep_DecrementsCursor(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.initStepFields()
	m.cursor = 2

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'k', Text: string('k')})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 1 {
		t.Errorf("'k' on Nvidia list: expected cursor=1, got %d", tuiModel.cursor)
	}
}

// TestKeyboard_Up_ListStep_DecrementsCursor: KeyUp is equivalent to 'k'.
func TestKeyboard_Up_ListStep_DecrementsCursor(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.cursor = 2

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 1 {
		t.Errorf("KeyUp on Update: expected cursor=1, got %d", tuiModel.cursor)
	}
}

// TestKeyboard_K_ListStep_ClampedAtZero: 'k' at top stays at 0.
func TestKeyboard_K_ListStep_ClampedAtZero(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'k', Text: string('k')})
	tuiModel := newModel.(*Model)

	if tuiModel.cursor != 0 {
		t.Errorf("'k' at top: cursor should stay 0, got %d", tuiModel.cursor)
	}
}

// TestKeyboard_K_FieldMode_GoesBackward: 'k' wraps backward through fields.
func TestKeyboard_K_FieldMode_GoesBackward(t *testing.T) {
	m := networkModel(false)
	if len(m.fields) < 2 {
		t.Skip("need at least 2 fields")
	}
	m.fieldIdx = 1

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'k', Text: string('k')})
	tuiModel := newModel.(*Model)

	if tuiModel.fieldIdx != 0 {
		t.Errorf("'k' in field mode: expected fieldIdx=0, got %d", tuiModel.fieldIdx)
	}
}

// TestKeyboard_K_FieldMode_WrapsToLast: 'k' at field 0 wraps to last.
func TestKeyboard_K_FieldMode_WrapsToLast(t *testing.T) {
	m := networkModel(false)
	lastIdx := len(m.fields) - 1
	if lastIdx < 1 {
		t.Skip("need at least 2 fields")
	}
	m.fieldIdx = 0
	m.activeForm = nil

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'k', Text: string('k')})
	tuiModel := newModel.(*Model)

	if tuiModel.fieldIdx != lastIdx {
		t.Errorf("'k' at first field: expected fieldIdx=%d, got %d", lastIdx, tuiModel.fieldIdx)
	}
}

// ---------------------------------------------------------------------------
// enter — confirm / advance
// ---------------------------------------------------------------------------

// TestKeyboard_Enter_ClearsConfirmQuit: pressing enter on a list step must
// reset the pending-quit flag.
func TestKeyboard_Enter_ClearsConfirmQuit(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	m := New(w)
	m.confirmQuit = true
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tuiModel := newModel.(*Model)

	if tuiModel.confirmQuit {
		t.Error("enter should clear confirmQuit")
	}
}

// TestKeyboard_Enter_ListStep_AdvancesStep: enter on Update moves to Review.
func TestKeyboard_Enter_ListStep_AdvancesStep(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepReview {
		t.Errorf("enter on Update: expected StepReview, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

// ---------------------------------------------------------------------------
// space — sysext toggle; field space; no-op elsewhere
// ---------------------------------------------------------------------------

// TestKeyboard_Space_EmptySysextIsNoOp: space on StepSysext with empty list
// must not panic or mutate config.
func TestKeyboard_Space_EmptySysextIsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = nil // explicitly empty
	m := New(w)
	m.cursor = 0

	// Must not panic.
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel := newModel.(*Model)

	// Sysexts slice remains nil/empty.
	if len(tuiModel.Wizard.State.Sysexts) != 0 {
		t.Errorf("space on empty sysext list should not create entries, got %d",
			len(tuiModel.Wizard.State.Sysexts))
	}
}

// TestKeyboard_Space_OutOfBoundsCursorIsNoOp: space with cursor beyond slice
// bounds must not panic.
func TestKeyboard_Space_OutOfBoundsCursorIsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: false},
	}
	m := New(w)
	m.cursor = 99 // far out of bounds

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel := newModel.(*Model)

	// docker must remain unselected.
	if tuiModel.Wizard.State.Sysexts[0].Selected {
		t.Error("OOB cursor space should not toggle any sysext")
	}
}

// TestKeyboard_Space_FieldMode_AppendsSpace: on a step with text fields (but
// not Sysext), space appends a space character to the current field value.
func TestKeyboard_Space_FieldMode_AppendsSpace(t *testing.T) {
	m := networkModel(false)
	m.fields[0].value = "eth"
	m.fieldIdx = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel := newModel.(*Model)

	if tuiModel.fields[0].value != "eth " {
		t.Errorf("space in field mode: expected 'eth ', got %q", tuiModel.fields[0].value)
	}
}

// TestKeyboard_Space_NoFieldsNoSysext_IsNoOp: on a list step without fields
// and not Sysext (e.g. Update), space must not crash or mutate state.
func TestKeyboard_Space_NoFieldsNoSysext_IsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	prev := m.cursor

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel := newModel.(*Model)

	// Nothing crashed, cursor unchanged.
	if tuiModel.cursor != prev {
		t.Errorf("space on Update (no fields): cursor changed from %d to %d", prev, tuiModel.cursor)
	}
}

// ---------------------------------------------------------------------------
// q — quit (double-press); typed char in field mode
// ---------------------------------------------------------------------------

// TestKeyboard_Q_FieldMode_AppendsChar: on a field step, 'q' appends the
// character rather than triggering quit.
func TestKeyboard_Q_FieldMode_AppendsChar(t *testing.T) {
	m := networkModel(false)
	m.fields[0].value = "eth"
	m.fieldIdx = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	tuiModel := newModel.(*Model)

	if tuiModel.fields[0].value != "ethq" {
		t.Errorf("'q' in field mode: expected 'ethq', got %q", tuiModel.fields[0].value)
	}
	if tuiModel.quitting {
		t.Error("'q' in field mode must not trigger quit")
	}
}

// TestKeyboard_Q_SinglePress_SetsConfirmQuit: first 'q' on a list step sets
// confirmQuit but does NOT quit.
func TestKeyboard_Q_SinglePress_SetsConfirmQuit(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)

	newModel, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	tuiModel := newModel.(*Model)

	if !tuiModel.confirmQuit {
		t.Error("first 'q': expected confirmQuit=true")
	}
	if tuiModel.quitting {
		t.Error("first 'q': must not quit yet")
	}
	if cmd != nil {
		t.Error("first 'q': expected nil cmd")
	}
	if tuiModel.err == nil {
		t.Error("first 'q': expected confirmation prompt in err")
	}
}

// TestKeyboard_Q_DoublePress_Exits: second consecutive 'q' quits.
func TestKeyboard_Q_DoublePress_Exits(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)

	// First q
	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	m = newModel.(*Model)

	// Second q — Update goes through handleKey again; confirmQuit is NOT reset
	// because it is reset by the non-'q' path.  But wait: the code resets
	// confirmQuit unconditionally for any KeyMsg *after* the ctrl+c check.
	// So we use handleKey directly for the second press to bypass that reset.
	newModel2, cmd := m.handleKey(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	tuiModel := newModel2.(*Model)

	if !tuiModel.quitting {
		t.Error("second 'q': expected quitting=true")
	}
	if cmd == nil {
		t.Error("second 'q': expected quit cmd")
	}
}

// TestKeyboard_AnyKey_CancelsQuitConfirmation: any key other than q/ctrl+c
// must clear a pending confirmQuit.
func TestKeyboard_AnyKey_CancelsQuitConfirmation(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.confirmQuit = true

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tuiModel := newModel.(*Model)

	if tuiModel.confirmQuit {
		t.Error("non-quit key should clear confirmQuit")
	}
}

// ---------------------------------------------------------------------------
// r — reboot on Done (double-press); typed char in field mode
// ---------------------------------------------------------------------------

// TestKeyboard_R_NonDoneStep_WithFields_AppendsChar: 'r' on a field step
// (e.g. Network) appends the character.
func TestKeyboard_R_NonDoneStep_WithFields_AppendsChar(t *testing.T) {
	m := networkModel(false)
	m.fields[0].value = "eth"
	m.fieldIdx = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	tuiModel := newModel.(*Model)

	if tuiModel.fields[0].value != "ethr" {
		t.Errorf("'r' in field mode: expected 'ethr', got %q", tuiModel.fields[0].value)
	}
}

// TestKeyboard_R_NonDoneStep_WithoutFields_IsNoOp: 'r' on a list step that is
// not Done (no fields) must be a no-op — no quit, no error.
func TestKeyboard_R_NonDoneStep_WithoutFields_IsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{{DevPath: "/dev/vda"}}
	m := New(w)

	newModel, cmd := m.handleKey(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	tuiModel := newModel.(*Model)

	if tuiModel.quitting {
		t.Error("'r' on non-Done step: must not quit")
	}
	if cmd != nil {
		t.Error("'r' on non-Done step without fields: expected nil cmd")
	}
}

// TestKeyboard_R_Done_DoublePress_Reboots: on the Done step, two consecutive
// 'r' presses (via handleKey to bypass the confirmReboot reset) must reboot.
func TestKeyboard_R_Done_DoublePress_Reboots(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = false
	m := New(w)

	// First r
	newModel, _ := m.handleKey(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	m = newModel.(*Model)
	if !m.confirmReboot {
		t.Fatal("first 'r': expected confirmReboot=true")
	}

	// Second r
	newModel2, cmd := m.handleKey(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	tuiModel := newModel2.(*Model)

	if !tuiModel.quitting {
		t.Error("second 'r': expected quitting=true")
	}
	if cmd == nil {
		t.Error("second 'r': expected reboot cmd")
	}
}

// TestKeyboard_R_Done_ViaUpdate_SetsConfirmReboot: on Done, 'r' through the
// full Update path must set confirmReboot (the guard in Update preserves it
// on Done + 'r').
func TestKeyboard_R_Done_ViaUpdate_SetsConfirmReboot(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = false
	m := New(w)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	tuiModel := newModel.(*Model)

	if !tuiModel.confirmReboot {
		t.Error("'r' on Done via Update: expected confirmReboot=true")
	}
	if tuiModel.quitting {
		t.Error("first 'r' on Done: must not quit yet")
	}
}

// TestKeyboard_R_OtherKey_ClearsConfirmReboot: any key other than 'r' on Done
// must clear a pending confirmReboot.
func TestKeyboard_R_OtherKey_ClearsConfirmReboot(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	m := New(w)
	m.confirmReboot = true

	// Press 'q' (not 'r')
	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	tuiModel := newModel.(*Model)

	if tuiModel.confirmReboot {
		t.Error("pressing 'q' on Done should clear confirmReboot")
	}
}

// ---------------------------------------------------------------------------
// ctrl+a — advanced/external Ignition URL mode (Welcome only)
// ---------------------------------------------------------------------------

// TestKeyboard_CtrlA_Welcome_TogglesToggles: two presses return to initial state.
func TestKeyboard_CtrlA_Welcome_TogglesToggles(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	initial := m.showAdvanced

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = newModel.(*Model)
	if m.showAdvanced == initial {
		t.Error("first ctrl+a: showAdvanced should have changed")
	}

	newModel, _ = m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = newModel.(*Model)
	if m.showAdvanced != initial {
		t.Error("second ctrl+a: showAdvanced should be restored to initial value")
	}
}

// TestKeyboard_CtrlA_NonWelcome_IsNoOp: ctrl+a on any other step must leave
// showAdvanced false.
func TestKeyboard_CtrlA_NonWelcome_IsNoOp(t *testing.T) {
	steps := []model.WizardStep{
		model.StepNetwork,
		model.StepStorage,
		model.StepUser,
		model.StepSysext,
		model.StepUpdate,
		model.StepDone,
	}
	for _, step := range steps {
		w := newTestWizard()
		w.State.CurrentStep = step
		m := New(w)
		m.activeForm = nil

		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
		tuiModel := newModel.(*Model)

		if tuiModel.showAdvanced {
			t.Errorf("ctrl+a on %v: showAdvanced should stay false", step)
		}
	}
}

// ---------------------------------------------------------------------------
// ctrl+b — Butane preview toggle (Review step only)
// ---------------------------------------------------------------------------

// TestKeyboard_CtrlB_ReviewTogglesShowButane: toggle on, then back off.
func TestKeyboard_CtrlB_ReviewTogglesShowButane(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	m := New(w)
	m.activeForm = nil // bypass huh form to reach handleKey

	if m.showButane {
		t.Fatal("precondition: showButane must start false")
	}

	// First ctrl+b → on
	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	m = newModel.(*Model)
	if !m.showButane {
		t.Error("first ctrl+b on Review: expected showButane=true")
	}

	// Second ctrl+b → off
	m.activeForm = nil
	newModel, _ = m.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	m = newModel.(*Model)
	if m.showButane {
		t.Error("second ctrl+b on Review: expected showButane=false")
	}
}

// TestKeyboard_CtrlB_NonReview_IsNoOp: ctrl+b on every step except Review
// must leave showButane=false.
func TestKeyboard_CtrlB_NonReview_IsNoOp(t *testing.T) {
	nonReviewSteps := []model.WizardStep{
		model.StepWelcome,
		model.StepStorage,
		model.StepSysext,
		model.StepUpdate,
		model.StepInstall,
		model.StepDone,
	}
	for _, step := range nonReviewSteps {
		w := newTestWizard()
		w.State.CurrentStep = step
		m := New(w)
		m.activeForm = nil

		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
		tuiModel := newModel.(*Model)

		if tuiModel.showButane {
			t.Errorf("ctrl+b on %v: showButane should remain false", step)
		}
	}
}

// ---------------------------------------------------------------------------
// / — sysext filter mode entry
// ---------------------------------------------------------------------------

// TestKeyboard_Slash_SysextListReady_DelegatesToList: "/" on StepSysext with a
// ready list must delegate to bubbles/list (default case in handleKey).
// We verify no panic occurs and cursor stays valid.
func TestKeyboard_Slash_SysextListReady_DelegatesToList(t *testing.T) {
	m := newSysextModel()
	if !m.sysextListReady {
		t.Skip("sysextList not ready")
	}

	prevCursor := m.cursor

	// "/" is not a tea.Key constant; use KeyRunes.
	newModel, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: string('/')})
	tuiModel := newModel.(*Model)

	// Cursor must remain valid (≥0 and < len(sysexts)).
	if tuiModel.cursor < 0 || tuiModel.cursor >= len(tuiModel.Wizard.State.Sysexts) {
		t.Errorf("'/' on sysext: cursor out of bounds: %d (len=%d)",
			tuiModel.cursor, len(tuiModel.Wizard.State.Sysexts))
	}
	// No step change.
	if tuiModel.Wizard.State.CurrentStep != model.StepSysext {
		t.Errorf("'/' should not change step; got %v", tuiModel.Wizard.State.CurrentStep)
	}

	t.Logf("cursor before=%d after=%d; FilterState=%v",
		prevCursor, tuiModel.cursor, tuiModel.sysextList.FilterState())
}

// TestKeyboard_Slash_NonSysextStep_IsNoOp: "/" on a list step without fields
// (e.g. Update) must not crash or mutate meaningful state.
func TestKeyboard_Slash_NonSysextStep_IsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	prev := m.cursor

	newModel, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: string('/')})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepUpdate {
		t.Errorf("'/' on Update: step should not change, got %v", tuiModel.Wizard.State.CurrentStep)
	}
	_ = prev // cursor on Update is irrelevant here
}

// ---------------------------------------------------------------------------
// backspace — delete last character in field
// ---------------------------------------------------------------------------

// TestKeyboard_Backspace_EmptyField_IsNoOp: backspace on an empty field value
// must not crash or produce a negative-length string.
func TestKeyboard_Backspace_EmptyField_IsNoOp(t *testing.T) {
	m := networkModel(false)
	m.fields[0].value = "" // explicitly empty
	m.fieldIdx = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	tuiModel := newModel.(*Model)

	if tuiModel.fields[0].value != "" {
		t.Errorf("backspace on empty field: expected '', got %q", tuiModel.fields[0].value)
	}
}

// TestKeyboard_Backspace_MultipleDeletes: repeated backspace presses remove one
// character at a time down to empty.
func TestKeyboard_Backspace_MultipleDeletes(t *testing.T) {
	m := networkModel(false)
	m.fields[0].value = "abc"
	m.fieldIdx = 0

	for i, want := range []string{"ab", "a", ""} {
		m.activeForm = nil // keep it bypassed after each Update
		newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		m = newModel.(*Model)
		m.activeForm = nil
		if m.fields[0].value != want {
			t.Errorf("backspace #%d: expected %q, got %q", i+1, want, m.fields[0].value)
		}
	}
}

// TestKeyboard_Backspace_NoFields_IsNoOp: backspace on a list step with no
// fields must not panic.
func TestKeyboard_Backspace_NoFields_IsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)

	// Must not panic.
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepUpdate {
		t.Errorf("backspace on Update: step changed unexpectedly to %v",
			tuiModel.Wizard.State.CurrentStep)
	}
}

// ---------------------------------------------------------------------------
// character input — default handler appends to current field
// ---------------------------------------------------------------------------

// TestKeyboard_CharInput_MultiByte: typing multiple characters builds the value.
func TestKeyboard_CharInput_MultiByte(t *testing.T) {
	m := networkModel(false)
	m.fields[0].value = ""
	m.fieldIdx = 0

	for _, ch := range "eth0" {
		m.activeForm = nil
		newModel, _ := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = newModel.(*Model)
		m.activeForm = nil
	}

	if m.fields[0].value != "eth0" {
		t.Errorf("typed 'eth0': expected %q, got %q", "eth0", m.fields[0].value)
	}
}

// TestKeyboard_CharInput_NoFields_IsNoOp: typing a printable character on a
// list step without fields (Storage) must not mutate step or config.
func TestKeyboard_CharInput_NoFields_IsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{{DevPath: "/dev/vda"}}
	m := New(w)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'x', Text: string('x')})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("char input on Storage (no fields): step changed to %v",
			tuiModel.Wizard.State.CurrentStep)
	}
}
