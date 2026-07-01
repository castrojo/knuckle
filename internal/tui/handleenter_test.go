package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbluefin/knuckle/internal/model"
)

// --- handleEnter: StepUpdate ---

func TestHandleEnter_UpdateStrategy_Reboot(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}

	m := New(w)
	m.cursor = 0 // "reboot"

	_, _ = m.handleEnter()

	if m.Wizard.State.Config.UpdateStrategy.RebootStrategy != "reboot" {
		t.Errorf("expected reboot strategy, got %q", m.Wizard.State.Config.UpdateStrategy.RebootStrategy)
	}
	if m.Wizard.State.CurrentStep != model.StepReview {
		t.Errorf("expected StepReview after Update, got %v", m.Wizard.State.CurrentStep)
	}
}

func TestHandleEnter_UpdateStrategy_Off(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}

	m := New(w)
	m.cursor = 1 // "off"

	_, _ = m.handleEnter()

	if m.Wizard.State.Config.UpdateStrategy.RebootStrategy != "off" {
		t.Errorf("expected off strategy, got %q", m.Wizard.State.Config.UpdateStrategy.RebootStrategy)
	}
}

func TestHandleEnter_UpdateStrategy_EtcdLock(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepUpdate
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}

	m := New(w)
	m.cursor = 2 // "etcd-lock"

	_, _ = m.handleEnter()

	if m.Wizard.State.Config.UpdateStrategy.RebootStrategy != "etcd-lock" {
		t.Errorf("expected etcd-lock strategy, got %q", m.Wizard.State.Config.UpdateStrategy.RebootStrategy)
	}
}

// --- handleEnter: StepStorage with IgnitionURL ---

func TestHandleEnter_StorageWithIgnitionURL_SkipsToReview(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	w.State.Config.Channel = "stable"
	w.State.Disks = []model.DiskInfo{
		{DevPath: "/dev/vda", Model: "QEMU", SizeHuman: "50G"},
	}

	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tuiModel := newModel.(*Model)

	if tuiModel.Wizard.State.CurrentStep != model.StepReview {
		t.Errorf("expected StepReview (IgnitionURL skip), got %v", tuiModel.Wizard.State.CurrentStep)
	}
	if tuiModel.Wizard.State.Config.Disk.DevPath != "/dev/vda" {
		t.Errorf("expected disk to be set, got %q", tuiModel.Wizard.State.Config.Disk.DevPath)
	}
}

func TestHandleEnter_StorageWithIgnitionURL_ValidationError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	w.State.Config.Channel = "stable"
	// No disks — cursor won't select a disk, so Disk.DevPath stays empty → validation fails
	w.State.Disks = []model.DiskInfo{}

	m := New(w)
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tuiModel := newModel.(*Model)

	// Should stay on Storage with an error
	if tuiModel.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("expected to stay on StepStorage on validation error, got %v", tuiModel.Wizard.State.CurrentStep)
	}
	if tuiModel.err == nil {
		t.Error("expected validation error, got nil")
	}
}

// --- handleEnter: StepWelcome ---

func TestHandleEnter_Welcome_ChannelSelection(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "stable" // pre-set to pass validation
	w.State.Config.Hostname = "test"

	m := New(w)
	// Phase 1: OS picker — select Flatcar (cursor 0)
	m.cursor = 0
	_, _ = m.handleEnter()

	// Phase 2: channel picker — select "beta" (index 2 in stable,lts,beta,alpha)
	m.cursor = 2
	_, _ = m.handleEnter()

	if m.Wizard.State.Config.Channel != "beta" {
		t.Errorf("expected channel beta, got %q", m.Wizard.State.Config.Channel)
	}
}

func TestHandleEnter_Welcome_WithIgnitionURL(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "https://example.com/config.ign"

	m := New(w)
	// Phase 1: OS picker — select Flatcar
	m.cursor = 0
	_, _ = m.handleEnter()

	// Phase 2: channel picker — with IgnitionURL set, should skip to Storage
	m.cursor = 0
	_, _ = m.handleEnter()

	if m.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("expected StepStorage (IgnitionURL skip), got %v", m.Wizard.State.CurrentStep)
	}
}

// --- handleEnter: StepInstall ---

func TestHandleEnter_Install_StartsInstall(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}

	m := New(w)
	m.installing = false

	_, cmd := m.handleEnter()

	if !m.installing {
		t.Error("expected installing=true after Enter on Install step")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (startInstall)")
	}
}

func TestHandleEnter_Install_AlreadyInstalling(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepInstall
	m := New(w)
	m.installing = true // already running

	_, cmd := m.handleEnter()

	// Should be a no-op
	if cmd != nil {
		t.Error("expected nil cmd when already installing")
	}
}

// --- handleEnter: StepNvidia ---

func TestHandleEnter_Nvidia_SelectsDriver(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepNvidia
	w.State.Config.Channel = "stable"
	w.State.Config.Hostname = "test"
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/vda"}
	w.State.Config.Users = []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA"}}}
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA"}
	w.State.Sysexts = []model.SysextEntry{{Name: "nvidia-runtime", Selected: true}}

	m := New(w)
	m.cursor = 1 // second driver option

	_, _ = m.handleEnter()

	if m.Wizard.State.Config.NvidiaDriverVersion != model.NvidiaDriverOptions[1].ID {
		t.Errorf("expected driver %q, got %q",
			model.NvidiaDriverOptions[1].ID, m.Wizard.State.Config.NvidiaDriverVersion)
	}
}

// --- handleEnter: validation error on Next() ---

func TestHandleEnter_ValidationError(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	// No disk selected → validateStorage will fail
	w.State.Config.Channel = "stable"
	w.State.Disks = []model.DiskInfo{}

	m := New(w)

	_, _ = m.handleEnter()

	if m.err == nil {
		t.Error("expected validation error from Next()")
	}
	// Should stay on same step
	if m.Wizard.State.CurrentStep != model.StepStorage {
		t.Errorf("expected to stay on StepStorage, got %v", m.Wizard.State.CurrentStep)
	}
}

// --- maxCursor ---

func TestMaxCursor_AllSteps(t *testing.T) {
	tests := []struct {
		step   model.WizardStep
		disks  int
		expect int
	}{
		{model.StepWelcome, 0, 3}, // 3 OS choices (Flatcar | FCOS | Bluefin Server) in OS picker
		{model.StepStorage, 3, 3}, // number of disks
		{model.StepStorage, 0, 0}, // no disks
		{model.StepSysext, 0, 0},  // empty sysexts
		{model.StepNvidia, 0, len(model.NvidiaDriverOptions)},
		{model.StepUpdate, 0, 3},  // 3 strategies
		{model.StepNetwork, 0, 1}, // default fallback
		{model.StepReview, 0, 1},  // default
	}

	for _, tc := range tests {
		w := newTestWizard()
		w.State.CurrentStep = tc.step
		for i := 0; i < tc.disks; i++ {
			w.State.Disks = append(w.State.Disks, model.DiskInfo{DevPath: "/dev/vda"})
		}
		m := New(w)
		got := m.maxCursor()
		if got != tc.expect {
			t.Errorf("maxCursor(%v, disks=%d) = %d, want %d", tc.step, tc.disks, got, tc.expect)
		}
	}
}

func TestMaxCursor_SysextWithEntries(t *testing.T) {
	w := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker"},
		{Name: "wasmtime"},
	}
	m := New(w)
	if got := m.maxCursor(); got != 2 {
		t.Errorf("maxCursor(StepSysext) with 2 entries = %d, want 2", got)
	}
}
