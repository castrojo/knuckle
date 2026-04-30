package tui

import (
"context"
"fmt"
"strings"
"time"

tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"

"github.com/castrojo/knuckle/internal/github"
"github.com/castrojo/knuckle/internal/model"
"github.com/castrojo/knuckle/internal/wizard"
)

var (
titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).MarginBottom(1)
stepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)

// Model is the top-level Bubble Tea model
type Model struct {
Wizard   *wizard.Wizard
width    int
height   int
err      error
quitting bool
cursor   int
fields   []field
fieldIdx int
}

type field struct {
label string
value string
key   string
}

// New creates a new TUI model
func New(w *wizard.Wizard) *Model {
m := &Model{Wizard: w}
m.initStepFields()
return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
switch msg := msg.(type) {
case tea.KeyMsg:
return m.handleKey(msg)
case tea.WindowSizeMsg:
m.width = msg.Width
m.height = msg.Height
return m, nil
}
return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
// Only allow 'q' to quit when NOT editing text fields
switch msg.String() {
case "ctrl+c":
m.quitting = true
return m, tea.Quit
case "q":
// Only quit if we're not in a field-editing step
if len(m.fields) == 0 {
m.quitting = true
return m, tea.Quit
}
// Otherwise treat as regular character input
m.fields[m.fieldIdx].value += "q"
return m, nil
case "enter":
return m.handleEnter()
case "tab", "down":
if len(m.fields) > 0 {
m.fieldIdx = (m.fieldIdx + 1) % len(m.fields)
} else {
m.cursor++
// Clamp cursor to list bounds
maxCursor := m.maxCursor()
if m.cursor >= maxCursor {
m.cursor = maxCursor - 1
}
if m.cursor < 0 {
m.cursor = 0
}
}
return m, nil
case "shift+tab", "up":
if len(m.fields) > 0 {
m.fieldIdx--
if m.fieldIdx < 0 {
m.fieldIdx = len(m.fields) - 1
}
} else if m.cursor > 0 {
m.cursor--
}
return m, nil
case "backspace":
if len(m.fields) > 0 && len(m.fields[m.fieldIdx].value) > 0 {
m.fields[m.fieldIdx].value = m.fields[m.fieldIdx].value[:len(m.fields[m.fieldIdx].value)-1]
}
return m, nil
case "esc":
m.Wizard.Previous()
m.err = nil
m.initStepFields()
return m, nil
case " ":
if m.Wizard.State.CurrentStep == model.StepSysext && m.cursor < len(m.Wizard.State.Sysexts) {
m.Wizard.State.Sysexts[m.cursor].Selected = !m.Wizard.State.Sysexts[m.cursor].Selected
m.Wizard.State.Config.Sysexts = m.Wizard.State.Sysexts
} else if len(m.fields) > 0 {
m.fields[m.fieldIdx].value += " "
}
return m, nil
default:
if len(m.fields) > 0 && len(msg.String()) == 1 {
m.fields[m.fieldIdx].value += msg.String()
}
return m, nil
}
}

// maxCursor returns the number of selectable items in list-based steps
func (m *Model) maxCursor() int {
switch m.Wizard.State.CurrentStep {
case model.StepStorage:
return len(m.Wizard.State.Disks)
case model.StepSysext:
return len(m.Wizard.State.Sysexts)
case model.StepUpdate:
return 3
default:
return 1
}
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
step := m.Wizard.State.CurrentStep
m.applyFields()

switch step {
case model.StepWelcome:
// If IgnitionURL is set, skip directly to Storage
if m.Wizard.State.Config.IgnitionURL != "" {
m.Wizard.GoToStep(model.StepStorage)
m.err = nil
m.cursor = 0
m.initStepFields()
return m, nil
}
case model.StepStorage:
if m.cursor < len(m.Wizard.State.Disks) {
m.Wizard.State.Config.Disk = m.Wizard.State.Disks[m.cursor]
}
// If IgnitionURL is set, skip to Review after Storage
if m.Wizard.State.Config.IgnitionURL != "" {
if err := m.Wizard.ValidateCurrentStep(); err != nil {
m.err = err
return m, nil
}
m.Wizard.GoToStep(model.StepReview)
m.err = nil
m.cursor = 0
m.initStepFields()
return m, nil
}
case model.StepUpdate:
strategies := []string{"reboot", "off", "etcd-lock"}
if m.cursor >= 0 && m.cursor < len(strategies) {
m.Wizard.State.Config.UpdateStrategy.RebootStrategy = strategies[m.cursor]
}
case model.StepInstall:
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()
if err := m.Wizard.Execute(ctx); err != nil {
m.err = err
return m, nil
}
}

if err := m.Wizard.Next(); err != nil {
m.err = err
return m, nil
}

m.err = nil
m.cursor = 0
m.initStepFields()

if m.Wizard.State.CurrentStep == model.StepDone {
return m, tea.Quit
}
return m, nil
}

