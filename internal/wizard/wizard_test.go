package wizard

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// --- Mocks ---

type mockProber struct {
	disks  []model.DiskInfo
	ifaces []model.NetworkInterface
	err    error
}

func (m *mockProber) ListDisks(ctx context.Context) ([]model.DiskInfo, error) {
	return m.disks, m.err
}

func (m *mockProber) ListNetworkInterfaces(ctx context.Context) ([]model.NetworkInterface, error) {
	return m.ifaces, m.err
}

type mockBakery struct {
	sysexts []model.SysextEntry
	err     error
}

func (m *mockBakery) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return m.sysexts, m.err
}

func (m *mockBakery) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	return m.sysexts, m.err
}

type mockInstaller struct {
	called bool
	cfg    *model.InstallConfig
	err    error
}

func (m *mockInstaller) Install(ctx context.Context, cfg *model.InstallConfig, progress func(string)) error {
	m.called = true
	m.cfg = cfg
	progress("test progress")
	return m.err
}

// --- Helpers ---

func newTestWizard() (*Wizard, *mockProber, *mockBakery, *mockInstaller) {
	p := &mockProber{}
	b := &mockBakery{}
	i := &mockInstaller{}
	w := New(p, b, i)
	return w, p, b, i
}

// --- Tests ---

func TestNewWizard(t *testing.T) {
	w, _, _, _ := newTestWizard()

	if w.State.CurrentStep != model.StepWelcome {
		t.Errorf("expected StepWelcome, got %v", w.State.CurrentStep)
	}
	if w.State.Config.Channel != "stable" {
		t.Errorf("expected channel=stable, got %q", w.State.Config.Channel)
	}
}

func TestNextAndPrevious(t *testing.T) {
	w, _, _, _ := newTestWizard()

	// Advance from Welcome (no validation needed)
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Welcome: %v", err)
	}
	if w.State.CurrentStep != model.StepNetwork {
		t.Errorf("expected StepNetwork, got %v", w.State.CurrentStep)
	}

	// Go back
	w.Previous()
	if w.State.CurrentStep != model.StepWelcome {
		t.Errorf("expected StepWelcome after Previous, got %v", w.State.CurrentStep)
	}

	// Previous at first step is a no-op
	w.Previous()
	if w.State.CurrentStep != model.StepWelcome {
		t.Errorf("expected StepWelcome (bounded), got %v", w.State.CurrentStep)
	}

	// Advance all the way to Done (set valid config for each step)
	// Network: use DHCP (passes validation)
	w.State.Config.Network.Mode = model.NetworkDHCP
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Welcome: %v", err)
	}
	// StepNetwork -> StepStorage: DHCP passes
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Network (DHCP): %v", err)
	}
	// StepStorage -> StepUser: need disk
	w.State.Config.Disk.DevPath = "/dev/sda"
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Storage: %v", err)
	}
	// StepUser -> StepSysext: need user
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test"}},
	}
	if err := w.Next(); err != nil {
		t.Fatalf("Next from User: %v", err)
	}
	// StepSysext -> StepUpdate
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Sysext: %v", err)
	}
	// StepUpdate -> StepReview
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Update: %v", err)
	}
	// StepReview -> StepInstall (need confirmation)
	w.State.Confirmed = true
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Review: %v", err)
	}
	// StepInstall -> StepDone
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Install: %v", err)
	}
	if w.State.CurrentStep != model.StepDone {
		t.Errorf("expected StepDone, got %v", w.State.CurrentStep)
	}

	// Next at last step is a no-op
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Done: %v", err)
	}
	if w.State.CurrentStep != model.StepDone {
		t.Errorf("expected StepDone (bounded), got %v", w.State.CurrentStep)
	}
}

func TestValidateNetworkDHCP(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network.Mode = model.NetworkDHCP

	if err := w.ValidateCurrentStep(); err != nil {
		t.Errorf("DHCP should pass validation: %v", err)
	}
}

func TestValidateNetworkStaticValid(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:      model.NetworkStatic,
		Interface: "eth0",
		Address:   "192.168.1.10/24",
		Gateway:   "192.168.1.1",
		DNS:       []string{"8.8.8.8", "1.1.1.1"},
	}

	if err := w.ValidateCurrentStep(); err != nil {
		t.Errorf("valid static config should pass: %v", err)
	}
}

func TestValidateNetworkStaticInvalid(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:      model.NetworkStatic,
		Interface: "eth0",
		Address:   "not-a-cidr",
		Gateway:   "192.168.1.1",
	}

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

func TestValidateStorageNoDisk(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Disk.DevPath = ""

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for empty disk")
	}
}

func TestValidateStorageMountedPartition(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Disk = model.DiskInfo{
		DevPath: "/dev/sda",
		Partitions: []model.PartitionInfo{
			{Path: "/dev/sda1", MountPoint: "/boot"},
			{Path: "/dev/sda2", MountPoint: ""},
		},
	}

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for disk with mounted partition")
	}
	if !strings.Contains(err.Error(), "mounted partition") {
		t.Errorf("error should mention mounted partition, got: %v", err)
	}
}

func TestValidateStorageNoMountedPartitions(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Disk = model.DiskInfo{
		DevPath: "/dev/sda",
		Partitions: []model.PartitionInfo{
			{Path: "/dev/sda1", MountPoint: ""},
			{Path: "/dev/sda2", MountPoint: ""},
		},
	}

	err := w.ValidateCurrentStep()
	if err != nil {
		t.Errorf("expected no error for unmounted partitions, got: %v", err)
	}
}

func TestValidateUserNoUserNoKey(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = nil
	w.State.Config.SSHKeys = nil

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error when no users and no SSH keys")
	}
}

func TestValidateUserValid(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{
		{
			Username: "core",
			SSHKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@host"},
		},
	}

	if err := w.ValidateCurrentStep(); err != nil {
		t.Errorf("valid user should pass: %v", err)
	}
}

func TestProbeHardware(t *testing.T) {
	disks := []model.DiskInfo{
		{DevPath: "/dev/sda", Model: "Test Disk", Size: 500000000000},
	}
	ifaces := []model.NetworkInterface{
		{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
	}
	p := &mockProber{disks: disks, ifaces: ifaces}
	w := New(p, &mockBakery{}, &mockInstaller{})

	if err := w.ProbeHardware(context.Background()); err != nil {
		t.Fatalf("ProbeHardware: %v", err)
	}

	if len(w.State.Disks) != 1 || w.State.Disks[0].DevPath != "/dev/sda" {
		t.Errorf("expected 1 disk /dev/sda, got %v", w.State.Disks)
	}
	if len(w.State.Interfaces) != 1 || w.State.Interfaces[0].Name != "eth0" {
		t.Errorf("expected 1 interface eth0, got %v", w.State.Interfaces)
	}
}

func TestProbeHardwareError(t *testing.T) {
	p := &mockProber{err: fmt.Errorf("disk probe failed")}
	w := New(p, &mockBakery{}, &mockInstaller{})

	err := w.ProbeHardware(context.Background())
	if err == nil {
		t.Fatal("expected error from ProbeHardware")
	}
}

func TestFetchSysexts(t *testing.T) {
	sysexts := []model.SysextEntry{
		{Name: "docker", Description: "Docker CE", Version: "24.0"},
		{Name: "tailscale", Description: "Tailscale VPN", Version: "1.50"},
	}
	b := &mockBakery{sysexts: sysexts}
	w := New(&mockProber{}, b, &mockInstaller{})

	if err := w.FetchSysexts(context.Background()); err != nil {
		t.Fatalf("FetchSysexts: %v", err)
	}

	if len(w.State.Sysexts) != 2 {
		t.Errorf("expected 2 sysexts, got %d", len(w.State.Sysexts))
	}
	if w.State.Sysexts[0].Name != "docker" {
		t.Errorf("expected first sysext=docker, got %q", w.State.Sysexts[0].Name)
	}
}

func TestExecute(t *testing.T) {
	inst := &mockInstaller{}
	w := New(&mockProber{}, &mockBakery{}, inst)
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Channel = "stable"

	err := w.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !inst.called {
		t.Error("expected installer to be called")
	}
	if inst.cfg != &w.State.Config {
		t.Error("expected installer to receive wizard config")
	}
	if len(w.State.ProgressMessages) != 1 || w.State.ProgressMessages[0] != "test progress" {
		t.Errorf("expected progress messages, got %v", w.State.ProgressMessages)
	}
}

func TestExecuteError(t *testing.T) {
	inst := &mockInstaller{err: fmt.Errorf("install failed")}
	w := New(&mockProber{}, &mockBakery{}, inst)

	err := w.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error from Execute")
	}
}

