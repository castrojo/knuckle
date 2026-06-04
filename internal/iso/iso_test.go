package iso

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateInstallerIgnition(t *testing.T) {
	t.Run("flatcar without SSH key", func(t *testing.T) {
		data, err := GenerateInstallerIgnition("flatcar", "")
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

		// Flatcar unit must NOT have tty1 conflict directives
		if strings.Contains(contents, "Conflicts=getty@tty1.service") {
			t.Error("flatcar unit must not contain Conflicts=getty@tty1.service")
		}

		// No passwd section when no SSH key
		if _, exists := raw["passwd"]; exists {
			t.Error("passwd should be omitted when no SSH key provided")
		}
	})

	t.Run("flatcar with SSH key", func(t *testing.T) {
		key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@example.com"
		data, err := GenerateInstallerIgnition("flatcar", key)
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

	t.Run("flatcar output is valid JSON", func(t *testing.T) {
		data, err := GenerateInstallerIgnition("flatcar", "ssh-rsa AAAA foo@bar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !json.Valid(data) {
			t.Error("output is not valid JSON")
		}
	})

	t.Run("empty os defaults to flatcar unit", func(t *testing.T) {
		data, err := GenerateInstallerIgnition("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		systemd := raw["systemd"].(map[string]any)
		units := systemd["units"].([]any)
		knuckleUnit := units[1].(map[string]any)
		contents := knuckleUnit["contents"].(string)
		if strings.Contains(contents, "Conflicts=getty@tty1.service") {
			t.Error("default (empty os) unit must not contain Conflicts=getty@tty1.service")
		}
	})

	t.Run("fcos unit has tty1 conflict directives", func(t *testing.T) {
		data, err := GenerateInstallerIgnition("fcos", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}

		systemd, ok := raw["systemd"].(map[string]any)
		if !ok {
			t.Fatal("missing systemd key")
		}
		units := systemd["units"].([]any)
		knuckleUnit := units[1].(map[string]any)
		contents, ok := knuckleUnit["contents"].(string)
		if !ok || contents == "" {
			t.Error("fcos knuckle unit has no contents")
		}
		if !strings.Contains(contents, "Conflicts=getty@tty1.service") {
			t.Error("fcos unit must contain Conflicts=getty@tty1.service")
		}
		if !strings.Contains(contents, "Before=getty@tty1.service") {
			t.Error("fcos unit must contain Before=getty@tty1.service")
		}
	})

	t.Run("fcos with SSH key", func(t *testing.T) {
		key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 fcos@test"
		data, err := GenerateInstallerIgnition("fcos", key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		passwd, ok := raw["passwd"].(map[string]any)
		if !ok {
			t.Fatal("missing passwd when SSH key provided for fcos")
		}
		users := passwd["users"].([]any)
		u := users[0].(map[string]any)
		keys := u["sshAuthorizedKeys"].([]any)
		if len(keys) != 1 || keys[0] != key {
			t.Errorf("sshAuthorizedKeys = %v, want [%s]", keys, key)
		}
	})

	t.Run("fcos output is valid JSON", func(t *testing.T) {
		data, err := GenerateInstallerIgnition("fcos", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !json.Valid(data) {
			t.Error("output is not valid JSON")
		}
	})
}
