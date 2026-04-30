// Package wizard provides the wizard subsystem for knuckle.
// It manages installation state transitions and orchestrates the install flow.
package wizard

import (
"context"
"fmt"

"github.com/castrojo/knuckle/internal/bakery"
"github.com/castrojo/knuckle/internal/install"
"github.com/castrojo/knuckle/internal/model"
"github.com/castrojo/knuckle/internal/probe"
"github.com/castrojo/knuckle/internal/validate"
)

// State holds the complete wizard state
type State struct {
CurrentStep model.WizardStep
Config      model.InstallConfig

// Discovered hardware
Disks      []model.DiskInfo
Interfaces []model.NetworkInterface
Sysexts    []model.SysextEntry

// User confirmed destructive operation
Confirmed bool

// Error from the last operation
Err error

// Installation progress messages
ProgressMessages []string
}

// Wizard manages the installer workflow
type Wizard struct {
State     *State
Prober    probe.Prober
Bakery    bakery.Client
Installer install.Installer
}

// New creates a new Wizard with the given dependencies
func New(prober probe.Prober, bakeryClient bakery.Client, installer install.Installer) *Wizard {
return &Wizard{
State: &State{
CurrentStep: model.StepWelcome,
Config: model.InstallConfig{
Channel:        "stable",
UpdateStrategy: model.UpdateStrategy{RebootStrategy: "reboot"},
},
},
Prober:    prober,
Bakery:    bakeryClient,
Installer: installer,
}
}

// Next advances to the next step if validation passes
func (w *Wizard) Next() error {
if err := w.ValidateCurrentStep(); err != nil {
return err
}

if w.State.CurrentStep < model.StepDone {
w.State.CurrentStep++
}
return nil
}

// Previous goes back to the previous step
func (w *Wizard) Previous() {
if w.State.CurrentStep > model.StepWelcome {
w.State.CurrentStep--
}
}

// GoToStep jumps to a specific step (for review screen navigation)
func (w *Wizard) GoToStep(step model.WizardStep) {
if step >= model.StepWelcome && step <= model.StepDone {
w.State.CurrentStep = step
}
}

// ValidateCurrentStep validates the data for the current step
func (w *Wizard) ValidateCurrentStep() error {
switch w.State.CurrentStep {
case model.StepWelcome:
return nil // no validation needed
case model.StepNetwork:
return w.validateNetwork()
case model.StepStorage:
return w.validateStorage()
case model.StepUser:
return w.validateUser()
case model.StepSysext:
return nil // sysext selection is optional
case model.StepUpdate:
return nil // update strategy selection is optional (defaults to "reboot")
case model.StepReview:
if !w.State.Confirmed {
return fmt.Errorf("type YES to confirm installation")
}
return nil
case model.StepInstall:
return nil // install step validates on execute
default:
return nil
}
}

func (w *Wizard) validateNetwork() error {
cfg := w.State.Config.Network
if cfg.Mode == model.NetworkStatic {
if err := validate.CIDR(cfg.Address); err != nil {
return fmt.Errorf("network address: %w", err)
}
if err := validate.Gateway(cfg.Gateway); err != nil {
return fmt.Errorf("gateway: %w", err)
}
for _, dns := range cfg.DNS {
if err := validate.DNSServer(dns); err != nil {
return fmt.Errorf("DNS server: %w", err)
}
}
}
return nil
}

func (w *Wizard) validateStorage() error {
if w.State.Config.Disk.DevPath == "" {
return fmt.Errorf("no disk selected")
}
for _, p := range w.State.Config.Disk.Partitions {
if p.MountPoint != "" {
return fmt.Errorf("disk %s has mounted partition %s at %s — unmount before installing", w.State.Config.Disk.DevPath, p.Path, p.MountPoint)
}
}
return validate.DiskPath(w.State.Config.Disk.DevPath)
}

func (w *Wizard) validateUser() error {
// Validate hostname
if w.State.Config.Hostname != "" {
if err := validate.Hostname(w.State.Config.Hostname); err != nil {
return err
}
}
// Must have at least one user or SSH key
if len(w.State.Config.Users) == 0 && len(w.State.Config.SSHKeys) == 0 {
return fmt.Errorf("at least one user or SSH key is required")
}
for _, user := range w.State.Config.Users {
if err := validate.Username(user.Username); err != nil {
return err
}
for _, key := range user.SSHKeys {
if err := validate.SSHPublicKey(key); err != nil {
return err
}
}
}
for _, key := range w.State.Config.SSHKeys {
if err := validate.SSHPublicKey(key); err != nil {
return err
}
}
return nil
}

// ProbeHardware discovers disks and network interfaces
func (w *Wizard) ProbeHardware(ctx context.Context) error {
disks, err := w.Prober.ListDisks(ctx)
if err != nil {
return fmt.Errorf("probing disks: %w", err)
}
w.State.Disks = disks

ifaces, err := w.Prober.ListNetworkInterfaces(ctx)
if err != nil {
return fmt.Errorf("probing network: %w", err)
}
w.State.Interfaces = ifaces

return nil
}

// FetchSysexts loads the sysext catalog
func (w *Wizard) FetchSysexts(ctx context.Context) error {
sysexts, err := w.Bakery.FetchCatalog(ctx)
if err != nil {
return fmt.Errorf("fetching sysext catalog: %w", err)
}
w.State.Sysexts = sysexts
return nil
}

// Execute runs the installation
func (w *Wizard) Execute(ctx context.Context) error {
w.State.ProgressMessages = nil
progress := func(msg string) {
w.State.ProgressMessages = append(w.State.ProgressMessages, msg)
}
return w.Installer.Install(ctx, &w.State.Config, progress)
}

// IsFirstStep returns true if on the first step
func (w *Wizard) IsFirstStep() bool {
return w.State.CurrentStep == model.StepWelcome
}

// IsLastStep returns true if on the final step
func (w *Wizard) IsLastStep() bool {
return w.State.CurrentStep == model.StepDone
}

// StepCount returns the total number of steps
func StepCount() int {
return int(model.StepDone) + 1
}