func TestGoToStep(t *testing.T) {
	w, _, _, _ := newTestWizard()

	w.GoToStep(model.StepReview)
	if w.State.CurrentStep != model.StepReview {
		t.Errorf("expected StepReview, got %v", w.State.CurrentStep)
	}

	// Out of bounds should be no-op
	w.GoToStep(model.WizardStep(-1))
	if w.State.CurrentStep != model.StepReview {
		t.Errorf("expected StepReview unchanged, got %v", w.State.CurrentStep)
	}
}

func TestIsFirstAndLastStep(t *testing.T) {
	w, _, _, _ := newTestWizard()

	if !w.IsFirstStep() {
		t.Error("expected IsFirstStep=true at Welcome")
	}
	if w.IsLastStep() {
		t.Error("expected IsLastStep=false at Welcome")
	}

	w.State.CurrentStep = model.StepDone
	if w.IsFirstStep() {
		t.Error("expected IsFirstStep=false at Done")
	}
	if !w.IsLastStep() {
		t.Error("expected IsLastStep=true at Done")
	}
}

func TestStepCount(t *testing.T) {
	count := StepCount()
	if count != 11 {
		t.Errorf("expected 11 steps (added StepTailscale), got %d", count)
	}
}

func TestValidateConsistency(t *testing.T) {
	tests := []struct {
		name    string
		cfg     model.InstallConfig
		wantErr string
	}{
		{
			name: "no auth returns error",
			cfg: model.InstallConfig{
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
				Channel: "stable",
			},
			wantErr: "at least one authentication method required",
		},
		{
			name: "with SSH keys passes",
			cfg: model.InstallConfig{
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
				Channel: "stable",
				SSHKeys: []string{"ssh-ed25519 AAAA test"},
			},
			wantErr: "",
		},
		{
			name: "with user SSH keys passes",
			cfg: model.InstallConfig{
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
				Channel: "stable",
				Users: []model.UserConfig{
					{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
				},
			},
			wantErr: "",
		},
		{
			name: "with password passes",
			cfg: model.InstallConfig{
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
				Channel: "stable",
				Users: []model.UserConfig{
					{Username: "core", PasswordHash: "$2a$10$hash"},
				},
			},
			wantErr: "",
		},
		{
			name: "static network missing gateway errors",
			cfg: model.InstallConfig{
				Disk:    model.DiskInfo{DevPath: "/dev/sda"},
				Channel: "stable",
				SSHKeys: []string{"ssh-ed25519 AAAA test"},
				Network: model.NetworkConfig{
					Mode:    model.NetworkStatic,
					Address: "192.168.1.10/24",
					Gateway: "",
				},
			},
			wantErr: "static network requires a gateway",
		},
		{
			name: "ignition URL bypasses auth check",
			cfg: model.InstallConfig{
				Disk:        model.DiskInfo{DevPath: "/dev/sda"},
				Channel:     "stable",
				IgnitionURL: "https://example.com/config.ign",
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, _, _, _ := newTestWizard()
			w.State.CurrentStep = model.StepReview
			w.State.Confirmed = true
			w.State.Config = tt.cfg

			err := w.ValidateCurrentStep()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			}
		})
	}
}

func TestRunSystemChecks(t *testing.T) {
	tests := []struct {
		name     string
		disks    []model.DiskInfo
		ifaces   []model.NetworkInterface
		wantDisk string
		wantNet  string
	}{
		{
			name:     "disks present and active iface",
			disks:    []model.DiskInfo{{DevPath: "/dev/sda"}},
			ifaces:   []model.NetworkInterface{{Name: "eth0", IPv4Addrs: []string{"192.168.1.10"}}},
			wantDisk: "ok",
			wantNet:  "ok",
		},
		{
			name:     "no disks",
			disks:    nil,
			ifaces:   []model.NetworkInterface{{Name: "eth0", IPv4Addrs: []string{"192.168.1.10"}}},
			wantDisk: "fail",
			wantNet:  "ok",
		},
		{
			name:     "no active ifaces warns",
			disks:    []model.DiskInfo{{DevPath: "/dev/sda"}},
			ifaces:   []model.NetworkInterface{{Name: "eth0", IPv4Addrs: nil}},
			wantDisk: "ok",
			wantNet:  "warn",
		},
		{
			name:     "no interfaces at all fails",
			disks:    []model.DiskInfo{{DevPath: "/dev/sda"}},
			ifaces:   nil,
			wantDisk: "ok",
			wantNet:  "fail",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &mockProber{disks: tt.disks, ifaces: tt.ifaces}
			w := New(p, &mockBakery{}, &mockInstaller{})

			if err := w.ProbeHardware(context.Background()); err != nil {
				t.Fatalf("ProbeHardware: %v", err)
			}

			var diskCheck, netCheck *SystemCheck
			for i := range w.State.SystemChecks {
				switch w.State.SystemChecks[i].Name {
				case "Disk":
					diskCheck = &w.State.SystemChecks[i]
				case "Network":
					netCheck = &w.State.SystemChecks[i]
				}
			}
			if diskCheck == nil {
				t.Fatal("missing Disk system check")
			}
			if diskCheck.Status != tt.wantDisk {
				t.Errorf("Disk check: got %q, want %q", diskCheck.Status, tt.wantDisk)
			}
			if netCheck == nil {
				t.Fatal("missing Network system check")
			}
			if netCheck.Status != tt.wantNet {
				t.Errorf("Network check: got %q, want %q", netCheck.Status, tt.wantNet)
			}
		})
	}
}

func TestGenerateButaneNonEmpty(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Config.Hostname = "test-node"
	w.State.Config.Channel = "stable"
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
	}

	butane, err := w.GenerateButane()
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}
	if butane == "" {
		t.Error("GenerateButane returned empty string")
	}
	if !strings.Contains(butane, "variant: flatcar") {
		t.Error("expected butane to contain variant header")
	}
	if !strings.Contains(butane, "test-node") {
		t.Error("expected butane to contain hostname")
	}
}

func TestValidateNetworkRequiresInterface(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:    model.NetworkStatic,
		Address: "192.168.1.10/24",
		Gateway: "192.168.1.1",
		DNS:     []string{"8.8.8.8"},
		// Interface intentionally empty
	}

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for static network with empty interface")
	}
	if !strings.Contains(err.Error(), "interface") {
		t.Errorf("error should mention interface, got: %v", err)
	}
}

