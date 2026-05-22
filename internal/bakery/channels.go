package bakery

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// channelHTTPClient is the shared HTTP client used for fetching Flatcar channel information,
// enabling connection reuse across parallel requests.
var channelHTTPClient = &http.Client{Timeout: 30 * time.Second}

// ChannelInfo holds version details for a Flatcar release channel.
type ChannelInfo struct {
	Channel    string // stable, beta, alpha
	Version    string // e.g. "4593.2.0"
	BuildDate  string // e.g. "2026-04-14"
	Kernel     string // e.g. "6.12.81"
	Systemd    string // e.g. "257.9"
	Docker     string // e.g. "28.0.4"
	Containerd string // e.g. "2.1.5"
	Ignition   string // e.g. "2.24.0"
	Etcd       string // e.g. "3.5.18"

	// Verification status
	SBOMVerified   bool // SBOM JSON was successfully parsed
	DigestVerified bool // SHA512 digest matched
	SignedDigest   bool // GPG-signed digest file was present
}

// FetchChannelInfo fetches version info for a given channel from the Flatcar release server.
// It targets the amd64 architecture. Use FetchChannelInfoArch for other architectures.
func FetchChannelInfo(ctx context.Context, channel string) (*ChannelInfo, error) {
	return FetchChannelInfoArch(ctx, channel, "amd64")
}

// FetchChannelInfoArch fetches version info for a given channel and architecture.
// arch must be "amd64" or "arm64". LTS is only available for amd64.
func FetchChannelInfoArch(ctx context.Context, channel, arch string) (*ChannelInfo, error) {
	if arch != "amd64" && arch != "arm64" {
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}
	if channel == "lts" && arch == "arm64" {
		return nil, fmt.Errorf("LTS channel is not available for arm64")
	}
	// Flatcar release server uses "arm64-usr" for ARM64 and "amd64-usr" for x86-64.
	archDir := arch + "-usr"
	versionURL := fmt.Sprintf("https://%s.release.flatcar-linux.net/%s/current/version.txt", channel, archDir)
	pkgURL := fmt.Sprintf("https://%s.release.flatcar-linux.net/%s/current/flatcar_production_image_packages.txt", channel, archDir)
	return fetchChannelInfoFromURLs(ctx, channel, versionURL, pkgURL)
}

// fetchChannelInfoFromURLs is the internal implementation that accepts URLs (for testing).
func fetchChannelInfoFromURLs(ctx context.Context, channel, versionURL, pkgURL string) (*ChannelInfo, error) {
	info := &ChannelInfo{Channel: channel}

	// Fetch version.txt for FLATCAR_VERSION (lightweight, always available)
	versionBody, err := httpGet(ctx, versionURL)
	if err != nil {
		return nil, fmt.Errorf("fetching version.txt for %s: %w", channel, err)
	}
	parseVersionTxt(versionBody, info)

	// Fetch SBOM JSON (authoritative structured source for package versions)
	sbomURL := strings.Replace(pkgURL, "flatcar_production_image_packages.txt", "flatcar_production_image_sbom.json", 1)
	sbomUsed := false
	var sbomBody string
	if sbomURL != pkgURL { // only try SBOM if URL was actually different
		if body, err := httpGet(ctx, sbomURL); err == nil {
			sbomBody = body
			parseSBOMJSON(body, info)
			if info.Kernel != "" {
				sbomUsed = true
				info.SBOMVerified = true
			}
		}
	}
	if !sbomUsed {
		// Fallback to text package list
		pkgBody, err := httpGet(ctx, pkgURL)
		if err != nil {
			return nil, fmt.Errorf("fetching package list for %s: %w", channel, err)
		}
		parsePackageList(pkgBody, info)
	}

	// Verify SBOM digest and GPG signature if available
	if sbomUsed && sbomBody != "" {
		digestURL := sbomURL + ".DIGESTS"
		if digestBody, err := httpGet(ctx, digestURL); err == nil {
			info.DigestVerified = verifySHA512(sbomBody, digestBody)
			// Cryptographic GPG verification against the embedded Flatcar key.
			ascURL := digestURL + ".asc"
			if ascBody, err := httpGet(ctx, ascURL); err == nil {
				if ok := verifyFlatcarSignature(ascBody); ok {
					info.SignedDigest = true
				}
			}
		}
	}

	// Fetch docker sysext package list (no JSON SBOM available for sysexts)
	dockerPkgURL := strings.Replace(pkgURL, "flatcar_production_image_packages.txt", "rootfs-included-sysexts/docker-flatcar_packages.txt", 1)
	if body, err := httpGet(ctx, dockerPkgURL); err == nil {
		parseSysextPackageList(body, info)
	}

	// Fetch containerd sysext package list
	containerdPkgURL := strings.Replace(pkgURL, "flatcar_production_image_packages.txt", "rootfs-included-sysexts/containerd-flatcar_packages.txt", 1)
	if body, err := httpGet(ctx, containerdPkgURL); err == nil {
		parseSysextPackageList(body, info)
	}

	return info, nil
}

