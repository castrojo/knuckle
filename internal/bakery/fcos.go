package bakery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/validate"
)

const (
	// FCOSCatalogURL is the GitHub Releases API for the fedora-sysexts/community catalog.
	FCOSCatalogURL = "https://api.github.com/repos/fedora-sysexts/community/releases?per_page=100"

	// maxFCOSCatalogPages caps pagination for the FCOS sysext catalog.
	// fedora-sysexts/community has 1,583+ releases (as of 2026); at 100 per page that is
	// ~16 pages. A cap of 20 provides headroom for future growth.
	// If the cap is reached while a next page still exists, partial results are returned
	// and a slog warning is emitted (soft failure — partial catalog > no catalog).
	maxFCOSCatalogPages = 20
)

// NewFCOSHTTPClient creates a bakery client targeting the fedora-sysexts/community catalog.
func NewFCOSHTTPClient() *HTTPClient {
	return &HTTPClient{
		CatalogURL: FCOSCatalogURL,
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
		AuthToken: githubTokenFromEnv(),
	}
}

// ParseFCOSTagName parses a fedora-sysexts/community release tag into its components.
//
// Tag format: <name>-<version>-<fedoraVersion>-<arch>
//   - arch:          "x86-64" or "arm64"
//   - fedoraVersion: a pure integer (e.g. 43, 44)
//   - version:       may contain hyphens, digits, letters, and dots (RPM version/release)
//   - name:          alphanumeric with hyphens (e.g. "docker-ce", "1password-gui")
//
// The name/version split uses a left-to-right scan for the first "-<digit>" boundary
// after at least one character. This correctly handles names beginning with digits
// ("1password-gui") and names containing hyphens ("docker-ce", "cloud-hypervisor").
//
// Floating tags (e.g. "tailscale", "vscode", "latest") that carry no arch suffix are
// rejected — callers should skip those via the non-nil error return.
func ParseFCOSTagName(tag string) (name, version, fedoraVersion, arch string, err error) {
	// Step 1: identify and strip the arch suffix.
	// Note: "x86-64" contains a hyphen, so we must match it as a whole suffix string
	// rather than splitting on the last hyphen.
	var archSuffix string
	switch {
	case strings.HasSuffix(tag, "-arm64"):
		arch = "arm64"
		archSuffix = "-arm64"
	case strings.HasSuffix(tag, "-x86-64"):
		arch = "x86-64"
		archSuffix = "-x86-64"
	default:
		return "", "", "", "", fmt.Errorf("no recognized arch suffix in tag %q", tag)
	}
	remaining := tag[:len(tag)-len(archSuffix)]

	// Step 2: strip fedoraVersion — the last hyphen-delimited segment (pure integer).
	lastDash := strings.LastIndex(remaining, "-")
	if lastDash < 0 {
		return "", "", "", "", fmt.Errorf("no fedora version segment in tag %q", tag)
	}
	fedoraVersionStr := remaining[lastDash+1:]
	fver, parseErr := strconv.Atoi(fedoraVersionStr)
	if parseErr != nil || fver <= 0 {
		return "", "", "", "", fmt.Errorf("invalid fedora version %q in tag %q", fedoraVersionStr, tag)
	}
	fedoraVersion = fedoraVersionStr
	remaining = remaining[:lastDash]

	// Step 3: split name from version at the first "-<digit>" boundary (left-to-right).
	// Names must have at least one character before the split point.
	splitIdx := -1
	for i := 1; i < len(remaining); i++ {
		if remaining[i-1] == '-' && unicode.IsDigit(rune(remaining[i])) {
			splitIdx = i - 1
			break
		}
	}
	if splitIdx < 0 {
		return "", "", "", "", fmt.Errorf("cannot split name/version in tag %q", tag)
	}

	name = remaining[:splitIdx]
	version = remaining[splitIdx+1:]
	if name == "" || version == "" {
		return "", "", "", "", fmt.Errorf("empty name or version in tag %q", tag)
	}
	return name, version, fedoraVersion, arch, nil
}