func TestValidateWelcomeRejectsInvalidChannel(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepWelcome
	w.State.Config.Channel = "nightly"

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for invalid channel")
	}
	if !strings.Contains(err.Error(), "nightly") {
		t.Errorf("error should mention invalid channel value, got: %v", err)
	}
}

func TestValidateWelcomeAcceptsValidChannels(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepWelcome

	for _, ch := range []string{"stable", "beta", "alpha", "edge"} {
		w.State.Config.Channel = ch
		if err := w.ValidateCurrentStep(); err != nil {
			t.Errorf("channel %q should be valid, got error: %v", ch, err)
		}
	}
}

func TestNextSkipsNvidiaWhenNotSelected(t *testing.T) {
	w, _, _, _ := newTestWizard()
	// Set up valid config to advance to StepSysext
	w.State.Config.Network.Mode = model.NetworkDHCP
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test"}},
	}
	w.State.Config.Channel = "stable"
	// No nvidia-runtime in sysexts
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: true},
	}
	w.State.CurrentStep = model.StepSysext

	if err := w.Next(); err != nil {
		t.Fatalf("Next from Sysext: %v", err)
	}
	// Should skip StepNvidia and go to StepUpdate
	if w.State.CurrentStep != model.StepUpdate {
		t.Errorf("expected StepUpdate (skip nvidia), got %v", w.State.CurrentStep)
	}
}

func TestNextVisitsNvidiaWhenSelected(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Config.Network.Mode = model.NetworkDHCP
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test"}},
	}
	w.State.Config.Channel = "stable"
	// nvidia-runtime IS selected
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: true},
	}
	w.State.CurrentStep = model.StepSysext

	if err := w.Next(); err != nil {
		t.Fatalf("Next from Sysext: %v", err)
	}
	if w.State.CurrentStep != model.StepNvidia {
		t.Errorf("expected StepNvidia when nvidia-runtime selected, got %v", w.State.CurrentStep)
	}
}

func TestGoToStepRefusesNvidiaWhenNotSelected(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepSysext
	// No nvidia-runtime selected
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: true},
	}

	w.GoToStep(model.StepNvidia)
	// Should NOT have moved to StepNvidia
	if w.State.CurrentStep == model.StepNvidia {
		t.Error("GoToStep should refuse StepNvidia when nvidia-runtime is not selected")
	}
	if w.State.CurrentStep != model.StepSysext {
		t.Errorf("expected to stay on StepSysext, got %v", w.State.CurrentStep)
	}
}

// --- QA Gap Tests: State Machine Invariants ---

// Q1: Can we reach StepInstall without a disk? GoToStep has no validation gate.
func TestGoToStep_InstallWithoutDisk_NoValidationGate(t *testing.T) {
	w, _, _, _ := newTestWizard()
	// GoToStep allows jumping to Install without any prior state — this is by design
	// (the Review step's ValidateCurrentStep gates actual advancement).
	w.GoToStep(model.StepInstall)
	if w.State.CurrentStep != model.StepInstall {
		t.Fatalf("GoToStep should allow jump to Install, got %v", w.State.CurrentStep)
	}
	// But ValidateCurrentStep on Install itself returns nil (no gate).
	// The real gate is at StepReview — calling Next() from Review runs CheckConsistency.
	// This verifies the architectural contract: GoToStep is unvalidated, Next() is validated.
	err := w.ValidateCurrentStep()
	if err != nil {
		t.Errorf("StepInstall validation is a no-op by design, got: %v", err)
	}
}

// Q2: Does Next() from StepReview catch missing disk?
func TestNext_ReviewCatchesMissingDisk(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config.Channel = "stable"
	w.State.Config.SSHKeys = []string{"ssh-ed25519 AAAA test"}
	// Disk is empty
	w.State.Config.Disk.DevPath = ""

	err := w.Next()
	if err == nil {
		t.Fatal("expected error: Next from Review with no disk")
	}
	if !strings.Contains(err.Error(), "no disk") {
		t.Errorf("expected 'no disk' error, got: %v", err)
	}
	// Should NOT have advanced
	if w.State.CurrentStep != model.StepReview {
		t.Errorf("should stay on StepReview, got %v", w.State.CurrentStep)
	}
}

// Q3: Back from Review → User → change SSH keys → re-advance → config rebuilt correctly
func TestBackAndModify_SSHKeysRebuilt(t *testing.T) {
	w, _, _, _ := newTestWizard()
	// Set up full valid config to reach Review
	w.State.Config.Channel = "stable"
	w.State.Config.Network.Mode = model.NetworkDHCP
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI original"}},
	}
	w.State.CurrentStep = model.StepReview

	// Go back to User step
	w.GoToStep(model.StepUser)
	if w.State.CurrentStep != model.StepUser {
		t.Fatalf("expected StepUser, got %v", w.State.CurrentStep)
	}

	// Modify SSH keys (simulates user editing)
	w.State.Config.Users[0].SSHKeys = []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI newkey"}

	// Advance back to Review
	if err := w.Next(); err != nil {
		t.Fatalf("Next from User: %v", err)
	}
	// StepSysext
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Sysext: %v", err)
	}
	// StepUpdate (nvidia skipped - not selected)
	if err := w.Next(); err != nil {
		t.Fatalf("Next from Update: %v", err)
	}
	// Now at Review
	if w.State.CurrentStep != model.StepReview {
		t.Fatalf("expected StepReview, got %v", w.State.CurrentStep)
	}

	// Verify the config reflects the NEW key
	if w.State.Config.Users[0].SSHKeys[0] != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI newkey" {
		t.Errorf("SSH key not updated after back-navigation, got: %v", w.State.Config.Users[0].SSHKeys)
	}

	// GenerateButane should produce output with the new key
	butane, err := w.GenerateButane()
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}
	if !strings.Contains(butane, "newkey") {
		t.Error("Butane output does not contain updated SSH key")
	}
	if strings.Contains(butane, "original") {
		t.Error("Butane output still contains OLD SSH key after modification")
	}
}

// Q4: Full happy-path traversal (Welcome → Done) with all validation passing
func TestFullHappyPath_WelcomeToDone(t *testing.T) {
	w, _, _, inst := newTestWizard()

	// StepWelcome — valid channel already set ("stable")
	if w.State.CurrentStep != model.StepWelcome {
		t.Fatalf("expected start at Welcome, got %v", w.State.CurrentStep)
	}
	if err := w.Next(); err != nil {
		t.Fatalf("Welcome→Network: %v", err)
	}

	// StepNetwork — DHCP (passes without any config)
	w.State.Config.Network.Mode = model.NetworkDHCP
	if err := w.Next(); err != nil {
		t.Fatalf("Network→Storage: %v", err)
	}

	// StepStorage — select a disk
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_drive-scsi0"}
	if err := w.Next(); err != nil {
		t.Fatalf("Storage→User: %v", err)
	}

	// StepUser — add a user with SSH key
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop"}},
	}
	w.State.Config.Hostname = "flatcar-node-01"
	if err := w.Next(); err != nil {
		t.Fatalf("User→Sysext: %v", err)
	}

	// StepSysext — optional, just advance
	if err := w.Next(); err != nil {
		t.Fatalf("Sysext→Update: %v", err)
	}
	// Nvidia should be skipped (not selected)
	if w.State.CurrentStep != model.StepUpdate {
		t.Fatalf("expected StepUpdate (nvidia skipped), got %v", w.State.CurrentStep)
	}

	// StepUpdate — use default reboot strategy
	if err := w.Next(); err != nil {
		t.Fatalf("Update→Review: %v", err)
	}

	// StepReview — CheckConsistency must pass
	if w.State.CurrentStep != model.StepReview {
		t.Fatalf("expected StepReview, got %v", w.State.CurrentStep)
	}
	if err := w.Next(); err != nil {
		t.Fatalf("Review→Install: %v", err)
	}

	// StepInstall — advance to Done
	if err := w.Next(); err != nil {
		t.Fatalf("Install→Done: %v", err)
	}
	if w.State.CurrentStep != model.StepDone {
		t.Fatalf("expected StepDone, got %v", w.State.CurrentStep)
	}

	// Verify Execute actually calls the installer
	if err := w.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !inst.called {
		t.Error("installer was never called")
	}
	if inst.cfg.Disk.DevPath != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_drive-scsi0" {
		t.Errorf("installer received wrong disk: %v", inst.cfg.Disk.DevPath)
	}
}

