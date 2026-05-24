package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/probe"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

func newTestWizard() *wizard.Wizard {
	return wizard.New(nil, nil, nil)
}

func TestNewModel(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	if m.Wizard != w {
		t.Error("model should reference the wizard")
	}
	if m.Wizard.State.CurrentStep != model.StepWelcome {
		t.Errorf("expected StepWelcome, got %v", m.Wizard.State.CurrentStep)
	}
}

func TestViewWelcome(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	// Init form to trigger rendering
	if m.activeForm != nil {
		m.activeForm.Init()
	}
	view := m.View()
	if !strings.Contains(view, "K N U C K L E") && !strings.Contains(view, "knuckle") && !strings.Contains(view, "Knuckle") {
		t.Error("view should contain title")
	}
	// huh form renders after init
	if len(view) < 50 {
		t.Errorf("view too short — form may not have rendered: %q", view)
	}
}

func TestViewReview(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "testhost"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda", SizeHuman: "500 GB"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	m := New(w)
	// Init the form to trigger rendering
	if m.activeForm != nil {
		m.activeForm.Init()
	}
	view := m.View()
	// The review form shows a confirm dialog with summary
	if !strings.Contains(view, "stable") && !strings.Contains(view, "install") {
		t.Errorf("review should show install confirmation, got:\n%s", view[:min(300, len(view))])
	}
}

func TestHandleQuit(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	// First Ctrl+C triggers confirmation
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	tuiModel := newModel.(*Model)
	if tuiModel.quitting {
		t.Error("should not quit on first ctrl+c")
	}
	if !tuiModel.confirmQuit {
		t.Error("should show quit confirmation")
	}
	if cmd != nil {
		t.Error("should not return quit cmd on first press")
	}
	// Second Ctrl+C quits
	newModel, cmd = tuiModel.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	tuiModel = newModel.(*Model)
	if !tuiModel.quitting {
		t.Error("should be quitting after second ctrl+c")
	}
	if cmd == nil {
		t.Error("should return quit cmd")
	}
}

func TestQuitRequiresDoublePress(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage // non-form step
	m := New(w)

	// First Ctrl+C
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	tuiModel := newModel.(*Model)
	if !tuiModel.confirmQuit {
		t.Error("first ctrl+c should set confirmQuit")
	}

	// Any other key cancels
	newModel, _ = tuiModel.Update(tea.KeyMsg{Type: tea.KeyDown})
	tuiModel = newModel.(*Model)
	if tuiModel.confirmQuit {
		t.Error("other key should cancel quit confirmation")
	}
}

func TestHandleEnterAdvancesNonFormStep(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext // non-form step
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	m := New(w)

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tuiModel := newModel.(*Model)
	if tuiModel.Wizard.State.CurrentStep != model.StepUpdate {
		t.Errorf("expected StepUpdate after Enter on Sysext, got %v", tuiModel.Wizard.State.CurrentStep)
	}
}

func TestFormCompleteAdvancesWizard(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	m := New(w)

	// Simulate Welcome form completion
	m.onFormComplete()
	if m.Wizard.State.CurrentStep != model.StepNetwork {
		t.Errorf("expected StepNetwork after Welcome form, got %v", m.Wizard.State.CurrentStep)
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := hashPassword("testpass")
	if err != nil {
		t.Fatalf("hashPassword error: %v", err)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if !strings.HasPrefix(hash, "$2a$") {
		t.Errorf("hash should be bcrypt format, got: %s", hash[:10])
	}
}

func TestMultiSSHKeySplitting(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"ssh-ed25519 AAAA test", 1},
		{"ssh-ed25519 AAAA one;ssh-rsa BBBB two", 2},
		{"", 0},
	}
	for _, tt := range tests {
		keys := splitSSHKeys(tt.input)
		if len(keys) != tt.want {
			t.Errorf("splitSSHKeys(%q) = %d keys, want %d", tt.input, len(keys), tt.want)
		}
	}
}

