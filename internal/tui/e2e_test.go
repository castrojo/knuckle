package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/projectbluefin/knuckle/internal/model"
)

// TestFullFlowE2E simulates the EXACT user journey through all 9 steps.
// This test failed before the fix and catches the blank screen bug.
func TestFullFlowE2E(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping TTY test in CI — no /dev/tty available")
	}
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Disks = []model.DiskInfo{
		{DevPath: "/dev/vda", Model: "QEMU HARDDISK", SizeHuman: "20 GB", Transport: "virtio"},
	}

	m := New(w)

	// Step 1: Welcome (form step) - simulate Init
	steps := []string{}
	cmd := m.Init()
	m = runCmds(t, m, cmd, 10)
	steps = append(steps, captureStep(t, m, "Welcome"))

	// Complete Welcome form
	m.onFormComplete()
	cmd = m.Init()
	if m.activeForm != nil {
		cmd = m.activeForm.Init()
	}
	m = runCmds(t, m, cmd, 10)
	steps = append(steps, captureStep(t, m, "Network"))

	// Step 2: Network (form step) - complete it
	m.onFormComplete()
	if m.activeForm != nil {
		cmd = m.activeForm.Init()
		m = runCmds(t, m, cmd, 10)
	}
	steps = append(steps, captureStep(t, m, "Storage"))

	// Step 3: Storage (non-form) - press Enter
	m.cursor = 0
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*Model)
	m = runCmds(t, m, cmd, 10)
	steps = append(steps, captureStep(t, m, "User"))

	// Step 4: User (form step) - THIS IS THE ONE THAT WAS BLANK
	if m.activeForm == nil {
		t.Fatal("BLANK SCREEN BUG: activeForm is nil at User step")
	}
	view := m.View()
	if !strings.Contains(view, "Hostname") && !strings.Contains(view, "System Identity") {
		t.Fatalf("User step renders blank. View:\n%s", view)
	}

	// Complete User form
	m.usernameInput = "core"
	m.onFormComplete()
	if m.activeForm != nil {
		cmd = m.activeForm.Init()
		m = runCmds(t, m, cmd, 10)
	}
	steps = append(steps, captureStep(t, m, "Sysext"))

	// Step 5: Sysext (non-form) - press Enter
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*Model)
	m = runCmds(t, m, cmd, 10)
	steps = append(steps, captureStep(t, m, "Update"))

	// Step 6: Update (non-form) - press Enter
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*Model)
	m = runCmds(t, m, cmd, 10)
	steps = append(steps, captureStep(t, m, "Review"))

	// Step 7: Review (form step) - should show confirmation
	if m.activeForm == nil {
		t.Fatal("BLANK SCREEN BUG: activeForm is nil at Review step")
	}

	fmt.Printf("\n=== E2E FLOW TRACE ===\n")
	for _, s := range steps {
		fmt.Println(s)
	}
}

func runCmds(t *testing.T, m *Model, cmd tea.Cmd, maxRounds int) *Model {
	t.Helper()
	for i := 0; i < maxRounds && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		var newModel tea.Model
		newModel, cmd = m.Update(msg)
		m = newModel.(*Model)
	}
	return m
}

func captureStep(t *testing.T, m *Model, expected string) string {
	t.Helper()
	step := m.Wizard.State.CurrentStep.String()
	view := m.View()
	hasContent := len(strings.TrimSpace(view)) > 100
	status := "✓"
	if !hasContent {
		status = "✗ BLANK"
	}
	return fmt.Sprintf("  %s Step %s (at %s, view=%d chars)", status, expected, step, len(view))
}
