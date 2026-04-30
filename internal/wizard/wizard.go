// Package wizard provides the wizard subsystem for knuckle.
// It manages installation state transitions and orchestrates the install flow.
package wizard

import (
"context"
"fmt"

"github.com/castrojo/knuckle/internal/bakery"
"github.com/castrojo/knuckle/internal/ignition"
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

// Channel version info (fetched at startup)
Channels []bakery.ChannelInfo

// System checks (populated at startup)
SystemChecks []SystemCheck

// User confirmed destructive operation
Confirmed bool

// Error from the last operation
Err error

// Installation progress messages
ProgressMessages []string
}

// SystemCheck represents a pre-flight system check result
type SystemCheck struct {
Name   string
Status string // "ok", "warn", "fail"
Detail string
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
return w.validateWelcome()
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
return w.validateConsistency()
case model.StepInstall:
return nil // install step validates on execute
default:
return nil
}
}

func (w *Wizard) validateWelcome() error {
validChannels := map[string]bool{"stable": true, "beta": true, "alpha": true, "edge": true}
if w.State.Config.Channel != "" && !validChannels[w.State.Config.Channel] {
return fmt.Errorf("invalid channel %q (must be stable, beta, alpha, or edge)", w.State.Config.Channel)
}
if w.State.Config.IgnitionURL != "" {
if err := validate.URL(w.State.Config.IgnitionURL); err != nil {
return fmt.Errorf("ignition URL: %w", err)
}
}
return nil
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

// validateConsistency checks for contradictions in the final config
func (w *Wizard) validateConsistency() error {
cfg := w.State.Config

// Must have a disk selected
if cfg.Disk.DevPath == "" {
return fmt.Errorf("no disk selected")
}

// Must have authentication (SSH key or password)
hasSSH := len(cfg.SSHKeys) > 0
hasPassword := false
for _, u := range cfg.Users {
if len(u.SSHKeys) > 0 {
hasSSH = true
}
if u.PasswordHash != "" {
hasPassword = true
}
}
if !hasSSH && !hasPassword && cfg.IgnitionURL == "" {
return fmt.Errorf("no authentication configured: add SSH keys or a password")
}

// Warn if static network but no gateway
if cfg.Network.Mode == model.NetworkStatic && cfg.Network.Gateway == "" {
return fmt.Errorf("static network requires a gateway")
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

// Run system checks after hardware probe
w.runSystemChecks()

return nil
}

// runSystemChecks performs pre-flight verification
func (w *Wizard) runSystemChecks() {
w.State.SystemChecks = nil

// Check: at least one eligible disk
if len(w.State.Disks) > 0 {
w.State.SystemChecks = append(w.State.SystemChecks, SystemCheck{
Name: "Disk", Status: "ok",
Detail: fmt.Sprintf("%d eligible disk(s) found", len(w.State.Disks)),
})
} else {
w.State.SystemChecks = append(w.State.SystemChecks, SystemCheck{
Name: "Disk", Status: "fail",
Detail: "no eligible disks found (need ≥8GB, non-removable, non-boot)",
})
}

// Check: at least one network interface
activeIfaces := 0
for _, iface := range w.State.Interfaces {
if len(iface.IPv4Addrs) > 0 {
activeIfaces++
}
}
if activeIfaces > 0 {
w.State.SystemChecks = append(w.State.SystemChecks, SystemCheck{
Name: "Network", Status: "ok",
Detail: fmt.Sprintf("%d interface(s) with IPv4", activeIfaces),
})
} else if len(w.State.Interfaces) > 0 {
w.State.SystemChecks = append(w.State.SystemChecks, SystemCheck{
Name: "Network", Status: "warn",
Detail: "interfaces found but none have IPv4 addresses",
})
} else {
w.State.SystemChecks = append(w.State.SystemChecks, SystemCheck{
Name: "Network", Status: "fail",
Detail: "no network interfaces detected",
})
}

// Check: sysext catalog availability
if len(w.State.Sysexts) > 0 {
w.State.SystemChecks = append(w.State.SystemChecks, SystemCheck{
Name: "Sysext Catalog", Status: "ok",
Detail: fmt.Sprintf("%d extensions available", len(w.State.Sysexts)),
})
} else {
w.State.SystemChecks = append(w.State.SystemChecks, SystemCheck{
Name: "Sysext Catalog", Status: "warn",
Detail: "catalog unavailable — sysext selection will be skipped",
})
}
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

// FetchChannels loads version info for all release channels
func (w *Wizard) FetchChannels(ctx context.Context) error {
channels, err := bakery.FetchAllChannels(ctx)
if err != nil {
return err
}
w.State.Channels = channels
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

// GenerateButane renders the current config as Butane YAML for preview.
func (w *Wizard) GenerateButane() (string, error) {
gen := ignition.NewGenerator()
return gen.GenerateButane(&w.State.Config)
}
