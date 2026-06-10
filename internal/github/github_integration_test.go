//go:build integration

package github

import (
	"testing"
)

// TestFetchKeys_RealUser makes a live HTTP call to github.com.
// Run with: go test -tags integration ./internal/github/...
func TestFetchKeys_RealUser(t *testing.T) {
	keys, err := FetchKeys("castrojo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("expected at least one key")
	}
	for _, k := range keys {
		if !hasValidPrefix(k) {
			t.Errorf("key doesn't look like SSH key: %s", k[:40])
		}
	}
}

// TestFetchKeys_InvalidUser makes a live HTTP call to github.com.
// Run with: go test -tags integration ./internal/github/...
func TestFetchKeys_InvalidUser(t *testing.T) {
	_, err := FetchKeys("this-user-definitely-does-not-exist-xyzzy-99999")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}
