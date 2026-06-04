// Package fcos provides utilities for interacting with Fedora CoreOS stream
// metadata endpoints and deriving Fedora version information from them.
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
	// StreamMetaBaseURL is the base URL for FCOS stream metadata JSON files.
	StreamMetaBaseURL = "https://builds.coreos.fedoraproject.org/streams"

	defaultTimeout = 30 * time.Second
)

// streamHTTPClient is the shared HTTP client for FCOS stream metadata fetches.
var streamHTTPClient = &http.Client{Timeout: defaultTimeout}

// fcosStreamJSON is the minimal subset of the FCOS stream metadata JSON needed
// to extract the Fedora major version.  Full schema at:
// https://builds.coreos.fedoraproject.org/streams/stable.json
type fcosStreamJSON struct {
	Architectures map[string]fcosArch `json:"architectures"`
}

type fcosArch struct {
	Artifacts map[string]fcosArtifact `json:"artifacts"`
}

type fcosArtifact struct {
	// Release is the FCOS release string, e.g. "44.20260510.3.1".
	// The first component is the Fedora major version.
	Release string `json:"release"`
}

// FetchStreamFedoraVersion returns the Fedora major version number for a given
// FCOS stream (e.g. "stable" → 44).  Needed to filter fedora-sysexts/community
// assets by the correct Fedora version.
//
// The version is derived from the release string embedded in the stream metadata
// JSON (e.g. "44.20260510.3.1" → 44).
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	return fetchStreamFedoraVersion(ctx, StreamMetaBaseURL, stream, streamHTTPClient)
}

// fetchStreamFedoraVersion is the injectable implementation used by tests.
func fetchStreamFedoraVersion(ctx context.Context, baseURL, stream string, client *http.Client) (int, error) {
	if stream == "" {
		return 0, fmt.Errorf("FCOS stream name must not be empty")
	}

	url := baseURL + "/" + stream + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating FCOS stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching FCOS stream %q: %w", stream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("FCOS stream %q returned HTTP %d", stream, resp.StatusCode)
	}

	const maxSize = 5 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return 0, fmt.Errorf("reading FCOS stream response: %w", err)
	}

	return parseFedoraVersionFromStreamJSON(stream, body)
}

// parseFedoraVersionFromStreamJSON extracts the Fedora major version from the
// raw FCOS stream metadata JSON bytes.
func parseFedoraVersionFromStreamJSON(stream string, body []byte) (int, error) {
	var doc fcosStreamJSON
	if err := json.Unmarshal(body, &doc); err != nil {
		return 0, fmt.Errorf("parsing FCOS stream JSON for %q: %w", stream, err)
	}

	// Prefer x86_64; fall back to aarch64 if absent.  Both architectures
	// always carry the same Fedora major version, so the choice is cosmetic.
	for _, archKey := range []string{"x86_64", "aarch64"} {
		arch, ok := doc.Architectures[archKey]
		if !ok {
			continue
		}
		// Prefer the "metal" artifact; fall back to any artifact.
		if metal, ok := arch.Artifacts["metal"]; ok && metal.Release != "" {
			return parseFedoraVersionFromRelease(stream, metal.Release)
		}
		for _, art := range arch.Artifacts {
			if art.Release != "" {
				return parseFedoraVersionFromRelease(stream, art.Release)
			}
		}
	}

	return 0, fmt.Errorf("FCOS stream %q: no release version found in JSON", stream)
}

// parseFedoraVersionFromRelease extracts the Fedora major version integer from
// an FCOS release string such as "44.20260510.3.1".
func parseFedoraVersionFromRelease(stream, release string) (int, error) {
	dotIdx := strings.IndexByte(release, '.')
	var majorStr string
	if dotIdx < 0 {
		majorStr = release
	} else {
		majorStr = release[:dotIdx]
	}
	if majorStr == "" {
		return 0, fmt.Errorf("FCOS stream %q: unexpected release format %q", stream, release)
	}
	v, err := strconv.Atoi(majorStr)
	if err != nil {
		return 0, fmt.Errorf("FCOS stream %q: parsing Fedora version from %q: %w", stream, release, err)
	}
	return v, nil
}
