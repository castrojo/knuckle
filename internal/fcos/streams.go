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
	// StreamBaseURL is the Fedora CoreOS stream metadata endpoint.
	// Exported so tests can construct a custom URL pattern with the test server address.
	StreamBaseURL = "https://builds.coreos.fedoraproject.org/streams/%s.json"

	// maxStreamBodySize caps the stream JSON response to prevent unbounded memory use.
	maxStreamBodySize = 2 << 20 // 2 MB
)

var streamHTTPClient = &http.Client{Timeout: 30 * time.Second}

// fcosStreamJSON captures the relevant parts of the FCOS stream metadata JSON.
// Full schema: https://builds.coreos.fedoraproject.org/streams/stable.json
type fcosStreamJSON struct {
	Architectures map[string]fcosArchJSON `json:"architectures"`
}

type fcosArchJSON struct {
	Artifacts map[string]fcosArtifactJSON `json:"artifacts"`
}

type fcosArtifactJSON struct {
	// Release is the full FCOS release string, e.g. "44.20260510.3.1".
	// The first component is the Fedora major version.
	Release string `json:"release"`
}

// FetchStreamFedoraVersion returns the Fedora major version number for a given
// FCOS stream (e.g. "stable" → 44). Needed to filter fedora-sysexts/community
// assets by the correct Fedora version.
//
// The Fedora major version is parsed from the first component of the release
// string returned by the FCOS stream metadata API, e.g. "44.20260510.3.1" → 44.
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	return FetchStreamFedoraVersionFromURL(ctx, stream, StreamBaseURL)
}

// FetchStreamFedoraVersionFromURL is the injectable version of FetchStreamFedoraVersion
// that accepts a printf-style URL pattern (with %s for the stream name).
// Used by tests to point at a local httptest server.
func FetchStreamFedoraVersionFromURL(ctx context.Context, stream, urlPattern string) (int, error) {
	url := fmt.Sprintf(urlPattern, stream)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating FCOS stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := streamHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching FCOS stream %q: %w", stream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("FCOS stream %q returned HTTP %d", stream, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStreamBodySize))
	if err != nil {
		return 0, fmt.Errorf("reading FCOS stream response: %w", err)
	}

	var data fcosStreamJSON
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, fmt.Errorf("parsing FCOS stream JSON: %w", err)
	}

	release := findRelease(data)
	if release == "" {
		return 0, fmt.Errorf("no release found in FCOS stream %q metadata", stream)
	}

	return parseReleaseMajor(release)
}

// findRelease searches architectures for any artifact release string.
// Prefers x86_64/metal as the most reliable bare-metal artifact.
func findRelease(data fcosStreamJSON) string {
	// Preferred order: x86_64 metal, x86_64 any, aarch64 metal, any.
	if arch, ok := data.Architectures["x86_64"]; ok {
		if art, ok := arch.Artifacts["metal"]; ok && art.Release != "" {
			return art.Release
		}
		for _, art := range arch.Artifacts {
			if art.Release != "" {
				return art.Release
			}
		}
	}
	for _, arch := range data.Architectures {
		for _, art := range arch.Artifacts {
			if art.Release != "" {
				return art.Release
			}
		}
	}
	return ""
}

// parseReleaseMajor extracts the Fedora major version from a release string
// like "44.20260510.3.1" → 44.
func parseReleaseMajor(release string) (int, error) {
	parts := strings.SplitN(release, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("unexpected release format %q", release)
	}
	v, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parsing major version from release %q: %w", release, err)
	}
	if v < 1 {
		return 0, fmt.Errorf("invalid major version %d in release %q", v, release)
	}
	return v, nil
}
