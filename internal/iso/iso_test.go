package iso

import (
	"encoding/json"
	"testing"
)

func TestGenerateInstallerIgnition(t *testing.T) {
	t.Run("without SSH key", func(t *testing.T) {
		data, err := GenerateInstallerIgnition("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		// Check Ignition version
		ign, ok := raw["ignition"].(map[string]any)
		if !ok {
			t.Fatal("missing ignition key")
		}
		if v := ign["version"]; v != "3.3.0" {
			t.Errorf("version = %q, want %q", v, "3.3.0")
		}

		// Check systemd units exist
		systemd, ok := raw["systemd"].(map[string]any)
		if !ok {
			t.Fatal("missing systemd key")
		}
		units, ok := systemd["units"].([]any)
		if !ok || len(units) != 2 {
			t.Fatalf("expected 2 units, got %v", units)
		}

		// Verify knuckle unit
		knuckleUnit := units[1].(map[string]any)
		if knuckleUnit["name"] != "knuckle-installer.service" {
			t.Errorf("unit name = %q, want knuckle-installer.service", knuckleUnit["name"])
		}
		contents, ok := knuckleUnit["contents"].(string)
		if !ok || contents == "" {
			t.Error("knuckle unit has no contents")
		}

		// No passwd section when no SSH key
		if _, exists := raw["passwd"]; exists {
			t.Error("passwd should be omitted when no SSH key provided")
		}
	})

	t.Run("with SSH key", func(t *testing.T) {
		key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAATEST test@example.com"
		data, err := GenerateInstallerIgnition(key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		passwd, ok := raw["passwd"].(map[string]any)
		if !ok {
			t.Fatal("missing passwd when SSH key provided")
		}
		users, ok := passwd["users"].([]any)
		if !ok || len(users) != 1 {
			t.Fatal("expected 1 user")
		}
		u := users[0].(map[string]any)
		if u["name"] != "core" {
			t.Errorf("user name = %q, want core", u["name"])
		}
		keys := u["sshAuthorizedKeys"].([]any)
		if len(keys) != 1 || keys[0] != key {
			t.Errorf("sshAuthorizedKeys = %v, want [%s]", keys, key)
		}
	})

	t.Run("output is valid JSON", func(t *testing.T) {
		data, err := GenerateInstallerIgnition("ssh-rsa AAAA foo@bar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !json.Valid(data) {
			t.Error("output is not valid JSON")
		}
	})
}
