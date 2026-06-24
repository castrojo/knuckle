package bakery

import (
	"fmt"
	"strings"
	"unicode"
)

// ParseFCOSTagName extracts the name, version, fedoraVersion, and arch from a
// fedora-sysexts/community release tag.
//
// Tag format: <name>-<version_components>-<fedoraVersion>-<arch>
//
// The arch suffix is always "x86-64" or "arm64".
// The fedoraVersion is a 2-or-more digit decimal integer (e.g. "44").
// The version is everything between the name and the fedoraVersion.
//
// Tags without an arch suffix (bare pointer tags such as "vscode", "latest", or
// per-name "docker-ce") are rejected with a descriptive error.
//
// Examples:
//
//	tailscale-0-1.98.3-1-44-x86-64       → tailscale, 0-1.98.3-1, 44, x86-64
//	docker-ce-3-29.5.3-1.fc44-44-x86-64  → docker-ce, 3-29.5.3-1.fc44, 44, x86-64
//	vscode-1.123.0-1780481629.el8-43-arm64 → vscode, 1.123.0-1780481629.el8, 43, arm64
//	1password-gui-8.12.22-1-44-x86-64    → 1password-gui, 8.12.22-1, 44, x86-64
//	cloud-hypervisor-51.0.0-39.38-43-x86-64 → cloud-hypervisor, 51.0.0-39.38, 43, x86-64
func ParseFCOSTagName(tag string) (name, version, fedoraVersion, arch string, err error) {
	// 1. Strip arch suffix to reveal <name>-<version>-<fedoraVersion>.
	var rest string
	switch {
	case strings.HasSuffix(tag, "-x86-64"):
		arch = "x86-64"
		rest = tag[:len(tag)-len("-x86-64")]
	case strings.HasSuffix(tag, "-arm64"):
		arch = "arm64"
		rest = tag[:len(tag)-len("-arm64")]
	default:
		return "", "", "", "", fmt.Errorf("fcos tag %q: no arch suffix (x86-64 or arm64); bare pointer tags are skipped", tag)
	}

	// 2. Strip the trailing all-digit fedoraVersion segment.
	lastDash := strings.LastIndex(rest, "-")
	if lastDash < 0 {
		return "", "", "", "", fmt.Errorf("fcos tag %q: missing fedora version segment", tag)
	}
	candidate := rest[lastDash+1:]
	if !isAllDigits(candidate) || candidate == "" {
		return "", "", "", "", fmt.Errorf("fcos tag %q: expected all-digit fedora version before arch, got %q", tag, candidate)
	}
	fedoraVersion = candidate
	rest = rest[:lastDash]

	if rest == "" {
		return "", "", "", "", fmt.Errorf("fcos tag %q: no name or version after stripping arch and fedora version", tag)
	}

	// 3. Split the remaining string into <name>-<version> at the boundary where
	//    the version components begin.
	//
	//    Heuristic (covers all observed fedora-sysexts/community tags):
	//    Scan segments left-to-right; version starts at the first segment that:
	//      a) contains a "." (version number or epoch+release), OR
	//      b) is a pure integer AND the next segment contains a "." (RPM epoch).
	//
	//    This correctly handles:
	//      - Simple versions:    "dnclient-0.9.4"      → name="dnclient"
	//      - Epoch-prefixed:     "tailscale-0-1.98.3"  → epoch "0" before "1.98.3"
	//      - Multi-word names:   "docker-ce-3-29.5.3"  → name="docker-ce"
	//      - Digit-starting:     "1password-gui-8.12"  → name="1password-gui"
	segments := strings.Split(rest, "-")
	splitIdx := -1
	for i, seg := range segments {
		if strings.ContainsRune(seg, '.') {
			splitIdx = i
			break
		}
		if isAllDigits(seg) && i+1 < len(segments) && strings.ContainsRune(segments[i+1], '.') {
			splitIdx = i
			break
		}
	}

	switch {
	case splitIdx < 0:
		return "", "", "", "", fmt.Errorf("fcos tag %q: could not find version boundary in %q", tag, rest)
	case splitIdx == 0:
		return "", "", "", "", fmt.Errorf("fcos tag %q: no name segment before version in %q", tag, rest)
	}

	name = strings.Join(segments[:splitIdx], "-")
	version = strings.Join(segments[splitIdx:], "-")
	return name, version, fedoraVersion, arch, nil
}

// isAllDigits reports whether s is non-empty and contains only ASCII decimal digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
