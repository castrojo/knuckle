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
	// FCOSCatalogURL is the GitHub Releases API for the fedora-sysexts/community
	// sysext catalog.  This catalog is updated daily and covers both x86-64 and
	// arm64 builds for all supported Fedora major versions.
	FCOSCatalogURL = "https://api.github.com/repos/fedora-sysexts/community/releases?per_page=100"

	// fcosMaxCatalogPages caps the number of GitHub API pages fetched for the
	// FCOS catalog.  fedora-sysexts/community had 1,583+ releases as of 2025,
	// which fits in ≤16 pages at 100/page.  20 pages (2,000 releases) provides
	// comfortable headroom while preventing unbounded loops.
	//
	// Design note: we rely on the fact that the newest release per extension is
	// always in the first N pages (the API returns newest-first and extensions
	// are released frequently).  If the page cap is hit while a Link: next
	// header is still present, FetchCatalogFCOS returns an error rather than
	// silently returning a partial catalog.
	fcosMaxCatalogPages = 20
)

// fcosCDNNote: asset URLs in fedora-sysexts/community point to github.com,
// NOT the extensions.fcos.fr CDN mentioned in the issue.  Verified 2025-06-04:
// all browser_download_url values have the form:
//
//	https://github.com/fedora-sysexts/community/releases/download/<tag>/<file>
//
// The existing maxSysextURLLen (2048) and HTTPS enforcement in ignition.go
// handle these URLs without modification.

// FCOSClient wraps HTTPClient with a pinned fedoraVersion and implements the
// Client interface for fetching the fedora-sysexts/community catalog.
//
// The fedoraVersion is derived at startup from FetchStreamFedoraVersion and
// stored here so that FetchCatalogArch can filter assets for the correct Fedora
// release without requiring callers to thread the version through every call.
type FCOSClient struct {
	HTTP          *http.Client
	CatalogURL    string
	AuthToken     string
	FedoraVersion int
}

// NewFCOSClient creates an FCOSClient pointed at the fedora-sysexts/community
// catalog, filtering for the given Fedora major version.
func NewFCOSClient(fedoraVersion int) *FCOSClient {
	return &FCOSClient{
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
		CatalogURL:    FCOSCatalogURL,
		AuthToken:     githubTokenFromEnv(),
		FedoraVersion: fedoraVersion,
	}
}

// NewFCOSClientWithURL creates an FCOSClient pointing at a custom catalog URL
// (for testing).
func NewFCOSClientWithURL(url string, fedoraVersion int) *FCOSClient {
	return &FCOSClient{
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
		CatalogURL:    url,
		AuthToken:     githubTokenFromEnv(),
		FedoraVersion: fedoraVersion,
	}
}

// FetchCatalog implements Client.  Delegates to FetchCatalogArch with "amd64".
func (c *FCOSClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return c.FetchCatalogArch(ctx, "amd64")
}

// FetchCatalogArch implements Client.  Delegates to FetchCatalogFCOS with the
// stored FedoraVersion.
func (c *FCOSClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	httpClient := &HTTPClient{
		CatalogURL: c.CatalogURL,
		HTTP:       c.HTTP,
		AuthToken:  c.AuthToken,
	}
	return httpClient.FetchCatalogFCOS(ctx, arch, c.FedoraVersion)
}

// FetchCatalogFCOS fetches sysexts from the fedora-sysexts/community catalog
// built for the given arch and Fedora major version.
//
// arch must be "amd64" or "arm64".  fedoraVersion is an integer such as 44,
// obtained from FetchStreamFedoraVersion in internal/fcos.
//
// The catalog is paginated; this method follows Link headers up to
// fcosMaxCatalogPages pages.  If the cap is hit while more pages remain, an
// error is returned to avoid silently delivering a partial catalog.
//
// Entries are deduplicated by name; the newest release per extension wins.
// Curated metadata (description, category, support tier) is applied from
// fcos_descriptions.go when available.
func (c *HTTPClient) FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error) {
	var assetSuffix string
	switch arch {
	case "amd64":
		assetSuffix = "x86-64"
	case "arm64":
		assetSuffix = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	fedoraVersionStr := strconv.Itoa(fedoraVersion)

	const maxResponseSize = 5 << 20
	var allReleases []githubRelease
	nextURL := c.CatalogURL

	for page := 0; page < fcosMaxCatalogPages && nextURL != ""; page++ {
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
			return nil, fmt.Errorf("FCOS catalog returned status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading FCOS catalog response (page %d): %w", page+1, err)
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

		linkNext, hasNext := parseLinkNext(resp.Header.Get("Link"))
		nextURL = ""
		if hasNext {
			if page+1 >= fcosMaxCatalogPages {
				// Cap reached; refuse to return a silently truncated catalog.
				return nil, fmt.Errorf("FCOS catalog exceeds page cap (%d pages × 100): catalog may have grown beyond %d releases; raise fcosMaxCatalogPages",
					fcosMaxCatalogPages, fcosMaxCatalogPages*100)
			}
			nextURL = linkNext
		}
	}

	seen := make(map[string]bool)
	sysexts := make([]model.SysextEntry, 0, len(allReleases))

	for _, rel := range allReleases {
		name, version, tagFedoraVer, tagArch, err := ParseFCOSTagName(rel.TagName)
		if err != nil {
			continue // bare pointer tags and malformed tags are expected; skip silently
		}

		// Filter: only include assets built for the requested Fedora version and arch.
		if tagFedoraVer != fedoraVersionStr || tagArch != assetSuffix {
			continue
		}

		if seen[name] {
			continue // deduplicate — keep first (newest)
		}

		if validate.SysextName(name) != nil {
			continue
		}

		// Find the .raw asset and SHA256SUMS file.
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

		seen[name] = true

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
			Selected:    false,
		})
	}

	return sysexts, nil
}

// FCOSMockClient is a test double for FCOSClient that implements Client and
// exposes the FedoraVersion field for assertions in tests.
type FCOSMockClient struct {
	*MockClient
	FedoraVersion int
}

// compile-time assertion: FCOSClient implements Client.
var _ Client = (*FCOSClient)(nil)

// compile-time assertion: FCOSMockClient implements Client.
var _ Client = (*FCOSMockClient)(nil)