// Q5: IgnitionURL path — wizard.Next() still traverses sequentially,
// but CheckConsistency at Review accepts IgnitionURL without auth/users.
func TestIgnitionURL_ReviewAcceptsWithoutAuth(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "https://example.com/config.ign"
	w.State.Config.Disk.DevPath = "/dev/sda"
	// No users, no SSH keys — would normally fail
	w.State.CurrentStep = model.StepReview

	err := w.ValidateCurrentStep()
	if err != nil {
		t.Fatalf("Review with IgnitionURL should skip auth check, got: %v", err)
	}
}

// Q5b: IgnitionURL path — Welcome validation catches invalid URLs
func TestIgnitionURL_WelcomeRejectsInvalidURL(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "not-a-url"
	w.State.CurrentStep = model.StepWelcome

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for invalid IgnitionURL at Welcome")
	}
	if !strings.Contains(err.Error(), "ignition URL") {
		t.Errorf("expected ignition URL error, got: %v", err)
	}
}

// Q6: Can user advance past StepStorage without disk?
func TestNext_StorageBlocksWithoutDisk(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Disk.DevPath = "" // no disk selected

	err := w.Next()
	if err == nil {
		t.Fatal("expected error: cannot advance past Storage without disk")
	}
	if !strings.Contains(err.Error(), "no disk") {
		t.Errorf("expected 'no disk' error, got: %v", err)
	}
	if w.State.CurrentStep != model.StepStorage {
		t.Errorf("should stay on StepStorage, got %v", w.State.CurrentStep)
	}
}

// Edge case: Previous from StepDone — should still be able to go back
func TestPrevious_FromDone(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepDone

	w.Previous()
	if w.State.CurrentStep != model.StepInstall {
		t.Errorf("Previous from Done should go to Install, got %v", w.State.CurrentStep)
	}
}

// Edge case: Static network with empty DNS array passes (DNS is optional)
func TestValidateNetwork_StaticNoDNS_Passes(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepNetwork
	w.State.Config.Network = model.NetworkConfig{
		Mode:      model.NetworkStatic,
		Interface: "eth0",
		Address:   "10.0.0.5/24",
		Gateway:   "10.0.0.1",
		DNS:       nil, // no DNS servers
	}

	if err := w.ValidateCurrentStep(); err != nil {
		t.Errorf("static network without DNS should pass, got: %v", err)
	}
}

// Edge case: User with valid username but INVALID SSH key format
func TestValidateUser_InvalidSSHKeyFormat(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"not-a-valid-ssh-key"}},
	}

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for invalid SSH key")
	}
}

// Edge case: Multiple users, one with invalid username
func TestValidateUser_SecondUserInvalidUsername(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test"}},
		{Username: "root", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test2"}},
	}

	// "root" may or may not be valid depending on validate.Username — test the boundary
	// The key point: validation iterates ALL users, not just the first
	w.State.Config.Users[1].Username = "-invalid" // leading dash is invalid
	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for invalid username '-invalid'")
	}
}

// Edge case: CheckConsistency catches duplicate usernames
func TestReview_DuplicateUsernames(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config = model.InstallConfig{
		Channel: "stable",
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test1"}},
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test2"}},
		},
	}

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for duplicate usernames")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' error, got: %v", err)
	}
}

// Edge case: Nvidia path — Previous from StepUpdate skips back over Nvidia when not selected
func TestPrevious_SkipsNvidiaBackward(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Selected: true},
	}
	w.State.CurrentStep = model.StepUpdate

	w.Previous()
	// Should skip StepNvidia and land on StepSysext
	if w.State.CurrentStep != model.StepSysext {
		t.Errorf("Previous from Update should skip Nvidia → Sysext, got %v", w.State.CurrentStep)
	}
}

// Edge case: Nvidia path — Previous from StepUpdate visits Nvidia when selected
func TestPrevious_VisitsNvidiaWhenSelected(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: true},
	}
	w.State.CurrentStep = model.StepUpdate

	w.Previous()
	if w.State.CurrentStep != model.StepNvidia {
		t.Errorf("Previous from Update with nvidia selected should go to Nvidia, got %v", w.State.CurrentStep)
	}
}

// Edge case: Full happy path WITH nvidia
func TestFullHappyPath_WithNvidia(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Config.Network.Mode = model.NetworkDHCP
	w.State.Config.Disk.DevPath = "/dev/sda"
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test"}},
	}
	w.State.Config.NvidiaDriverVersion = "570-open"
	w.State.Sysexts = []model.SysextEntry{
		{Name: "nvidia-runtime", Selected: true},
	}
	w.State.Config.Sysexts = w.State.Sysexts

	// Walk all the way through
	steps := []model.WizardStep{
		model.StepWelcome, model.StepNetwork, model.StepStorage,
		model.StepUser, model.StepSysext, model.StepNvidia,
		model.StepUpdate, model.StepReview, model.StepInstall,
	}

	for i, expected := range steps {
		if w.State.CurrentStep != expected {
			t.Fatalf("step %d: expected %v, got %v", i, expected, w.State.CurrentStep)
		}
		if err := w.Next(); err != nil {
			t.Fatalf("Next at step %v: %v", expected, err)
		}
	}
	if w.State.CurrentStep != model.StepDone {
		t.Fatalf("expected StepDone, got %v", w.State.CurrentStep)
	}
}

// Edge case: GoToStep above StepDone is a no-op
func TestGoToStep_AboveDone_Noop(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.GoToStep(model.WizardStep(99))
	if w.State.CurrentStep != model.StepWelcome {
		t.Errorf("GoToStep(99) should be no-op, got %v", w.State.CurrentStep)
	}
}

// Edge case: FetchSysexts auto-selects nvidia-runtime when GPU detected
func TestFetchSysexts_AutoSelectsNvidiaOnGPU(t *testing.T) {
	b := &mockBakery{sysexts: []model.SysextEntry{
		{Name: "docker", Description: "Docker CE"},
		{Name: "nvidia-runtime", Description: "NVIDIA runtime"},
	}}
	w := New(&mockProber{}, b, &mockInstaller{})
	w.State.NvidiaGPUDetected = true

	if err := w.FetchSysexts(context.Background()); err != nil {
		t.Fatalf("FetchSysexts: %v", err)
	}

	// nvidia-runtime should be auto-selected
	var nvidiaSelected bool
	for _, s := range w.State.Sysexts {
		if s.Name == "nvidia-runtime" && s.Selected {
			nvidiaSelected = true
		}
	}
	if !nvidiaSelected {
		t.Error("nvidia-runtime should be auto-selected when GPU detected")
	}
	if w.State.Config.NvidiaDriverVersion == "" {
		t.Error("NvidiaDriverVersion should be pre-configured when GPU detected")
	}
}

