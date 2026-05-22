package bakery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/validate"
)

var reSHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

const (
	// DefaultCatalogURL is the GitHub Releases API for the sysext-bakery repo.
	// The bakery spans multiple pages; FetchCatalogArch follows Link headers automatically.
	DefaultCatalogURL = "https://api.github.com/repos/flatcar/sysext-bakery/releases?per_page=100"
	defaultTimeout    = 30 * time.Second
	// maxCatalogPages caps the number of API pages fetched to prevent unbounded loops.
	maxCatalogPages = 10
)

// Client is the interface for fetching the sysext catalog
type Client interface {
	FetchCatalog(ctx context.Context) ([]model.SysextEntry, error)
	// FetchCatalogArch fetches the catalog and selects assets for the given arch.
	// arch must be "amd64" or "arm64". Falls back to x86-64 assets for amd64.
	FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error)
}

// HTTPClient fetches the catalog from the GitHub Releases API
type HTTPClient struct {
	CatalogURL string
	HTTP       *http.Client
}

// NewHTTPClient creates a new bakery HTTP client
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		CatalogURL: DefaultCatalogURL,
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// NewHTTPClientWithURL creates a client pointing at a custom catalog URL (for testing)
func NewHTTPClientWithURL(url string) *HTTPClient {
	return &HTTPClient{
		CatalogURL: url,
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// githubRelease represents a single release from the GitHub Releases API.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (c *HTTPClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return c.FetchCatalogArch(ctx, "amd64")
}

// FetchCatalogArch fetches the catalog and selects assets for the given arch.
// Flatcar Bakery asset naming: "<name>-<ver>-x86-64.raw" (amd64) and "<name>-<ver>-arm64.raw" (arm64).
//
// The GitHub Releases API is paginated — the bakery spans multiple pages. This method
// follows the Link: <url>; rel="next" response header until all pages are exhausted or
// maxCatalogPages is reached. Entries are deduplicated by name (first/newest wins).
// Curated metadata (description, category, support tier) is applied from descriptions.go.
func (c *HTTPClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	// Map Go arch name to the suffix used in Flatcar Bakery asset filenames.
	var assetSuffix string
	switch arch {
	case "amd64":
		assetSuffix = "x86-64"
	case "arm64":
		assetSuffix = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	// Fetch all pages, following Link headers.
	const maxResponseSize = 5 << 20
	var allReleases []githubRelease
	nextURL := c.CatalogURL

	for page := 0; page < maxCatalogPages && nextURL != ""; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request (page %d): %w", page+1, err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "knuckle/1.0")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching catalog (page %d): %w", page+1, err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("catalog returned status %d", resp.StatusCode)
		}

		// Limit each response body to 5MB.
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response (page %d): %w", page+1, err)
		}
		if int64(len(body)) >= maxResponseSize {
			return nil, fmt.Errorf("catalog response exceeds 5MB size limit")
		}

		var releases []githubRelease
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, fmt.Errorf("parsing catalog JSON (page %d): %w", page+1, err)
		}

		if len(releases) == 0 {
			break // empty page — done
		}
		allReleases = append(allReleases, releases...)

		// Follow Link: <url>; rel="next" header if present.
		nextURL, _ = parseLinkNext(resp.Header.Get("Link"))
	}

	// Build deduplicated entries from all pages.
	seen := make(map[string]bool)
	sysexts := make([]model.SysextEntry, 0, len(allReleases))

	for _, rel := range allReleases {
		name, version := ParseTagName(rel.TagName)
		if name == "" {
			continue
		}

		// Deduplicate by name — keep first (latest) since API returns newest first.
		if seen[name] {
			continue
		}

		// Find the asset matching the requested arch suffix AND the SHA256SUMS file.
		var downloadURL, sha256sumsURL string
		for _, asset := range rel.Assets {
			switch {
			case asset.Name == "SHA256SUMS":
				sha256sumsURL = asset.BrowserDownloadURL
			case strings.Contains(asset.Name, assetSuffix) && strings.HasSuffix(asset.Name, ".raw"):
				if downloadURL == "" {
					downloadURL = asset.BrowserDownloadURL
				}
			}
		}
		if downloadURL == "" {
			continue
		}
		if validate.SysextName(name) != nil {
			continue
		}

		seen[name] = true

		// Fetch the SHA256 hash for this asset (best-effort — soft fail on error).
		sha256Hash := ""
		if sha256sumsURL != "" {
			// Raw filename is the last path segment of the download URL.
			rawFilename := downloadURL[strings.LastIndex(downloadURL, "/")+1:]
			if h, err := c.fetchSHA256ForAsset(ctx, sha256sumsURL, rawFilename); err == nil {
				sha256Hash = h
			}
		}

		// Start with the GitHub release body (fallback for unknown extensions).
		description := truncateDescription(rel.Body, 80)
		category := ""
		supportTier := ""

		// Override with curated metadata when available.
		if meta, ok := Lookup(name); ok {
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
			Selected:    false,
		})
	}

	return sysexts, nil
}

