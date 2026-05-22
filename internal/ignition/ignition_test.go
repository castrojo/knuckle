package ignition

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
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

func TestGenerateButaneTimezoneAndStatic(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "combo-node",
		Timezone: "America/New_York",
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "eth0",
			Address:   "10.0.0.50/24",
			Gateway:   "10.0.0.1",
			DNS:       []string{"1.1.1.1"},
		},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The static network file MUST appear under storage.files, not storage.links.
	// Find positions: "files:" must come before "10-static.network",
	// and "10-static.network" must come before "links:".
	filesIdx := strings.Index(output, "files:")
	staticIdx := strings.Index(output, "10-static.network")
	linksIdx := strings.Index(output, "links:")

	if filesIdx < 0 {
		t.Fatal("missing files: section")
	}
	if staticIdx < 0 {
		t.Fatal("missing 10-static.network entry")
	}
	if linksIdx < 0 {
		t.Fatal("missing links: section")
	}

	if staticIdx < filesIdx {
		t.Errorf("static network (pos %d) appears before files: (pos %d)", staticIdx, filesIdx)
	}
	if staticIdx > linksIdx {
		t.Errorf("static network (pos %d) appears after links: (pos %d) — should be under files:", staticIdx, linksIdx)
	}

	// Also verify timezone link is present
	if !strings.Contains(output, "/usr/share/zoneinfo/America/New_York") {
		t.Error("missing timezone zoneinfo target")
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

func TestGenerateButaneDefaultChannel(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "default-channel-node",
		Channel:  "", // empty — should default to "stable"
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "GROUP=stable") {
		t.Errorf("expected GROUP=stable when channel is empty, got output:\n%s", output)
	}
}

func TestGenerateButaneTimezoneAndSysexts(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "sysext-tz-node",
		Timezone: "Europe/Berlin",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA test"}},
		},
		Sysexts: []model.SysextEntry{
			{Name: "docker", Version: "24.0.7", URL: "https://example.com/docker.raw", Selected: true},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sysext file must be under storage.files, NOT storage.links
	filesIdx := strings.Index(output, "files:")
	linksIdx := strings.Index(output, "links:")
	sysextIdx := strings.Index(output, "/etc/extensions/docker.raw")

	if filesIdx < 0 {
		t.Fatal("expected files: section")
	}
	if linksIdx < 0 {
		t.Fatal("expected links: section for timezone")
	}
	if sysextIdx < 0 {
		t.Fatal("expected sysext file entry")
	}

	// Sysext must appear BEFORE links section (i.e., under files)
	if sysextIdx > linksIdx {
		t.Errorf("BUG: sysext file appears AFTER links: section — would be parsed as a link.\nfiles: at %d, links: at %d, sysext at %d", filesIdx, linksIdx, sysextIdx)
	}

	// Also verify timezone link is correct
	if !strings.Contains(output, "/usr/share/zoneinfo/Europe/Berlin") {
		t.Error("missing timezone link target")
	}
}

func TestGenerateButaneDuplicateUsers(t *testing.T) {
	// Duplicate usernames must be caught by validate.CheckConsistency before
	// reaching the generator. This test documents that GenerateButane itself
	// does not panic — enforcement happens upstream.
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "dup-user-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA key1"}},
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA key2"}},
		},
	}
	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane should not error on duplicate usernames (enforcement is upstream): %v", err)
	}
	_ = output // caller responsible for deduplication via validate.CheckConsistency
}