func TestSystemChecksDisplayInWelcome(t *testing.T) {
	w := newTestWizard()
	w.State.SystemChecks = []wizard.SystemCheck{
		{Name: "Disk", Status: "ok", Detail: "2 disk(s) found"},
		{Name: "Network", Status: "warn", Detail: "no IPv4"},
	}
	w.State.CurrentStep = model.StepNetwork // dots show on non-Welcome steps
	m := New(w)
	m.initForm()
	view := m.View()
	// Zen chrome shows system status as colored dots (●) on non-Welcome steps
	if !strings.Contains(view, "●") {
		t.Error("non-welcome step should display system check status dots")
	}
}

func TestButanePreviewToggle(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage // Non-form step where ctrl+b works
	m := New(w)
	// Note: Butane preview only works on Review step, which is now a form
	// This test verifies the ctrl+b key doesn't crash on non-review steps
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	_ = newModel.(*Model) // no panic = pass
}

func TestVersionFieldFromConfig(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Version = "4593.2.1"
	m := New(w)
	// The huh form reads directly from config, verify it's wired
	if m.Wizard.State.Config.Version != "4593.2.1" {
		t.Error("version should be preserved in config")
	}
}

func TestTimezoneDefault(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Timezone = ""
	w.State.CurrentStep = model.StepUser
	m := New(w)

	// Simulate form complete without setting timezone
	m.usernameInput = "core"
	m.onFormComplete()

	if m.Wizard.State.Config.Timezone != "UTC" {
		t.Errorf("expected UTC default, got %q", m.Wizard.State.Config.Timezone)
	}
}

func TestRebootRequiresDoublePress(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = false
	m := New(w)

	// First 'r' press should ask for confirmation, not quit
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	newModel, cmd := m.Update(msg)
	updatedModel := newModel.(*Model)

	if updatedModel.quitting {
		t.Error("first r press should not quit")
	}
	if cmd != nil {
		t.Error("first r press should not produce a command")
	}
	if !updatedModel.confirmReboot {
		t.Error("confirmReboot should be set after first r press")
	}
	if updatedModel.err == nil || updatedModel.err.Error() != "press r again to confirm reboot" {
		t.Errorf("expected confirmation prompt, got err=%v", updatedModel.err)
	}
}

func TestRebootDryRunDoesNotReboot(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = true
	m := New(w)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	newModel, cmd := m.Update(msg)
	updatedModel := newModel.(*Model)

	if updatedModel.quitting {
		t.Error("dry-run should not quit/reboot")
	}
	if cmd != nil {
		t.Error("dry-run should not produce reboot command")
	}
	if updatedModel.err == nil || !strings.Contains(updatedModel.err.Error(), "dry-run") {
		t.Errorf("expected dry-run message, got err=%v", updatedModel.err)
	}
}

func TestDoneViewDryRun(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = true
	m := New(w)

	view := m.viewDone()
	if !strings.Contains(view, "dry-run") {
		t.Error("done view should mention dry-run")
	}
	if strings.Contains(view, "reboot") {
		t.Error("done view should not mention reboot in dry-run mode")
	}
}

func TestViewSysextEmpty(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = nil
	m := New(w)
	view := m.View()
	if !strings.Contains(view, "No extensions available") {
		t.Errorf("empty sysext view should show 'No extensions available', got: %q", view)
	}
}

