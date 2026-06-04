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
	// The community repo has 1500+ releases (one per extension × version × arch × Fedora version),
	// so 20 pages (up to 2000 releases) ensures all unique extension names are covered
	// given that the API returns newest releases first.
	maxFCOSCatalogPages = 20
)

// NewFCOSHTTPClient creates an HTTPClient pre-configured for the FCOS sysext catalog
// at fedora-sysexts/community.
func NewFCOSHTTPClient() *HTTPClient {
	c := NewHTTPClient()
	c.CatalogURL = FCOSCatalogURL
	return c
}

// ParseFCOSTagName parses a fedora-sysexts/community release tag into its components.
//
// Tag format: <name>-<version>-<fedoraVersion>-<arch>
// where arch is "x86-64" or "arm64" and fedoraVersion is all-digits (e.g. "44").
// "Index-only" tags (e.g. "tailscale", "latest") that lack an arch suffix return an error.
//
// Examples:
//
//	tailscale-0-1.98.3-1-44-x86-64     → name=tailscale, version=0-1.98.3-1, fedoraVersion=44, arch=x86-64
//	docker-ce-3-29.5.3-1.fc44-44-arm64 → name=docker-ce, version=3-29.5.3-1.fc44, fedoraVersion=44, arch=arm64
//	dnclient-0.9.4-44-x86-64           → name=dnclient, version=0.9.4, fedoraVersion=44, arch=x86-64
func ParseFCOSTagName(tag string) (name, version, fedoraVersion, arch string, err error) {
	// Step 1: strip known architecture suffix (must be the last component).
	for _, knownArch := range []string{"x86-64", "arm64"} {
		if strings.HasSuffix(tag, "-"+knownArch) {
			arch = knownArch
			tag = tag[:len(tag)-len("-"+knownArch)]
			break
		}
	}
	if arch == "" {
		return "", "", "", "", fmt.Errorf("no arch suffix in tag %q", tag)
	}

	// Step 2: strip the last segment — it must be an all-digit Fedora major version.
	lastDash := strings.LastIndex(tag, "-")
	if lastDash < 0 {
		return "", "", "", "", fmt.Errorf("missing fedora version segment in %q", tag)
	}
	fedoraVersion = tag[lastDash+1:]
	if fedoraVersion == "" {
		return "", "", "", "", fmt.Errorf("empty fedora version segment in %q", tag)
	}
	for _, ch := range fedoraVersion {
		if ch < '0' || ch > '9' {
			return "", "", "", "", fmt.Errorf("fedora version segment %q is not all-digits in %q", fedoraVersion, tag)
		}
	}
	tag = tag[:lastDash]

	// Step 3: find the first "-<digit>" boundary to split name from version.
	// This correctly handles names like "docker-ce" (hyphen but no leading digit).
	idx := -1
	for i := 0; i < len(tag)-1; i++ {
		if tag[i] == '-' && tag[i+1] >= '0' && tag[i+1] <= '9' {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Name-only tag (no version component) — e.g. name= "myext", version = "".
		name = tag
		if name == "" {
			return "", "", "", "", fmt.Errorf("empty name in tag")
		}
		return name, "", fedoraVersion, arch, nil
	}

	name = tag[:idx]
	version = tag[idx+1:]
	if name == "" {
		return "", "", "", "", fmt.Errorf("empty name in tag")
	}
	return name, version, fedoraVersion, arch, nil
}

// FetchCatalogFCOS fetches FCOS community sysexts for the given architecture and
// Fedora major version. arch must be "amd64" or "arm64".
// If fedoraVersion is 0, no Fedora version filtering is applied.
//
// The method pages through the GitHub Releases API (up to maxFCOSCatalogPages pages),
// deduplicates by name (keeping the newest release per extension), and applies curated
// metadata from fcos_descriptions.go when available.
func (c *HTTPClient) FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error) {
	// Map Go arch to the FCOS tag arch suffix.
	var tagArch string
	switch arch {
	case "amd64":
		tagArch = "x86-64"
	case "arm64":
		tagArch = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	const maxResponseSize = 5 << 20 // 5 MB per page
	var allReleases []githubRelease
	nextURL := c.CatalogURL

	for page := 0; page < maxFCOSCatalogPages && nextURL != ""; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request (page %d): %w", page+1, err)
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
			return nil, fmt.Errorf("reading FCOS catalog (page %d): %w", page+1, err)
		}
		if int64(len(body)) >= maxResponseSize {
			return nil, fmt.Errorf("FCOS catalog response exceeds 5MB size limit")
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
	}

	// Build deduplicated entries (newest-first per name).
	seen := make(map[string]bool)
	sysexts := make([]model.SysextEntry, 0)

	for _, rel := range allReleases {
		name, version, relFedoraVer, relArch, err := ParseFCOSTagName(rel.TagName)
		if err != nil {
			continue // skip index-only / unrecognised tags
		}
		if relArch != tagArch {
			continue
		}
		if fedoraVersion > 0 {
			fv, err := strconv.Atoi(relFedoraVer)
			if err != nil || fv != fedoraVersion {
				continue
			}
		}
		if seen[name] {
			continue // keep only the newest release per extension
		}

		// Find the .raw asset matching the exact arch+fedoraVersion suffix and SHA256SUMS.
		assetSuffix := "-" + relFedoraVer + "-" + relArch + ".raw"
		var downloadURL, sha256sumsURL string
		for _, asset := range rel.Assets {
			switch {
			case asset.Name == "SHA256SUMS":
				sha256sumsURL = asset.BrowserDownloadURL
			case strings.HasSuffix(asset.Name, assetSuffix):
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

		// Fetch SHA256 hash for the asset (best-effort; soft-fail).
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

		if meta, ok := LookupFCOS(name); ok {
			if meta.Short != "" {
				description = meta.Short
			}
			category = meta.Category
			supportTier = meta.SupportTier
		} else {
			category = "Community"
		}

		seen[name] = true
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
