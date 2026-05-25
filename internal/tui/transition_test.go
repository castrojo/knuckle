package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbluefin/knuckle/internal/model"
)

// TestStorageToUserTransition verifies that advancing from Storage (non-form)
// to User (form) correctly initializes the huh form so it renders.
// This was the root cause of the "blank User screen" bug.
func TestStorageToUserTransition(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	w.State.Disks = []model.DiskInfo{{DevPath: "/dev/vda", Model: "QEMU", SizeHuman: "20G"}}

	m := New(w)
	m.cursor = 0

	// Press Enter on Storage step — should advance to User
	newModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newModel.(*Model)

	if m.Wizard.State.CurrentStep != model.StepUser {
		t.Fatalf("expected User step, got %v", m.Wizard.State.CurrentStep)
	}

	// The activeForm MUST be set for form steps
	if m.activeForm == nil {
		t.Fatal("activeForm is nil after transitioning to User step — form won't render")
	}

	// Execute the Init cmd (simulates Bubble Tea runtime)
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			newModel, cmd = m.Update(msg)
			m = newModel.(*Model)
			// Run follow-up commands
			for i := 0; i < 5 && cmd != nil; i++ {
				msg = cmd()
				if msg == nil {
					break
				}
				newModel, cmd = m.Update(msg)
				m = newModel.(*Model)
			}
		}
	}

	// Now the View should contain actual form content, not just whitespace
	view := m.render()
	if !strings.Contains(view, "Hostname") && !strings.Contains(view, "System Identity") {
		t.Errorf("User step should render form fields after init.\nGot view (%d chars):\n%s", len(view), view[:min(500, len(view))])
	}
}

// TestFormStepTransitionsAlwaysInitForm verifies all non-form → form transitions.
func TestFormStepTransitionsAlwaysInitForm(t *testing.T) {
	transitions := []struct {
		from model.WizardStep
		to   model.WizardStep
	}{
		{model.StepStorage, model.StepUser},  // Storage → User
		{model.StepSysext, model.StepUpdate}, // Sysext → Update (non-form → non-form, control)
		{model.StepUpdate, model.StepReview}, // Update → Review (non-form → form)
	}

	for _, tt := range transitions {
		t.Run(tt.from.String()+"→"+tt.to.String(), func(t *testing.T) {
			w := newTestWizard()
			w.State.CurrentStep = tt.from
			w.State.Config.Channel = "stable"
			w.State.Config.Hostname = "test"
			w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
			w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
			w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
			w.State.Disks = []model.DiskInfo{{DevPath: "/dev/vda", Model: "QEMU", SizeHuman: "20G"}}
			m := New(w)
			m.cursor = 0

			newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			m = newModel.(*Model)

			if m.Wizard.State.CurrentStep != tt.to {
				t.Fatalf("expected %v, got %v", tt.to, m.Wizard.State.CurrentStep)
			}

			// For form steps, activeForm must be set
			switch tt.to {
			case model.StepUser, model.StepReview, model.StepWelcome, model.StepNetwork:
				if m.activeForm == nil {
					t.Errorf("activeForm is nil at form step %v — will render blank", tt.to)
				}
			}
		})
	}
}
