package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// ── Init: KNUCKLE_TEST_TUI_AUTO_QUIT branch ───────────────────────────────────

func TestInit_AutoQuit_SetsQuittingAndReturnsQuit(t *testing.T) {
	t.Setenv("KNUCKLE_TEST_TUI_AUTO_QUIT", "1")
	m := New(newTestWizard())
	cmd := m.Init()
	if !m.quitting {
		t.Error("Init with KNUCKLE_TEST_TUI_AUTO_QUIT=1 should set m.quitting")
	}
	if cmd == nil {
		t.Error("Init with KNUCKLE_TEST_TUI_AUTO_QUIT=1 should return a non-nil tea.Quit cmd")
	}
}

// ── render(): StepInstall and StepDone branches ───────────────────────────────

func TestRender_StepInstall_ReturnsContent(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	m := New(w)
	m.installing = true
	out := m.render()
	if len(out) == 0 {
		t.Error("render() at StepInstall should produce non-empty output")
	}
}

func TestRender_StepDone_ReturnsContent(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.Channel = "stable"
	m := New(w)
	out := m.render()
	if len(out) == 0 {
		t.Error("render() at StepDone should produce non-empty output")
	}
}

// ── viewStorage: empty Model and non-selected disk branch ─────────────────────

func TestViewStorage_EmptyModel_ShowsUnknownDisk(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{
		{
			Model:     "",
			SizeHuman: "256 GB",
			DevPath:   "/dev/sda",
			Path:      "/dev/disk/by-id/some-disk",
		},
	}
	m := New(w)
	out := m.viewStorage()
	if !strings.Contains(out, "Unknown Disk") {
		t.Errorf("viewStorage should display 'Unknown Disk' when Model is empty, got: %q", out)
	}
}

func TestViewStorage_MultiDisk_NonSelectedBranch(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Disks = []model.DiskInfo{
		{Model: "Samsung 970 EVO", SizeHuman: "1 TB", DevPath: "/dev/nvme0n1"},
		{Model: "Seagate Barracuda", SizeHuman: "4 TB", DevPath: "/dev/sda"},
	}
	m := New(w)
	m.cursor = 0 // disk[1] is non-selected → exercises the else branch
	out := m.viewStorage()
	if !strings.Contains(out, "Seagate Barracuda") {
		t.Errorf("viewStorage should render non-selected disk, got: %q", out)
	}
}

// ── applyFields: StepUser password-too-long error path ───────────────────────

func TestApplyFields_User_PasswordTooLong_SetsErr(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	m.Wizard.State.CurrentStep = model.StepUser
	m.fields = []field{
		{key: "password", value: strings.Repeat("x", 73)},
	}
	m.applyFields()
	if m.err == nil {
		t.Error("applyFields should set m.err when password exceeds 72 bytes")
	}
}

// ── viewStorage: long-model padding clamp (padding < 2) ──────────────────────

func TestViewStorage_LongModelClampspadding(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	// Model+size > 54 chars forces padding < 2 → clamped to 2.
	w.State.Disks = []model.DiskInfo{
		{
			Model:     strings.Repeat("A", 50),
			SizeHuman: "9999 GB",
			DevPath:   "/dev/sda",
		},
	}
	m := New(w)
	out := m.viewStorage()
	if !strings.Contains(out, strings.Repeat("A", 50)) {
		t.Errorf("viewStorage should render long model name, got: %q", out)
	}
}