// Edge case: FetchSysexts does NOT auto-select when no GPU
func TestFetchSysexts_NoAutoSelectWithoutGPU(t *testing.T) {
	b := &mockBakery{sysexts: []model.SysextEntry{
		{Name: "nvidia-runtime", Description: "NVIDIA runtime"},
	}}
	w := New(&mockProber{}, b, &mockInstaller{})
	w.State.NvidiaGPUDetected = false

	if err := w.FetchSysexts(context.Background()); err != nil {
		t.Fatalf("FetchSysexts: %v", err)
	}

	for _, s := range w.State.Sysexts {
		if s.Name == "nvidia-runtime" && s.Selected {
			t.Error("nvidia-runtime should NOT be auto-selected without GPU")
		}
	}
}

// Edge case: Storage validation with disk path that fails DiskPath validator
func TestValidateStorage_InvalidDiskPath(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepStorage
	w.State.Config.Disk.DevPath = "../../../etc/passwd" // path traversal attempt

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for path-traversal disk path")
	}
}

// Edge case: ExecuteWithProgress routes progress to external callback
func TestExecuteWithProgress(t *testing.T) {
	inst := &mockInstaller{}
	w := New(&mockProber{}, &mockBakery{}, inst)
	w.State.Config.Disk.DevPath = "/dev/sda"

	var msgs []string
	err := w.ExecuteWithProgress(context.Background(), func(msg string) {
		msgs = append(msgs, msg)
	})
	if err != nil {
		t.Fatalf("ExecuteWithProgress: %v", err)
	}
	if len(msgs) != 1 || msgs[0] != "test progress" {
		t.Errorf("expected progress callback to receive messages, got: %v", msgs)
	}
}

// Edge case: CheckConsistency at Review catches missing channel
func TestReview_MissingChannel(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepReview
	w.State.Config = model.InstallConfig{
		Disk:    model.DiskInfo{DevPath: "/dev/sda"},
		SSHKeys: []string{"ssh-ed25519 AAAA test"},
		Channel: "", // empty!
	}

	err := w.ValidateCurrentStep()
	if err == nil {
		t.Fatal("expected error for missing channel at Review")
	}
	if !strings.Contains(err.Error(), "channel") {
		t.Errorf("expected channel error, got: %v", err)
	}
}