// FetchCatalogFCOS fetches sysexts from fedora-sysexts/community for the given architecture
// and Fedora major version. fedoraVersion is an integer (e.g. 44) obtained from
// fcos.FetchStreamFedoraVersion. Only assets that match both arch and fedoraVersion are included.
//
// The catalog is paginated; up to maxFCOSCatalogPages pages are fetched. If the cap is reached
// while more pages exist, partial results are returned alongside a slog warning.
//
// Asset URLs are served from github.com/fedora-sysexts/community/releases/download/…
// (not from the extensions.fcos.fr CDN). This was confirmed against the live API on 2026-05.
func (c *HTTPClient) FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error) {
	// Map Go arch identifier to the suffix used in fedora-sysexts/community asset names.
	var archSuffix string
	switch arch {
	case "amd64":
		archSuffix = "x86-64"
	case "arm64":
		archSuffix = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	// Asset filename ends with "-<fedoraVersion>-<archSuffix>.raw".
	rawSuffix := fmt.Sprintf("-%d-%s.raw", fedoraVersion, archSuffix)

	const maxResponseSize = 5 << 20
	var allReleases []githubRelease
	nextURL := c.CatalogURL
	truncated := false

	for page := 0; page < maxFCOSCatalogPages && nextURL != ""; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating FCOS catalog request (page %d): %w", page+1, err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "knuckle/1.0")
		if c.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.AuthToken)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching FCOS catalog (page %d): %w", page+1, err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("FCOS catalog returned HTTP %d", resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading FCOS catalog response (page %d): %w", page+1, err)
		}
		if int64(len(body)) >= maxResponseSize {
			return nil, fmt.Errorf("FCOS catalog response page %d exceeds 5MiB size limit", page+1)
		}

		var releases []githubRelease
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, fmt.Errorf("parsing FCOS catalog JSON (page %d): %w", page+1, err)
		}
		if len(releases) == 0 {
			break
		}
		allReleases = append(allReleases, releases...)

		nextURL, _ = parseLinkNext(resp.Header.Get("Link"))
		if page+1 == maxFCOSCatalogPages && nextURL != "" {
			truncated = true
		}
	}

	if truncated {
		slog.Warn("FCOS catalog pagination cap reached; sysext list may be incomplete",
			"max_pages", maxFCOSCatalogPages)
	}

	// Filter by arch and fedoraVersion, then deduplicate by name (API returns newest first).
	seen := make(map[string]bool)
	sysexts := make([]model.SysextEntry, 0)

	for _, rel := range allReleases {
		name, version, relFedVStr, relArch, parseErr := ParseFCOSTagName(rel.TagName)
		if parseErr != nil {
			continue // floating tag or unparseable — skip
		}

		// Filter by arch.
		if relArch != archSuffix {
			continue
		}

		// Filter by Fedora version.
		relFedV, _ := strconv.Atoi(relFedVStr)
		if relFedV != fedoraVersion {
			continue
		}

		// Deduplicate by name — keep first (newest) since API returns newest-first.
		if seen[name] {
			continue
		}

		// Find the .raw asset and SHA256SUMS for this release.
		var downloadURL, sha256sumsURL string
		for _, asset := range rel.Assets {
			switch {
			case asset.Name == "SHA256SUMS":
				sha256sumsURL = asset.BrowserDownloadURL
			case strings.HasSuffix(asset.Name, rawSuffix):
				if downloadURL == "" {
					downloadURL = asset.BrowserDownloadURL
				}
			}
		}
		if downloadURL == "" {
			continue
		}
		if len(downloadURL) > maxSysextURLLen {
			continue
		}
		if sha256sumsURL != "" && len(sha256sumsURL) > maxSysextURLLen {
			sha256sumsURL = ""
		}
		if validate.SysextName(name) != nil {
			continue
		}

		seen[name] = true

		// Fetch SHA256 hash — hard-fail if present but unresolvable (avoid catalog entry with empty hash).
		sha256Hash := ""
		if sha256sumsURL != "" {
			rawFilename := downloadURL[strings.LastIndex(downloadURL, "/")+1:]
			h, hashErr := c.fetchSHA256ForAsset(ctx, sha256sumsURL, rawFilename)
			if hashErr != nil {
				// SHA256SUMS asset is present but could not be fetched/parsed; skip this entry
				// rather than producing one with an empty hash that would pass integrity checks.
				continue
			}
			sha256Hash = h
		}

		description := truncateDescription(rel.Body, 80)
		category := ""
		supportTier := ""

		if meta, ok := FCOSLookup(name); ok {
			if meta.Short != "" {
				description = meta.Short
			}
			category = meta.Category
			supportTier = meta.SupportTier
		}

		sysexts = append(sysexts, model.SysextEntry{
			Name:        name,
			Description: description,
			Version:     version,
			URL:         downloadURL,
			Sha256:      sha256Hash,
			Category:    category,
			SupportTier: supportTier,
		})
	}

	return sysexts, nil
}
