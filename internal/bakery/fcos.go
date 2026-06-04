package bakery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/validate"
)

const (
	// FCOSCatalogURL is the GitHub Releases API for the fedora-sysexts/community repo.
	FCOSCatalogURL = "https://api.github.com/repos/fedora-sysexts/community/releases?per_page=100"

	// maxFCOSCatalogPages caps the number of API pages fetched for the FCOS catalog.
	// fedora-sysexts/community has 1,583+ releases (16+ pages at 100/page). Since
	// releases are returned newest-first and extensions are deduplicated by name
	// (first/newest wins), 16 pages covers the full catalog. Extensions that only
	// exist in older releases beyond this cap will be silently missing.
	maxFCOSCatalogPages = 16
)

// FCOSClient fetches the sysext catalog from fedora-sysexts/community.
type FCOSClient struct {
	*HTTPClient
	FedoraVersion int
}

// NewFCOSClient creates a client for the fedora-sysexts/community catalog.
// fedoraVersion is the Fedora major version (e.g. 41, 44) obtained from
// fcos.FetchStreamFedoraVersion — used to filter assets by Fedora version.
func NewFCOSClient(fedoraVersion int) *FCOSClient {
	c := NewHTTPClientWithURL(FCOSCatalogURL)
	return &FCOSClient{
		HTTPClient:    c,
		FedoraVersion: fedoraVersion,
	}
}

// FetchCatalog fetches the FCOS catalog for amd64.
func (c *FCOSClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return c.FetchCatalogArch(ctx, "amd64")
}

// FetchCatalogArch fetches the FCOS sysext catalog filtered by architecture
// and the client's FedoraVersion.
//
// The fedora-sysexts/community tag format is:
//
//	<name>-<rpm-fields>-<fedoraVersion>-<arch>
//
// Only releases matching the target Fedora version and architecture are included.
// Entries are deduplicated by name (first/newest wins).
func (c *FCOSClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	var archSuffix string
	switch arch {
	case "amd64":
		archSuffix = "x86-64"
	case "arm64":
		archSuffix = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	var allReleases []githubRelease
	nextURL := c.CatalogURL

	for page := 0; page < maxFCOSCatalogPages && nextURL != ""; page++ {
		releases, next, err := c.fetchPage(ctx, nextURL)
		if err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			break
		}
		allReleases = append(allReleases, releases...)
		nextURL = next
	}

	seen := make(map[string]bool)
	sysexts := make([]model.SysextEntry, 0, len(allReleases))

	fedoraStr := strconv.Itoa(c.FedoraVersion)

	for _, rel := range allReleases {
		parsed, err := ParseFCOSTagName(rel.TagName)
		if err != nil {
			continue
		}

		if parsed.Arch != archSuffix {
			continue
		}
		if parsed.FedoraVersion != fedoraStr {
			continue
		}

		if seen[parsed.Name] {
			continue
		}

		// Find the .raw asset and SHA256SUMS
		var downloadURL, sha256sumsURL string
		for _, asset := range rel.Assets {
			switch {
			case asset.Name == "SHA256SUMS":
				sha256sumsURL = asset.BrowserDownloadURL
			case strings.HasSuffix(asset.Name, ".raw"):
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
		if validate.SysextName(parsed.Name) != nil {
			continue
		}

		seen[parsed.Name] = true

		sha256Hash := ""
		if sha256sumsURL != "" {
			rawFilename := downloadURL[strings.LastIndex(downloadURL, "/")+1:]
			if h, err := c.fetchSHA256ForAsset(ctx, sha256sumsURL, rawFilename); err == nil {
				sha256Hash = h
			}
		}

		description := truncateDescription(rel.Body, 80)
		category := ""
		supportTier := ""

		if meta, ok := FCOSLookup(parsed.Name); ok {
			if meta.Short != "" {
				description = meta.Short
			}
			category = meta.Category
			supportTier = meta.SupportTier
		}

		sysexts = append(sysexts, model.SysextEntry{
			Name:        parsed.Name,
			Description: description,
			Version:     parsed.Version,
			URL:         downloadURL,
			Sha256:      sha256Hash,
			Category:    category,
			SupportTier: supportTier,
			Selected:    false,
		})
	}

	return sysexts, nil
}

// fetchPage fetches a single page of releases and returns the next URL (if any).
func (c *FCOSClient) fetchPage(ctx context.Context, pageURL string) ([]githubRelease, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating FCOS catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "knuckle/1.0")
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching FCOS catalog: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("FCOS catalog returned status %d", resp.StatusCode)
	}

	const maxResponseSize = 5 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	_ = resp.Body.Close()
	if err != nil {
		return nil, "", fmt.Errorf("reading FCOS catalog response: %w", err)
	}
	if int64(len(body)) >= maxResponseSize {
		return nil, "", fmt.Errorf("FCOS catalog response exceeds 5MB size limit")
	}

	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, "", fmt.Errorf("parsing FCOS catalog JSON: %w", err)
	}

	next, _ := parseLinkNext(resp.Header.Get("Link"))
	return releases, next, nil
}

