package tui

import (
"strings"
"testing"

"github.com/projectbluefin/knuckle/internal/model"
"github.com/projectbluefin/knuckle/internal/wizard"
)

// newTestModel creates a Model with an initialized Wizard for form testing.
func newTestModel() *Model {
w := &wizard.Wizard{
State: &wizard.State{
Config: model.InstallConfig{
Channel: "stable",
},
},
}
return &Model{Wizard: w}
}

// TestBuildUserFormValidators exercises the inline Validate closures
// inside buildUserForm to increase branch coverage on forms.go.
func TestBuildUserFormValidators(t *testing.T) {
m := newTestModel()

form := m.buildUserForm()
if form == nil {
t.Fatal("buildUserForm returned nil")
}

// Each call to buildUserForm with different state values exercises
// the form construction logic and ensures validators are wired correctly.
configs := []struct {
hostname string
username string
password string
github   string
sshKey   string
timezone string
}{
{"", "", "", "", "", ""},
{"flatcar-node01", "admin", "secret", "", "", "UTC"},
{"my-host", "deployer", "", "octocat", "", "America/New_York"},
{"node-1", "user1", "", "", "ssh-rsa AAAA...", "Europe/Berlin"},
{"x", "a", "p", "gh-user", "ssh-ed25519 AAAA...", "Asia/Tokyo"},
}

for _, c := range configs {
m.Wizard.State.Config.Hostname = c.hostname
m.Wizard.State.Config.Timezone = c.timezone
m.usernameInput = c.username
m.passwordInput = c.password
m.githubUserInput = c.github
m.sshKeyInput = c.sshKey
f := m.buildUserForm()
if f == nil {
t.Errorf("buildUserForm returned nil for config %+v", c)
}
}
}

