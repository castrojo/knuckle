package bakery

import (
	"strconv"
	"strings"
)

// ParseFCOSTagName extracts the extension name, version, Fedora major version,
// and architecture from a fedora-sysexts/community release tag.
//
// Tag format examples:
//
//	tailscale-0-1.98.4-1-44-x86-64
//	vscode-1.122.1-1780040915.el8-44-arm64
//	docker-ce-3-29.5.2-1.fc44-44-x86-64
//	cilium-cli-0.19.4-44-x86-64
//	virtctl-1.5.0-44-x86-64
//
// The last segments encode <fedora-version>-<arch>, where arch is "x86-64" or
// "arm64". Everything between the extension name and the fedora-version is
// treated as the version string.
//
// Index tags (bare names like "tailscale", "vscode", "latest") return empty
// strings and ok=false.
func ParseFCOSTagName(tag string) (name, version, fedoraVersion, arch string, ok bool) {
	// Detect and strip arch suffix.
	switch {
	case strings.HasSuffix(tag, "-x86-64"):
		arch = "x86-64"
		tag = tag[:len(tag)-len("-x86-64")]
	case strings.HasSuffix(tag, "-arm64"):
		arch = "arm64"
		tag = tag[:len(tag)-len("-arm64")]
	default:
		return "", "", "", "", false
	}

	// The last hyphen-separated segment is the Fedora major version.
	lastDash := strings.LastIndex(tag, "-")
	if lastDash < 0 {
		return "", "", "", "", false
	}
	fedoraVersion = tag[lastDash+1:]
	tag = tag[:lastDash]

	if _, err := strconv.Atoi(fedoraVersion); err != nil || fedoraVersion == "" {
		return "", "", "", "", false
	}

	// Now tag is "<name>-<version-parts>". The name is everything up to the
	// first segment that is purely numeric or numeric-with-dots (no letters).
	// This distinguishes version segments like "8.12.22" or "0" from name
	// segments like "1password" which start with a digit but contain letters.
	parts := strings.Split(tag, "-")
	if len(parts) < 2 {
		return "", "", "", "", false
	}

	versionStart := -1
	for i := 1; i < len(parts); i++ {
		if isVersionSegment(parts[i]) {
			versionStart = i
			break
		}
	}
	if versionStart < 0 {
		return "", "", "", "", false
	}

	name = strings.Join(parts[:versionStart], "-")
	version = strings.Join(parts[versionStart:], "-")

	if name == "" || version == "" {
		return "", "", "", "", false
	}

	return name, version, fedoraVersion, arch, true
}

// isVersionSegment returns true if s looks like the start of a version string
// rather than part of a package name. It must start with a digit and either:
//   - contain a dot (e.g. "9.12.git.270", "1.122.1", "51.0.0")
//   - be purely numeric (e.g. "0" for an RPM epoch, "3" for docker-ce epoch)
//
// This distinguishes version segments from name segments like "1password"
// which start with a digit but are clearly words.
func isVersionSegment(s string) bool {
	if len(s) == 0 || s[0] < '0' || s[0] > '9' {
		return false
	}
	if strings.ContainsRune(s, '.') {
		return true
	}
	// Purely numeric (no letters, no dots) — RPM epoch or simple version.
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
