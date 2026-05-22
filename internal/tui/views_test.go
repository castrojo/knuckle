package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// ── wordWrap ─────────────────────────────────────────────────────────────────

func TestWordWrap_Empty(t *testing.T) {
	lines := wordWrap("", 80)
	if len(lines) != 1 || lines[0] != "" {
		t.Errorf("empty string: want [\"\"], got %v", lines)
	}
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	lines := wordWrap("hello world", 0)
	if len(lines) != 1 {
		t.Errorf("zero width: want 1 line, got %d", len(lines))
	}
}

func TestWordWrap_FitsOnOneLine(t *testing.T) {
	lines := wordWrap("hello world", 80)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Errorf("got %v", lines)
	}
}

func TestWordWrap_BreaksOnBoundary(t *testing.T) {
	// "hello world" — width 5 should break between words
	lines := wordWrap("hello world", 5)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "hello" || lines[1] != "world" {
		t.Errorf("unexpected: %v", lines)
	}
}

func TestWordWrap_WhitespaceOnly(t *testing.T) {
	lines := wordWrap("   ", 80)
	// strings.Fields on whitespace-only → empty slice → return [""]
	if len(lines) != 1 {
		t.Errorf("whitespace-only: want 1 line, got %d: %v", len(lines), lines)
	}
}

func TestWordWrap_LongWord(t *testing.T) {
	// Single word longer than width — should not be split (no hyphenation)
	lines := wordWrap("superlongword", 5)
	if len(lines) != 1 || lines[0] != "superlongword" {
		t.Errorf("long word: want [\"superlongword\"], got %v", lines)
	}
}

// ── viewInstall ───────────────────────────────────────────────────────────────

func TestViewInstall_InitialState(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	m := New(w)
	view := m.viewInstall()
	if !strings.Contains(view, "Press Enter to start installation") {
		t.Errorf("initial install view should prompt to press Enter, got:\n%s", view)
	}
}

func TestViewInstall_WithProgress(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	w.State.ProgressMessages = []string{"Downloading Flatcar...", "Writing partition table..."}
	m := New(w)
	view := m.viewInstall()
	if !strings.Contains(view, "Downloading Flatcar") {
		t.Errorf("install view should show progress messages, got:\n%s", view)
	}
	if !strings.Contains(view, "Writing partition table") {
		t.Errorf("install view should show all progress messages, got:\n%s", view)
	}
}

// ── viewDone ──────────────────────────────────────────────────────────────────

func TestViewDone_Basic(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "my-server"
	w.State.Config.Disk = model.DiskInfo{Model: "Samsung SSD", SizeHuman: "500 GB"}
	w.State.Config.Users = []model.UserConfig{{Username: "core"}}
	m := New(w)
	view := m.viewDone()
	for _, want := range []string{"Installation Complete", "stable", "my-server", "Samsung SSD", "core"} {
		if !strings.Contains(view, want) {
			t.Errorf("viewDone should contain %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "dry-run") {
		t.Errorf("non-dry-run viewDone must not contain 'dry-run', got:\n%s", view)
	}
	if !strings.Contains(view, "Press r twice to reboot") {
		t.Errorf("non-dry-run viewDone should show reboot prompt, got:\n%s", view)
	}
}

func TestViewDone_DryRun(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = true
	m := New(w)
	view := m.viewDone()
	if !strings.Contains(view, "dry-run") {
		t.Errorf("dry-run done view should mention dry-run, got:\n%s", view)
	}
	if !strings.Contains(view, "Press q to exit") {
		t.Errorf("dry-run done view should show quit prompt, got:\n%s", view)
	}
}

func TestViewDone_WithNvidia(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.NvidiaDriverVersion = "550-open"
	m := New(w)
	view := m.viewDone()
	if !strings.Contains(view, "NVIDIA") {
		t.Errorf("viewDone with nvidia should mention NVIDIA, got:\n%s", view)
	}
	if !strings.Contains(view, "nvidia-smi") {
		t.Errorf("viewDone with nvidia should include nvidia-smi command, got:\n%s", view)
	}
}

func TestViewDone_DiskDevPath(t *testing.T) {
	// When disk has no Model, falls back to DevPath.
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/sda"}
	m := New(w)
	view := m.viewDone()
	if !strings.Contains(view, "/dev/sda") {
		t.Errorf("viewDone should show DevPath when Model is empty, got:\n%s", view)
	}
}

func TestViewDone_RebootPrompt(t *testing.T) {
	// Non-dry-run shows reboot prompt.
	w := newTestWizard()
	w.State.CurrentStep = model.StepDone
	w.State.Config.DryRun = false
	m := New(w)
	view := m.viewDone()
	if !strings.Contains(view, "reboot") {
		t.Errorf("non-dry-run done view should show reboot prompt, got:\n%s", view)
	}
}

// ── viewSysext ────────────────────────────────────────────────────────────────

func TestViewSysext_Empty(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = nil
	m := New(w)
	view := m.viewSysext()
	if !strings.Contains(view, "No extensions available") {
		t.Errorf("empty sysext view should note no extensions, got:\n%s", view)
	}
}

func TestViewSysext_WithEntries(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated, Selected: true},
		{Name: "kubernetes", SupportTier: bakery.TierMaintained},
	}
	m := New(w)
	// sysextListReady is false → fallback manual rendering path
	view := m.viewSysext()
	if !strings.Contains(view, "docker") {
		t.Errorf("viewSysext should contain docker entry, got:\n%s", view)
	}
}

