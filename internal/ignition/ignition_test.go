package ignition

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castrojo/knuckle/internal/model"
)

func TestGenerateButaneDHCP(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "flatcar-node01",
		Network: model.NetworkConfig{
			Mode: model.NetworkDHCP,
		},
		Users: []model.UserConfig{
			{
				Username: "admin",
				SSHKeys:  []string{"ssh-ed25519 AAAAC3test admin@host"},
				Groups:   []string{"sudo", "docker"},
			},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key fields present
	if !strings.Contains(output, "variant: flatcar") {
		t.Error("missing variant header")
	}
	if !strings.Contains(output, `inline: "flatcar-node01"`) {
		t.Error("missing hostname")
	}
	if !strings.Contains(output, `name: "admin"`) {
		t.Error("missing user")
	}
	if !strings.Contains(output, "ssh-ed25519 AAAAC3test") {
		t.Error("missing SSH key")
	}
	// Static network block must NOT appear
	if strings.Contains(output, "10-static.network") {
		t.Error("static network block should not appear in DHCP config")
	}

	// Golden file comparison
	golden := filepath.Join("..", "..", "testdata", "ignition_dhcp.yaml")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(output), 0644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
	}
	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file: %v (run with UPDATE_GOLDEN=1 to create)", err)
	}
	if output != string(expected) {
		t.Errorf("output does not match golden file.\nGot:\n%s\nWant:\n%s", output, string(expected))
	}
}

func TestGenerateButaneStatic(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "flatcar-static",
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "eth0",
			Address:   "192.168.1.100/24",
			Gateway:   "192.168.1.1",
			DNS:       []string{"8.8.8.8", "8.8.4.4"},
		},
		Users: []model.UserConfig{
			{
				Username: "core",
				SSHKeys:  []string{"ssh-rsa AAAABtest core@host"},
			},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify static network block
	if !strings.Contains(output, "10-static.network") {
		t.Error("missing static network file")
	}
	if !strings.Contains(output, "Address=192.168.1.100/24") {
		t.Error("missing address")
	}
	if !strings.Contains(output, "Gateway=192.168.1.1") {
		t.Error("missing gateway")
	}
	if !strings.Contains(output, "DNS=8.8.8.8") {
		t.Error("missing DNS entry")
	}
	if !strings.Contains(output, "Name=eth0") {
		t.Error("missing interface name")
	}

	// Golden file comparison
	golden := filepath.Join("..", "..", "testdata", "ignition_static.yaml")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(output), 0644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
	}
	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file: %v (run with UPDATE_GOLDEN=1 to create)", err)
	}
	if output != string(expected) {
		t.Errorf("output does not match golden file.\nGot:\n%s\nWant:\n%s", output, string(expected))
	}
}

func TestGenerateButaneWithSysexts(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "sysext-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core"},
		},
		Sysexts: []model.SysextEntry{
			{Name: "docker", Version: "24.0.7", URL: "https://extensions.flatcar.org/docker-24.0.7.raw", Selected: true},
			{Name: "vim", Version: "9.0", URL: "https://extensions.flatcar.org/vim-9.0.raw", Selected: false},
			{Name: "tailscale", Version: "1.56.1", URL: "https://extensions.flatcar.org/tailscale-1.56.1.raw", Selected: true},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "/etc/extensions/docker.raw") {
		t.Error("missing docker sysext file entry")
	}
	if !strings.Contains(output, "source: \"https://extensions.flatcar.org/docker-24.0.7.raw\"") {
		t.Error("missing docker sysext URL source")
	}
	if !strings.Contains(output, "/etc/extensions/tailscale.raw") {
		t.Error("missing tailscale sysext file entry")
	}
	if !strings.Contains(output, "source: \"https://extensions.flatcar.org/tailscale-1.56.1.raw\"") {
		t.Error("missing tailscale sysext URL source")
	}
	if !strings.Contains(output, "systemd-sysext.service") {
		t.Error("missing systemd-sysext service unit")
	}
	// vim should NOT appear (Selected=false)
	if strings.Contains(output, "vim") {
		t.Error("unselected sysext 'vim' should not appear")
	}
}

func TestGenerateButaneNilConfig(t *testing.T) {
	g := NewGenerator()
	_, err := g.GenerateButane(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config cannot be nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateButaneDefaultCoreUser(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "default-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		SSHKeys:  []string{"ssh-ed25519 AAAAdefault user@laptop"},
		// No Users specified — should fall back to "core"
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, `name: "core"`) {
		t.Error("expected default 'core' user when no users specified")
	}
	if !strings.Contains(output, "ssh-ed25519 AAAAdefault") {
		t.Error("expected SSH keys on default core user")
	}
}

func TestFilterSelected(t *testing.T) {
	input := []model.SysextEntry{
		{Name: "a", Selected: true},
		{Name: "b", Selected: false},
		{Name: "c", Selected: true},
		{Name: "d", Selected: false},
	}

	result := filterSelected(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 selected, got %d", len(result))
	}
	if result[0].Name != "a" {
		t.Errorf("expected first selected to be 'a', got %q", result[0].Name)
	}
	if result[1].Name != "c" {
		t.Errorf("expected second selected to be 'c', got %q", result[1].Name)
	}
}

func TestFilterSelectedEmpty(t *testing.T) {
	result := filterSelected(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = filterSelected([]model.SysextEntry{})
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestGenerateButaneTimezoneLink(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "tz-node",
		Timezone: "America/New_York",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "/etc/localtime") {
		t.Error("expected /etc/localtime link for timezone")
	}
	if !strings.Contains(output, "/usr/share/zoneinfo/America/New_York") {
		t.Error("expected zoneinfo target for timezone")
	}
}

func TestGenerateButaneTimezoneAbsentWhenEmpty(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "no-tz-node",
		Timezone: "",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(output, "/etc/localtime") {
		t.Error("timezone link should not appear when timezone is empty")
	}
}

func TestGenerateButanePasswordAuthYes(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "pw-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{
				Username:     "admin",
				PasswordHash: "$2a$10$somehash",
				SSHKeys:      []string{"ssh-ed25519 AAAA test"},
			},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "PasswordAuthentication yes") {
		t.Error("expected PasswordAuthentication yes when user has password")
	}
}

func TestGenerateButanePasswordAuthNo(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "nopw-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{
				Username: "admin",
				SSHKeys:  []string{"ssh-ed25519 AAAA test"},
			},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "PasswordAuthentication no") {
		t.Error("expected PasswordAuthentication no when no user has password")
	}
	if strings.Contains(output, "PasswordAuthentication yes") {
		t.Error("should not have PasswordAuthentication yes without password")
	}
}
