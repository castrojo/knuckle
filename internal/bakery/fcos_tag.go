package bakery

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseFCOSTagName extracts the sysext name, version, Fedora major version, and
// architecture from a fedora-sysexts/community release tag.
//
// Tag format (RPM-derived):
//
//	<name>-[<epoch>-]<ver>-<release>-<fedoraVersion>-<arch>
//
// Examples:
//
//	tailscale-0-1.98.3-1-44-x86-64        → name=tailscale, version=0-1.98.3-1, fedoraVersion=44, arch=x86-64
//	vscode-1.122.1-1780040915.el8-44-arm64 → name=vscode, version=1.122.1-1780040915.el8, fedoraVersion=44, arch=arm64
//	docker-ce-3-29.5.1-1.fc44-44-x86-64   → name=docker-ce, version=3-29.5.1-1.fc44, fedoraVersion=44, arch=x86-64
//	1password-gui-8.12.22-1-44-x86-64      → name=1password-gui, version=8.12.22-1, fedoraVersion=44, arch=x86-64
//
// Tags without an arch suffix (alias/latest tags like "tailscale", "vscode") return an error
// and should be skipped by callers.
func ParseFCOSTagName(tag string) (name, version, fedoraVersion, arch string, err error) {
	// Step 1: strip arch suffix — must be the last segment.
	for _, a := range []string{"x86-64", "arm64"} {
		if strings.HasSuffix(tag, "-"+a) {
			arch = a
			tag = tag[:len(tag)-len(a)-1]
			break
		}
	}
	if arch == "" {
		return "", "", "", "", fmt.Errorf("no arch suffix in FCOS tag %q", tag)
	}

	// Step 2: strip the Fedora major version (trailing integer segment).
	idx := strings.LastIndex(tag, "-")
	if idx < 0 {
		return "", "", "", "", fmt.Errorf("no Fedora version segment in tag %q", tag)
	}
	fverStr := tag[idx+1:]
	fver, parseErr := strconv.Atoi(fverStr)
	if parseErr != nil || fver < 1 {
		return "", "", "", "", fmt.Errorf("invalid Fedora version %q in tag", fverStr)
	}
	fedoraVersion = fverStr
	remaining := tag[:idx]

	// Step 3: find the name/version boundary in the remaining string.
	//
	// The name is all dash-separated segments before the version starts.
	// The version starts at the first segment that contains a dot (e.g. "1.98.3").
	// If the segment immediately preceding the dotted segment is a pure integer,
	// it is an RPM epoch and is included in the version (not the name).
	//
	// For packages without dots in the version (edge case), we fall back to the
	// first segment that starts with a digit after a non-digit segment.
	parts := strings.Split(remaining, "-")
	nameEnd := findNameEnd(parts)
	if nameEnd <= 0 {
		return "", "", "", "", fmt.Errorf("cannot determine package name from tag %q", tag)
	}

	name = strings.Join(parts[:nameEnd], "-")
	version = strings.Join(parts[nameEnd:], "-")
	return name, version, fedoraVersion, arch, nil
}

// findNameEnd returns the index in parts where the version begins (exclusive end of name).
// Returns -1 if the boundary cannot be determined.
func findNameEnd(parts []string) int {
	// Find the first segment containing a dot (version-like segment).
	dotIdx := -1
	for i, p := range parts {
		if strings.ContainsRune(p, '.') {
			dotIdx = i
			break
		}
	}

	if dotIdx > 0 {
		// If the segment immediately before the dotted segment is a pure integer
		// AND it is not the very first segment (i.e. there is at least one name
		// segment before the epoch), treat it as an RPM epoch — it belongs to
		// the version, not the name.
		prev := dotIdx - 1
		if prev > 0 {
			if _, err := strconv.Atoi(parts[prev]); err == nil {
				return prev // epoch starts the version; name ends before it
			}
		}
		return dotIdx
	}

	if dotIdx == 0 {
		// Version starts at the very beginning — no name. Shouldn't happen.
		return -1
	}

	// No dot found — fall back: first segment starting with a digit that follows
	// at least one non-digit-starting segment.
	for i, p := range parts {
		if i > 0 && len(p) > 0 && unicode.IsDigit(rune(p[0])) {
			return i
		}
	}
	return -1
}
