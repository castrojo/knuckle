package ignition

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// === Q1: Zero users — does the template produce valid Butane? ===

func TestGenerateButane_ZeroUsers_NoSSHKeys(t *testing.T) {
	// Zero users AND zero SSHKeys — what happens?
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "empty-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{},
		SSHKeys:  []string{},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall to the else branch and create "core" user
	if !strings.Contains(output, `name: "core"`) {
		t.Error("expected default 'core' user when Users slice is empty")
	}

	// But with no SSHKeys, we get a user with no auth at all!
	// This should still produce valid Butane that compiles.
	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("generated Butane for zero-user config fails compilation: %v\nButane:\n%s", compileErr, output)
	}
}

func TestGenerateButane_ZeroUsers_WithSSHKeys(t *testing.T) {
	// Zero users but top-level SSHKeys present — should attach to "core"
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "keyed-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{},
		SSHKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@host"},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, `name: "core"`) {
		t.Error("expected default 'core' user")
	}
	if !strings.Contains(output, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7") {
		t.Error("expected SSH key on default core user")
	}

	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("fails compilation: %v", compileErr)
	}
}

func TestGenerateButane_NilUsers_NilSSHKeys(t *testing.T) {
	// nil Users (not empty slice) and nil SSHKeys
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "nil-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    nil,
		SSHKeys:  nil,
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// nil and empty slice behave the same in range — should produce core user
	if !strings.Contains(output, `name: "core"`) {
		t.Error("expected default 'core' user with nil Users")
	}

	// Must compile successfully
	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("fails compilation: %v\nButane:\n%s", compileErr, output)
	}
}

// === Q2: Users with empty SSHKeys AND empty PasswordHash ===

func TestGenerateButane_UserNoAuth(t *testing.T) {
	// User exists but has no SSH keys and no password — essentially locked out
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "noauth-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{
				Username:     "admin",
				SSHKeys:      []string{},
				PasswordHash: "",
				Groups:       []string{"sudo"},
			},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User should appear
	if !strings.Contains(output, `name: "admin"`) {
		t.Error("expected admin user")
	}

	// No ssh_authorized_keys block should appear for this user
	if strings.Contains(output, "ssh_authorized_keys") {
		t.Error("ssh_authorized_keys should not appear for user with empty SSHKeys")
	}

	// No password_hash either
	if strings.Contains(output, "password_hash") {
		t.Error("password_hash should not appear for user with empty PasswordHash")
	}

	// Must still produce valid Butane (even if the user is inaccessible)
	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("fails compilation: %v\nButane:\n%s", compileErr, output)
	}
}

func TestGenerateButane_MultipleUsersOnlyOneHasAuth(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "mixed-auth-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "locked", SSHKeys: []string{}, PasswordHash: ""},
			{Username: "admin", SSHKeys: []string{"ssh-ed25519 AAAA key"}, PasswordHash: ""},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, `name: "locked"`) {
		t.Error("missing locked user")
	}
	if !strings.Contains(output, `name: "admin"`) {
		t.Error("missing admin user")
	}

	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("fails compilation: %v", compileErr)
	}
}

// === Q3: Special characters in hostname, username, SSH key comment ===

func TestGenerateButane_SpecialCharsHostname(t *testing.T) {
	g := NewGenerator()
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{"quotes in hostname", `my"host`, false},
		{"backslash in hostname", `my\host`, false},
		{"tab in hostname", "my\thost", false},
		{"unicode in hostname", "flatcar-nöde", false},
		{"empty hostname", "", false}, // generator doesn't validate — that's validate pkg
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &model.InstallConfig{
				Hostname: tc.hostname,
				Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
				Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
			}

			output, err := g.GenerateButane(cfg)
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil {
				return
			}

			// Every generated Butane must compile — if special chars break YAML, this catches it
			_, compileErr := CompileToIgnition(output)
			if compileErr != nil {
				t.Errorf("hostname %q breaks Butane compilation: %v\nButane:\n%s", tc.hostname, compileErr, output)
			}
		})
	}
}

func TestGenerateButane_SpecialCharsUsername(t *testing.T) {
	g := NewGenerator()
	tests := []struct {
		name     string
		username string
	}{
		{"username with dots", "john.doe"},
		{"username with dash", "my-user"},
		{"username with underscore", "my_user"},
		{"username with quotes", `user"name`},
		{"username with backslash", `user\name`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &model.InstallConfig{
				Hostname: "test-node",
				Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
				Users: []model.UserConfig{
					{Username: tc.username, SSHKeys: []string{"ssh-ed25519 AAAA k"}},
				},
			}

			output, err := g.GenerateButane(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Must produce valid YAML that compiles
			_, compileErr := CompileToIgnition(output)
			if compileErr != nil {
				t.Errorf("username %q breaks Butane compilation: %v\nButane:\n%s", tc.username, compileErr, output)
			}
		})
	}
}

