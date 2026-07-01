package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestUpdate_WindowSizeMsg(t *testing.T) {
	w := newTestWizard()
	m := New(w)

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	tuiModel := newModel.(*Model)

	if tuiModel.width != 120 || tuiModel.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", tuiModel.width, tuiModel.height)
	}
}

func TestUpdate_InstallDoneMsg_Success(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	m := New(w)
	m.installing = true

	newModel, _ := m.Update(installDoneMsg{err: nil})
	tuiModel := newModel.(*Model)

	if tuiModel.installing {
		t.Error("expected installing=false after installDoneMsg")
	}
	if tuiModel.Wizard.State.CurrentStep != model.StepDone {
		t.Errorf("expected StepDone, got %v", tuiModel.Wizard.State.CurrentStep)
	}
	if tuiModel.err != nil {
		t.Errorf("unexpected error: %v", tuiModel.err)
	}
}

func TestUpdate_InstallDoneMsg_Error(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	m := New(w)
	m.installing = true

	newModel, _ := m.Update(installDoneMsg{err: errTest})
	tuiModel := newModel.(*Model)

	if tuiModel.installing {
		t.Error("expected installing=false after error")
	}
	if tuiModel.err == nil {
		t.Error("expected error to be set")
	}
	// Should NOT advance to Done on error
	if tuiModel.Wizard.State.CurrentStep != model.StepInstall {
		t.Errorf("expected to stay on Install step, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

func TestUpdate_FetchKeysMsg_Success(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	w.State.Config.SSHKeys = []string{}
	m := New(w)
	m.fetching = true

	keys := []string{"ssh-ed25519 AAAA github@key"}
	newModel, _ := m.Update(fetchKeysMsg{keys: keys, err: nil})
	tuiModel := newModel.(*Model)

	if tuiModel.fetching {
		t.Error("expected fetching=false")
	}
	if tuiModel.err != nil {
		t.Errorf("unexpected error: %v", tuiModel.err)
	}
	// Should have advanced past User step
	if tuiModel.Wizard.State.CurrentStep == model.StepUser {
		t.Error("expected to advance past User step after key fetch")
	}
	// Keys should be applied
	if len(tuiModel.Wizard.State.Config.Users[0].SSHKeys) == 0 {
		t.Error("expected SSH keys to be merged into user config")
	}
}

func TestUpdate_FetchKeysMsg_Error(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	m := New(w)
	m.fetching = true

	newModel, _ := m.Update(fetchKeysMsg{keys: nil, err: errTest})
	tuiModel := newModel.(*Model)

	if tuiModel.fetching {
		t.Error("expected fetching=false")
	}
	if tuiModel.err == nil {
		t.Error("expected error to be set")
	}
	if tuiModel.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected to stay on User step, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

func TestUpdate_InstallProgressMsg(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	m := New(w)

	newModel, cmd := m.Update(installProgressMsg("Downloading image..."))
	tuiModel := newModel.(*Model)

	if len(tuiModel.Wizard.State.ProgressMessages) != 1 {
		t.Fatalf("expected 1 progress message, got %d", len(tuiModel.Wizard.State.ProgressMessages))
	}
	if tuiModel.Wizard.State.ProgressMessages[0] != "Downloading image..." {
		t.Errorf("expected 'Downloading image...', got %q", tuiModel.Wizard.State.ProgressMessages[0])
	}
	if cmd == nil {
		t.Error("expected cmd (progress animation + wait)")
	}
}

func TestUpdate_CtrlA_TogglesAdvancedOnWelcome(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	m := New(w)

	if m.showAdvanced {
		t.Error("showAdvanced should start false")
	}

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	tuiModel := newModel.(*Model)
	if !tuiModel.showAdvanced {
		t.Error("expected showAdvanced=true after Ctrl+A")
	}

	newModel, _ = tuiModel.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	tuiModel = newModel.(*Model)
	if tuiModel.showAdvanced {
		t.Error("expected showAdvanced=false after second Ctrl+A")
	}
}

func TestUpdate_CtrlA_NoOpOnOtherSteps(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	m := New(w)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	tuiModel := newModel.(*Model)
	if tuiModel.showAdvanced {
		t.Error("Ctrl+A should not toggle advanced on non-Welcome steps")
	}
}

func TestMaxCursor(t *testing.T) {
	tests := []struct {
		step     model.WizardStep
		setup    func(m *Model)
		expected int
	}{
		{
			step:     model.StepWelcome,
			expected: 3, // OS picker (Flatcar | FCOS | Bluefin Server) — shown first via osSubView
		},
		{
			step: model.StepStorage,
			setup: func(m *Model) {
				m.Wizard.State.Disks = []model.DiskInfo{
					{DevPath: "/dev/vda"},
					{DevPath: "/dev/sda"},
				}
			},
			expected: 2,
		},
		{
			step: model.StepSysext,
			setup: func(m *Model) {
				m.Wizard.State.Sysexts = []model.SysextEntry{
					{Name: "docker"},
					{Name: "tailscale"},
					{Name: "wasmtime"},
				}
			},
			expected: 3,
		},
		{
			step:     model.StepNvidia,
			expected: len(model.NvidiaDriverOptions),
		},
		{
			step:     model.StepUpdate,
			expected: 3,
		},
		{
			step:     model.StepNetwork, // default case
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.step.String(), func(t *testing.T) {
			w := newTestWizard()
			w.State.CurrentStep = tt.step
			m := New(w)
			if tt.setup != nil {
				tt.setup(m)
			}
			got := m.maxCursor()
			if got != tt.expected {
				t.Errorf("maxCursor() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestHandleKey_CursorClamp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate // 3 options
	m := New(w)
	m.cursor = 2

	// Press down — should clamp at max-1
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tuiModel := newModel.(*Model)
	if tuiModel.cursor != 2 {
		t.Errorf("cursor should clamp at 2, got %d", tuiModel.cursor)
	}

	// Press up
	newModel, _ = tuiModel.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	tuiModel = newModel.(*Model)
	if tuiModel.cursor != 1 {
		t.Errorf("cursor should be 1 after up, got %d", tuiModel.cursor)
	}
}

func TestHandleKey_CursorUp_AtZero(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{{DevPath: "/dev/vda"}}
	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	tuiModel := newModel.(*Model)
	if tuiModel.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", tuiModel.cursor)
	}
}

func TestHandleKey_SpaceTogglesSysext(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: false},
		{Name: "tailscale", Selected: false},
	}
	m := New(w)
	m.cursor = 0

	// Space toggles first sysext
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel := newModel.(*Model)
	if !tuiModel.Wizard.State.Sysexts[0].Selected {
		t.Error("expected sysext[0] to be selected")
	}

	// Second space un-toggles
	newModel, _ = tuiModel.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel = newModel.(*Model)
	if tuiModel.Wizard.State.Sysexts[0].Selected {
		t.Error("expected sysext[0] to be deselected")
	}
}

func TestHandleKey_SpaceNvidiaSync(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: false},
	}
	m := New(w)
	m.cursor = 0

	// Toggle nvidia-runtime ON
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel := newModel.(*Model)
	if tuiModel.Wizard.State.Config.NvidiaDriverVersion == "" {
		t.Error("expected NvidiaDriverVersion to be set when nvidia-runtime selected")
	}

	// Toggle nvidia-runtime OFF
	newModel, _ = tuiModel.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tuiModel = newModel.(*Model)
	if tuiModel.Wizard.State.Config.NvidiaDriverVersion != "" {
		t.Error("expected NvidiaDriverVersion cleared when nvidia-runtime deselected")
	}
}

// TestHandleKey_CtrlB_TogglesButane tests Ctrl+B on Review step.
// Review has activeForm, so we nil it to test handleKey directly.
func TestHandleKey_CtrlB_TogglesButane(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	m := New(w)
	m.activeForm = nil

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	tuiModel := newModel.(*Model)
	if !tuiModel.showButane {
		t.Error("expected showButane=true after Ctrl+B on Review")
	}
}

func TestHandleKey_Reboot_DryRun(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = true
	m := New(w)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	tuiModel := newModel.(*Model)
	if tuiModel.err == nil {
		t.Error("expected dry-run reboot message")
	}
	if tuiModel.quitting {
		t.Error("should not quit in dry-run reboot")
	}
}

// TestHandleKey_Reboot_Confirmation_Direct exercises reboot double-press
// through handleKey directly. Note: Update() clears confirmReboot before
// reaching handleKey, making the double-r pattern broken through Update().
// This is a known bug (filed separately).
func TestHandleKey_Reboot_Confirmation_Direct(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = false
	m := New(w)

	// First 'r' via handleKey directly
	newModel, _ := m.handleKey(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	tuiModel := newModel.(*Model)
	if !tuiModel.confirmReboot {
		t.Error("expected confirmReboot=true after first r")
	}

	// Second 'r' via handleKey (bypasses Update's clear)
	newModel, cmd := tuiModel.handleKey(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	tuiModel = newModel.(*Model)
	if !tuiModel.quitting {
		t.Error("expected quitting=true after double r")
	}
	if cmd == nil {
		t.Error("expected quit cmd")
	}
}

func TestHandleKey_Backspace_DeletesFieldChar(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	m := New(w)
	m.activeForm = nil // bypass huh form to test raw field editing
	m.initStepFields()
	m.fields[0].value = "eth0"
	m.fieldIdx = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	tuiModel := newModel.(*Model)
	if tuiModel.fields[0].value != "eth" {
		t.Errorf("expected 'eth' after backspace, got %q", tuiModel.fields[0].value)
	}
}

func TestHandleKey_CharInput_AppendsToField(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	m := New(w)
	m.activeForm = nil // bypass huh form to test raw field editing
	m.initStepFields()
	m.fields[0].value = "et"
	m.fieldIdx = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'h', Text: string('h')})
	tuiModel := newModel.(*Model)
	if tuiModel.fields[0].value != "eth" {
		t.Errorf("expected 'eth' after typing h, got %q", tuiModel.fields[0].value)
	}
}

func TestHandleKey_TabCyclesFields(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	m := New(w)
	m.activeForm = nil // bypass huh form to test raw field editing
	m.initStepFields()
	m.fieldIdx = 0

	// Tab should advance
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	tuiModel := newModel.(*Model)
	if tuiModel.fieldIdx != 1 {
		t.Errorf("expected fieldIdx=1, got %d", tuiModel.fieldIdx)
	}

	// Tab cycles around
	tuiModel.fieldIdx = len(tuiModel.fields) - 1
	tuiModel.activeForm = nil // keep bypassed after Update
	newModel, _ = tuiModel.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	tuiModel = newModel.(*Model)
	if tuiModel.fieldIdx != 0 {
		t.Errorf("expected fieldIdx=0 (wrap), got %d", tuiModel.fieldIdx)
	}
}

func TestHandleKey_ShiftTabGoesBack(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	m := New(w)
	m.activeForm = nil

	// Shift+Tab goes back to previous step
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	tuiModel := newModel.(*Model)
	if tuiModel.Wizard.State.CurrentStep != model.StepNetwork {
		t.Errorf("expected StepNetwork after shift+tab, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

func TestHandleKey_ShiftTabOnWelcomeIsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	m := New(w)
	m.activeForm = nil

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	tuiModel := newModel.(*Model)
	if tuiModel.Wizard.State.CurrentStep != model.StepWelcome {
		t.Errorf("shift+tab on Welcome should be no-op, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

func TestHandleKey_EscGoesBack(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	m := New(w)
	m.err = errTest // set an error to verify it clears

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)
	if tuiModel.err != nil {
		t.Error("expected error cleared on Esc")
	}
	// Should go back one step
	if tuiModel.Wizard.State.CurrentStep == model.StepSysext {
		t.Error("expected to navigate back from Sysext")
	}
}

var errTest = fmt.Errorf("test error")

// ── spinner.TickMsg and progress.FrameMsg ─────────────────────────────────────

func TestUpdate_SpinnerTickMsg(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	// spinner.TickMsg is a concrete type from the bubbles/spinner package;
	// fire it through Update and confirm the model still runs without panic.
	tickMsg := m.spinner.Tick()
	newModel, cmd := m.Update(tickMsg)
	if newModel == nil {
		t.Fatal("Update returned nil model for spinner.TickMsg")
	}
	if cmd == nil {
		t.Error("expected a follow-up cmd from spinner tick")
	}
}

func TestUpdate_ProgressFrameMsg(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	// Simulate a progress.FrameMsg by setting progress in motion then
	// driving the animation one frame.
	_, setCmd := m.Update(m.progress.SetPercent(0.5))
	if setCmd == nil {
		t.Skip("progress.SetPercent returned nil cmd, can't drive frame")
	}
	frameMsg := setCmd()
	newModel, _ := m.Update(frameMsg)
	if newModel == nil {
		t.Fatal("Update returned nil model for progress.FrameMsg")
	}
}

// ── WindowSizeMsg with active form ────────────────────────────────────────────

func TestUpdate_WindowSizeMsg_WithActiveForm(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	m := New(w)
	m.initForm() // sets m.activeForm
	if m.activeForm == nil {
		t.Skip("initForm did not set activeForm for StepNetwork")
	}
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	got := newModel.(*Model)
	if got.width != 200 || got.height != 50 {
		t.Errorf("size not propagated: got %dx%d", got.width, got.height)
	}
}

// ── ctrl+c second-press quits from Update ────────────────────────────────────

func TestUpdate_CtrlC_FirstPress_SetsConfirm(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	newModel, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	got := newModel.(*Model)
	if !got.confirmQuit {
		t.Error("first ctrl+c should set confirmQuit")
	}
}

func TestUpdate_CtrlC_SecondPress_Quits(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.confirmQuit = true
	newModel, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	got := newModel.(*Model)
	if !got.quitting {
		t.Error("second ctrl+c should set quitting")
	}
	if cmd == nil {
		t.Error("second ctrl+c should return a quit cmd")
	}
}
