package github

import (
	"testing"
)

func TestFetchKeys_EmptyUsername(t *testing.T) {
	_, err := FetchKeys("")
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestFetchKeys_InvalidUser(t *testing.T) {
	_, err := FetchKeys("this-user-definitely-does-not-exist-xyzzy-99999")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestFetchKeys_RealUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}
	keys, err := FetchKeys("castrojo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("expected at least one key")
	}
	// Keys should start with ssh- or sk-
	for _, k := range keys {
		if !hasValidPrefix(k) {
			t.Errorf("key doesn't look like SSH key: %s", k[:40])
		}
	}
}

func hasValidPrefix(key string) bool {
	prefixes := []string{"ssh-rsa", "ssh-ed25519", "ssh-dss", "ecdsa-sha2",
		"sk-ssh-ed25519", "sk-ecdsa-sha2"}
	for _, p := range prefixes {
		if len(key) > len(p) && key[:len(p)] == p {
			return true
		}
	}
	return false
}
