// Package fcos provides helpers for interacting with Fedora CoreOS metadata.
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

// StreamsBaseURL is the FCOS stream metadata endpoint.
// It is a variable (not a constant) to allow test injection of an httptest.Server URL.
var StreamsBaseURL = "https://builds.coreos.fedoraproject.org/streams"

const (
	// streamsMaxResponseSize caps the stream JSON response at 5 MiB.
	streamsMaxResponseSize = 5 << 20
)

var streamsHTTPClient = &http.Client{Timeout: 30 * time.Second}

type fcosStreamDoc struct {
	Architectures map[string]fcosArchDoc `json:"architectures"`
}

type fcosArchDoc struct {
	Artifacts map[string]fcosArtifact `json:"artifacts"`
}

type fcosArtifact struct {
	Release string `json:"release"`
}

// FetchStreamFedoraVersion returns the Fedora major version number for an FCOS stream.
// For example, "stable" → 44 when the stable stream ships Fedora 44 packages.
// The version is parsed from the first component of any artifacts[*].release field
// (format: "44.20260510.3.1"). All artifacts within a stream share the same major version.
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	switch stream {
	case "stable", "testing", "next":
	default:
		return 0, fmt.Errorf("unsupported FCOS stream %q: must be stable, testing, or next", stream)
	}

	url := fmt.Sprintf("%s/%s.json", StreamsBaseURL, stream)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating FCOS stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := streamsHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching FCOS stream %s: %w", stream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("FCOS stream %s returned HTTP %d", stream, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, streamsMaxResponseSize))
	if err != nil {
		return 0, fmt.Errorf("reading FCOS stream response: %w", err)
	}

	return parseFCOSStreamVersion(body)
}

// parseFCOSStreamVersion extracts the Fedora major version from FCOS stream JSON.
func parseFCOSStreamVersion(body []byte) (int, error) {
	var doc fcosStreamDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return 0, fmt.Errorf("parsing FCOS stream JSON: %w", err)
	}

	// All artifacts within a stream share the same Fedora major version.
	// Return the first valid one found.
	for _, arch := range doc.Architectures {
		for _, artifact := range arch.Artifacts {
			if artifact.Release == "" {
				continue
			}
			// Release format: "44.20260510.3.1" — the major version is the first component.
			parts := strings.SplitN(artifact.Release, ".", 2)
			ver, err := strconv.Atoi(parts[0])
			if err != nil || ver <= 0 {
				continue
			}
			return ver, nil
		}
	}

	return 0, fmt.Errorf("no Fedora version found in FCOS stream JSON")
}
