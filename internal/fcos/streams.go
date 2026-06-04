// Package fcos provides helpers for querying Fedora CoreOS stream metadata.
package fcos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	streamBaseURL      = "https://builds.coreos.fedoraproject.org/streams/"
	defaultTimeout     = 30 * time.Second
	maxStreamBodyBytes = 2 << 20 // 2 MB — stream JSON is ~200 KB
)

// streamMetadata is the minimal structure of the FCOS stream JSON we need.
type streamMetadata struct {
	Architectures map[string]struct {
		Artifacts map[string]struct {
			Release struct {
				Version string `json:"version"`
			} `json:"release"`
		} `json:"artifacts"`
	} `json:"architectures"`
}

// FetchStreamFedoraVersion returns the Fedora major version number for a given
// FCOS stream (e.g. "stable" → 41). The version is parsed from the first
// component of the FCOS release version string (e.g. "41.20250223.3.0" → 41).
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	return FetchStreamFedoraVersionWithClient(ctx, stream, &http.Client{Timeout: defaultTimeout})
}

// FetchStreamFedoraVersionWithClient is like FetchStreamFedoraVersion but
// accepts a custom HTTP client (for testing).
func FetchStreamFedoraVersionWithClient(ctx context.Context, stream string, client *http.Client) (int, error) {
	if stream == "" {
		return 0, fmt.Errorf("stream name is required")
	}
	url := streamBaseURL + stream + ".json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching stream metadata for %q: %w", stream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("stream metadata for %q returned HTTP %d", stream, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStreamBodyBytes))
	if err != nil {
		return 0, fmt.Errorf("reading stream metadata: %w", err)
	}

	var meta streamMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return 0, fmt.Errorf("parsing stream metadata: %w", err)
	}

	version, err := extractVersion(meta)
	if err != nil {
		return 0, fmt.Errorf("extracting version from stream %q: %w", stream, err)
	}

	return parseFedoraMajor(version)
}

// extractVersion pulls the release version string from the first architecture
// entry's first artifact in the stream metadata.
func extractVersion(meta streamMetadata) (string, error) {
	for _, arch := range meta.Architectures {
		for _, artifact := range arch.Artifacts {
			if artifact.Release.Version != "" {
				return artifact.Release.Version, nil
			}
		}
	}
	return "", fmt.Errorf("no release version found in stream metadata")
}

// parseFedoraMajor extracts the Fedora major version from an FCOS version
// string like "41.20250223.3.0" → 41.
func parseFedoraMajor(version string) (int, error) {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("empty version string")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parsing Fedora major version from %q: %w", version, err)
	}
	return major, nil
}