func TestViewSysext_SelectedCount(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: true},
		{Name: "kubernetes", Selected: true},
		{Name: "helm", Selected: false},
	}
	m := New(w)
	view := m.viewSysext()
	if !strings.Contains(view, "2 selected") {
		t.Errorf("viewSysext header should show '2 selected', got:\n%s", view)
	}
}

func TestViewSysext_NvidiaDetected(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.NvidiaGPUDetected = true
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated},
	}
	m := New(w)
	view := m.viewSysext()
	if !strings.Contains(view, "NVIDIA GPU detected") {
		t.Errorf("viewSysext should show NVIDIA notice, got:\n%s", view)
	}
}

// ── viewWithForm ──────────────────────────────────────────────────────────────

func TestViewWithForm_ShowsError(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.initForm()
	m.err = fmt.Errorf("something went wrong")
	view := m.viewWithForm()
	if !strings.Contains(view, "something went wrong") {
		t.Errorf("viewWithForm should render error, got:\n%s", view)
	}
}

func TestViewWithForm_ShowsFetching(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	m.initForm()
	m.fetching = true
	view := m.viewWithForm()
	if !strings.Contains(view, "Fetching SSH keys") {
		t.Errorf("viewWithForm should show fetching indicator, got:\n%s", view)
	}
}

// ── waitForProgress ───────────────────────────────────────────────────────────

func TestWaitForProgress_ChannelClosed(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	ch := make(chan string)
	close(ch)
	m.progressCh = ch
	cmdFn := m.waitForProgress()
	msg := cmdFn()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("closed channel should yield installDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Errorf("closed channel should yield nil error, got %v", done.err)
	}
}

func TestWaitForProgress_ProgressMessage(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	ch := make(chan string, 1)
	ch <- "Downloading..."
	m.progressCh = ch
	cmdFn := m.waitForProgress()
	msg := cmdFn()
	progMsg, ok := msg.(installProgressMsg)
	if !ok {
		t.Fatalf("progress message should yield installProgressMsg, got %T", msg)
	}
	if string(progMsg) != "Downloading..." {
		t.Errorf("expected 'Downloading...', got %q", progMsg)
	}
}

func TestWaitForProgress_ErrorMessage(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	ch := make(chan string, 1)
	ch <- "ERROR:disk not found"
	m.progressCh = ch
	cmdFn := m.waitForProgress()
	msg := cmdFn()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("error message should yield installDoneMsg, got %T", msg)
	}
	if done.err == nil {
		t.Fatal("error message should have non-nil error")
	}
	if !strings.Contains(done.err.Error(), "disk not found") {
		t.Errorf("expected 'disk not found' in error, got %v", done.err)
	}
}

func TestWaitForProgress_PanicMessage(t *testing.T) {
	w := newTestWizard()
	m := New(w)
	ch := make(chan string, 1)
	ch <- "PANIC: nil pointer"
	m.progressCh = ch
	cmdFn := m.waitForProgress()
	msg := cmdFn()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("panic message should yield installDoneMsg, got %T", msg)
	}
	if done.err == nil {
		t.Fatal("panic message should have non-nil error")
	}
	if strings.HasPrefix(done.err.Error(), "PANIC:") {
		t.Errorf("PANIC prefix should be stripped from user-facing error, got: %v", done.err)
	}
}

// ── sysextItem.FilterValue ────────────────────────────────────────────────────

func TestSysextItemFilterValue(t *testing.T) {
	item := sysextItem{
		idx: 0,
		entry: model.SysextEntry{
			Name:        "docker",
			Category:    "Containers",
			SupportTier: bakery.TierIntegrated,
		},
	}
	fv := item.FilterValue()
	for _, want := range []string{"docker", "Containers", bakery.TierIntegrated} {
		if !strings.Contains(fv, want) {
			t.Errorf("FilterValue() should contain %q, got %q", want, fv)
		}
	}
}

// ── hashPassword ──────────────────────────────────────────────────────────────

func TestHashPassword_TooLong(t *testing.T) {
	// bcrypt rejects passwords > 72 bytes
	long := strings.Repeat("a", 73)
	_, err := hashPassword(long)
	if err == nil {
		t.Error("expected error for password > 72 bytes")
	}
}

func TestHashPassword_AtLimit(t *testing.T) {
	exactly72 := strings.Repeat("a", 72)
	hash, err := hashPassword(exactly72)
	if err != nil {
		t.Errorf("password of exactly 72 bytes should succeed, got: %v", err)
	}
	if hash == "" {
		t.Error("hash should not be empty at 72-byte limit")
	}
}

// ── viewInstall (active install path) ────────────────────────────────────────

func TestViewInstall_ActivelyInstalling(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	w.State.ProgressMessages = []string{"Partitioning disk..."}
	m := New(w)
	m.installing = true
	view := m.viewInstall()
	if !strings.Contains(view, "Partitioning disk") {
		t.Errorf("active install view should show prior phases, got:\n%s", view)
	}
	if !strings.Contains(view, "Working...") {
		t.Errorf("active install view should show spinner text, got:\n%s", view)
	}
	if strings.Contains(view, "Press Enter") {
		t.Errorf("active install view must not show Enter prompt, got:\n%s", view)
	}
}
