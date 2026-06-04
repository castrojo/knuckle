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
	streamBaseURL  = "https://builds.coreos.fedoraproject.org/streams/"
	defaultTimeout = 30 * time.Second
	// maxStreamResponseSize caps stream metadata reads at 2MB to prevent unbounded allocation.
	maxStreamResponseSize = 2 << 20
)

// streamMetadata is the minimal shape of an FCOS stream JSON needed to extract
// the Fedora major version. The full spec is at:
// https://github.com/coreos/stream-metadata-go
type streamMetadata struct {
	Architectures map[string]struct {
		Artifacts map[string]struct {
			Release string `json:"release"`
		} `json:"artifacts"`
	} `json:"architectures"`
}

// FetchStreamFedoraVersion returns the Fedora major version number for a given
// FCOS stream (e.g. "stable" → 44). Needed to filter fedora-sysexts/community
// assets by the correct Fedora version.
//
// The version is parsed from the "metal" artifact release string (e.g.
// "44.20260510.3.1" → 44). The first dot-separated component is always the
// Fedora major version.
func FetchStreamFedoraVersion(ctx context.Context, stream string) (int, error) {
	return fetchStreamFedoraVersionWithClient(ctx, stream, &http.Client{Timeout: defaultTimeout})
}

func fetchStreamFedoraVersionWithClient(ctx context.Context, stream string, client *http.Client) (int, error) {
	url := streamBaseURL + stream + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating FCOS stream request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching FCOS stream %q: %w", stream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("FCOS stream %q returned HTTP %d", stream, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStreamResponseSize))
	if err != nil {
		return 0, fmt.Errorf("reading FCOS stream %q: %w", stream, err)
	}

	var meta streamMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return 0, fmt.Errorf("parsing FCOS stream %q JSON: %w", stream, err)
	}

	// Try x86_64 first, then aarch64 — we only need the Fedora major version
	// which is the same across architectures.
	for _, archKey := range []string{"x86_64", "aarch64"} {
		arch, ok := meta.Architectures[archKey]
		if !ok {
			continue
		}
		metal, ok := arch.Artifacts["metal"]
		if !ok {
			continue
		}
		if metal.Release == "" {
			continue
		}
		return parseFedoraVersion(metal.Release)
	}

	return 0, fmt.Errorf("FCOS stream %q: no architecture with metal artifact found", stream)
}

// parseFedoraVersion extracts the Fedora major version from an FCOS release
// string like "44.20260510.3.1".
func parseFedoraVersion(release string) (int, error) {
	parts := strings.SplitN(release, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("empty release string")
	}
	v, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parsing Fedora version from %q: %w", release, err)
	}
	if v < 1 {
		return 0, fmt.Errorf("invalid Fedora version %d from %q", v, release)
	}
	return v, nil
}