// parseLinkNext extracts the "next" page URL from a GitHub API Link response header.
// Returns ("", false) when there is no next page.
// Example header: `<https://api.github.com/...?page=2>; rel="next", <...>; rel="last"`
func parseLinkNext(linkHeader string) (string, bool) {
	if linkHeader == "" {
		return "", false
	}
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		rawURL := strings.TrimSpace(segments[0])
		rawURL = strings.Trim(rawURL, "<>")
		for _, seg := range segments[1:] {
			if strings.TrimSpace(seg) == `rel="next"` {
				return rawURL, true
			}
		}
	}
	return "", false
}

// fetchSHA256ForAsset downloads the SHA256SUMS file from sha256sumsURL and returns
// the hex SHA256 hash for rawFilename. Returns ("", nil) when the file is found but
// the hash for rawFilename is not in it. Returns an error only on network/parse failure.
// Callers should treat ("", err) as a soft failure and proceed without verification.
func (c *HTTPClient) fetchSHA256ForAsset(ctx context.Context, sha256sumsURL, rawFilename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sha256sumsURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating SHA256SUMS request: %w", err)
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching SHA256SUMS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SHA256SUMS returned HTTP %d", resp.StatusCode)
	}

	// SHA256SUMS files are small (≤64KB).
	const maxSHA256Size = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSHA256Size))
	if err != nil {
		return "", fmt.Errorf("reading SHA256SUMS: %w", err)
	}

	// Format: "<hash>  <filename>" (two spaces) or "<hash> <filename>" (one space).
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// fields[1] may be "./filename" or just "filename".
		baseName := fields[1]
		if idx := strings.LastIndex(baseName, "/"); idx >= 0 {
			baseName = baseName[idx+1:]
		}
		if baseName == rawFilename {
			hash := fields[0]
			if !reSHA256.MatchString(hash) {
				return "", fmt.Errorf("malformed SHA256 hash %q for %s", hash, rawFilename)
			}
			return hash, nil
		}
	}
	return "", nil // file found but hash for this asset not listed
}

// ParseTagName extracts sysext name and version from a release tag.
// Formats: "<name>-v<version>" or "<name>-<version>"
func ParseTagName(tag string) (name, version string) {
	// Try splitting on "-v" followed by a digit (find last occurrence)
	for i := len(tag) - 1; i >= 1; i-- {
		if tag[i-1] == '-' && tag[i] == 'v' && i+1 < len(tag) && unicode.IsDigit(rune(tag[i+1])) {
			return tag[:i-1], tag[i+1:]
		}
	}

	// Fallback: find first segment that starts with a digit after a '-'
	for i := 1; i < len(tag); i++ {
		if tag[i-1] == '-' && unicode.IsDigit(rune(tag[i])) {
			return tag[:i-1], tag[i:]
		}
	}

	return "", ""
}

// truncateDescription trims and truncates a description to maxLen characters.
func truncateDescription(s string, maxLen int) string {
	// Take only first line
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen-3] + "..."
	}
	return s
}

// MockClient is a test double that returns preconfigured results
type MockClient struct {
	Entries []model.SysextEntry
	Err     error
}

func (m *MockClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return m.Entries, m.Err
}

// FetchCatalogArch returns entries whose URL contains the arch-specific suffix,
// mirroring the filtering that HTTPClient performs. If no entries have
// arch-specific URLs (e.g. test fixtures with plain URLs), all entries are returned.
func (m *MockClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	suffix := "x86-64"
	if arch == "arm64" {
		suffix = "arm64"
	}
	var filtered []model.SysextEntry
	for _, e := range m.Entries {
		if strings.Contains(e.URL, suffix) || (!strings.Contains(e.URL, "x86-64") && !strings.Contains(e.URL, "arm64")) {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 && len(m.Entries) > 0 {
		// No arch-specific URLs at all — return all entries (plain URL test fixtures)
		return m.Entries, nil
	}
	return filtered, nil
}
