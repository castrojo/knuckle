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
	maxStreamRespBytes = 2 << 20
)

var streamHTTPClient = &http.Client{Timeout: 30 * time.Second}

// streamMetadata mirrors the subset of the FCOS stream JSON we need.
type streamMetadata struct {
	Architectures map[string]struct {
		Artifacts map[string]struct {
			Release string `json:"release"`
		} `json:"artifacts"`
	} `json:"architectures"`
}

// FetchStreamFedoraVersion returns the Fedora major version number for a given
// FCOS stream (e.g. "stable" → 44). The version is extracted from the stream
// metadata at builds.coreos.fedoraproject.org.
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	return fetchStreamFedoraVersionFromURL(ctx, streamBaseURL+stream+".json")
}

func fetchStreamFedoraVersionFromURL(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := streamHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching FCOS stream metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("FCOS stream metadata returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStreamRespBytes))
	if err != nil {
		return 0, fmt.Errorf("reading FCOS stream metadata: %w", err)
	}

	var meta streamMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return 0, fmt.Errorf("parsing FCOS stream JSON: %w", err)
	}

	return extractFedoraVersion(meta)
}

func extractFedoraVersion(meta streamMetadata) (int, error) {
	arch, ok := meta.Architectures["x86_64"]
	if !ok {
		return 0, fmt.Errorf("x86_64 architecture not found in stream metadata")
	}
	metal, ok := arch.Artifacts["metal"]
	if !ok {
		return 0, fmt.Errorf("metal artifact not found in stream metadata")
	}

	release := metal.Release
	if release == "" {
		return 0, fmt.Errorf("empty release version in stream metadata")
	}

	// Release format: "44.20260510.3.1" — first component is the Fedora major version.
	dot := strings.IndexByte(release, '.')
	if dot <= 0 {
		return 0, fmt.Errorf("unexpected release version format %q", release)
	}

	major, err := strconv.Atoi(release[:dot])
	if err != nil {
		return 0, fmt.Errorf("parsing Fedora major version from %q: %w", release, err)
	}

	return major, nil
}