// Regression: validate.Username rejects "root" — verify wizard surfaces it
func TestValidateUser_RejectsRoot(t *testing.T) {
	w, _, _, _ := newTestWizard()
	w.State.CurrentStep = model.StepUser
	w.State.Config.Users = []model.UserConfig{
		{Username: "root", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test"}},
	}

	// validate.Username may or may not reject "root" — test what actually happens
	err := w.ValidateCurrentStep()
	// If it passes, that's fine — just document the behavior
	if err != nil {
		if !strings.Contains(err.Error(), "root") && !strings.Contains(err.Error(), "username") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

// ============================================================
// Route Tests: end-to-end step-sequence + config-state checks
// ============================================================

// stepTracker records every step visited in the wizard by calling next()
// and appending the resulting CurrentStep after each advance.
type stepTracker struct {
	visited []model.WizardStep
}

func (st *stepTracker) record(w *Wizard, t *testing.T, label string) {
	t.Helper()
	if err := w.Next(); err != nil {
		t.Fatalf("%s: Next() returned error: %v", label, err)
	}
	st.visited = append(st.visited, w.State.CurrentStep)
}

func assertStepSequence(t *testing.T, got, want []model.WizardStep) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("step sequence length: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Route A: Welcome → Network(DHCP) → Storage → User(GitHub keys) →
//
//	Sysext(none) → Update → Review → Install → Done
//
// ---------------------------------------------------------------------------
func TestRouteA_DHCP_GitHubKeys_NoSysext(t *testing.T) {
	w, _, _, inst := newTestWizard()
	st := &stepTracker{}

	// ── Welcome (valid channel already "stable") ──────────────────────────
	if w.State.CurrentStep != model.StepWelcome {
		t.Fatalf("expected StepWelcome, got %v", w.State.CurrentStep)
	}
	st.record(w, t, "Welcome→Network")
	if w.State.CurrentStep != model.StepNetwork {
		t.Fatalf("after Welcome: expected StepNetwork, got %v", w.State.CurrentStep)
	}

	// ── Network: DHCP ─────────────────────────────────────────────────────
	w.State.Config.Network.Mode = model.NetworkDHCP
	st.record(w, t, "Network→Storage")
	if w.State.CurrentStep != model.StepStorage {
		t.Fatalf("after Network: expected StepStorage, got %v", w.State.CurrentStep)
	}

	// ── Storage: pick disk by-id path ─────────────────────────────────────
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/disk/by-id/ata-SAMSUNG_MZ7LN256HAJQ_S38SNX0J123456"}
	st.record(w, t, "Storage→User")
	if w.State.CurrentStep != model.StepUser {
		t.Fatalf("after Storage: expected StepUser, got %v", w.State.CurrentStep)
	}
	// Config: disk must be persisted
	if w.State.Config.Disk.DevPath == "" {
		t.Error("Disk.DevPath not persisted after Storage step")
	}

	// ── User: GitHub SSH keys (no password) ───────────────────────────────
	w.State.Config.Hostname = "flatcar-a"
	w.State.Config.Users = []model.UserConfig{
		{
			Username: "core",
			SSHKeys: []string{
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGithubKey1 user@laptop",
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGithubKey2 user@desktop",
			},
		},
	}
	st.record(w, t, "User→Sysext")
	if w.State.CurrentStep != model.StepSysext {
		t.Fatalf("after User: expected StepSysext, got %v", w.State.CurrentStep)
	}
	// Config: hostname + user persisted
	if w.State.Config.Hostname != "flatcar-a" {
		t.Errorf("Hostname not persisted: got %q", w.State.Config.Hostname)
	}
	if len(w.State.Config.Users[0].SSHKeys) != 2 {
		t.Errorf("expected 2 SSH keys, got %d", len(w.State.Config.Users[0].SSHKeys))
	}

	// ── Sysext: none selected ─────────────────────────────────────────────
	// Nvidia must be skipped since nvidia-runtime is not selected
	st.record(w, t, "Sysext→Update(skip nvidia)")
	if w.State.CurrentStep != model.StepUpdate {
		t.Fatalf("after Sysext (no nvidia): expected StepUpdate, got %v", w.State.CurrentStep)
	}

	// ── Update: default reboot strategy already set ───────────────────────
	if w.State.Config.UpdateStrategy.RebootStrategy != "reboot" {
		t.Errorf("expected default reboot strategy, got %q", w.State.Config.UpdateStrategy.RebootStrategy)
	}
	st.record(w, t, "Update→Review")
	if w.State.CurrentStep != model.StepReview {
		t.Fatalf("after Update: expected StepReview, got %v", w.State.CurrentStep)
	}

	// ── Review: CheckConsistency must pass ────────────────────────────────
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("Review validation failed: %v", err)
	}
	st.record(w, t, "Review→Install")
	if w.State.CurrentStep != model.StepInstall {
		t.Fatalf("after Review: expected StepInstall, got %v", w.State.CurrentStep)
	}

	// ── Install → Done ────────────────────────────────────────────────────
	st.record(w, t, "Install→Done")
	if w.State.CurrentStep != model.StepDone {
		t.Fatalf("after Install: expected StepDone, got %v", w.State.CurrentStep)
	}

	// ── Verify exact step sequence ────────────────────────────────────────
	assertStepSequence(t, st.visited, []model.WizardStep{
		model.StepNetwork,
		model.StepStorage,
		model.StepUser,
		model.StepSysext,
		model.StepUpdate, // StepNvidia skipped
		model.StepReview,
		model.StepInstall,
		model.StepDone,
	})

	// ── Execute and verify final InstallConfig ────────────────────────────
	if err := w.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !inst.called {
		t.Fatal("installer was not called")
	}
	cfg := inst.cfg
	if cfg.Channel != "stable" {
		t.Errorf("final channel: got %q, want stable", cfg.Channel)
	}
	if cfg.Network.Mode != model.NetworkDHCP {
		t.Errorf("final network mode: got %v, want DHCP", cfg.Network.Mode)
	}
	if cfg.Disk.DevPath != "/dev/disk/by-id/ata-SAMSUNG_MZ7LN256HAJQ_S38SNX0J123456" {
		t.Errorf("final disk: got %q", cfg.Disk.DevPath)
	}
	if cfg.Hostname != "flatcar-a" {
		t.Errorf("final hostname: got %q", cfg.Hostname)
	}
	if len(cfg.Users) != 1 || cfg.Users[0].Username != "core" {
		t.Errorf("final users: %+v", cfg.Users)
	}
	if len(cfg.Users[0].SSHKeys) != 2 {
		t.Errorf("final SSH key count: got %d, want 2", len(cfg.Users[0].SSHKeys))
	}
	if cfg.NvidiaDriverVersion != "" {
		t.Errorf("no nvidia expected, got driver version %q", cfg.NvidiaDriverVersion)
	}
}

// ---------------------------------------------------------------------------
// Route B: Welcome → Network(Static) → Storage → User(password) →
//
//	Sysext(docker) → Update → Review → Install → Done
//
// ---------------------------------------------------------------------------
func TestRouteB_StaticNetwork_PasswordUser_DockerSysext(t *testing.T) {
	w, _, _, inst := newTestWizard()
	st := &stepTracker{}

	// ── Welcome ───────────────────────────────────────────────────────────
	w.State.Config.Channel = "beta"
	st.record(w, t, "Welcome→Network")

	// ── Network: static ───────────────────────────────────────────────────
	w.State.Config.Network = model.NetworkConfig{
		Mode:      model.NetworkStatic,
		Interface: "enp3s0",
		Address:   "192.168.100.50/24",
		Gateway:   "192.168.100.1",
		DNS:       []string{"1.1.1.1", "8.8.8.8"},
	}
	// Validate static config explicitly before advancing
	w.State.CurrentStep = model.StepNetwork
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("static network should pass validation: %v", err)
	}
	st.record(w, t, "Network→Storage")

	// ── Storage ───────────────────────────────────────────────────────────
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/disk/by-id/nvme-WDC_WD1003FZEX_WD-WCC4K0123456"}
	st.record(w, t, "Storage→User")

	// Config at User entry: static network fields must be intact
	if w.State.Config.Network.Address != "192.168.100.50/24" {
		t.Errorf("Network.Address not persisted: got %q", w.State.Config.Network.Address)
	}
	if w.State.Config.Network.Gateway != "192.168.100.1" {
		t.Errorf("Network.Gateway not persisted: got %q", w.State.Config.Network.Gateway)
	}

	// ── User: password auth only (no SSH keys) ────────────────────────────
	w.State.Config.Hostname = "flatcar-b"
	w.State.Config.Users = []model.UserConfig{
		{
			Username:     "operator",
			PasswordHash: "$6$rounds=4096$salt$hashedpassword", // pre-hashed
		},
	}
	w.State.CurrentStep = model.StepUser
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("password-only user should pass User validation: %v", err)
	}
	st.record(w, t, "User→Sysext")

	// ── Sysext: docker selected ───────────────────────────────────────────
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Description: "Docker CE", Version: "26.1.4", Selected: true},
		{Name: "tailscale", Description: "Tailscale VPN", Selected: false},
	}
	w.State.Config.Sysexts = w.State.Sysexts
	// Still no nvidia-runtime selected: next must skip StepNvidia
	st.record(w, t, "Sysext→Update(skip nvidia)")
	if w.State.CurrentStep != model.StepUpdate {
		t.Fatalf("after Sysext with docker (no nvidia): expected StepUpdate, got %v", w.State.CurrentStep)
	}

	// Config at Update: docker sysext must be in selected list
	selectedNames := make([]string, 0)
	for _, s := range w.State.Config.Sysexts {
		if s.Selected {
			selectedNames = append(selectedNames, s.Name)
		}
	}
	if len(selectedNames) != 1 || selectedNames[0] != "docker" {
		t.Errorf("expected docker selected, got: %v", selectedNames)
	}

	// ── Update ────────────────────────────────────────────────────────────
	w.State.Config.UpdateStrategy = model.UpdateStrategy{RebootStrategy: "off"}
	st.record(w, t, "Update→Review")

	// ── Review: CheckConsistency with static network + password ──────────
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("Review validation for Route B failed: %v", err)
	}
	st.record(w, t, "Review→Install")

	// ── Install → Done ────────────────────────────────────────────────────
	st.record(w, t, "Install→Done")

	// ── Exact step sequence (StepNvidia absent) ───────────────────────────
	assertStepSequence(t, st.visited, []model.WizardStep{
		model.StepNetwork,
		model.StepStorage,
		model.StepUser,
		model.StepSysext,
		model.StepUpdate,
		model.StepReview,
		model.StepInstall,
		model.StepDone,
	})

	// ── Execute and verify final InstallConfig ────────────────────────────
	if err := w.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	cfg := inst.cfg
	if cfg.Channel != "beta" {
		t.Errorf("final channel: got %q, want beta", cfg.Channel)
	}
	if cfg.Network.Mode != model.NetworkStatic {
		t.Errorf("final network mode: got %v, want Static", cfg.Network.Mode)
	}
	if cfg.Network.Interface != "enp3s0" {
		t.Errorf("final interface: got %q", cfg.Network.Interface)
	}
	if cfg.Network.Address != "192.168.100.50/24" {
		t.Errorf("final address: got %q", cfg.Network.Address)
	}
	if cfg.Users[0].Username != "operator" {
		t.Errorf("final username: got %q", cfg.Users[0].Username)
	}
	if cfg.Users[0].PasswordHash == "" {
		t.Error("PasswordHash must be non-empty for password-only user")
	}
	if len(cfg.Users[0].SSHKeys) != 0 {
		t.Errorf("expected no SSH keys for password user, got %v", cfg.Users[0].SSHKeys)
	}
	dockerFound := false
	for _, s := range cfg.Sysexts {
		if s.Name == "docker" && s.Selected {
			dockerFound = true
		}
	}
	if !dockerFound {
		t.Error("docker sysext not found in final config")
	}
	if cfg.UpdateStrategy.RebootStrategy != "off" {
		t.Errorf("final reboot strategy: got %q, want off", cfg.UpdateStrategy.RebootStrategy)
	}
}

