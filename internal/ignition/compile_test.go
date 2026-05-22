package ignition

import (
	"strings"
	"testing"
)

func TestCompileToIgnition_ValidButane(t *testing.T) {
	butane := `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      mode: 0644
      overwrite: true
      contents:
        inline: test-node
`
	got, err := CompileToIgnition(butane)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty Ignition JSON")
	}
	if !strings.Contains(got, `"ignition"`) {
		t.Errorf("output missing ignition key: %s", got[:min(100, len(got))])
	}
	if !strings.Contains(got, `"version"`) {
		t.Errorf("output missing version field: %s", got[:min(100, len(got))])
	}
}

func TestCompileToIgnition_InvalidYAML(t *testing.T) {
	_, err := CompileToIgnition("not: valid: butane: {{{")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestCompileToIgnition_MissingVariant(t *testing.T) {
	// YAML without variant field should fail
	_, err := CompileToIgnition(`storage:
  files:
    - path: /etc/hostname
      contents:
        inline: test
`)
	if err == nil {
		t.Fatal("expected error for missing variant")
	}
}

func TestCompileToIgnition_UnsupportedVariant(t *testing.T) {
	_, err := CompileToIgnition(`variant: nonexistent
version: 1.5.0
storage:
  files:
    - path: /etc/hostname
      contents:
        inline: test
`)
	if err == nil {
		t.Fatal("expected error for unsupported variant")
	}
}

func TestCompileToIgnition_FullConfig(t *testing.T) {
	// Test with a config similar to what knuckle generates
	butane := `variant: flatcar
version: 1.1.0
storage:
  files:
    - path: /etc/hostname
      mode: 0644
      overwrite: true
      contents:
        inline: flatcar-node01
    - path: /etc/flatcar/update.conf
      mode: 0644
      overwrite: true
      contents:
        inline: |
          GROUP=stable
          REBOOT_STRATEGY=reboot
passwd:
  users:
    - name: core
      ssh_authorized_keys:
        - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@test
`
	got, err := CompileToIgnition(butane)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "flatcar-node01") {
		t.Error("Ignition JSON missing hostname")
	}
	if !strings.Contains(got, "ssh-ed25519") {
		t.Error("Ignition JSON missing SSH key")
	}
}
