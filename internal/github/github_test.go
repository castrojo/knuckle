package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	for _, k := range keys {
		if !hasValidPrefix(k) {
			t.Errorf("key doesn't look like SSH key: %s", k[:40])
		}
	}
}

func TestClient_FetchKeys_WithTestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/testuser.keys":
			_, _ = fmt.Fprintln(w, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 testuser@github")
		case "/nokeys.keys":
			_, _ = fmt.Fprintln(w, "")
		case "/gone.keys":
			w.WriteHeader(404)
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}

	t.Run("success", func(t *testing.T) {
		keys, err := client.FetchKeys(context.Background(), "testuser")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}
		if !hasValidPrefix(keys[0]) {
			t.Errorf("key doesn't look valid: %s", keys[0])
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := client.FetchKeys(context.Background(), "gone")
		if err == nil {
			t.Fatal("expected error for 404")
		}
	})

	t.Run("no keys", func(t *testing.T) {
		_, err := client.FetchKeys(context.Background(), "nokeys")
		if err == nil {
			t.Fatal("expected error for user with no keys")
		}
	})

	t.Run("empty username", func(t *testing.T) {
		_, err := client.FetchKeys(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty username")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.FetchKeys(ctx, "testuser")
		if err == nil {
			t.Fatal("expected error for cancelled context")
		}
	})
}

func TestMockClient(t *testing.T) {
	mock := &MockClient{
		Keys: map[string][]string{
			"alice": {"ssh-ed25519 AAAAC3 alice@test"},
		},
	}

	// Verify it satisfies the interface
	var _ KeyFetcher = mock

	keys, err := mock.FetchKeys(context.Background(), "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Test error path
	mock.Err = fmt.Errorf("network down")
	_, err = mock.FetchKeys(context.Background(), "alice")
	if err == nil {
		t.Fatal("expected error from mock")
	}
}

// Verify Client satisfies KeyFetcher at compile time.
var _ KeyFetcher = (*Client)(nil)

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