func TestViewSysextWithEntries(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "kubernetes", Version: "1.36.1", Description: "K8s", Category: "Orchestration", SupportTier: "Flatcar Integrated", URL: "https://example.com/k8s.raw"},
		{Name: "docker", Version: "28.0.0", Description: "Docker", Category: "Container Runtime", SupportTier: "Flatcar Integrated", URL: "https://example.com/docker.raw"},
		{Name: "btop", Version: "1.4.0", Description: "btop monitor", Category: "Utilities", SupportTier: "Bakery Maintained", URL: "https://example.com/btop.raw"},
	}
	m := New(w)
	view := m.View()

	// Version must be visible in the list row.
	if !strings.Contains(view, "v1.36.1") {
		t.Error("version v1.36.1 should appear in sysext list row")
	}
	if !strings.Contains(view, "v28.0.0") {
		t.Error("version v28.0.0 should appear in sysext list row")
	}

	// Extension names must be visible.
	if !strings.Contains(view, "kubernetes") {
		t.Error("kubernetes should appear in sysext view")
	}
	if !strings.Contains(view, "docker") {
		t.Error("docker should appear in sysext view")
	}
	if !strings.Contains(view, "btop") {
		t.Error("btop should appear in sysext view")
	}

	// Tier section headers must appear.
	if !strings.Contains(view, "Flatcar Integrated") {
		t.Error("tier header 'Flatcar Integrated' should appear in sysext view")
	}
	if !strings.Contains(view, "Bakery Maintained") {
		t.Error("tier header 'Bakery Maintained' should appear in sysext view")
	}

	// Selected count must appear.
	if !strings.Contains(view, "0 selected") {
		t.Error("selected count '0 selected' should appear in header")
	}
}

func TestViewSysextSelectedCount(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", Category: "Container Runtime", SupportTier: "Flatcar Integrated", Selected: true},
		{Name: "btop", Version: "1.4.0", Category: "Utilities", SupportTier: "Bakery Maintained", Selected: false},
	}
	m := New(w)
	view := m.View()
	if !strings.Contains(view, "1 selected") {
		t.Errorf("expected '1 selected' in header, got view: %q", view)
	}
}

func TestViewSysextDetailPanelOnCursor(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "ollama", Version: "0.13.2", Description: "Run LLMs locally", Category: "AI / ML", SupportTier: "Bakery Maintained", URL: "https://example.com/ollama.raw"},
	}
	m := New(w)
	m.cursor = 0
	m.width = 120 // wide enough for panel
	view := m.View()

	// Detail panel must show version and support tier.
	if !strings.Contains(view, "Version:") {
		t.Error("detail panel should show 'Version:' label")
	}
	if !strings.Contains(view, "Support:") {
		t.Error("detail panel should show 'Support:' label")
	}
	// Caveat from curated catalog must appear for ollama.
	if !strings.Contains(view, "publicly accessible") && !strings.Contains(view, "OLLAMA_HOST") {
		t.Error("ollama detail panel should contain its caveat about public API access")
	}
}

func TestViewSysextCursorIndexSafety(t *testing.T) {
	// Verifies Approach A: m.cursor always indexes Sysexts[] directly.
	// Space-toggle at cursor must toggle the correct entry regardless of tier grouping.
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "kubernetes", Version: "1.36.1", SupportTier: "Flatcar Integrated"},
		{Name: "btop", Version: "1.4.0", SupportTier: "Bakery Maintained"},
		{Name: "wasmtime", Version: "44.0.1", SupportTier: "Experimental"},
	}
	m := New(w)

	// cursor=0 → kubernetes; space should toggle kubernetes
	m.cursor = 0
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !w.State.Sysexts[0].Selected {
		t.Error("space on cursor=0 should select Sysexts[0] (kubernetes)")
	}
	if w.State.Sysexts[1].Selected || w.State.Sysexts[2].Selected {
		t.Error("space on cursor=0 must not affect Sysexts[1] or Sysexts[2]")
	}

	// cursor=2 → wasmtime (across tier boundary); space should toggle wasmtime
	m.cursor = 2
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !w.State.Sysexts[2].Selected {
		t.Error("space on cursor=2 should select Sysexts[2] (wasmtime)")
	}
	if !w.State.Sysexts[0].Selected {
		t.Error("Sysexts[0] should still be selected")
	}
	if w.State.Sysexts[1].Selected {
		t.Error("Sysexts[1] should remain unselected")
	}
}