func (m *Model) applyFields() {
cfg := &m.Wizard.State.Config
switch m.Wizard.State.CurrentStep {
case model.StepWelcome:
for _, f := range m.fields {
switch f.key {
case "channel":
if f.value != "" {
cfg.Channel = f.value
}
case "ignition_url":
cfg.IgnitionURL = f.value
}
}
case model.StepNetwork:
for _, f := range m.fields {
switch f.key {
case "interface":
cfg.Network.Interface = f.value
case "address":
cfg.Network.Address = f.value
case "gateway":
cfg.Network.Gateway = f.value
case "dns":
if f.value != "" {
cfg.Network.DNS = strings.Split(f.value, ",")
}
}
}
// Switch to static mode if any static fields are filled in
if cfg.Network.Address != "" || cfg.Network.Gateway != "" {
cfg.Network.Mode = model.NetworkStatic
} else {
cfg.Network.Mode = model.NetworkDHCP
}
case model.StepUser:
for _, f := range m.fields {
switch f.key {
case "hostname":
cfg.Hostname = f.value
case "username":
if f.value != "" {
if len(cfg.Users) == 0 {
cfg.Users = append(cfg.Users, model.UserConfig{
Username: f.value,
Groups:   []string{"sudo", "docker"},
})
} else {
cfg.Users[0].Username = f.value
}
}
case "github_user":
if f.value != "" {
keys, err := github.FetchKeys(f.value)
if err != nil {
m.err = err
return
}
cfg.SSHKeys = keys
// Also attach to user
if len(cfg.Users) > 0 {
cfg.Users[0].SSHKeys = keys
}
}
case "ssh_key":
if f.value != "" {
cfg.SSHKeys = []string{f.value}
if len(cfg.Users) > 0 {
cfg.Users[0].SSHKeys = []string{f.value}
}
}
}
}
case model.StepReview:
for _, f := range m.fields {
if f.key == "confirm" {
m.Wizard.State.Confirmed = (strings.ToUpper(strings.TrimSpace(f.value)) == "YES")
}
}
}
}

func (m *Model) initStepFields() {
m.fields = nil
m.fieldIdx = 0
switch m.Wizard.State.CurrentStep {
case model.StepWelcome:
m.fields = []field{
{label: "Channel (stable/beta/alpha/edge)", key: "channel", value: m.Wizard.State.Config.Channel},
{label: "External Ignition URL (skip wizard)", key: "ignition_url", value: m.Wizard.State.Config.IgnitionURL},
}
case model.StepNetwork:
m.fields = []field{
{label: "Interface", key: "interface", value: m.Wizard.State.Config.Network.Interface},
{label: "IP Address (CIDR)", key: "address", value: m.Wizard.State.Config.Network.Address},
{label: "Gateway", key: "gateway", value: m.Wizard.State.Config.Network.Gateway},
{label: "DNS (comma-separated)", key: "dns", value: strings.Join(m.Wizard.State.Config.Network.DNS, ",")},
}
case model.StepUser:
username := ""
if len(m.Wizard.State.Config.Users) > 0 {
username = m.Wizard.State.Config.Users[0].Username
}
sshKey := ""
if len(m.Wizard.State.Config.SSHKeys) > 0 {
sshKey = m.Wizard.State.Config.SSHKeys[0]
}
m.fields = []field{
{label: "Hostname", key: "hostname", value: m.Wizard.State.Config.Hostname},
{label: "Username", key: "username", value: username},
{label: "GitHub Username (fetches SSH keys)", key: "github_user", value: ""},
{label: "— OR — SSH Public Key (manual)", key: "ssh_key", value: sshKey},
}
case model.StepReview:
m.fields = []field{
{label: "Type YES to confirm installation", key: "confirm", value: ""},
}
case model.StepUpdate:
// No fields — cursor-select screen
}
}


