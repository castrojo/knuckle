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
//	tailscale-0-1.98.3-1-44-x86-64       → ("tailscale", "0-1.98.3-1", "44", "x86-64")
//	docker-ce-3-29.5.1-1.fc44-44-x86-64  → ("docker-ce", "3-29.5.1-1.fc44", "44", "x86-64")
//	vscodium-1.121.03429-el8-44-arm64    → ("vscodium", "1.121.03429-el8", "44", "arm64")
//	1password-gui-8.12.21-1-43-x86-64    → ("1password-gui", "8.12.21-1", "43", "x86-64")
//
// Meta-releases without version info (e.g. "tailscale", "latest") return all empty strings.
func ParseFCOSTagName(tag string) (name, version, fedoraVersion, arch string, err error) {
	// Handle x86-64 suffix (contains a hyphen within the arch token).
	if strings.HasSuffix(tag, "-x86-64") {
		arch = "x86-64"
		tag = strings.TrimSuffix(tag, "-x86-64")
	} else if strings.HasSuffix(tag, "-arm64") {
		arch = "arm64"
		tag = strings.TrimSuffix(tag, "-arm64")
	} else {
		return "", "", "", "", nil
	}

	// The last segment before arch is the Fedora major version (a bare integer).
	lastDash := strings.LastIndex(tag, "-")
	if lastDash < 0 {
		return "", "", "", "", nil
	}

	candidate := tag[lastDash+1:]
	if _, parseErr := strconv.Atoi(candidate); parseErr != nil {
		return "", "", "", "", nil
	}
	fedoraVersion = candidate
	tag = tag[:lastDash]

	// Now split the remaining tag into name and version.
	// The name is everything before the first segment that starts with a digit.
	// We scan from left to right: the first '-' followed by a digit marks the version start.
	nameEnd := -1
	for i := 1; i < len(tag); i++ {
		if tag[i-1] == '-' && i < len(tag) && tag[i] >= '0' && tag[i] <= '9' {
			nameEnd = i - 1
			break
		}
	}

	if nameEnd < 0 {
		return "", "", "", "", nil
	}

	name = tag[:nameEnd]
	version = tag[nameEnd+1:]

	return name, version, fedoraVersion, arch, nil
}