func TestNvidiaAutoSelectShownInView(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.NvidiaGPUDetected = true
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Version: "1.17.9", Category: "GPU / Accelerators",
			SupportTier: "Bakery Maintained", Selected: true},
	}
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.width = 120
	view := m.View()

	if !strings.Contains(view, "NVIDIA GPU detected") {
		t.Error("sysext view should show GPU auto-detect notice when NvidiaGPUDetected=true")
	}
}

func TestViewNvidiaScreen(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.NvidiaGPUDetected = true
	w.State.NvidiaGPUs = []probe.NvidiaGPUInfo{{PCIAddress: "0000:01:00.0", PCIClass: "0x030200"}}
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.cursor = 0
	m.width = 120

	view := m.View()

	// Header.
	if !strings.Contains(view, "NVIDIA GPU Setup") {
		t.Error("GPU Setup screen should show 'NVIDIA GPU Setup' heading")
	}
	// GPU detection.
	if !strings.Contains(view, "NVIDIA GPU detected") {
		t.Error("GPU Setup screen should show GPU detection status")
	}
	if !strings.Contains(view, "0000:01:00.0") {
		t.Error("GPU Setup screen should show PCI address of detected GPU")
	}
	// Two-component explanation.
	if !strings.Contains(view, "kernel driver") && !strings.Contains(view, "Flatcar-official") {
		t.Error("GPU Setup screen should explain the kernel driver component")
	}
	if !strings.Contains(view, "Container Toolkit") {
		t.Error("GPU Setup screen should explain the Container Toolkit component")
	}
	if !strings.Contains(view, "enabled-sysext.conf") {
		t.Error("GPU Setup screen should mention enabled-sysext.conf")
	}
	// Driver picker.
	if !strings.Contains(view, "570") {
		t.Error("GPU Setup screen should show 570 driver series")
	}
	if !strings.Contains(view, "460") {
		t.Error("GPU Setup screen should show legacy 460 driver series")
	}
}

func TestViewNvidiaNoGPUDetected(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.NvidiaGPUDetected = false
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.width = 120

	view := m.View()
	if !strings.Contains(view, "No NVIDIA GPU detected") {
		t.Error("should show warning when no GPU was detected")
	}
}

func TestNvidiaScreenCursorNavigation(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	// initStepFields() should position cursor at default driver.
	m.initStepFields()

	// Cursor should be at index of default series.
	wantIdx := 0
	for i, opt := range model.NvidiaDriverOptions {
		if opt.ID == model.DefaultNvidiaDriverSeries {
			wantIdx = i
			break
		}
	}
	if m.cursor != wantIdx {
		t.Errorf("cursor should start at index %d (default driver), got %d", wantIdx, m.cursor)
	}
}

func TestNvidiaScreenEnterConfirmsSelection(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	// Add a sysext to satisfy any wizard nav guards.
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: true},
	}
	m := New(w)
	m.cursor = 3 // 460 (legacy proprietary)

	_, _ = m.handleEnter()
	if w.State.Config.NvidiaDriverVersion != model.NvidiaDriverOptions[3].ID {
		t.Errorf("Enter on cursor=3 should set driver to %q, got %q",
			model.NvidiaDriverOptions[3].ID, w.State.Config.NvidiaDriverVersion)
	}
}

func TestNvidiaDriverVersionSetOnToggle(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Version: "1.17.9", Category: "GPU / Accelerators",
			SupportTier: "Bakery Maintained", Selected: false},
	}
	m := New(w)
	m.cursor = 0

	// Space to select nvidia-runtime — driver version should be set to default.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !w.State.Sysexts[0].Selected {
		t.Error("nvidia-runtime should be selected after space")
	}
	if w.State.Config.NvidiaDriverVersion == "" {
		t.Errorf("NvidiaDriverVersion should be set to default when nvidia-runtime is selected, got empty")
	}

	// Space again to deselect — driver version should be cleared.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if w.State.Sysexts[0].Selected {
		t.Error("nvidia-runtime should be deselected after second space")
	}
	if w.State.Config.NvidiaDriverVersion != "" {
		t.Errorf("NvidiaDriverVersion should be cleared when nvidia-runtime is deselected, got %q",
			w.State.Config.NvidiaDriverVersion)
	}
}