// FetchAllChannels fetches info for stable, beta, alpha, lts in parallel for amd64.
// Use FetchAllChannelsArch to target a specific architecture.
func FetchAllChannels(ctx context.Context) ([]ChannelInfo, error) {
	return FetchAllChannelsArch(ctx, "amd64")
}

// FetchAllChannelsArch fetches info for all channels available on the given arch.
// LTS is excluded for arm64 (not published by Flatcar).
func FetchAllChannelsArch(ctx context.Context, arch string) ([]ChannelInfo, error) {
	if arch != "amd64" && arch != "arm64" {
		return nil, fmt.Errorf("unsupported architecture %q: must be amd64 or arm64", arch)
	}
	channels := []string{"stable", "beta", "alpha"}
	if arch == "amd64" {
		channels = append(channels, "lts")
	}
	archDir := arch + "-usr"
	return fetchAllChannelsWithURLFn(ctx, func(channel string) (string, string) {
		base := fmt.Sprintf("https://%s.release.flatcar-linux.net/%s/current", channel, archDir)
		return base + "/version.txt", base + "/flatcar_production_image_packages.txt"
	}, channels...)
}

// fetchAllChannelsWithURLFn is a helper for parallel channel fetching with custom URLs.
// Channels that fail are skipped and a combined error is returned alongside the
// partial results so callers can choose to proceed with what succeeded.
func fetchAllChannelsWithURLFn(ctx context.Context, urlFn func(string) (string, string), channels ...string) ([]ChannelInfo, error) {
	if len(channels) == 0 {
		channels = []string{"stable", "beta", "alpha", "lts"}
	}
	results := make([]ChannelInfo, len(channels))
	errs := make([]error, len(channels))

	var wg sync.WaitGroup
	for i, ch := range channels {
		wg.Add(1)
		go func(idx int, channel string) {
			defer wg.Done()
			vURL, pURL := urlFn(channel)
			info, err := fetchChannelInfoFromURLs(ctx, channel, vURL, pURL)
			if err != nil {
				errs[idx] = fmt.Errorf("%s: %w", channel, err)
				return
			}
			results[idx] = *info
		}(i, ch)
	}
	wg.Wait()

	// Collect successful results and accumulate per-channel errors.
	var out []ChannelInfo
	var firstErr error
	for i, err := range errs {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if results[i].Channel != "" {
			out = append(out, results[i])
		}
	}
	if firstErr != nil && len(out) == 0 {
		return nil, firstErr
	}

	return out, firstErr
}

// httpGet performs a GET request with the shared client and returns the body as a string.
func httpGet(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := channelHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	const maxSize = 5 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// parseVersionTxt parses key=value lines from version.txt into the ChannelInfo.
func parseVersionTxt(body string, info *ChannelInfo) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], "\"")

		switch key {
		case "FLATCAR_VERSION":
			info.Version = val
		case "FLATCAR_BUILD_ID":
			// Format: "2026-04-14-0806" — extract date portion
			if len(val) >= 10 {
				info.BuildDate = val[:10]
			} else {
				info.BuildDate = val
			}
		}
	}
}

