// Package fcos provides utilities for Fedora CoreOS stream metadata.
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
	streamBaseURL    = "https://builds.coreos.fedoraproject.org/streams/"
	streamTimeout    = 15 * time.Second
	maxStreamRespLen = 1 << 20 // 1 MiB — stream JSON is ~50 KiB
)

// streamMetadata is the minimal subset of the FCOS stream JSON we need.
type streamMetadata struct {
	Architectures map[string]struct {
		Artifacts map[string]struct {
			Release string `json:"release"`
		} `json:"artifacts"`
	} `json:"architectures"`
}

// httpClient is overridable for tests.
var httpClient = &http.Client{Timeout: streamTimeout}

// FetchStreamFedoraVersion returns the Fedora major version number for a given
// FCOS stream (e.g. "stable" → 44). The version is parsed from the stream's
// release string (e.g. "44.20260510.3.1" → 44).
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	return fetchStreamFromURL(ctx, streamBaseURL+stream+".json")
}

func fetchStreamFromURL(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching FCOS stream %q: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("FCOS stream returned HTTP %d for %q", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStreamRespLen))
	if err != nil {
		return 0, fmt.Errorf("reading stream metadata: %w", err)
	}

	var meta streamMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return 0, fmt.Errorf("parsing stream JSON: %w", err)
	}

	return extractFedoraVersion(meta)
}

// extractFedoraVersion pulls the Fedora major version from any architecture's
// metal artifact release string. The release string format is
// "<fedora-major>.<date>.<minor>.<patch>" (e.g. "44.20260510.3.1").
func extractFedoraVersion(meta streamMetadata) (int, error) {
	for _, archData := range meta.Architectures {
		for _, artifact := range archData.Artifacts {
			if artifact.Release == "" {
				continue
			}
			parts := strings.SplitN(artifact.Release, ".", 2)
			v, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			return v, nil
		}
	}
	return 0, fmt.Errorf("no release version found in stream metadata")
}

// FetchStreamVersion returns the full release version string for a given FCOS
// stream (e.g. "stable" → "44.20260510.3.1").
func FetchStreamVersion(ctx context.Context, stream string) (string, error) {
	url := streamBaseURL + stream + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching FCOS stream %q: %w", stream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("FCOS stream %q returned HTTP %d", stream, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStreamRespLen))
	if err != nil {
		return "", fmt.Errorf("reading stream metadata: %w", err)
	}

	var meta streamMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", fmt.Errorf("parsing stream JSON: %w", err)
	}

	for _, archData := range meta.Architectures {
		for _, artifact := range archData.Artifacts {
			if artifact.Release != "" {
				return artifact.Release, nil
			}
		}
	}
	return "", fmt.Errorf("no release version found in stream metadata")
}