func TestGenerateButane_SSHKeyWithSpecialComment(t *testing.T) {
	g := NewGenerator()
	tests := []struct {
		name string
		key  string
	}{
		{"key with email comment", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 user@host.example.com"},
		{"key with spaces in comment", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 John Doe's Key"},
		{"key with quotes in comment", `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 "quoted comment"`},
		{"key with backslash", `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 C:\Users\test`},
		{"key with unicode", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 ñoño@café"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &model.InstallConfig{
				Hostname: "test-node",
				Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
				Users: []model.UserConfig{
					{Username: "core", SSHKeys: []string{tc.key}},
				},
			}

			output, err := g.GenerateButane(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, compileErr := CompileToIgnition(output)
			if compileErr != nil {
				t.Errorf("SSH key %q breaks Butane compilation: %v\nButane:\n%s", tc.key, compileErr, output)
			}
		})
	}
}

// === Q4: Very long SSH keys (4096-bit RSA ≈ 700 chars) ===

func TestGenerateButane_LongRSAKey(t *testing.T) {
	// Simulate a 4096-bit RSA key (~550 chars base64 + prefix + comment)
	longKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDjTid/Xxpik4yiFwhLdPiIkG28XBLeIqvb0/nAwjyYJU8KU7qy91tGCFf/09D3VTnJbp3jfrOwboxb4iL+BiowC5bhbdJtHkQ89tx/xDw8ljrOx025UWp6EvOrD+rk7Aw4kYnLJ0CA5MvzdgVOal0brgHIpw34hbrP/yPNdv/H8VMsZBT+pXDQP0JcGe0K8HRM54cn/xIrSYnUvEZBb+kpscPXJtUGFNDSFxFp7fPhlViYLxDuNQtRgc7u3mAMuLMbxI6JxkIsvZ14PxxFTQ4Vq+BnJEazHgFn3wz86dHqanwx/sE9bBWsk7fhV2rfWpI1WI4KaTVfgeFaJ404VRkP" +
		strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", 10) +
		" user@very-long-key-host.example.com"

	if len(longKey) < 700 {
		t.Fatalf("test key should be >700 chars, got %d", len(longKey))
	}

	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "longkey-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{longKey}},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Key must appear in full (not truncated)
	if !strings.Contains(output, longKey[:50]) {
		t.Error("long key appears truncated in output")
	}

	// Must produce valid Butane
	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Errorf("long RSA key breaks compilation: %v", compileErr)
	}
}

func TestGenerateButane_MultipleSSHKeysPerUser(t *testing.T) {
	g := NewGenerator()
	keys := []string{
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 key1@host",
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDjTid/Xxpik4yiFwhLdPiIkG28XBLeIqvb0/nAwjyYJU8KU7qy91tGCFf/09D3VTnJbp3jfrOwboxb4iL+BiowC5bhbdJtHkQ89tx/xDw8ljrOx025UWp6EvOrD+rk7Aw4kYnLJ0CA5MvzdgVOal0brgHIpw34hbrP/yPNdv/H8VMsZBT+pXDQP0JcGe0K8HRM54cn/xIrSYnUvEZBb+kpscPXJtUGFNDSFxFp7fPhlViYLxDuNQtRgc7u3mAMuLMbxI6JxkIsvZ14PxxFTQ4Vq+BnJEazHgFn3wz86dHqanwx/sE9bBWsk7fhV2rfWpI1WI4KaTVfgeFaJ404VRkP" + strings.Repeat("X", 500) + " key2@host",
		"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAI key3@host",
	}

	cfg := &model.InstallConfig{
		Hostname: "multikey-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: keys},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, k := range keys {
		if !strings.Contains(output, k[:30]) {
			t.Errorf("key %d not found in output", i)
		}
	}

	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("multiple keys break compilation: %v", compileErr)
	}
}

// === Q5: Network config with empty DNS list in static mode ===

func TestGenerateButane_StaticNetworkEmptyDNS(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "no-dns-node",
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "eth0",
			Address:   "10.0.0.50/24",
			Gateway:   "10.0.0.1",
			DNS:       []string{}, // empty DNS list
		},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Static network file must appear
	if !strings.Contains(output, "10-static.network") {
		t.Error("missing static network file")
	}
	// No DNS= lines should appear
	if strings.Contains(output, "DNS=") {
		t.Error("DNS= lines should not appear when DNS list is empty")
	}

	// Must compile
	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("static config with empty DNS fails compilation: %v\nButane:\n%s", compileErr, output)
	}
}

