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

	// maxFCOSCatalogPages is higher than Flatcar's maxCatalogPages because
	// fedora-sysexts/community has 1,500+ releases. At 100/page, 20 pages
	// covers the full catalog. Extensions only in older releases (page >20)
	// would be silently missing — see the issue for discussion.
	maxFCOSCatalogPages = 20
)

// FCOSClient fetches the sysext catalog from fedora-sysexts/community and
// filters by architecture and Fedora major version.
type FCOSClient struct {
	CatalogURL    string
	HTTP          *http.Client
	AuthToken     string
	FedoraVersion int
}

// NewFCOSClient creates a client for the fedora-sysexts/community catalog.
// fedoraVersion is the Fedora major version obtained from FetchStreamFedoraVersion.
func NewFCOSClient(fedoraVersion int) *FCOSClient {
	return &FCOSClient{
		CatalogURL:    FCOSCatalogURL,
		HTTP:          &http.Client{Timeout: defaultTimeout},
		AuthToken:     githubTokenFromEnv(),
		FedoraVersion: fedoraVersion,
	}
}

func (c *FCOSClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return c.FetchCatalogArch(ctx, "amd64")
}

// FetchCatalogArch fetches FCOS sysexts for the given arch and the client's
// configured Fedora version. Only releases whose tag encodes a matching Fedora
// version and architecture are included. Entries are deduplicated by name
// (newest first). Download URLs point to GitHub releases (the CDN at
// extensions.fcos.fr is not used by the API).
func (c *FCOSClient) FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error) {
	var tagArch string
	switch arch {
	case "amd64":
		tagArch = "x86-64"
	case "arm64":
		tagArch = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}

	wantFedora := strconv.Itoa(c.FedoraVersion)

	const maxResponseSize = 5 << 20
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
			return nil, fmt.Errorf("FCOS catalog returned status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response (page %d): %w", page+1, err)
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
	sysexts := make([]model.SysextEntry, 0, len(allReleases))

	for _, rel := range allReleases {
		name, version, fedoraVer, parsedArch, ok := ParseFCOSTagName(rel.TagName)
		if !ok {
			continue
		}

		if fedoraVer != wantFedora || parsedArch != tagArch {
			continue
		}

		if seen[name] {
			continue
		}

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

// fetchSHA256ForAsset is identical to HTTPClient.fetchSHA256ForAsset — reused
// via composition since FCOSClient shares the same SHA256SUMS format.
func (c *FCOSClient) fetchSHA256ForAsset(ctx context.Context, sha256sumsURL, rawFilename string) (string, error) {
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

	const maxSHA256Size = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSHA256Size))
	if err != nil {
		return "", fmt.Errorf("reading SHA256SUMS: %w", err)
	}

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
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
	return "", nil
}

// Verify FCOSClient implements Client.
var _ Client = (*FCOSClient)(nil)
