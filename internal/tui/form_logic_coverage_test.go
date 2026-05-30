package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// TestOnFormComplete_TailscaleInvalidKey_ReinitsForm covers form_logic.go:125.4,125.14 —
// when Tailscale auth key fails validation, the error is set, initForm() is called, and
// since StepTailscale has a form, the positive branch (return m.activeForm.Init()) is hit.
func TestOnFormComplete_TailscaleInvalidKey_ReinitsForm(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepTailscale
	// Mark tailscale as selected so Previous() won't skip it.
	w.State.Sysexts = []model.SysextEntry{{Name: "tailscale", Selected: true}}
	// Use an AuthKey that fails validate.TailscaleAuthKey (not a valid tskey prefix).
	w.State.Config.Tailscale.AuthKey = "not-a-valid-tskey"
	m := New(w)
	// Sync tailscale input fields from state.
	m.tailscaleAuthKeyIn = w.State.Config.Tailscale.AuthKey
	m.tailscaleModeIn = model.TailscaleModeConnect

	cmd := m.onFormComplete()

	if m.err == nil {
		t.Fatal("expected validation error for invalid tailscale auth key")
		return
	}
	if !strings.Contains(m.err.Error(), "tailscale") {
		t.Errorf("expected tailscale-related error, got: %v", m.err)
	}
	if m.activeForm == nil {
		t.Error("expected non-nil activeForm after reinit at StepTailscale (form_logic.go:125)")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd from activeForm.Init() (form_logic.go:125)")
	}
	if m.Wizard.State.CurrentStep != model.StepTailscale {
		t.Errorf("expected to remain at StepTailscale, got %v", m.Wizard.State.CurrentStep)
	}
}

// TestOnFormComplete_ReviewConfirmedNextError_ReinitsForm covers form_logic.go:164.3,164.13 —
// when Next() fails at a confirmed StepReview (CheckConsistency error), the model reinits
// the Review form and calls Init() directly (no nil guard; Review always has a form).
func TestOnFormComplete_ReviewConfirmedNextError_ReinitsForm(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config.Channel = "stable"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	// Duplicate username triggers validate.CheckConsistency to return an error.
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}},
		{Username: "core", SSHKeys: []string{"ssh-ed25519 BBBB"}},
	}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	w.State.Confirmed = true
	m := New(w)

	cmd := m.onFormComplete()

	if m.err == nil {
		t.Error("expected error from failed Next() (duplicate username)")
	}
	if m.Wizard.State.Confirmed {
		t.Error("expected Confirmed to be reset after Next() error at Review")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd from reinited Review form (form_logic.go:164)")
	}
	if m.Wizard.State.CurrentStep != model.StepReview {
		t.Errorf("expected to remain at StepReview, got %v", m.Wizard.State.CurrentStep)
	}
}

// TestOnFormComplete_Storage_AdvancesToUserWithForm covers form_logic.go:187.22,190.4 —
// the positive branch of the success path when Next() advances to a step that has a form.
// Storage → User is the first transition where the new step (User) has a huh form.
func TestOnFormComplete_Storage_AdvancesToUserWithForm(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Channel = "stable"
	// /dev/vda passes validate.DiskPath (starts with /dev/, no ..)
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	m := New(w)

	cmd := m.onFormComplete()

	if m.err != nil {
		t.Errorf("unexpected error advancing from Storage: %v", m.err)
	}
	if m.Wizard.State.CurrentStep != model.StepUser {
		t.Errorf("expected StepUser after Storage onFormComplete, got %v", m.Wizard.State.CurrentStep)
	}
	if m.activeForm == nil {
		t.Error("expected non-nil activeForm after advancing to StepUser (form_logic.go:187)")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd from activeForm.Init() (form_logic.go:187)")
	}
}
