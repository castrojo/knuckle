// Package github fetches SSH public keys from GitHub user profiles.
package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/projectbluefin/knuckle/internal/validate"
)

// KeyFetcher fetches SSH public keys for a user.
type KeyFetcher interface {
	FetchKeys(ctx context.Context, username string) ([]string, error)
}

// Client fetches SSH keys from GitHub's public API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient creates a GitHub client with a 10s timeout.
func NewClient() *Client {
	return &Client{
		BaseURL: "https://github.com",
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchKeys retrieves SSH public keys for a GitHub username.
// Returns the keys as a slice of strings (one per line from <BaseURL>/<user>.keys).
func (c *Client) FetchKeys(ctx context.Context, username string) ([]string, error) {
	if username == "" {
		return nil, fmt.Errorf("empty GitHub username")
	}
	if err := validate.GitHubUsername(username); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s.keys", c.BaseURL, username)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keys: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("GitHub user %q not found", username)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	// Limit read to 1MB to prevent abuse
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	const maxKeys = 50
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			keys = append(keys, line)
		}
		if len(keys) >= maxKeys {
			break
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("GitHub user %q has no public SSH keys", username)
	}

	return keys, nil
}

var defaultClient = NewClient()

// FetchKeys fetches SSH keys using the default client (backward compat).
func FetchKeys(username string) ([]string, error) {
	return defaultClient.FetchKeys(context.Background(), username)
}

// MockClient is a test double for KeyFetcher.
type MockClient struct {
	Keys map[string][]string
	Err  error
}

// FetchKeys returns pre-configured keys or error for testing.
func (m *MockClient) FetchKeys(_ context.Context, username string) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Keys[username], nil
}
