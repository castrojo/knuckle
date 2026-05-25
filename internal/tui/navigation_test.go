package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestFullWizardNavigation(t *testing.T) {
	w := newTestWizard()
	w.State.Disks = []model.DiskInfo{
		{DevPath: "/dev/vda", Model: "QEMU", SizeHuman: "50 GB", Transport: "virtio"},
	}
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
	}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA test"}
	w.State.Config.Hostname = "test-node"
	w.State.Config.Channel = "stable"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}

	m := New(w)

	// Form-based steps (Welcome, Network, User) advance via onFormComplete()
	// Simulate form completion for Welcome
	m.Wizard.State.CurrentStep = model.StepWelcome
	cmd := m.onFormComplete()
	_ = cmd

	if m.Wizard.State.CurrentStep != model.StepNetwork {
		t.Fatalf("after Welcome complete: expected Network, got %v", m.Wizard.State.CurrentStep)
	}

	// Simulate Network form complete
	cmd = m.onFormComplete()
	_ = cmd
	if m.Wizard.State.CurrentStep != model.StepStorage {
		t.Fatalf("after Network complete: expected Storage, got %v", m.Wizard.State.CurrentStep)
	}

	// Storage is non-form: press Enter to advance
	m.cursor = 0
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newModel.(*Model)
	if m.Wizard.State.CurrentStep != model.StepUser {
		t.Fatalf("after Storage: expected User, got %v", m.Wizard.State.CurrentStep)
	}

	// Simulate User form complete
	m.usernameInput = "core"
	cmd = m.onFormComplete()
	_ = cmd
	if m.Wizard.State.CurrentStep != model.StepSysext {
		t.Fatalf("after User complete: expected Sysext, got %v", m.Wizard.State.CurrentStep)
	}

	// Sysext is non-form: press Enter
	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newModel.(*Model)
	if m.Wizard.State.CurrentStep != model.StepUpdate {
		t.Fatalf("after Sysext: expected Update, got %v", m.Wizard.State.CurrentStep)
	}

	// Update is non-form: press Enter
	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newModel.(*Model)
	if m.Wizard.State.CurrentStep != model.StepReview {
		t.Fatalf("after Update: expected Review, got %v", m.Wizard.State.CurrentStep)
	}
}

func TestBackNavigation(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage // Non-form step
	m := New(w)

	// Press Esc on non-form step
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepNetwork {
		t.Errorf("expected Network after Esc from Storage, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

func TestIgnitionURLSkipsSteps(t *testing.T) {
	w := newTestWizard()
	w.State.Disks = []model.DiskInfo{
		{DevPath: "/dev/vda", Model: "QEMU", SizeHuman: "50 GB"},
	}
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	m := New(w)

	// Simulate Welcome form complete with IgnitionURL set
	cmd := m.onFormComplete()
	_ = cmd

	if m.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("expected skip to StepStorage with IgnitionURL, got %v", m.Wizard.State.CurrentStep)
	}
}