func TestGenerateButane_StaticNetworkNilDNS(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "nil-dns-node",
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "enp0s3",
			Address:   "192.168.1.50/24",
			Gateway:   "192.168.1.1",
			DNS:       nil, // nil DNS list
		},
		Users: []model.UserConfig{
			{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(output, "DNS=") {
		t.Error("DNS= lines should not appear when DNS is nil")
	}

	_, compileErr := CompileToIgnition(output)
	if compileErr != nil {
		t.Fatalf("static config with nil DNS fails compilation: %v", compileErr)
	}
}

// === Q6: Multiple sysexts — order preserved? Duplicates handled? ===

func TestGenerateButane_SysextOrderPreserved(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "order-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			{Name: "zz-last", URL: "https://example.com/zz.raw", Selected: true},
			{Name: "aa-first", URL: "https://example.com/aa.raw", Selected: true},
			{Name: "mm-middle", URL: "https://example.com/mm.raw", Selected: true},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Order must match input order, not sorted
	zzIdx := strings.Index(output, "zz-last.raw")
	aaIdx := strings.Index(output, "aa-first.raw")
	mmIdx := strings.Index(output, "mm-middle.raw")

	if zzIdx < 0 || aaIdx < 0 || mmIdx < 0 {
		t.Fatal("missing sysext entries")
	}

	if zzIdx > aaIdx {
		t.Error("order not preserved: zz-last should appear before aa-first (input order)")
	}
	if aaIdx > mmIdx {
		t.Error("order not preserved: aa-first should appear before mm-middle (input order)")
	}
}

func TestGenerateButane_DuplicateSysexts(t *testing.T) {
	// Duplicate sysext names — generator does NOT deduplicate (that's validate's job)
	// This documents behavior: both appear in output
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "dup-sysext-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			{Name: "docker", URL: "https://example.com/docker-v1.raw", Selected: true},
			{Name: "docker", URL: "https://example.com/docker-v2.raw", Selected: true},
		},
	}

	output, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both URLs appear — the LAST one wins on disk since they write to same path
	count := strings.Count(output, "/etc/extensions/docker.raw")
	if count != 2 {
		t.Errorf("expected 2 docker.raw entries (duplicates not deduplicated), got %d", count)
	}

	// This is a semantic bug: two files with same path will conflict in Ignition.
	// Butane compiler should catch it.
	_, compileErr := CompileToIgnition(output)
	if compileErr == nil {
		// If it compiles, Ignition will use the last entry.
		// Document this as a finding — it's not validated by butane.
		t.Log("NOTE: duplicate sysext paths compile successfully — Ignition uses last-wins semantics")
	} else {
		t.Logf("NOTE: duplicate paths caught by butane compiler: %v", compileErr)
	}
}

// === Q7: Does CompileToIgnition validate the generated Butane? ===

func TestCompileToIgnition_InvalidMode(t *testing.T) {
	// Butane with invalid file mode — does butane catch it?
	butane := `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      mode: 99999
      contents:
        inline: test
`
	_, err := CompileToIgnition(butane)
	// Document whether butane validates mode values
	if err != nil {
		t.Logf("butane catches invalid mode: %v", err)
	} else {
		t.Log("NOTE: butane does NOT validate file mode values (99999 accepted)")
	}
}

func TestCompileToIgnition_InvalidFilePath(t *testing.T) {
	// Relative path should be rejected by Ignition spec
	butane := `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: relative/path
      contents:
        inline: test
`
	_, err := CompileToIgnition(butane)
	if err == nil {
		t.Error("expected error for relative file path — Ignition requires absolute paths")
	} else {
		t.Logf("butane catches relative path: %v", err)
	}
}

func TestCompileToIgnition_EmptyUserName(t *testing.T) {
	butane := `variant: flatcar
version: 1.1.0
passwd:
  users:
    - name: ""
`
	_, err := CompileToIgnition(butane)
	if err == nil {
		t.Log("NOTE: butane accepts empty username — Ignition does not validate user names at compile time")
	} else {
		t.Logf("butane catches empty username: %v", err)
	}
}

func TestCompileToIgnition_DuplicateFilePaths(t *testing.T) {
	// Two files at the same path — does butane catch the conflict?
	butane := `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      contents:
        inline: first
    - path: /etc/hostname
      contents:
        inline: second
`
	_, err := CompileToIgnition(butane)
	if err == nil {
		t.Log("NOTE: butane accepts duplicate file paths — last-wins semantics in Ignition runtime")
	} else {
		t.Logf("butane catches duplicate paths: %v", err)
	}
}

