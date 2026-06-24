package bakery

import (
	"strings"
	"testing"
)

// TestParseFCOSTagName covers the real tag formats observed in the
// fedora-sysexts/community GitHub Releases API (per_page=100 on 2025-06-04).
func TestParseFCOSTagName(t *testing.T) {
	tests := []struct {
		tag             string
		wantName        string
		wantVersion     string
		wantFedoraVer   string
		wantArch        string
		wantErrContains string
	}{
		// ── Real catalog examples ──────────────────────────────────────────
		{
			tag:      "tailscale-0-1.98.3-1-44-x86-64",
			wantName: "tailscale", wantVersion: "0-1.98.3-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "tailscale-0-1.98.3-1-43-arm64",
			wantName: "tailscale", wantVersion: "0-1.98.3-1",
			wantFedoraVer: "43", wantArch: "arm64",
		},
		{
			tag:      "docker-ce-3-29.5.3-1.fc44-44-x86-64",
			wantName: "docker-ce", wantVersion: "3-29.5.3-1.fc44",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "docker-ce-3-29.5.3-1.fc43-43-arm64",
			wantName: "docker-ce", wantVersion: "3-29.5.3-1.fc43",
			wantFedoraVer: "43", wantArch: "arm64",
		},
		{
			tag:      "vscode-1.123.0-1780481629.el8-43-arm64",
			wantName: "vscode", wantVersion: "1.123.0-1780481629.el8",
			wantFedoraVer: "43", wantArch: "arm64",
		},
		{
			tag:      "vscode-1.123.0-1780481612.el8-44-x86-64",
			wantName: "vscode", wantVersion: "1.123.0-1780481612.el8",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "vscodium-1.121.03429-el8-44-x86-64",
			wantName: "vscodium", wantVersion: "1.121.03429-el8",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "dnclient-0.9.4-44-x86-64",
			wantName: "dnclient", wantVersion: "0.9.4",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "dnclient-0.9.4-43-arm64",
			wantName: "dnclient", wantVersion: "0.9.4",
			wantFedoraVer: "43", wantArch: "arm64",
		},
		{
			tag:      "cloud-hypervisor-51.0.0-39.38-43-x86-64",
			wantName: "cloud-hypervisor", wantVersion: "51.0.0-39.38",
			wantFedoraVer: "43", wantArch: "x86-64",
		},
		{
			tag:      "cloud-hypervisor-51.0.0-39.38-43-arm64",
			wantName: "cloud-hypervisor", wantVersion: "51.0.0-39.38",
			wantFedoraVer: "43", wantArch: "arm64",
		},
		{
			tag:      "cilium-cli-0.19.4-44-x86-64",
			wantName: "cilium-cli", wantVersion: "0.19.4",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "bitwarden-2026.5.0-1-44-x86-64",
			wantName: "bitwarden", wantVersion: "2026.5.0-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "1password-gui-8.12.22-1-44-x86-64",
			wantName: "1password-gui", wantVersion: "8.12.22-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "openconnect-9.12.git.270.549fd2d-0.fc44-44-x86-64",
			wantName: "openconnect", wantVersion: "9.12.git.270.549fd2d-0.fc44",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "openh264-2.6.0-3.fc44-44-x86-64",
			wantName: "openh264", wantVersion: "2.6.0-3.fc44",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "glab-1.97.0-1-44-x86-64",
			wantName: "glab", wantVersion: "1.97.0-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "glab-1.97.0-1-44-arm64",
			wantName: "glab", wantVersion: "1.97.0-1",
			wantFedoraVer: "44", wantArch: "arm64",
		},
		{
			tag:      "netbird-0.71.2-1-44-x86-64",
			wantName: "netbird", wantVersion: "0.71.2-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "netbird-ui-0.71.2-1-44-x86-64",
			wantName: "netbird-ui", wantVersion: "0.71.2-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "littlesnitch-1.0.9-1-44-x86-64",
			wantName: "littlesnitch", wantVersion: "1.0.9-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "virtctl-1.5.0-44-x86-64",
			wantName: "virtctl", wantVersion: "1.5.0",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "microsoft-edge-148.0.7778.178-1-44-x86-64",
			wantName: "microsoft-edge", wantVersion: "148.0.7778.178-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "google-chrome-148.0.7778.178-1-44-x86-64",
			wantName: "google-chrome", wantVersion: "148.0.7778.178-1",
			wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "1password-gui-8.12.21-1-43-arm64",
			wantName: "1password-gui", wantVersion: "8.12.21-1",
			wantFedoraVer: "43", wantArch: "arm64",
		},

		// ── Error cases: bare pointer tags ────────────────────────────────
		{tag: "vscode", wantErrContains: "arch suffix"},
		{tag: "tailscale", wantErrContains: "arch suffix"},
		{tag: "docker-ce", wantErrContains: "arch suffix"},
		{tag: "latest", wantErrContains: "arch suffix"},

		// ── Error cases: malformed ────────────────────────────────────────
		{tag: "noarch-44", wantErrContains: "arch suffix"},
		{tag: "-x86-64", wantErrContains: "fedora version"},
		{tag: "name-noversion-x86-64", wantErrContains: "fedora version"},
		// rest == "" after stripping arch and fedoraVersion
		{tag: "-44-x86-64", wantErrContains: "no name or version"},
		// no version boundary (no segment with "." and no epoch+dot pattern)
		{tag: "name-word-44-x86-64", wantErrContains: "could not find version boundary"},
		// version starts at segment 0 — no name before it
		{tag: "1.0.0-44-x86-64", wantErrContains: "no name segment"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			name, version, fedoraVer, arch, err := ParseFCOSTagName(tt.tag)
			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrContains)
				}
				if !containsStr(err.Error(), tt.wantErrContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("name: got %q, want %q", name, tt.wantName)
			}
			if version != tt.wantVersion {
				t.Errorf("version: got %q, want %q", version, tt.wantVersion)
			}
			if fedoraVer != tt.wantFedoraVer {
				t.Errorf("fedoraVersion: got %q, want %q", fedoraVer, tt.wantFedoraVer)
			}
			if arch != tt.wantArch {
				t.Errorf("arch: got %q, want %q", arch, tt.wantArch)
			}
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"44", true},
		{"0", true},
		{"123", true},
		{"", false},
		{"44a", false},
		{"a44", false},
		{"4.4", false},
		{"44 ", false},
	}
	for _, tt := range tests {
		got := isAllDigits(tt.input)
		if got != tt.want {
			t.Errorf("isAllDigits(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
