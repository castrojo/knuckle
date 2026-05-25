package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestMergeKeysVariadic(t *testing.T) {
	a := []string{"ssh-ed25519 AAAA local@host"}
	b := []string{"ssh-rsa BBBB manual@paste"}
	c := []string{"ssh-ed25519 CCCC github@user", "ssh-ed25519 AAAA local@host"} // dup

	result := mergeKeys(a, b, c)

	if len(result) != 3 {
		t.Fatalf("expected 3 unique keys, got %d: %v", len(result), result)
	}
	// Order: local, manual, github (deduped)
	want := []string{
		"ssh-ed25519 AAAA local@host",
		"ssh-rsa BBBB manual@paste",
		"ssh-ed25519 CCCC github@user",
	}
	for i, k := range want {
		if result[i] != k {
			t.Errorf("result[%d] = %q, want %q", i, result[i], k)
		}
	}
}

func TestMergeKeysPreservesManualOnGitHubFetch(t *testing.T) {
	// Simulates what happens when fetchKeysMsg arrives:
	// local keys + manual pasted keys + GitHub keys should all be preserved
	localKeys := []string{"ssh-ed25519 AAAA local@host"}
	manualKeys := splitSSHKeys("ssh-rsa BBBB manual@paste;ssh-ed25519 DDDD second@manual")
	githubKeys := []string{"ssh-ed25519 CCCC github@user"}

	result := mergeKeys(localKeys, manualKeys, githubKeys)

	if len(result) != 4 {
		t.Fatalf("expected 4 keys, got %d: %v", len(result), result)
	}

	// Verify manual keys survived
	found := map[string]bool{}
	for _, k := range result {
		found[k] = true
	}
	for _, mk := range manualKeys {
		if !found[mk] {
			t.Errorf("manual key %q was dropped after GitHub fetch", mk)
		}
	}
}

// ── handleKey: ctrl+c, q, r edge cases ───────────────────────────────────────

func TestHandleKey_CtrlC_FirstPress(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	newModel, _ := m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	got := newModel.(*Model)
	if !got.confirmQuit {
		t.Error("first ctrl+c in handleKey should set confirmQuit")
	}
	if got.err == nil {
		t.Error("expected error message for ctrl+c first press")
	}
}

func TestHandleKey_CtrlC_SecondPress_Quits(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.confirmQuit = true
	newModel, cmd := m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	got := newModel.(*Model)
	if !got.quitting {
		t.Error("second ctrl+c in handleKey should set quitting")
	}
	if cmd == nil {
		t.Error("expected quit cmd from second ctrl+c")
	}
}

func TestHandleKey_Q_NoFields_FirstPress(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	m := New(w)
	m.fields = nil // no fields → q triggers confirm-quit
	newModel, _ := m.handleKey(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	got := newModel.(*Model)
	if !got.confirmQuit {
		t.Error("first q with no fields should set confirmQuit")
	}
}

func TestHandleKey_Q_NoFields_SecondPress_Quits(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.fields = nil
	m.confirmQuit = true
	newModel, cmd := m.handleKey(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	got := newModel.(*Model)
	if !got.quitting {
		t.Error("second q with no fields should set quitting")
	}
	if cmd == nil {
		t.Error("expected quit cmd from second q")
	}
}

func TestHandleKey_Q_WithFields_AppendsChar(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	m := New(w)
	m.fields = []field{{label: "Disk", value: ""}}
	m.fieldIdx = 0
	m.handleKey(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	if m.fields[0].value != "q" {
		t.Errorf("q in field mode should append 'q', got %q", m.fields[0].value)
	}
}

func TestHandleKey_R_NonDoneStep_NoFields(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome // not Done
	m := New(w)
	m.fields = nil // no field → r falls through to return nil
	newModel, cmd := m.handleKey(tea.KeyPressMsg{Code: 'r', Text: string('r')})
	if newModel == nil {
		t.Fatal("handleKey returned nil model")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd for 'r' on non-Done step with no fields, got %v", cmd)
	}
}

func TestHandleKey_Up_WithFields_WrapsIndex(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.fields = []field{{label: "A"}, {label: "B"}, {label: "C"}}
	m.fieldIdx = 0 // already at top
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.fieldIdx != len(m.fields)-1 {
		t.Errorf("up at fieldIdx=0 should wrap to %d, got %d", len(m.fields)-1, m.fieldIdx)
	}
}