func TestYamlEscapeNewlines(t *testing.T) {
	g := NewGenerator()

	cfg := &model.InstallConfig{
		Hostname: "test\nhost",
		Users: []model.UserConfig{
			{Username: "user", SSHKeys: []string{"ssh-ed25519 AAAA"}, Groups: []string{"docker"}},
		},
		SSHKeys: []string{"ssh-ed25519 AAAA"},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The hostname should have escaped newline, not a literal one
	if strings.Contains(output, "test\nhost") {
		t.Error("literal newline found in output — yamlEscape should escape \\n")
	}
	if !strings.Contains(output, `test\nhost`) {
		t.Error("expected escaped \\n sequence in output")
	}
}

func TestGenerateButaneWithSysextSpecialCharsURL(t *testing.T) {
	gen := NewGenerator()
	cfg := &model.InstallConfig{
		Channel:  "stable",
		Hostname: "test-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			{Name: "test-ext", URL: `https://example.com/ext.raw?foo="bar"&baz=1`, Selected: true, Version: "1.0"},
		},
	}

	output, err := gen.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}

	// URL with quotes should be escaped and produce valid YAML
	if !strings.Contains(output, "test-ext.raw") {
		t.Error("missing sysext file entry")
	}
	// The URL should NOT contain unescaped double quotes that break YAML
	if strings.Contains(output, `source: "https://example.com/ext.raw?foo="bar"`) {
		t.Error("URL with quotes was not escaped — YAML will be invalid")
	}
	// Should contain the escaped version
	if !strings.Contains(output, `source: "https://example.com/ext.raw?foo=\"bar\"&baz=1"`) {
		t.Logf("output:\n%s", output)
		t.Error("URL should have escaped quotes")
	}
}

func TestGenerateButaneNoSysexts_NoService(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "no-sysext-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts:  []model.SysextEntry{},
	}
	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}
	if strings.Contains(output, "systemd-sysext.service") {
		t.Error("systemd-sysext.service must NOT be enabled when no sysexts selected")
	}
}

func TestGenerateButaneUnselectedSysextsOmitService(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "unselected-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			{Name: "docker", URL: "https://example.com/docker.raw", Selected: false},
		},
	}
	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}
	if strings.Contains(output, "systemd-sysext.service") {
		t.Error("systemd-sysext.service must NOT be enabled when all sysexts are unselected")
	}
	if strings.Contains(output, "/etc/extensions/docker.raw") {
		t.Error("unselected docker sysext must not appear in output")
	}
}

func TestGenerateButaneWithSysextSHA256(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "verified-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Sysexts: []model.SysextEntry{
			{
				Name:     "wasmtime",
				Version:  "44.0.1",
				URL:      "https://extensions.flatcar.org/wasmtime-v44.0.1-x86-64.raw",
				Sha256:   "e5336201eedf0c5e7620c6947c821009c362231f7c9023174b9c4f99a1f0ad1b",
				Selected: true,
			},
			{
				Name:     "docker",
				Version:  "28.0.0",
				URL:      "https://extensions.flatcar.org/docker-28.0.0-x86-64.raw",
				Sha256:   "", // no hash — verification block must be absent
				Selected: true,
			},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// wasmtime entry must include verification hash.
	if !strings.Contains(output, "sha256-e5336201eedf0c5e7620c6947c821009c362231f7c9023174b9c4f99a1f0ad1b") {
		t.Error("wasmtime entry should contain sha256 verification hash in Butane output")
	}
	if !strings.Contains(output, "verification:") {
		t.Error("Butane output should contain 'verification:' block for hashed sysext")
	}

	// docker entry has no hash — verification block must NOT appear for that entry.
	// We verify by checking the overall output doesn't have TWO verification blocks
	// (only one sysext has a hash).
	count := strings.Count(output, "verification:")
	if count != 1 {
		t.Errorf("expected exactly 1 verification block (only wasmtime has a hash), got %d", count)
	}
}

func TestGenerateButaneWithNvidiaDriverVersion(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname:            "gpu-node",
		Network:             model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:               []model.UserConfig{{Username: "core"}},
		NvidiaDriverVersion: "570-open",
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "/etc/flatcar/enabled-sysext.conf") {
		t.Error("expected /etc/flatcar/enabled-sysext.conf in output when NvidiaDriverVersion is set")
	}
	if !strings.Contains(output, "nvidia-drivers-570-open") {
		t.Error("expected nvidia-drivers-570-open in enabled-sysext.conf content")
	}
}

