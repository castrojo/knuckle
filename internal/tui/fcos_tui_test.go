package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

// ── OS picker sub-view ────────────────────────────────────────────────────────

func TestOSPicker_ShowsTwoOptions(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	m := New(w)
	// osSubView is true after initStepFields for StepWelcome
	if !m.osSubView {
		t.Fatal("osSubView should be true after New() at StepWelcome")
	}
	out := m.viewChannelCards()
	if !strings.Contains(out, "Flatcar Container Linux") {
		t.Errorf("OS picker should show 'Flatcar Container Linux': %q", out)
	}
	if !strings.Contains(out, "Fedora CoreOS") {
		t.Errorf("OS picker should show 'Fedora CoreOS': %q", out)
	}
}

func TestOSPicker_SelectFlatcar_ThenShowsChannelCards(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	m := New(w)

	// Phase 1: select Flatcar (cursor 0)
	m.cursor = 0
	_, _ = m.handleEnter()

	if m.Wizard.State.Config.OS != model.OSFlatcar {
		t.Errorf("expected OS=flatcar, got %q", m.Wizard.State.Config.OS)
	}
	if m.osSubView {
		t.Error("osSubView should be false after OS selection")
	}
	// Should now render channel cards, not OS picker
	out := m.viewChannelCards()
	if !strings.Contains(out, "Select a release channel") {
		t.Errorf("after OS selection, should show channel cards: %q", out[:min(200, len(out))])
	}
}

func TestOSPicker_SelectFCOS_ThenShowsStreamCards(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	m := New(w)

	// Phase 1: select FCOS (cursor 1)
	m.cursor = 1
	_, _ = m.handleEnter()

	if m.Wizard.State.Config.OS != model.OSFCOS {
		t.Errorf("expected OS=fcos, got %q", m.Wizard.State.Config.OS)
	}
	if m.osSubView {
		t.Error("osSubView should be false after OS selection")
	}
	// Should now render FCOS stream cards
	out := m.viewChannelCards()
	if !strings.Contains(out, "Fedora CoreOS stream") {
		t.Errorf("after FCOS selection, should show stream cards: %q", out[:min(200, len(out))])
	}
}

// ── FCOS stream cards ─────────────────────────────────────────────────────────

func TestFCOSStreamCards_ListsThreeStreams(t *testing.T) {
	w := newTestWizard()
	w.State.Config.OS = model.OSFCOS
	m := New(w)
	m.osSubView = false

	out := m.viewChannelCards()
	for _, stream := range []string{"Stable", "Testing", "Next"} {
		if !strings.Contains(out, stream) {
			t.Errorf("FCOS stream cards should contain %q: %q", stream, out[:min(300, len(out))])
		}
	}
}

