package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestClient_FetchKeys_InvalidBaseURL covers the http.NewRequestWithContext
// error path (github.go:47-49) by using a base URL containing a null byte,
// which Go's http package rejects as an invalid URL.
func TestClient_FetchKeys_InvalidBaseURL(t *testing.T) {
	client := &Client{
		BaseURL: "http://\x00invalid",
		HTTP:    &http.Client{},
	}
	_, err := client.FetchKeys(context.Background(), "testuser")
	if err == nil {
		t.Fatal("expected error for invalid base URL, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create request") {
		t.Errorf("expected 'failed to create request' error, got: %v", err)
	}
}

// TestClient_FetchKeys_RateLimited verifies that a 429 response is treated as
// a non-200 error with the status code in the message.
func TestClient_FetchKeys_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := client.FetchKeys(context.Background(), "testuser")
	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected error to contain status 429, got: %v", err)
	}
}

// TestClient_FetchKeys_BodyReadError covers the io.ReadAll error path
// (github.go:66-68) by using a custom RoundTripper that returns an error
// on the body Read.
func TestClient_FetchKeys_BodyReadError(t *testing.T) {
	transport := &errorBodyTransport{}
	client := &Client{
		BaseURL: "http://example.test",
		HTTP:    &http.Client{Transport: transport},
	}
	_, err := client.FetchKeys(context.Background(), "testuser")
	if err == nil {
		t.Fatal("expected error for body read failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read response") {
		t.Errorf("expected 'failed to read response' error, got: %v", err)
	}
}

// errorBodyTransport returns a 200 response whose body always errors on Read.
type errorBodyTransport struct{}

func (e *errorBodyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &errorReader{},
		Header:     make(http.Header),
	}, nil
}

type errorReader struct{}

func (r *errorReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("simulated body read error")
}

func (r *errorReader) Close() error { return nil }


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

func TestClient_FetchKeys_NonOKNonNotFound(t *testing.T) {
	// Covers the resp.StatusCode != 200 && != 404 path — returns "GitHub returned status N".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := client.FetchKeys(context.Background(), "someuser")
	if err == nil {
		t.Fatal("expected error for 503 response, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status code 503, got: %v", err)
	}
}

func TestClient_FetchKeys_InvalidUsernameFormat(t *testing.T) {
	// validate.GitHubUsername rejects usernames with consecutive hyphens or other
	// invalid characters — exercises the return nil, err path after the empty check.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // would succeed if we got this far
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := client.FetchKeys(context.Background(), "--invalid--")
	if err == nil {
		t.Fatal("expected error for invalid username format, got nil")
	}
}

func TestClient_FetchKeys_CapAt50(t *testing.T) {
	// Serve 55 SSH keys — FetchKeys should cap at 50.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 55; i++ {
			_, _ = fmt.Fprintf(w, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 key%d\n", i)
		}
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	keys, err := client.FetchKeys(context.Background(), "biguser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 50 {
		t.Errorf("expected 50 keys (cap), got %d", len(keys))
	}
}
