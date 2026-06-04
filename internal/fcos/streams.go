// Package fcos provides helpers for querying FCOS stream metadata.
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
	streamBaseURL = "https://builds.coreos.fedoraproject.org/streams"
	streamTimeout = 15 * time.Second
)

// streamJSON is the top-level structure of an FCOS stream JSON file.
// https://builds.coreos.fedoraproject.org/streams/<stream>.json
type streamJSON struct {
	Architectures map[string]archData `json:"architectures"`
}

type archData struct {
	Artifacts map[string]artifactData `json:"artifacts"`
}

type artifactData struct {
	Release string `json:"release"`
}

var httpClient = &http.Client{Timeout: streamTimeout}

// FetchStreamFedoraVersion fetches the FCOS stream JSON for the given stream name
// (e.g. "stable", "testing", "next") and returns the Fedora major version number
// embedded in the release string.
//
// The release string format is "<fedoraMajor>.<date>.<patch>.<build>" (e.g. "44.20260510.3.1"),
// so the Fedora major version is the first dot-separated component.
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	url := fmt.Sprintf("%s/%s.json", streamBaseURL, stream)
	return FetchStreamFedoraVersionFromURL(ctx, url)
}

// FetchStreamFedoraVersionFromURL is the URL-addressable variant of FetchStreamFedoraVersion.
// It is exported for testing with a local httptest server.
func FetchStreamFedoraVersionFromURL(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching stream JSON: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("stream returned HTTP %d", resp.StatusCode)
	}

	const maxSize = 2 << 20 // 2 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return 0, fmt.Errorf("reading response: %w", err)
	}

	var data streamJSON
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, fmt.Errorf("parsing stream JSON: %w", err)
	}

	// Iterate over any architecture / artifact pair to extract the release string.
	for _, arch := range data.Architectures {
		for _, artifact := range arch.Artifacts {
			if artifact.Release == "" {
				continue
			}
			majorStr, _, _ := strings.Cut(artifact.Release, ".")
			major, err := strconv.Atoi(majorStr)
			if err == nil && major > 0 {
				return major, nil
			}
		}
	}

	return 0, fmt.Errorf("no release version found in stream JSON")
}