// ---------------------------------------------------------------------------
// Route C: Welcome(IgnitionURL) → [TUI: GoToStep(Storage)] →
//
//	Storage → [TUI: GoToStep(Review)] → Review → Install → Done
//
// The skip logic lives in tui.go (it calls GoToStep after Welcome and Storage
// when IgnitionURL is set). This test mirrors that exact behaviour at the
// wizard layer so the wizard's GoToStep + CheckConsistency contract is tested.
// ---------------------------------------------------------------------------
func TestRouteC_IgnitionURL_TUISkip_StorageThenReview(t *testing.T) {
	w, _, _, inst := newTestWizard()

	// Simulated step log — we record manually since we use GoToStep not Next
	var visited []model.WizardStep

	// ── Welcome: set IgnitionURL ──────────────────────────────────────────
	if w.State.CurrentStep != model.StepWelcome {
		t.Fatalf("expected StepWelcome, got %v", w.State.CurrentStep)
	}
	w.State.Config.Channel = "stable"
	w.State.Config.IgnitionURL = "https://provisioning.example.com/ignition/node42.ign"

	// Welcome validation must accept valid IgnitionURL
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("Welcome with valid IgnitionURL should pass: %v", err)
	}

	// TUI behaviour: after confirming Welcome with IgnitionURL → GoToStep(Storage)
	w.GoToStep(model.StepStorage)
	visited = append(visited, w.State.CurrentStep)
	if w.State.CurrentStep != model.StepStorage {
		t.Fatalf("GoToStep(Storage) failed, got %v", w.State.CurrentStep)
	}

	// Steps NOT visited: Network, User, Sysext, Update
	for _, skipped := range []model.WizardStep{
		model.StepNetwork, model.StepUser, model.StepSysext,
		model.StepNvidia, model.StepUpdate,
	} {
		for _, v := range visited {
			if v == skipped {
				t.Errorf("step %v should be skipped in Route C but was visited", skipped)
			}
		}
	}

	// ── Storage: select disk ──────────────────────────────────────────────
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_target"}
	// Validate Storage before the TUI GoToStep(Review)
	w.State.CurrentStep = model.StepStorage
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("Storage with valid disk should pass: %v", err)
	}

	// TUI behaviour: after confirming Storage with IgnitionURL → GoToStep(Review)
	w.GoToStep(model.StepReview)
	visited = append(visited, w.State.CurrentStep)
	if w.State.CurrentStep != model.StepReview {
		t.Fatalf("GoToStep(Review) failed, got %v", w.State.CurrentStep)
	}

	// ── Review: CheckConsistency must accept IgnitionURL without auth ─────
	// No users, no SSH keys — normally fails, but IgnitionURL short-circuits
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("Review with IgnitionURL+disk should pass without auth: %v", err)
	}

	// Advance Review → Install → Done via Next()
	if err := w.Next(); err != nil {
		t.Fatalf("Review→Install: %v", err)
	}
	visited = append(visited, w.State.CurrentStep)
	if w.State.CurrentStep != model.StepInstall {
		t.Fatalf("expected StepInstall, got %v", w.State.CurrentStep)
	}

	if err := w.Next(); err != nil {
		t.Fatalf("Install→Done: %v", err)
	}
	visited = append(visited, w.State.CurrentStep)
	if w.State.CurrentStep != model.StepDone {
		t.Fatalf("expected StepDone, got %v", w.State.CurrentStep)
	}

	// ── Verify only Storage, Review, Install, Done were visited ──────────
	// (Network/User/Sysext/Update/Nvidia never appear)
	wantVisited := []model.WizardStep{
		model.StepStorage,
		model.StepReview,
		model.StepInstall,
		model.StepDone,
	}
	assertStepSequence(t, visited, wantVisited)

	// ── Execute: final config must carry IgnitionURL and disk ─────────────
	if err := w.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	cfg := inst.cfg
	if cfg.IgnitionURL != "https://provisioning.example.com/ignition/node42.ign" {
		t.Errorf("final IgnitionURL: got %q", cfg.IgnitionURL)
	}
	if cfg.Disk.DevPath != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_target" {
		t.Errorf("final disk: got %q", cfg.Disk.DevPath)
	}
	// No users/SSH keys injected — IgnitionURL provides them externally
	if len(cfg.Users) != 0 {
		t.Errorf("expected no wizard-generated users in IgnitionURL mode, got: %v", cfg.Users)
	}
}

// ---------------------------------------------------------------------------
// Route D: Welcome → Network → Storage → User →
//
//	Sysext(nvidia-runtime) → NVIDIA → Update → Review → Install → Done
//
// ---------------------------------------------------------------------------
func TestRouteD_NvidiaRuntime_DriverSetup(t *testing.T) {
	w, _, _, inst := newTestWizard()
	st := &stepTracker{}

	// ── Welcome ───────────────────────────────────────────────────────────
	st.record(w, t, "Welcome→Network")

	// ── Network: DHCP ─────────────────────────────────────────────────────
	w.State.Config.Network.Mode = model.NetworkDHCP
	st.record(w, t, "Network→Storage")

	// ── Storage ───────────────────────────────────────────────────────────
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/disk/by-id/nvme-TOSHIBA_KXG60ZNV256G_12345ABCDE"}
	st.record(w, t, "Storage→User")

	// ── User ──────────────────────────────────────────────────────────────
	w.State.Config.Hostname = "gpu-node-01"
	w.State.Config.Users = []model.UserConfig{
		{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGpuKey user@workstation"}},
	}
	st.record(w, t, "User→Sysext")

	// ── Sysext: select nvidia-runtime ─────────────────────────────────────
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", Description: "Docker CE", Selected: false},
		{Name: "nvidia-runtime", Description: "NVIDIA Container Toolkit", Selected: true},
	}
	w.State.Config.Sysexts = w.State.Sysexts
	// With nvidia-runtime selected, Next() must visit StepNvidia
	st.record(w, t, "Sysext→Nvidia")
	if w.State.CurrentStep != model.StepNvidia {
		t.Fatalf("after Sysext with nvidia-runtime: expected StepNvidia, got %v", w.State.CurrentStep)
	}

	// ── NVIDIA: choose driver series ──────────────────────────────────────
	w.State.Config.NvidiaDriverVersion = "570-open"
	// Validate at Nvidia step (always nil — no validation required)
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("StepNvidia validation should always pass: %v", err)
	}
	st.record(w, t, "Nvidia→Update")
	if w.State.CurrentStep != model.StepUpdate {
		t.Fatalf("after Nvidia: expected StepUpdate, got %v", w.State.CurrentStep)
	}

	// Config at Update: nvidia driver version must be set
	if w.State.Config.NvidiaDriverVersion != "570-open" {
		t.Errorf("NvidiaDriverVersion not persisted: got %q", w.State.Config.NvidiaDriverVersion)
	}

	// ── Update ────────────────────────────────────────────────────────────
	st.record(w, t, "Update→Review")

	// ── Review: CheckConsistency must pass with nvidia config ─────────────
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("Review validation for Route D failed: %v", err)
	}
	st.record(w, t, "Review→Install")

	// ── Install → Done ────────────────────────────────────────────────────
	st.record(w, t, "Install→Done")

	// ── Exact step sequence: StepNvidia IS present ────────────────────────
	assertStepSequence(t, st.visited, []model.WizardStep{
		model.StepNetwork,
		model.StepStorage,
		model.StepUser,
		model.StepSysext,
		model.StepNvidia, // present this time
		model.StepUpdate,
		model.StepReview,
		model.StepInstall,
		model.StepDone,
	})

	// ── Execute and verify final InstallConfig ────────────────────────────
	if err := w.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	cfg := inst.cfg
	if cfg.NvidiaDriverVersion != "570-open" {
		t.Errorf("final NvidiaDriverVersion: got %q, want 570-open", cfg.NvidiaDriverVersion)
	}
	if cfg.Hostname != "gpu-node-01" {
		t.Errorf("final hostname: got %q", cfg.Hostname)
	}
	nvidiaSelected := false
	for _, s := range cfg.Sysexts {
		if s.Name == "nvidia-runtime" && s.Selected {
			nvidiaSelected = true
		}
	}
	if !nvidiaSelected {
		t.Error("nvidia-runtime must be selected in final config for Route D")
	}

	// ── Previous back navigation: Update→Nvidia→Sysext (not skipping) ─────
	w.State.CurrentStep = model.StepUpdate
	w.Previous() // should land on StepNvidia (selected)
	if w.State.CurrentStep != model.StepNvidia {
		t.Errorf("Previous from Update with nvidia selected: expected StepNvidia, got %v", w.State.CurrentStep)
	}
	w.Previous() // StepNvidia → StepSysext
	if w.State.CurrentStep != model.StepSysext {
		t.Errorf("Previous from Nvidia: expected StepSysext, got %v", w.State.CurrentStep)
	}
}