func TestGenerateButaneNoNvidiaWhenVersionEmpty(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "plain-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		// NvidiaDriverVersion is empty — no NVIDIA setup
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(output, "enabled-sysext.conf") {
		t.Error("enabled-sysext.conf should NOT appear when NvidiaDriverVersion is empty")
	}
	if strings.Contains(output, "nvidia-drivers-") {
		t.Error("nvidia-drivers- prefix should NOT appear when NvidiaDriverVersion is empty")
	}
}

func TestGenerateButaneNvidiaWithSysexts(t *testing.T) {
	// Verify that nvidia-runtime sysext + NvidiaDriverVersion produces both:
	// 1. the sysext file entry (Container Toolkit download)
	// 2. the enabled-sysext.conf entry (kernel driver)
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "full-gpu-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Sysexts: []model.SysextEntry{
			{Name: "nvidia-runtime", Version: "1.17.9",
				URL: "https://extensions.flatcar.org/nvidia-runtime-v1.17.9-x86-64.raw", Selected: true},
		},
		NvidiaDriverVersion: "550-open",
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Container Toolkit sysext download
	if !strings.Contains(output, "/etc/extensions/nvidia-runtime.raw") {
		t.Error("expected nvidia-runtime.raw sysext download entry")
	}
	// Kernel driver activation
	if !strings.Contains(output, "nvidia-drivers-550-open") {
		t.Error("expected nvidia-drivers-550-open in enabled-sysext.conf")
	}
}

func TestGenerateButaneNvidiaDriverVersionEscaped(t *testing.T) {
	// Verify that a malicious NvidiaDriverVersion with newlines is escaped,
	// preventing YAML injection into the Butane template.
	// yamlEscape converts \n to literal \n text, so the injected content
	// stays on one line in the YAML block scalar — not parsed as a new entry.
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname:            "inject-test",
		Network:             model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:               []model.UserConfig{{Username: "core"}},
		NvidiaDriverVersion: "570-open\n    - path: /etc/shadow",
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The injected newline must be escaped. In YAML literal block (|), a real
	// newline would create a new line that could be parsed as a sibling YAML key.
	// After yamlEscape, the output should contain the escaped sequence on one line.
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "nvidia-drivers-") {
			// This line should contain the ENTIRE injected payload as escaped text
			if !strings.Contains(line, "nvidia-drivers-570-open") {
				t.Error("expected nvidia-drivers-570-open on the sysext line")
			}
			// The \n should be escaped to literal backslash-n, keeping payload on same line
			if !strings.Contains(line, `\n`) {
				t.Error("expected escaped \\n in output — newline not escaped")
			}
			break
		}
	}

	// Verify no line starts with "    - path: /etc/shadow" (would indicate YAML injection)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- path: /etc/shadow") {
			t.Fatal("YAML injection: /etc/shadow appeared as a separate YAML path entry")
		}
	}
}

func TestGenerateButaneSwapEnabledDefault(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "swap-default",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Swap:     model.SwapConfig{Enabled: true, SizeMB: 0},
	}
	out, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "/var/swapfile") {
		t.Error("expected /var/swapfile in output")
	}
	if !strings.Contains(out, "fallocate -l 4096M") {
		t.Errorf("expected default 4096 MiB swap, got:\n%s", out)
	}
	if !strings.Contains(out, "var-swapfile.swap") {
		t.Error("expected var-swapfile.swap unit")
	}
	if !strings.Contains(out, "knuckle-create-swapfile.service") {
		t.Error("expected knuckle-create-swapfile.service")
	}
}

func TestGenerateButaneSwapExplicitSize(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "swap-8g",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Swap:     model.SwapConfig{Enabled: true, SizeMB: 8192},
	}
	out, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "fallocate -l 8192M") {
		t.Errorf("expected 8192 MiB swap, got:\n%s", out)
	}
}

