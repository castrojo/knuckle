package tui

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// Tests for the tea.Model View() method. Existing tests call m.render()
// directly; these tests exercise the View() wrapper itself to cover the
// tea.NewView call and the AltScreen assignment.

func TestView_ReturnsNonNilView(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	v := m.View()
	if v.Content == "" {
		t.Error("View().Content must not be empty for a default model")
	}
	if !v.AltScreen {
		t.Error("View().AltScreen must be true")
	}
}

func TestView_AltScreenAlwaysTrue(t *testing.T) {
	// AltScreen must be true regardless of model state.
	steps := []model.WizardStep{
		model.StepWelcome,
		model.StepNetwork,
		model.StepStorage,
		model.StepUser,
		model.StepReview,
	}
	for _, step := range steps {
		w := newTestWizard()
		w.State.CurrentStep = step
		m := New(w)
		v := m.View()
		if !v.AltScreen {
			t.Errorf("step %v: View().AltScreen = false, want true", step)
		}
	}
}

func TestView_ContentMatchesRender(t *testing.T) {
	// View().Content must equal what render() returns.
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	want := m.render()
	v := m.View()
	if v.Content != want {
		t.Errorf("View().Content does not match render():\ngot:  %q\nwant: %q", v.Content, want)
	}
}

func TestView_QuittingState(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.quitting = true
	v := m.View()
	if v.Content != "Installation cancelled.\n" {
		t.Errorf("quitting state: View().Content = %q, want %q",
			v.Content, "Installation cancelled.\n")
	}
	if !v.AltScreen {
		t.Error("quitting state: View().AltScreen must still be true")
	}
}