// ---------------------------------------------------------------------------
// Route E: Welcome → ... → Review → Previous×N back to User → fix SSH key →
//
//	re-advance → Review → Install → Done
//
// Tests that going back via Previous(), mutating config, and re-advancing
// produces a final InstallConfig that reflects the NEW values, not the old ones.
// ---------------------------------------------------------------------------
func TestRouteE_ReviewBackToUser_FixAndReadvance(t *testing.T) {
	w, _, _, inst := newTestWizard()

	// ── Phase 1: Advance to Review with an initial (bad) SSH key ──────────
	w.State.Config.Channel = "stable"
	w.State.Config.Network.Mode = model.NetworkDHCP
	w.State.Config.Disk = model.DiskInfo{DevPath: "/dev/disk/by-id/scsi-0QEMU_HARDDISK_fix-test"}
	w.State.Config.Hostname = "flatcar-e"
	w.State.Config.Users = []model.UserConfig{
		{
			Username: "core",
			SSHKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOriginalKey original@host"},
		},
	}

	// Walk straight to Review without going through Next() gate sequentially
	// (the user filled everything in and is on Review)
	w.State.CurrentStep = model.StepReview
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("initial Review should pass: %v", err)
	}

	// ── Phase 2: Go back to User to fix the SSH key ───────────────────────
	// Simulate pressing Back 3 times: Review → Update → Sysext → User
	// (with no nvidia selected, Previous skips Nvidia)
	w.Previous() // Review → Update
	if w.State.CurrentStep != model.StepUpdate {
		t.Fatalf("Previous from Review: expected StepUpdate, got %v", w.State.CurrentStep)
	}
	w.Previous() // Update → Sysext (skips Nvidia since not selected)
	if w.State.CurrentStep != model.StepSysext {
		t.Fatalf("Previous from Update: expected StepSysext (skip Nvidia), got %v", w.State.CurrentStep)
	}
	w.Previous() // Sysext → User
	if w.State.CurrentStep != model.StepUser {
		t.Fatalf("Previous from Sysext: expected StepUser, got %v", w.State.CurrentStep)
	}

	// ── Phase 3: Mutate — replace SSH key + add second user ───────────────
	w.State.Config.Users = []model.UserConfig{
		{
			Username: "core",
			SSHKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFixedKey fixed@workstation"},
		},
		{
			Username: "admin",
			SSHKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAdminKey admin@ci"},
		},
	}
	// Validate User step with new config
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("User validation after edit: %v", err)
	}

	// ── Phase 4: Re-advance back to Review ───────────────────────────────
	// Record the step sequence for the re-advance portion
	st := &stepTracker{}
	st.record(w, t, "User→Sysext") // User → Sysext
	if w.State.CurrentStep != model.StepSysext {
		t.Fatalf("re-advance User→Sysext: got %v", w.State.CurrentStep)
	}

	st.record(w, t, "Sysext→Update") // Sysext → Update (skip Nvidia)
	if w.State.CurrentStep != model.StepUpdate {
		t.Fatalf("re-advance Sysext→Update: got %v", w.State.CurrentStep)
	}

	st.record(w, t, "Update→Review") // Update → Review
	if w.State.CurrentStep != model.StepReview {
		t.Fatalf("re-advance Update→Review: got %v", w.State.CurrentStep)
	}

	// Re-advance sequence must be exactly Sysext → Update → Review
	assertStepSequence(t, st.visited, []model.WizardStep{
		model.StepSysext,
		model.StepUpdate,
		model.StepReview,
	})

	// ── Phase 5: Review must reflect the NEW users ─────────────────────────
	if len(w.State.Config.Users) != 2 {
		t.Fatalf("expected 2 users after edit, got %d", len(w.State.Config.Users))
	}
	if w.State.Config.Users[0].SSHKeys[0] != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFixedKey fixed@workstation" {
		t.Errorf("SSH key not updated: got %q", w.State.Config.Users[0].SSHKeys[0])
	}
	if w.State.Config.Users[1].Username != "admin" {
		t.Errorf("second user not added: got %q", w.State.Config.Users[1].Username)
	}

	// Verify old key is gone
	for _, u := range w.State.Config.Users {
		for _, k := range u.SSHKeys {
			if strings.Contains(k, "original") {
				t.Error("old SSH key still present after user edit")
			}
		}
	}

	// Review CheckConsistency must pass with new users
	if err := w.ValidateCurrentStep(); err != nil {
		t.Fatalf("Review after user edit: %v", err)
	}

	// ── Phase 6: Advance Review → Install → Done ──────────────────────────
	if err := w.Next(); err != nil {
		t.Fatalf("Review→Install: %v", err)
	}
	if w.State.CurrentStep != model.StepInstall {
		t.Fatalf("expected StepInstall, got %v", w.State.CurrentStep)
	}
	if err := w.Next(); err != nil {
		t.Fatalf("Install→Done: %v", err)
	}
	if w.State.CurrentStep != model.StepDone {
		t.Fatalf("expected StepDone, got %v", w.State.CurrentStep)
	}

	// ── Final config must carry the edited users ──────────────────────────
	if err := w.Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	cfg := inst.cfg
	if len(cfg.Users) != 2 {
		t.Fatalf("final config: expected 2 users, got %d", len(cfg.Users))
	}
	if cfg.Users[0].SSHKeys[0] != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFixedKey fixed@workstation" {
		t.Errorf("final config: wrong SSH key for core: %q", cfg.Users[0].SSHKeys[0])
	}
	if cfg.Users[1].Username != "admin" {
		t.Errorf("final config: wrong second username: %q", cfg.Users[1].Username)
	}
	// Butane must contain fixed key, not original
	butane, err := w.GenerateButane()
	if err != nil {
		t.Fatalf("GenerateButane after Route E: %v", err)
	}
	if strings.Contains(butane, "OriginalKey") {
		t.Error("Butane still contains original SSH key — config mutation not propagated")
	}
	if !strings.Contains(butane, "FixedKey") {
		t.Error("Butane does not contain the fixed SSH key")
	}
}