// --- NVIDIA Edge Case Tests ---

func TestNvidiaCursorClampOnDownKey(t *testing.T) {
	// Verify cursor cannot exceed len(NvidiaDriverOptions)-1 by pressing down repeatedly.
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.initStepFields()

	max := len(model.NvidiaDriverOptions)
	// Press down more times than there are options.
	for i := 0; i < max+5; i++ {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	if m.cursor >= max {
		t.Errorf("cursor should be clamped to %d, got %d", max-1, m.cursor)
	}
	if m.cursor != max-1 {
		t.Errorf("cursor should be at last item %d, got %d", max-1, m.cursor)
	}
}

func TestNvidiaCursorCannotGoNegative(t *testing.T) {
	// Pressing up at cursor=0 should stay at 0.
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.initStepFields()
	m.cursor = 0

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("cursor should stay at 0 when pressing up at top, got %d", m.cursor)
	}
}

func TestNvidiaEnterWithInvalidCursorIsNoOp(t *testing.T) {
	// If cursor is somehow out of bounds, enter should not panic or mutate state.
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = "550-open"
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: true},
	}
	m := New(w)
	m.cursor = 99 // out of bounds

	_, _ = m.handleEnter()
	// Driver version should remain unchanged because guard prevented write.
	if w.State.Config.NvidiaDriverVersion != "550-open" {
		t.Errorf("enter with OOB cursor should not change driver version, got %q",
			w.State.Config.NvidiaDriverVersion)
	}
}

func TestNvidiaEnterWithNegativeCursorIsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = "535-open"
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: true},
	}
	m := New(w)
	m.cursor = -1

	_, _ = m.handleEnter()
	if w.State.Config.NvidiaDriverVersion != "535-open" {
		t.Errorf("enter with negative cursor should not change driver version, got %q",
			w.State.Config.NvidiaDriverVersion)
	}
}

func TestNvidiaSpaceKeyIsNoOp(t *testing.T) {
	// Space on NVIDIA step should not toggle anything or panic.
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.initStepFields()

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	// Should remain unchanged.
	if w.State.Config.NvidiaDriverVersion != model.DefaultNvidiaDriverSeries {
		t.Errorf("space on nvidia step should not change driver version, got %q",
			w.State.Config.NvidiaDriverVersion)
	}
}

func TestNvidiaBackNavigationPreservesCursorOnReentry(t *testing.T) {
	// Navigate to NVIDIA, select driver at cursor=2, go back, come forward again.
	// Cursor should restore to the configured driver position.
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.NvidiaDriverOptions[2].ID
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: true},
	}
	m := New(w)
	m.initStepFields()

	// Cursor should be at 2 (matching the configured driver).
	if m.cursor != 2 {
		t.Fatalf("cursor should start at 2, got %d", m.cursor)
	}

	// Press esc — goes back to sysext.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if w.State.CurrentStep != model.StepSysext {
		t.Fatalf("esc from nvidia should go to sysext, got %v", w.State.CurrentStep)
	}

	// Simulate re-entering NVIDIA by advancing forward.
	w.State.CurrentStep = model.StepNvidia
	m.initStepFields()

	// Cursor should restore to 2 (the configured driver).
	if m.cursor != 2 {
		t.Errorf("cursor should restore to 2 on re-entry, got %d", m.cursor)
	}
}