func TestFCOSStreamCards_ChannelList(t *testing.T) {
	w := newTestWizard()
	w.State.Config.OS = model.OSFCOS
	m := New(w)

	got := m.channelList()
	want := []string{"stable", "testing", "next"}
	if len(got) != len(want) {
		t.Fatalf("FCOS channelList length = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("channelList[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestFCOSStreamCards_WithVersionInfo(t *testing.T) {
	w := newTestWizard()
	w.State.Config.OS = model.OSFCOS
	w.State.FCOSStreams = []wizard.FCOSStreamInfo{
		{Stream: "stable", Version: "42.20250101.3.0"},
	}
	m := New(w)
	m.osSubView = false
	m.cursor = 0

	out := m.viewChannelCards()
	if !strings.Contains(out, "42.20250101.3.0") {
		t.Errorf("FCOS stream cards should show version: %q", out[:min(300, len(out))])
	}
}

// ── FCOS maxCursor ────────────────────────────────────────────────────────────

func TestMaxCursor_FCOSUpdate(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.OS = model.OSFCOS
	m := New(w)

	got := m.maxCursor()
	if got != 2 {
		t.Errorf("maxCursor(StepUpdate, FCOS) = %d, want 2", got)
	}
}

func TestMaxCursor_WelcomeChannelCards_Flatcar(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.OS = model.OSFlatcar
	m := New(w)
	m.osSubView = false // OS already chosen, now at channel picker

	got := m.maxCursor()
	if got != 4 {
		t.Errorf("maxCursor(StepWelcome, Flatcar channel picker) = %d, want 4", got)
	}
}

func TestMaxCursor_WelcomeChannelCards_FCOS(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.OS = model.OSFCOS
	m := New(w)
	m.osSubView = false // OS already chosen, now at stream picker

	got := m.maxCursor()
	if got != 3 {
		t.Errorf("maxCursor(StepWelcome, FCOS stream picker) = %d, want 3", got)
	}
}

// ── FCOS update strategy ──────────────────────────────────────────────────────

func TestViewUpdate_FCOS_ShowsZincatiOptions(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.OS = model.OSFCOS
	m := New(w)

	out := m.viewUpdate()
	if !strings.Contains(out, "Fedora CoreOS") {
		t.Errorf("FCOS viewUpdate should mention Fedora CoreOS: %q", out[:min(300, len(out))])
	}
	if !strings.Contains(out, "immediate") {
		t.Errorf("FCOS viewUpdate should show 'immediate': %q", out[:min(300, len(out))])
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("FCOS viewUpdate should show 'disabled': %q", out[:min(300, len(out))])
	}
	// Flatcar options should not appear
	if strings.Contains(out, "etcd-lock") {
		t.Errorf("FCOS viewUpdate should not show 'etcd-lock': %q", out[:min(300, len(out))])
	}
}

func TestHandleEnter_Update_FCOS_AppliesStrategy(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.OS = model.OSFCOS

	m := New(w)
	m.cursor = 0 // immediate
	_, _ = m.handleEnter()

	if m.Wizard.State.Config.UpdateStrategy.FCOSUpdateStrategy != model.FCOSStrategyImmediate {
		t.Errorf("expected FCOSUpdateStrategy=%q, got %q",
			model.FCOSStrategyImmediate, m.Wizard.State.Config.UpdateStrategy.FCOSUpdateStrategy)
	}
}

func TestHandleEnter_Update_FCOS_DisabledStrategy(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.OS = model.OSFCOS

	m := New(w)
	m.cursor = 1 // disabled
	_, _ = m.handleEnter()

	if m.Wizard.State.Config.UpdateStrategy.FCOSUpdateStrategy != model.FCOSStrategyDisabled {
		t.Errorf("expected FCOSUpdateStrategy=%q, got %q",
			model.FCOSStrategyDisabled, m.Wizard.State.Config.UpdateStrategy.FCOSUpdateStrategy)
	}
}

// ── FCOS hostname default ─────────────────────────────────────────────────────

func TestInitForm_FCOSHostnameDefault(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.OS = model.OSFCOS

	m := New(w)
	m.initForm()

	if m.Wizard.State.Config.Hostname != "fcos" {
		t.Errorf("FCOS hostname default should be 'fcos', got %q", m.Wizard.State.Config.Hostname)
	}
}

func TestInitForm_FlatcarHostnameDefault(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.OS = model.OSFlatcar

	m := New(w)
	m.initForm()

	if m.Wizard.State.Config.Hostname != "flatcar" {
		t.Errorf("Flatcar hostname default should be 'flatcar', got %q", m.Wizard.State.Config.Hostname)
	}
}

// ── FCOS review summary ───────────────────────────────────────────────────────

func TestReviewSummary_FCOSShowsOSRow(t *testing.T) {
	w := newTestWizard()
	w.State.Config.OS = model.OSFCOS
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "fcos-node"
	m := New(w)

	out := m.reviewSummary()
	if !strings.Contains(out, "OS: fcos") {
		t.Errorf("reviewSummary should show OS row for FCOS: %q", out)
	}
}

func TestBuildReviewForm_FCOSTitle(t *testing.T) {
	w := newTestWizard()
	w.State.Config.OS = model.OSFCOS
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "fcos"
	m := New(w)

	// The title is rendered inside the huh form — exercise the build path
	form := m.buildReviewForm()
	if form == nil {
		t.Error("buildReviewForm() returned nil for FCOS")
	}
}

// ── FCOS viewInstall / viewDone ───────────────────────────────────────────────

func TestViewInstall_FCOS(t *testing.T) {
	w := newTestWizard()
	w.State.Config.OS = model.OSFCOS
	m := New(w)

	out := m.viewInstall()
	if !strings.Contains(out, "Fedora CoreOS") {
		t.Errorf("viewInstall should say 'Fedora CoreOS' for FCOS: %q", out)
	}
}

func TestViewDone_FCOS_Links(t *testing.T) {
	w := newTestWizard()
	w.State.Config.OS = model.OSFCOS
	w.State.Config.Channel = "stable"
	m := New(w)

	out := m.viewDone()
	if !strings.Contains(out, "Fedora CoreOS") {
		t.Errorf("viewDone should mention 'Fedora CoreOS' for FCOS: %q", out[:min(300, len(out))])
	}
	if !strings.Contains(out, "fedoraproject.org") {
		t.Errorf("viewDone should show Fedora community link: %q", out[:min(300, len(out))])
	}
	// Flatcar-specific text should not appear
	if strings.Contains(out, "flatcar.org") {
		t.Errorf("viewDone should not show flatcar.org for FCOS: %q", out[:min(300, len(out))])
	}
}