// spdxDocument represents the relevant parts of an SPDX SBOM JSON.
type spdxDocument struct {
	Packages []spdxPackage `json:"packages"`
}

type spdxPackage struct {
	Name        string `json:"name"`
	VersionInfo string `json:"versionInfo"`
}

// parseSBOMJSON extracts package versions from the Flatcar SPDX SBOM JSON.
// This is the authoritative structured source — preferred over text package lists.
func parseSBOMJSON(body string, info *ChannelInfo) {
	var doc spdxDocument
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return // silently fall back to text parsing
	}

	for _, pkg := range doc.Packages {
		switch pkg.Name {
		case "sys-kernel/coreos-kernel":
			info.Kernel = pkg.VersionInfo
		case "sys-apps/systemd":
			info.Systemd = pkg.VersionInfo
		case "sys-apps/ignition":
			info.Ignition = pkg.VersionInfo
			// Strip -rN revision suffix
			if idx := strings.Index(info.Ignition, "-r"); idx >= 0 {
				info.Ignition = info.Ignition[:idx]
			}
		case "dev-db/etcd":
			info.Etcd = pkg.VersionInfo
		}
	}
}

// parsePackageList extracts kernel, systemd, ignition, and etcd versions from the base package list.
// Fallback when SBOM JSON is unavailable.
func parsePackageList(body string, info *ChannelInfo) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "sys-kernel/coreos-kernel-") {
			info.Kernel = extractVersionBeforeColons(line, "sys-kernel/coreos-kernel-")
		} else if strings.HasPrefix(line, "sys-apps/systemd-") {
			info.Systemd = extractVersionBeforeColons(line, "sys-apps/systemd-")
		} else if strings.HasPrefix(line, "sys-apps/ignition-") {
			info.Ignition = extractVersionBeforeColons(line, "sys-apps/ignition-")
			// Strip -rN revision suffix
			if idx := strings.Index(info.Ignition, "-r"); idx >= 0 {
				info.Ignition = info.Ignition[:idx]
			}
		} else if strings.HasPrefix(line, "dev-db/etcd-") {
			info.Etcd = extractVersionBeforeColons(line, "dev-db/etcd-")
		}
	}
}

// parseSysextPackageList extracts docker and containerd versions from sysext package lists.
func parseSysextPackageList(body string, info *ChannelInfo) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "app-containers/docker-") && !strings.Contains(line, "docker-cli") && !strings.Contains(line, "docker-buildx") {
			info.Docker = extractVersionBeforeColons(line, "app-containers/docker-")
		} else if strings.HasPrefix(line, "app-containers/containerd-") {
			info.Containerd = extractVersionBeforeColons(line, "app-containers/containerd-")
		}
	}
}

// extractVersionBeforeColons gets the version string between the prefix and "::".
// e.g. "sys-kernel/coreos-kernel-6.12.81::coreos-overlay" → "6.12.81"
func extractVersionBeforeColons(line, prefix string) string {
	after := strings.TrimPrefix(line, prefix)
	if idx := strings.Index(after, "::"); idx >= 0 {
		return after[:idx]
	}
	return after
}

// verifySHA512 checks if the SHA512 hash of content matches the digest file.
// The digest file format is: "<hash>  <filename>\n" per line.
func verifySHA512(content, digestBody string) bool {
	hash := sha512.Sum512([]byte(content))
	computed := hex.EncodeToString(hash[:])

	// Parse the DIGESTS file for SHA512 hash
	inSHA512 := false
	for _, line := range strings.Split(digestBody, "\n") {
		line = strings.TrimSpace(line)
		if line == "# SHA512 HASH" {
			inSHA512 = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			inSHA512 = false
			continue
		}
		if inSHA512 && line != "" {
			// Format: "<hash>  <filename>"
			parts := strings.Fields(line)
			if len(parts) >= 1 && parts[0] == computed {
				return true
			}
		}
	}
	return false
}