func TestNvidiaCharacterKeysAreNoOp(t *testing.T) {
	// Typing characters on NVIDIA step (no fields) should not mutate anything.
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.NvidiaDriverVersion = model.DefaultNvidiaDriverSeries
	m := New(w)
	m.initStepFields()

	// Type random characters.
	for _, ch := range "hello123" {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if w.State.Config.NvidiaDriverVersion != model.DefaultNvidiaDriverSeries {
		t.Errorf("character keys should not change driver version, got %q",
			w.State.Config.NvidiaDriverVersion)
	}
	if len(m.fields) != 0 {
		t.Errorf("nvidia step should have no fields, got %d", len(m.fields))
	}
}

func TestViewUpdateReboot(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.UpdateStrategy.RebootStrategy = "reboot"
	m := New(w)
	m.cursor = 0
	m.width = 120

	view := m.View()

	if !strings.Contains(view, "Update Strategy") {
		t.Error("view should contain 'Update Strategy' header")
	}
	if !strings.Contains(view, "reboot") && !strings.Contains(view, "Recommended") {
		t.Error("view should contain reboot option")
	}
	if !strings.Contains(view, "▸ reboot") {
		t.Error("reboot should be highlighted with cursor")
	}
}

func TestViewUpdateOff(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.UpdateStrategy.RebootStrategy = "off"
	m := New(w)
	m.cursor = 1
	m.width = 120

	view := m.View()

	if !strings.Contains(view, "▸ off") {
		t.Error("off option should be highlighted at cursor 1")
	}
	if !strings.Contains(view, "never applied automatically") {
		t.Error("view should show off strategy description")
	}
}

func TestViewUpdateEtcdLock(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.UpdateStrategy.RebootStrategy = "etcd-lock"
	m := New(w)
	m.cursor = 2
	m.width = 120

	view := m.View()

	if !strings.Contains(view, "▸ etcd-lock") {
		t.Error("etcd-lock option should be highlighted at cursor 2")
	}
	if !strings.Contains(view, "etcd distributed lock") {
		t.Error("view should show etcd-lock strategy description")
	}
}

func TestUpdateEnterSelectsReboot(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.cursor = 0

	_, _ = m.handleEnter()
	if w.State.Config.UpdateStrategy.RebootStrategy != "reboot" {
		t.Errorf("cursor=0 should set RebootStrategy to \"reboot\", got %q",
			w.State.Config.UpdateStrategy.RebootStrategy)
	}
}

func TestUpdateEnterSelectsOff(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.cursor = 1

	_, _ = m.handleEnter()
	if w.State.Config.UpdateStrategy.RebootStrategy != "off" {
		t.Errorf("cursor=1 should set RebootStrategy to \"off\", got %q",
			w.State.Config.UpdateStrategy.RebootStrategy)
	}
}

func TestUpdateEnterSelectsEtcdLock(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	m := New(w)
	m.cursor = 2

	_, _ = m.handleEnter()
	if w.State.Config.UpdateStrategy.RebootStrategy != "etcd-lock" {
		t.Errorf("cursor=2 should set RebootStrategy to \"etcd-lock\", got %q",
			w.State.Config.UpdateStrategy.RebootStrategy)
	}
}

func TestUpdateEnterOutOfBoundsIsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.UpdateStrategy.RebootStrategy = "reboot"
	m := New(w)
	m.cursor = 99

	_, _ = m.handleEnter()
	if w.State.Config.UpdateStrategy.RebootStrategy != "reboot" {
		t.Errorf("OOB cursor should not change strategy, got %q",
			w.State.Config.UpdateStrategy.RebootStrategy)
	}
}

func TestUpdateEnterNegativeCursorIsNoOp(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.UpdateStrategy.RebootStrategy = "off"
	m := New(w)
	m.cursor = -1

	_, _ = m.handleEnter()
	if w.State.Config.UpdateStrategy.RebootStrategy != "off" {
		t.Errorf("negative cursor should not change strategy, got %q",
			w.State.Config.UpdateStrategy.RebootStrategy)
	}
}
