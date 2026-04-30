package wizard

import (
"context"
"fmt"
"strings"
"testing"

"github.com/castrojo/knuckle/internal/model"
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
// StepSysext -> StepReview
if err := w.Next(); err != nil {
t.Fatalf("Next from Sysext: %v", err)
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
Mode:    model.NetworkStatic,
Address: "192.168.1.10/24",
Gateway: "192.168.1.1",
DNS:     []string{"8.8.8.8", "1.1.1.1"},
}

if err := w.ValidateCurrentStep(); err != nil {
t.Errorf("valid static config should pass: %v", err)
}
}

func TestValidateNetworkStaticInvalid(t *testing.T) {
w, _, _, _ := newTestWizard()
w.State.CurrentStep = model.StepNetwork
w.State.Config.Network = model.NetworkConfig{
Mode:    model.NetworkStatic,
Address: "not-a-cidr",
Gateway: "192.168.1.1",
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
if count != 8 {
t.Errorf("expected 8 steps, got %d", count)
}
}