func (m *Model) View() string {
if m.quitting {
return "Installation cancelled.\n"
}

var b strings.Builder
b.WriteString(titleStyle.Render("🔧 Knuckle — Flatcar Container Linux Installer"))
b.WriteString("\n")
b.WriteString(stepStyle.Render(fmt.Sprintf("Step %d/%d: %s",
int(m.Wizard.State.CurrentStep)+1,
wizard.StepCount(),
m.Wizard.State.CurrentStep.String())))
b.WriteString("\n\n")

switch m.Wizard.State.CurrentStep {
case model.StepWelcome:
b.WriteString(m.viewWelcome())
case model.StepNetwork:
b.WriteString(m.viewNetwork())
case model.StepStorage:
b.WriteString(m.viewStorage())
case model.StepUser:
b.WriteString(m.viewUser())
case model.StepSysext:
b.WriteString(m.viewSysext())
case model.StepUpdate:
b.WriteString(m.viewUpdate())
case model.StepReview:
b.WriteString(m.viewReview())
case model.StepInstall:
b.WriteString(m.viewInstall())
case model.StepDone:
b.WriteString(m.viewDone())
}

if m.err != nil {
b.WriteString("\n")
b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ %s", m.err.Error())))
b.WriteString("\n")
}

b.WriteString("\n")
b.WriteString(helpStyle.Render("↑/↓ navigate • enter confirm • esc back • q quit"))
return b.String()
}

func (m *Model) viewWelcome() string {
var b strings.Builder
b.WriteString(`Welcome to Knuckle!

This wizard will guide you through installing Flatcar Container Linux
on your system.

What this installer will do:
  • Detect your hardware (disks, network)
  • Configure networking (DHCP or static)
  • Set up user accounts and SSH keys
  • Optionally add system extensions
  • Write Flatcar to your selected disk

`)
for i, f := range m.fields {
cursor := "  "
if i == m.fieldIdx {
cursor = "▸ "
}
fmt.Fprintf(&b, "%s%s: %s\n", cursor, f.label, f.value)
}
b.WriteString("\nPress Enter to continue...")
return b.String()
}

func (m *Model) viewNetwork() string {
var b strings.Builder
b.WriteString("Network Configuration\n\n")
if len(m.Wizard.State.Interfaces) > 0 {
b.WriteString("Detected interfaces:\n")
for _, iface := range m.Wizard.State.Interfaces {
fmt.Fprintf(&b, "  • %s (%s) — %s\n", iface.Name, iface.MAC, iface.State)
}
b.WriteString("\n")
}
b.WriteString("Using DHCP by default. Fill in fields for static config:\n\n")
for i, f := range m.fields {
cursor := "  "
if i == m.fieldIdx {
cursor = "▸ "
}
fmt.Fprintf(&b, "%s%s: %s\n", cursor, f.label, f.value)
}
return b.String()
}

func (m *Model) viewStorage() string {
var b strings.Builder
b.WriteString("Select Target Disk\n\n")
if len(m.Wizard.State.Disks) == 0 {
b.WriteString("No disks detected!\n")
return b.String()
}
for i, disk := range m.Wizard.State.Disks {
cursor := "  "
if i == m.cursor {
cursor = "▸ "
}
line := fmt.Sprintf("%s%s — %s [%s] %s", cursor, disk.DevPath, disk.Model, disk.SizeHuman, disk.Transport)
if disk.Removable {
line += " (removable)"
}
if i == m.cursor {
b.WriteString(selectedStyle.Render(line))
} else {
b.WriteString(line)
}
b.WriteString("\n")
}
b.WriteString("\n⚠ WARNING: All data on the selected disk will be erased!\n")
return b.String()
}

func (m *Model) viewUser() string {
var b strings.Builder
b.WriteString("User Configuration\n\n")
b.WriteString("Enter a GitHub username to auto-fetch your SSH keys,\nor paste a key manually.\n\n")
for i, f := range m.fields {
cursor := "  "
if i == m.fieldIdx {
cursor = "▸ "
}
fmt.Fprintf(&b, "%s%s: %s\n", cursor, f.label, f.value)
}
if len(m.Wizard.State.Config.SSHKeys) > 0 {
fmt.Fprintf(&b, "\n  ✓ %d SSH key(s) configured\n", len(m.Wizard.State.Config.SSHKeys))
}
return b.String()
}

func (m *Model) viewSysext() string {
var b strings.Builder
b.WriteString("System Extensions (optional)\n\nSpace to toggle, Enter to continue:\n\n")
if len(m.Wizard.State.Sysexts) == 0 {
b.WriteString("No extensions available (catalog fetch may have failed)\n")
return b.String()
}
for i, ext := range m.Wizard.State.Sysexts {
cursor := "  "
if i == m.cursor {
cursor = "▸ "
}
check := "[ ]"
if ext.Selected {
check = "[✓]"
}
line := fmt.Sprintf("%s%s %s v%s — %s", cursor, check, ext.Name, ext.Version, ext.Description)
if i == m.cursor {
b.WriteString(selectedStyle.Render(line))
} else {
b.WriteString(line)
}
b.WriteString("\n")
}
return b.String()
}

