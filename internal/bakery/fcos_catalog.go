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
	// FCOSCatalogURL is the GitHub Releases API URL for the fedora-sysexts/community repo.
	// Each release corresponds to a single sysext build for one arch+Fedora version.
	FCOSCatalogURL = "https://api.github.com/repos/fedora-sysexts/community/releases?per_page=100"

	// fcosMaxCatalogPages caps pages fetched from the FCOS catalog.
	//
	// fedora-sysexts/community publishes 1,500+ releases (100 per page ≈ 16 pages as of
	// mid-2026). We use 20 pages (~2,000 releases) to leave headroom for growth.
	//
	// Design choice: we rely on the fact that the catalog releases extensions daily and
	// the newest build for each extension is always in the first ~16 pages. Extensions
	// that have not been updated in many months may appear only on later pages, but the
	// per-extension deduplication keeps only the newest version anyway, so fetching 20
	// pages reliably covers all actively maintained extensions.
	fcosMaxCatalogPages = 20
)

// FCOSClient is the interface for fetching the FCOS sysext catalog.
// It extends the base Client interface with an FCOS-specific method that
// accepts the Fedora major version for asset filtering.
type FCOSClient interface {
	Client
	// FetchCatalogFCOS fetches sysexts built for the given arch and Fedora major version.
	// fedoraVersion is an integer (e.g. 44) obtained from fcos.FetchStreamFedoraVersion.
	FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error)
}

// NewFCOSHTTPClient creates an HTTPClient pre-configured for the FCOS catalog.
func NewFCOSHTTPClient() *HTTPClient {
	return &HTTPClient{
		CatalogURL: FCOSCatalogURL,
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
		AuthToken: githubTokenFromEnv(),
	}
}

// FetchCatalogFCOS fetches sysexts built for the given arch and Fedora major version.
//
// Asset naming: "<name>-<epoch>-<ver>-<release>-<fedoraVersion>-<arch>.raw"
// Only releases whose parsed Fedora version matches fedoraVersion are included.
//
// fedora-sysexts/community publishes per-arch releases; each tag encodes both the
// arch and Fedora version, so filtering is done on the tag name rather than the
// asset filename.
//
// The GitHub Releases API is paginated. This method follows Link headers up to
// fcosMaxCatalogPages (20). Entries are deduplicated by name (newest wins).
// Curated metadata (description, category) is applied from fcos_descriptions.go.
//
// Download URLs from the GitHub Releases API are github.com URLs. The catalog's
// CDN (extensions.fcos.fr) is not used — the API returns github.com URLs only.
func (c *HTTPClient) FetchCatalogFCOS(ctx context.Context, arch string, fedoraVersion int) ([]model.SysextEntry, error) {
	// Map Go arch name to the suffix used in FCOS tag names.
	var tagArchSuffix string
	switch arch {
	case "amd64":
		tagArchSuffix = "x86-64"
	case "arm64":
		tagArchSuffix = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	if fedoraVersion < 1 {
		return nil, fmt.Errorf("invalid fedoraVersion %d: must be a positive integer", fedoraVersion)
	}
	wantFedoraVer := strconv.Itoa(fedoraVersion)

	const maxResponseSize = 5 << 20
	var allReleases []githubRelease
	nextURL := c.CatalogURL

	for page := 0; page < fcosMaxCatalogPages && nextURL != ""; page++ {
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
			return nil, fmt.Errorf("FCOS catalog returned status %d", resp.StatusCode)
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

	seen := make(map[string]bool)
	sysexts := make([]model.SysextEntry, 0)

	for _, rel := range allReleases {
		name, version, fedVer, tagArch, err := ParseFCOSTagName(rel.TagName)
		if err != nil {
			continue // alias/latest tags or malformed — skip silently
		}
		if tagArch != tagArchSuffix {
			continue
		}
		if fedVer != wantFedoraVer {
			continue
		}

		if seen[name] {
			continue // deduplicate — newest (first in API response) wins
		}

		// Validate the name early to avoid polluting the catalog with invalid entries.
		if validate.SysextName(name) != nil {
			continue
		}

		// Find the .raw asset.
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
			continue // no raw asset for this arch
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

		if meta, ok := LookupFCOS(name); ok {
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
