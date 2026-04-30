package tui

import (
"strings"
"testing"

tea "github.com/charmbracelet/bubbletea"

"github.com/castrojo/knuckle/internal/model"
"github.com/castrojo/knuckle/internal/wizard"
)

func newTestWizard() *wizard.Wizard {
return wizard.New(nil, nil, nil)
}

func TestNewModel(t *testing.T) {
w := newTestWizard()
m := New(w)
if m.Wizard != w {
t.Fatal("wizard not set")
}
if m.quitting {
t.Fatal("should not be quitting")
}
}

func TestViewWelcome(t *testing.T) {
w := newTestWizard()
m := New(w)
view := m.View()
if !strings.Contains(view, "Knuckle") {
t.Error("view should contain title")
}
if !strings.Contains(view, "Welcome") {
t.Error("view should contain welcome text")
}
}

func TestViewReview(t *testing.T) {
w := newTestWizard()
w.State.CurrentStep = model.StepReview
w.State.Config.Channel = "stable"
w.State.Config.Hostname = "testhost"
w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda", SizeHuman: "500 GB"}
m := New(w)
view := m.View()
if !strings.Contains(view, "stable") {
t.Error("review should show channel")
}
if !strings.Contains(view, "testhost") {
t.Error("review should show hostname")
}
}

func TestHandleQuit(t *testing.T) {
w := newTestWizard()
m := New(w)
newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
tuiModel := newModel.(*Model)
if !tuiModel.quitting {
t.Error("should be quitting after ctrl+c")
}
if cmd == nil {
t.Error("should return quit cmd")
}
}

func TestHandleEnterAdvances(t *testing.T) {
w := newTestWizard()
m := New(w)
newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
tuiModel := newModel.(*Model)
if tuiModel.Wizard.State.CurrentStep != model.StepNetwork {
t.Errorf("expected StepNetwork, got %v", tuiModel.Wizard.State.CurrentStep)
}
}