func (m *Model) viewUpdate() string {
var b strings.Builder
b.WriteString("Update Strategy\n\nChoose how Flatcar will handle OS updates:\n\n")

type option struct {
name string
desc []string
}
options := []option{
{"reboot (Recommended)", []string{
"Auto-update and reboot immediately when an update is applied.",
"Best for: single nodes, dev environments",
}},
{"off", []string{
"Updates are downloaded but never applied automatically.",
"You must run 'update_engine_client -update' manually.",
"Best for: manually managed infrastructure",
}},
{"etcd-lock", []string{
"Coordinates reboots with other nodes via etcd distributed lock.",
"Only one node reboots at a time in the cluster.",
"Best for: multi-node clusters running etcd",
}},
}

for i, opt := range options {
cursor := "  "
if i == m.cursor {
cursor = "▸ "
}
line := fmt.Sprintf("%s%s", cursor, opt.name)
if i == m.cursor {
b.WriteString(selectedStyle.Render(line))
} else {
b.WriteString(line)
}
b.WriteString("\n")
for _, d := range opt.desc {
b.WriteString(fmt.Sprintf("    %s\n", d))
}
b.WriteString("\n")
}
return b.String()
}

func (m *Model) viewReview() string {
var b strings.Builder
cfg := m.Wizard.State.Config
b.WriteString("Review Configuration\n\n")

if cfg.IgnitionURL != "" {
fmt.Fprintf(&b, "  Mode:      External Ignition\n")
fmt.Fprintf(&b, "  URL:       %s\n", cfg.IgnitionURL)
fmt.Fprintf(&b, "  Disk:      %s (%s)\n", cfg.Disk.DevPath, cfg.Disk.SizeHuman)
} else {
fmt.Fprintf(&b, "  Channel:   %s\n", cfg.Channel)
fmt.Fprintf(&b, "  Hostname:  %s\n", cfg.Hostname)
fmt.Fprintf(&b, "  Disk:      %s (%s)\n", cfg.Disk.DevPath, cfg.Disk.SizeHuman)
fmt.Fprintf(&b, "  Network:   %s\n", cfg.Network.Mode.String())
if cfg.Network.Mode == model.NetworkStatic {
fmt.Fprintf(&b, "  Address:   %s\n", cfg.Network.Address)
fmt.Fprintf(&b, "  Gateway:   %s\n", cfg.Network.Gateway)
}
if len(cfg.Users) > 0 {
fmt.Fprintf(&b, "  User:      %s\n", cfg.Users[0].Username)
}
if len(cfg.SSHKeys) > 0 {
fmt.Fprintf(&b, "  SSH Keys:  %d configured\n", len(cfg.SSHKeys))
}
selected := 0
for _, s := range cfg.Sysexts {
if s.Selected {
selected++
}
}
if selected > 0 {
fmt.Fprintf(&b, "  Sysexts:   %d selected\n", selected)
}
if cfg.UpdateStrategy.RebootStrategy != "" {
fmt.Fprintf(&b, "  Update:    %s\n", cfg.UpdateStrategy.RebootStrategy)
}
}

b.WriteString("\n⚠ ALL DATA ON " + cfg.Disk.DevPath + " WILL BE DESTROYED!\n\n")
for i, f := range m.fields {
cursor := "  "
if i == m.fieldIdx {
cursor = "▸ "
}
fmt.Fprintf(&b, "%s%s: %s\n", cursor, f.label, f.value)
}
return b.String()
}

func (m *Model) viewInstall() string {
var b strings.Builder
b.WriteString("Installing Flatcar Container Linux...\n\n")
for _, msg := range m.Wizard.State.ProgressMessages {
fmt.Fprintf(&b, "  ✓ %s\n", msg)
}
if m.Wizard.State.CurrentStep == model.StepInstall {
b.WriteString("\nPress Enter to start installation...")
}
return b.String()
}

func (m *Model) viewDone() string {
return `
✅ Installation Complete!

Flatcar Container Linux has been installed successfully.
Remove the installation media and reboot your system.

Press q to exit.
`
}

// Run starts the Bubble Tea program
func Run(w *wizard.Wizard) error {
m := New(w)
p := tea.NewProgram(m, tea.WithAltScreen())
_, err := p.Run()
return err
}