// FCOSTagParsed holds the components of a parsed fedora-sysexts/community tag.
type FCOSTagParsed struct {
	Name          string
	Version       string
	FedoraVersion string
	Arch          string
}

// ParseFCOSTagName parses a fedora-sysexts/community release tag.
//
// Tag format: <name-segments>-<version-segments>-<fedoraVersion>-<arch>
// where arch is "x86-64" or "arm64".
//
// Examples:
//
//	tailscale-0-1.98.3-1-44-x86-64     → {tailscale, 0-1.98.3-1, 44, x86-64}
//	docker-ce-3-29.4.0-1.fc43-43-x86-64 → {docker-ce, 3-29.4.0-1.fc43, 43, x86-64}
//	vscodium-1.121.03429-el8-44-arm64   → {vscodium, 1.121.03429-el8, 44, arm64}
//	1password-gui-8.12.6-1-43-x86-64   → {1password-gui, 8.12.6-1, 43, x86-64}
func ParseFCOSTagName(tag string) (FCOSTagParsed, error) {
	// Strip architecture suffix to determine arch and the remainder.
	var arch, remainder string
	if strings.HasSuffix(tag, "-x86-64") {
		arch = "x86-64"
		remainder = strings.TrimSuffix(tag, "-x86-64")
	} else if strings.HasSuffix(tag, "-arm64") {
		arch = "arm64"
		remainder = strings.TrimSuffix(tag, "-arm64")
	} else {
		return FCOSTagParsed{}, fmt.Errorf("tag %q has no recognized arch suffix", tag)
	}

	// The last segment of remainder is the Fedora version (a number like "43", "44").
	lastDash := strings.LastIndex(remainder, "-")
	if lastDash < 0 {
		return FCOSTagParsed{}, fmt.Errorf("tag %q has no Fedora version segment", tag)
	}
	fedoraVersion := remainder[lastDash+1:]
	if _, err := strconv.Atoi(fedoraVersion); err != nil {
		return FCOSTagParsed{}, fmt.Errorf("tag %q: Fedora version %q is not a number", tag, fedoraVersion)
	}
	remainder = remainder[:lastDash]

	// Now remainder is "<name-segments>-<version-segments>".
	// The name is everything before the first segment that starts with a digit.
	name, version := splitNameVersion(remainder)
	if name == "" {
		return FCOSTagParsed{}, fmt.Errorf("tag %q: could not extract name", tag)
	}

	return FCOSTagParsed{
		Name:          name,
		Version:       version,
		FedoraVersion: fedoraVersion,
		Arch:          arch,
	}, nil
}

// splitNameVersion splits "name-segments-version-segments" into name and version.
// The version starts at the first dash-separated segment that looks like a version
// number: starts with a digit and contains a dot (e.g. "8.12.6") or is a single
// digit that's followed by a dotted segment (epoch like "0" or "3").
// This avoids misidentifying names like "1password-gui" as starting with a version.
func splitNameVersion(s string) (name, version string) {
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if len(p) == 0 || p[0] < '0' || p[0] > '9' {
			continue
		}
		// Segment starts with digit. It's a version if:
		// 1. It contains a dot (e.g. "8.12.6", "1.98.3", "29.4.0")
		// 2. OR it's a pure number and the next segment contains a dot (epoch pattern: "0-1.98.3", "3-29.4.0")
		if strings.ContainsRune(p, '.') {
			return strings.Join(parts[:i], "-"), strings.Join(parts[i:], "-")
		}
		if i+1 < len(parts) && strings.ContainsRune(parts[i+1], '.') {
			return strings.Join(parts[:i], "-"), strings.Join(parts[i:], "-")
		}
	}
	return s, ""
}
