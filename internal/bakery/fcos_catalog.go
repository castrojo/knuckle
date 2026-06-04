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

	// maxFCOSCatalogPages is raised to 20 for FCOS because fedora-sysexts/community
	// has 1,500+ releases. At 100/page, 20 pages covers the 2,000 newest releases.
	// Extensions that exist only in older releases (page >20) are silently omitted;
	// this is acceptable because the catalog releases frequently and the newest
	// release per extension is always within the first ~15 pages.
	maxFCOSCatalogPages = 20
)

// FCOSClient fetches the sysext catalog from fedora-sysexts/community.
type FCOSClient struct {
	*HTTPClient
	FedoraVersion int
}

// NewFCOSClient creates a new FCOS bakery client.
// fedoraVersion is the Fedora major version (e.g. 44) from FetchStreamFedoraVersion.
func NewFCOSClient(fedoraVersion int) *FCOSClient {
	client := NewHTTPClient()
	client.CatalogURL = FCOSCatalogURL
	return &FCOSClient{
		HTTPClient:    client,
		FedoraVersion: fedoraVersion,
	}
}

// NewFCOSClientWithURL creates an FCOS client pointing at a custom catalog URL (for testing).
func NewFCOSClientWithURL(url string, fedoraVersion int) *FCOSClient {
	client := NewHTTPClientWithURL(url)
	return &FCOSClient{
		HTTPClient:    client,
		FedoraVersion: fedoraVersion,
	}
}

// FetchCatalog returns sysexts for the default architecture (amd64).
func (c *FCOSClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return c.FetchCatalogArch(ctx, "amd64")
}

// FetchCatalogArch fetches sysexts built for the given arch and the client's Fedora major version.
//
// Asset download URLs point to github.com (not a CDN). The fedora-sysexts/community
// repo does not use a separate CDN — all assets are served via GitHub Releases.
func (c *FCOSClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	return c.FetchCatalogFCOS(ctx, arch, c.FedoraVersion)
}

// FetchCatalogFCOS fetches sysexts built for the given arch and Fedora major version.
// fedoraVersion is an integer (e.g. 44) obtained from fcos.FetchStreamFedoraVersion.
func (c *FCOSClient) FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error) {
	var archSuffix string
	switch arch {
	case "amd64":
		archSuffix = "x86-64"
	case "arm64":
		archSuffix = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	fedoraStr := strconv.Itoa(fedoraVersion)

	allReleases, err := c.fetchAllPages(ctx, maxFCOSCatalogPages)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var sysexts []model.SysextEntry

	for _, rel := range allReleases {
		name, version, tagFedoraVer, tagArch, parseErr := ParseFCOSTagName(rel.TagName)
		if parseErr != nil || name == "" {
			continue
		}

		// Filter by Fedora version and architecture.
		if tagFedoraVer != fedoraStr {
			continue
		}
		if tagArch != archSuffix {
			continue
		}

		if seen[name] {
			continue
		}

		// Find the .raw asset and SHA256SUMS.
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
		if validate.SysextName(name) != nil {
			continue
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

// fetchAllPages fetches up to maxPages of releases from the GitHub API.
func (c *FCOSClient) fetchAllPages(ctx context.Context, maxPages int) ([]githubRelease, error) {
	var allReleases []githubRelease
	nextURL := c.CatalogURL

	for page := 0; page < maxPages && nextURL != ""; page++ {
		releases, next, err := c.fetchOnePage(ctx, nextURL, page)
		if err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			break
		}
		allReleases = append(allReleases, releases...)
		nextURL = next
	}

	return allReleases, nil
}

// fetchOnePage fetches a single page and returns the next URL from Link headers.
func (c *FCOSClient) fetchOnePage(ctx context.Context, url string, page int) ([]githubRelease, string, error) {
	const maxResponseSize = 5 << 20

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request (page %d): %w", page+1, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "knuckle/1.0")
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching FCOS catalog (page %d): %w", page+1, err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("FCOS catalog returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	_ = resp.Body.Close()
	if err != nil {
		return nil, "", fmt.Errorf("reading FCOS catalog response (page %d): %w", page+1, err)
	}
	if int64(len(body)) >= maxResponseSize {
		return nil, "", fmt.Errorf("FCOS catalog response exceeds 5MB size limit")
	}

	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, "", fmt.Errorf("parsing FCOS catalog JSON (page %d): %w", page+1, err)
	}

	nextURL, _ := parseLinkNext(resp.Header.Get("Link"))
	return releases, nextURL, nil
}