func TestCompileToIgnition_InvalidSHA256Length(t *testing.T) {
	// Short/invalid SHA256 hashes pass GenerateButane (template doesn't validate)
	// but FAIL at CompileToIgnition — butane validates hash length.
	// This is a defense-in-depth finding: the template is dumb, the compiler catches it.
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "bad-hash-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
		Sysexts: []model.SysextEntry{
			{Name: "docker", URL: "https://example.com/docker.raw", Sha256: "tooshort", Selected: true},
		},
	}

	butane, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane should not fail (no validation): %v", err)
	}

	// Template generation succeeds — hash is just interpolated
	if !strings.Contains(butane, "sha256-tooshort") {
		t.Error("expected invalid hash in Butane output")
	}

	// But compilation MUST fail
	_, compileErr := CompileToIgnition(butane)
	if compileErr == nil {
		t.Fatal("expected CompileToIgnition to reject invalid SHA256 hash length")
	}
	if !strings.Contains(compileErr.Error(), "incorrect size for hash sum") {
		t.Errorf("unexpected error message: %v", compileErr)
	}
}

// === End-to-end: GenerateButane → CompileToIgnition round-trip ===

func TestRoundTrip_MinimalConfig(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "roundtrip-node",
		Network:  model.NetworkConfig{Mode: model.NetworkDHCP},
		Users:    []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	butane, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}

	ignition, err := CompileToIgnition(butane)
	if err != nil {
		t.Fatalf("CompileToIgnition: %v\nButane:\n%s", err, butane)
	}

	if !strings.Contains(ignition, "roundtrip-node") {
		t.Error("hostname not in Ignition JSON")
	}
}

func TestRoundTrip_FullFeaturedConfig(t *testing.T) {
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "full-node",
		Timezone: "America/Chicago",
		Channel:  "beta",
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "ens192",
			Address:   "10.20.30.40/24",
			Gateway:   "10.20.30.1",
			DNS:       []string{"1.1.1.1", "8.8.8.8"},
		},
		Users: []model.UserConfig{
			{
				Username:     "deploy",
				SSHKeys:      []string{"ssh-ed25519 AAAA deploy-key"},
				PasswordHash: "$6$rounds=4096$salt$hash",
				Groups:       []string{"sudo", "docker", "systemd-journal"},
			},
			{
				Username: "monitor",
				SSHKeys:  []string{"ssh-ed25519 AAAA monitor-key"},
				Groups:   []string{"systemd-journal"},
			},
		},
		Sysexts: []model.SysextEntry{
			{Name: "docker", URL: "https://ext.flatcar.org/docker.raw", Sha256: "e5336201eedf0c5e7620c6947c821009c362231f7c9023174b9c4f99a1f0ad1b", Selected: true},
			{Name: "tailscale", URL: "https://ext.flatcar.org/tailscale.raw", Selected: true},
		},
		NvidiaDriverVersion: "550-open",
		UpdateStrategy: model.UpdateStrategy{
			RebootStrategy: "off",
		},
	}

	butane, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}

	ignition, err := CompileToIgnition(butane)
	if err != nil {
		t.Fatalf("CompileToIgnition failed on full config: %v\nButane:\n%s", err, butane)
	}

	// Verify key elements survive round-trip
	checks := []string{"full-node", "deploy", "monitor", "10.20.30.40", "docker", "tailscale", "550-open"}
	for _, check := range checks {
		if !strings.Contains(ignition, check) {
			t.Errorf("missing %q in Ignition JSON output", check)
		}
	}
}

func TestRoundTrip_StaticNoGateway(t *testing.T) {
	// Edge case: static mode but empty gateway — should still compile
	g := NewGenerator()
	cfg := &model.InstallConfig{
		Hostname: "no-gw-node",
		Network: model.NetworkConfig{
			Mode:      model.NetworkStatic,
			Interface: "eth0",
			Address:   "192.168.1.10/24",
			Gateway:   "", // empty gateway
			DNS:       []string{"8.8.8.8"},
		},
		Users: []model.UserConfig{{Username: "core", SSHKeys: []string{"ssh-ed25519 AAAA k"}}},
	}

	butane, err := g.GenerateButane(cfg)
	if err != nil {
		t.Fatalf("GenerateButane: %v", err)
	}

	// Gateway= line will be empty but present
	if !strings.Contains(butane, "Gateway=\n") && !strings.Contains(butane, "Gateway=") {
		// Actually check what happens
		t.Logf("Butane with empty gateway:\n%s", butane)
	}

	_, compileErr := CompileToIgnition(butane)
	if compileErr != nil {
		t.Fatalf("static config with empty gateway fails compilation: %v", compileErr)
	}
}
