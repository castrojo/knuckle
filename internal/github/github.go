// Package github fetches SSH public keys from GitHub user profiles.
package github

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchKeys retrieves SSH public keys for a GitHub username.
// Returns the keys as a slice of strings (one per line from https://github.com/<user>.keys).
func FetchKeys(username string) ([]string, error) {
	if username == "" {
		return nil, fmt.Errorf("empty GitHub username")
	}

	url := fmt.Sprintf("https://github.com/%s.keys", username)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keys: %w", err)
	}
	defer resp.Body.Close()

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

	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			keys = append(keys, line)
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("GitHub user %q has no public SSH keys", username)
	}

	return keys, nil
}