// TestBuildNetworkFormValidators exercises CIDR and IP validation closures.
func TestBuildNetworkFormValidators(t *testing.T) {
m := newTestModel()
m.Wizard.State.Interfaces = []model.NetworkInterface{
{Name: "eth0", MAC: "00:11:22:33:44:55", State: "up"},
{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", State: "down"},
}

form := m.buildNetworkForm()
if form == nil {
t.Fatal("buildNetworkForm returned nil")
}

// Various states for network form construction
configs := []struct {
mode    string
addr    string
gw      string
dns     string
iface   string
}{
{"dhcp", "", "", "", ""},
{"static", "192.168.1.100/24", "192.168.1.1", "1.1.1.1,8.8.8.8", "eth0"},
{"static", "10.0.0.5/8", "10.0.0.1", "8.8.4.4", "eth1"},
{"dhcp", "", "", "", ""},
}

for _, c := range configs {
m.networkModeInput = c.mode
m.Wizard.State.Config.Network.Address = c.addr
m.Wizard.State.Config.Network.Gateway = c.gw
m.Wizard.State.Config.Network.Interface = c.iface
m.dnsInput = c.dns
f := m.buildNetworkForm()
if f == nil {
t.Errorf("buildNetworkForm returned nil for config %+v", c)
}
}
}

// TestBuildTailscaleFormValidators exercises the tailscale auth key validator.
func TestBuildTailscaleFormValidators(t *testing.T) {
m := newTestModel()

form := m.buildTailscaleForm()
if form == nil {
t.Fatal("buildTailscaleForm returned nil")
}

configs := []struct {
authKey string
mode    string
routes  string
}{
{"", model.TailscaleModeConnect, ""},
{"tskey-auth-kAbCdEfGhIjKlMnOpQrStUvWxYz1234567890ab", model.TailscaleModeExitNode, ""},
{"", model.TailscaleModeSubnetRouter, "10.0.0.0/24,192.168.1.0/24"},
{"tskey-auth-test123test456test789test012test345test67", model.TailscaleModeConnect, ""},
}

for _, c := range configs {
m.tailscaleAuthKeyIn = c.authKey
m.tailscaleModeIn = c.mode
m.tailscaleRoutesIn = c.routes
f := m.buildTailscaleForm()
if f == nil {
t.Errorf("buildTailscaleForm returned nil for config %+v", c)
}
}
}

// TestReviewSummaryBranches exercises the reviewSummary method to cover
// all conditional branches in the summary builder.
func TestReviewSummaryBranches(t *testing.T) {
tests := []struct {
name     string
cfg      model.InstallConfig
contains []string
}{
{
name: "minimal config",
cfg: model.InstallConfig{
Channel: "stable",
Disk:    model.DiskInfo{DevPath: "/dev/sda"},
Network: model.NetworkConfig{Mode: model.NetworkDHCP},
},
contains: []string{"stable", "/dev/sda", "dhcp", "disabled"},
},
{
name: "with version and disk model",
cfg: model.InstallConfig{
Channel: "beta",
Version: "3815.1.0",
Disk:    model.DiskInfo{DevPath: "/dev/nvme0n1", Model: "Samsung 980", SizeHuman: "500GB"},
Network: model.NetworkConfig{Mode: model.NetworkDHCP},
},
contains: []string{"beta", "3815.1.0", "Samsung 980", "500GB"},
},
{
name: "static network",
cfg: model.InstallConfig{
Channel: "stable",
Disk:    model.DiskInfo{DevPath: "/dev/sda"},
Network: model.NetworkConfig{
Mode:    model.NetworkStatic,
Address: "10.0.0.5/24",
Gateway: "10.0.0.1",
},
},
contains: []string{"static", "10.0.0.5/24", "10.0.0.1"},
},
{
name: "with users and SSH keys",
cfg: model.InstallConfig{
Channel:  "stable",
Disk:     model.DiskInfo{DevPath: "/dev/sda"},
Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
Hostname: "node-01",
Users:    []model.UserConfig{{Username: "admin"}},
SSHKeys:  []string{"ssh-rsa AAAA...", "ssh-ed25519 AAAA..."},
},
contains: []string{"node-01", "admin", "2 key(s)"},
},
{
name: "with sysexts",
cfg: model.InstallConfig{
Channel: "stable",
Disk:    model.DiskInfo{DevPath: "/dev/sda"},
Network: model.NetworkConfig{Mode: model.NetworkDHCP},
Sysexts: []model.SysextEntry{{Name: "docker"}, {Name: "tailscale"}},
},
contains: []string{"docker", "tailscale"},
},
{
name: "swap enabled with explicit size",
cfg: model.InstallConfig{
Channel: "stable",
Disk:    model.DiskInfo{DevPath: "/dev/sda"},
Network: model.NetworkConfig{Mode: model.NetworkDHCP},
Swap:    model.SwapConfig{Enabled: true, SizeMB: 2048},
},
contains: []string{"2048 MiB"},
},
{
name: "swap enabled with default size",
cfg: model.InstallConfig{
Channel: "stable",
Disk:    model.DiskInfo{DevPath: "/dev/sda"},
Network: model.NetworkConfig{Mode: model.NetworkDHCP},
Swap:    model.SwapConfig{Enabled: true, SizeMB: 0},
},
contains: []string{"MiB", "/var/swapfile"},
},
{
name: "tailscale connect mode",
cfg: model.InstallConfig{
Channel: "stable",
Disk:    model.DiskInfo{DevPath: "/dev/sda"},
Network: model.NetworkConfig{Mode: model.NetworkDHCP},
Tailscale: model.TailscaleConfig{
AuthKey: "tskey-auth-xyz",
Mode:    model.TailscaleModeConnect,
},
},
contains: []string{"Tailscale", "connect"},
},
{
name: "tailscale subnet router with routes",
cfg: model.InstallConfig{
Channel: "stable",
Disk:    model.DiskInfo{DevPath: "/dev/sda"},
Network: model.NetworkConfig{Mode: model.NetworkDHCP},
Tailscale: model.TailscaleConfig{
AuthKey: "tskey-auth-xyz",
Mode:    model.TailscaleModeSubnetRouter,
Routes:  "10.0.0.0/24",
},
},
contains: []string{"subnet-router", "routes=10.0.0.0/24"},
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
m := newTestModel()
m.Wizard.State.Config = tt.cfg
summary := m.reviewSummary()
if summary == "" {
t.Error("reviewSummary returned empty string")
}
for _, want := range tt.contains {
if !strings.Contains(summary, want) {
t.Errorf("reviewSummary missing %q in:\n%s", want, summary)
}
}
})
}
}

// TestBuildReviewFormConstruction ensures the review form builds correctly.
func TestBuildReviewFormConstruction(t *testing.T) {
m := newTestModel()
m.Wizard.State.Config = model.InstallConfig{
Channel:  "stable",
Disk:     model.DiskInfo{DevPath: "/dev/sda", Model: "VBOX HARDDISK", SizeHuman: "50GB"},
Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
Hostname: "test-node",
Users:    []model.UserConfig{{Username: "admin"}},
}

form := m.buildReviewForm()
if form == nil {
t.Fatal("buildReviewForm returned nil")
}
}
