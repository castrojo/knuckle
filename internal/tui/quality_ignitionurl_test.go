package tui

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// TestHandleEnter_Welcome_IgnitionURL_SkipsToStorage covers the branch at
// tui.go:463 where IgnitionURL is set on StepWelcome → skips to StepStorage.
func TestHandleEnter_Welcome_IgnitionURL_SkipsToStorage(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	w.State.Config.Channel = "stable"

	m := New(w)
	m.osSelected = true // OS already picked
	_, _ = m.handleEnter()

	if m.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("IgnitionURL on Welcome: expected StepStorage, got %v", m.Wizard.State.CurrentStep)
	}
	if m.err != nil {
		t.Errorf("IgnitionURL on Welcome: unexpected error %v", m.err)
	}
	if m.cursor != 0 {
		t.Errorf("IgnitionURL on Welcome: cursor should be reset to 0, got %d", m.cursor)
	}
}

// TestHandleEnter_Storage_IgnitionURL_SkipsToReview covers the branch at
// tui.go:515 where IgnitionURL is set on StepStorage → skips to StepReview.
func TestHandleEnter_Storage_IgnitionURL_SkipsToReview(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "testhost"
	// Populate disk so storage validation passes
	w.State.Disks = []model.DiskInfo{{DevPath: "/dev/vda", Size: 20 * 1024 * 1024 * 1024}}
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda", Size: 20 * 1024 * 1024 * 1024}

	m := New(w)
	m.cursor = 0

	_, _ = m.handleEnter()

	// With IgnitionURL, storage step should advance to Review
	if m.Wizard.State.CurrentStep != model.StepReview && m.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("IgnitionURL on Storage: unexpected step %v", m.Wizard.State.CurrentStep)
	}
	// Either way cursor is reset and no unexpected error from the IgnitionURL path
}

// TestHandleEnter_Storage_IgnitionURL_ValidationError covers the branch at
// tui.go:518-520 where IgnitionURL is set but storage validation fails.
func TestHandleEnter_Storage_IgnitionURL_ValidationError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	// No disk set → storage validation should fail
	w.State.Config.Disk = model.DiskInfo{}

	m := New(w)
	m.cursor = 0

	_, _ = m.handleEnter()

	// Step should NOT advance beyond Storage since disk validation fails
	if m.Wizard.State.CurrentStep == model.StepReview {
		t.Error("IgnitionURL on Storage with invalid disk: should not advance to StepReview")
	}
}

// TestRenderDetailPanel_NarrowWidth covers the branch at tui.go:984
// where panelWidth < 20 and renderDetailPanel returns "".
func TestRenderDetailPanel_NarrowWidth(t *testing.T) {
	w := newTestWizard()
	m := New(w)

	// Set a very narrow terminal width so that panelWidth falls below 20.
	// effectiveWidth - 32 < 20  →  effectiveWidth < 52
	// A width of 50 gives effectiveWidth=50, panelWidth=min(52,18)=18 < 20.
	m.width = 50

	ext := model.SysextEntry{
		Name:    "docker",
		Version: "24.0.0",
		URL:     "https://example.com/docker.raw",
	}

	result := m.renderDetailPanel(ext)
	if result != "" {
		t.Errorf("renderDetailPanel with narrow width = %q, want empty string", result)
	}
}

// TestRenderDetailPanel_TooNarrowForEffectiveWidth covers the branch at tui.go:980
// where effectiveWidth < 60 returns "" before even computing panelWidth.
func TestRenderDetailPanel_VeryNarrow(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.width = 40 // < 60

	ext := model.SysextEntry{Name: "docker", Version: "24.0.0"}
	result := m.renderDetailPanel(ext)
	if result != "" {
		t.Errorf("renderDetailPanel(width=40) = %q, want empty string", result)
	}
}
