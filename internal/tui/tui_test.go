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
// First Ctrl+C triggers confirmation prompt
newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
tuiModel := newModel.(*Model)
if tuiModel.quitting {
t.Error("should not quit on first ctrl+c")
}
if tuiModel.confirmQuit == false {
t.Error("should show quit confirmation after first ctrl+c")
}
if cmd != nil {
t.Error("should not return quit cmd on first press")
}
// Second Ctrl+C actually quits
newModel, cmd = tuiModel.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
tuiModel = newModel.(*Model)
if !tuiModel.quitting {
t.Error("should be quitting after second ctrl+c")
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

func TestPasswordFieldMasked(t *testing.T) {
w := newTestWizard()
w.State.CurrentStep = model.StepUser
w.State.Config.Users = []model.UserConfig{{Username: "core"}}
m := New(w)
// Password field is index 3 in StepUser
m.fieldIdx = 3
m.fields[3].value = "secret123"

view := m.View()
if strings.Contains(view, "secret123") {
t.Error("password should be masked in view output")
}
if !strings.Contains(view, "•••••••••") {
t.Error("password should display as bullets")
}
}

func TestMultiSSHKeySplitting(t *testing.T) {
tests := []struct {
name  string
input string
want  int
}{
{"single key", "ssh-ed25519 AAAA test@host", 1},
{"two keys semicolon", "ssh-ed25519 AAAA k1; ssh-rsa BBBB k2", 2},
{"trailing semicolon", "ssh-ed25519 AAAA k1;", 1},
{"empty segments", ";;ssh-ed25519 AAAA k1;;", 1},
{"three keys", "key1;key2;key3", 3},
}
for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
got := splitSSHKeys(tt.input)
if len(got) != tt.want {
t.Errorf("splitSSHKeys(%q) = %d keys, want %d", tt.input, len(got), tt.want)
}
})
}
}

func TestButanePreviewToggle(t *testing.T) {
w := newTestWizard()
w.State.CurrentStep = model.StepReview
w.State.Config.Channel = "stable"
w.State.Config.Hostname = "node1"
w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda", SizeHuman: "100 GB"}
w.State.Config.Users = []model.UserConfig{
{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
}
m := New(w)

// Initially butane preview is hidden
view := m.View()
if !strings.Contains(view, "Press 'b' to preview Butane YAML") {
t.Error("expected butane preview hint before toggle")
}
if strings.Contains(view, "Butane YAML Preview") {
t.Error("butane preview should not show before toggle")
}

// Press 'b' to toggle preview on (confirm field must be empty)
m.fields[0].value = ""
newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
tuiModel := newModel.(*Model)
if !tuiModel.showButane {
t.Error("showButane should be true after pressing 'b'")
}
view = tuiModel.View()
if !strings.Contains(view, "Butane YAML Preview") {
t.Error("expected butane preview to be shown after toggle")
}
if !strings.Contains(view, "variant: flatcar") {
t.Error("expected butane content in preview")
}
}

func TestVersionFieldPropagation(t *testing.T) {
w := newTestWizard()
w.State.CurrentStep = model.StepWelcome
m := New(w)

// Version field is index 1 in StepWelcome
m.fields[1].value = "3815.2.0"
m.applyFields()

if w.State.Config.Version != "3815.2.0" {
t.Errorf("expected version=3815.2.0, got %q", w.State.Config.Version)
}

// Verify it appears in review
w.State.CurrentStep = model.StepReview
w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda", SizeHuman: "100 GB"}
m2 := New(w)
view := m2.View()
if !strings.Contains(view, "3815.2.0") {
t.Error("version should appear in review view")
}
}

func TestTimezoneFieldPropagation(t *testing.T) {
w := newTestWizard()
w.State.CurrentStep = model.StepUser
w.State.Config.Users = []model.UserConfig{{Username: "core"}}
m := New(w)

// Timezone is field index 1 in StepUser
m.fields[1].value = "America/New_York"
m.applyFields()

if w.State.Config.Timezone != "America/New_York" {
t.Errorf("expected timezone=America/New_York, got %q", w.State.Config.Timezone)
}

// Verify it shows in review
w.State.CurrentStep = model.StepReview
w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda", SizeHuman: "100 GB"}
m2 := New(w)
view := m2.View()
if !strings.Contains(view, "America/New_York") {
t.Error("timezone should appear in review view")
}
}

func TestSystemChecksDisplayInWelcome(t *testing.T) {
w := newTestWizard()
w.State.SystemChecks = []wizard.SystemCheck{
{Name: "Disk", Status: "ok", Detail: "2 eligible disk(s) found"},
{Name: "Network", Status: "warn", Detail: "interfaces found but none have IPv4 addresses"},
}
m := New(w)

view := m.View()
if !strings.Contains(view, "System checks") {
t.Error("expected system checks header")
}
if !strings.Contains(view, "✓ Disk") {
t.Error("expected OK check icon for disk")
}
if !strings.Contains(view, "⚠ Network") {
t.Error("expected warning icon for network")
}
if !strings.Contains(view, "2 eligible disk(s) found") {
t.Error("expected disk detail text")
}
}