func TestGenerateButaneSwapDisabled(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "no-swap",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Swap:     model.SwapConfig{Enabled: false},
	}
	out, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "/var/swapfile") {
		t.Error("/var/swapfile must not appear when Swap.Enabled = false")
	}
	if strings.Contains(out, "var-swapfile.swap") {
		t.Error("swap unit must not appear when Swap.Enabled = false")
	}
}

func TestGenerateButane_SysextHTTPURLRejected(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "node01",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA key"}}},
		Sysexts: []model.SysextEntry{
			{Name: "docker", URL: "http://evil.example.com/docker.raw", Selected: true},
		},
	}
	_, err := g.GenerateButane(cfg)
	if err == nil {
		t.Error("expected error for non-HTTPS sysext URL, got nil")
	}
}

func TestGenerateButaneTailscaleAuthKey(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "ts-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Tailscale: model.TailscaleConfig{
			AuthKey: "tskey-auth-abcdefghij0-secret-secret-secret-secret-1",
			Mode:    model.TailscaleModeConnect,
		},
	}

	out, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "/etc/tailscale/tailscale.env") {
		t.Error("expected /etc/tailscale/tailscale.env in Butane output")
	}
	if !strings.Contains(out, "TS_AUTHKEY=tskey-auth-") {
		t.Error("expected TS_AUTHKEY in tailscale.env contents")
	}
	if !strings.Contains(out, "tailscaled.service") {
		t.Error("expected tailscaled.service enabled")
	}
	if !strings.Contains(out, "knuckle-tailscale-up.service") {
		t.Error("expected knuckle-tailscale-up.service in systemd units")
	}
	if strings.Contains(out, "net.ipv4.ip_forward = 1") {
		t.Error("connect mode should not enable IP forwarding")
	}
	// The unit body references $${TS_EXTRA_ARGS} as an env var; the env file
	// itself should not define TS_EXTRA_ARGS= in connect mode (empty → expands
	// to an empty string).
	if strings.Contains(out, "TS_EXTRA_ARGS=") {
		t.Error("connect mode should not set TS_EXTRA_ARGS= in tailscale.env")
	}
}

func TestGenerateButaneTailscaleExitNode(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "ts-exit",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Tailscale: model.TailscaleConfig{
			AuthKey: "tskey-auth-abcdefghij0-secret-secret-secret-secret-1",
			Mode:    model.TailscaleModeExitNode,
		},
	}
	out, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "--advertise-exit-node") {
		t.Error("expected --advertise-exit-node in TS_EXTRA_ARGS")
	}
	if !strings.Contains(out, "net.ipv4.ip_forward = 1") {
		t.Error("expected IP forwarding sysctl for exit node")
	}
}

func TestGenerateButaneTailscaleSubnetRouter(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "ts-subnet",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core"}},
		Tailscale: model.TailscaleConfig{
			AuthKey: "tskey-auth-abcdefghij0-secret-secret-secret-secret-1",
			Mode:    model.TailscaleModeSubnetRouter,
			Routes:  "10.0.0.0/24,192.168.1.0/24",
		},
	}
	out, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "--advertise-routes=10.0.0.0/24,192.168.1.0/24") {
		t.Errorf("expected --advertise-routes in TS_EXTRA_ARGS, got:\n%s", out)
	}
	if !strings.Contains(out, "net.ipv4.ip_forward = 1") {
		t.Error("expected IP forwarding sysctl for subnet router")
	}
}

func TestGenerateButaneTailscaleSkippedWhenNoAuthKey(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname:  "ts-skip",
		Network:   model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:     []model.UserConfig{{Username: "core"}},
		Tailscale: model.TailscaleConfig{}, // no auth key
	}
	out, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "tailscale.env") {
		t.Error("tailscale.env should not appear when AuthKey is empty")
	}
	if strings.Contains(out, "tailscaled.service") {
		t.Error("tailscaled.service should not be force-enabled when AuthKey is empty")
	}
}
