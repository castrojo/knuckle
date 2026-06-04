package bakery_test

import (
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
)

func TestParseFCOSTagName(t *testing.T) {
	tests := []struct {
		tag           string
		wantName      string
		wantVersion   string
		wantFedoraVer string
		wantArch      string
		wantErr       bool
	}{
		// Single-word name, no epoch
		{
			tag:           "tailscale-1.98.3-1-44-x86-64",
			wantName:      "tailscale",
			wantVersion:   "1.98.3-1",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Single-word name with RPM epoch
		{
			tag:           "tailscale-0-1.98.3-1-44-x86-64",
			wantName:      "tailscale",
			wantVersion:   "0-1.98.3-1",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// ARM64 arch
		{
			tag:           "tailscale-0-1.98.3-1-43-arm64",
			wantName:      "tailscale",
			wantVersion:   "0-1.98.3-1",
			wantFedoraVer: "43",
			wantArch:      "arm64",
		},
		// vscode with .el8 release suffix
		{
			tag:           "vscode-1.122.1-1780040915.el8-44-arm64",
			wantName:      "vscode",
			wantVersion:   "1.122.1-1780040915.el8",
			wantFedoraVer: "44",
			wantArch:      "arm64",
		},
		// vscodium with .el8 release suffix
		{
			tag:           "vscodium-1.121.03429-el8-44-x86-64",
			wantName:      "vscodium",
			wantVersion:   "1.121.03429-el8",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Multi-word name with epoch (docker-ce, epoch=3)
		{
			tag:           "docker-ce-3-29.5.1-1.fc44-44-x86-64",
			wantName:      "docker-ce",
			wantVersion:   "3-29.5.1-1.fc44",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Multi-word name without epoch (netbird-ui)
		{
			tag:           "netbird-ui-0.71.2-1-44-x86-64",
			wantName:      "netbird-ui",
			wantVersion:   "0.71.2-1",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Name starting with digit (1password-gui)
		{
			tag:           "1password-gui-8.12.22-1-44-x86-64",
			wantName:      "1password-gui",
			wantVersion:   "8.12.22-1",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Multi-word name without epoch (google-chrome)
		{
			tag:           "google-chrome-148.0.7778.178-1-44-x86-64",
			wantName:      "google-chrome",
			wantVersion:   "148.0.7778.178-1",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Multi-word name without epoch (microsoft-edge)
		{
			tag:           "microsoft-edge-148.0.7778.178-1-43-x86-64",
			wantName:      "microsoft-edge",
			wantVersion:   "148.0.7778.178-1",
			wantFedoraVer: "43",
			wantArch:      "x86-64",
		},
		// Multi-word name without epoch (cloud-hypervisor)
		{
			tag:           "cloud-hypervisor-51.0.0-39.35-43-x86-64",
			wantName:      "cloud-hypervisor",
			wantVersion:   "51.0.0-39.35",
			wantFedoraVer: "43",
			wantArch:      "x86-64",
		},
		// Complex openconnect version string
		{
			tag:           "openconnect-9.12.git.268.a79c24e-0.fc44-44-x86-64",
			wantName:      "openconnect",
			wantVersion:   "9.12.git.268.a79c24e-0.fc44",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Date-based version (bitwarden)
		{
			tag:           "bitwarden-2026.5.0-1-43-x86-64",
			wantName:      "bitwarden",
			wantVersion:   "2026.5.0-1",
			wantFedoraVer: "43",
			wantArch:      "x86-64",
		},
		// littlesnitch arm64
		{
			tag:           "littlesnitch-1.0.9-1-44-arm64",
			wantName:      "littlesnitch",
			wantVersion:   "1.0.9-1",
			wantFedoraVer: "44",
			wantArch:      "arm64",
		},
		// Error: no arch suffix (alias tag like "tailscale")
		{
			tag:     "tailscale",
			wantErr: true,
		},
		// Error: no arch suffix (bare name tag like "vscode")
		{
			tag:     "vscode",
			wantErr: true,
		},
		// Error: "latest" tag
		{
			tag:     "latest",
			wantErr: true,
		},
		// Error: malformed — only arch, no version
		{
			tag:     "x86-64",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			name, version, fedoraVer, arch, err := bakery.ParseFCOSTagName(tc.tag)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseFCOSTagName(%q): expected error, got name=%q version=%q", tc.tag, name, version)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFCOSTagName(%q): unexpected error: %v", tc.tag, err)
			}
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if version != tc.wantVersion {
				t.Errorf("version = %q, want %q", version, tc.wantVersion)
			}
			if fedoraVer != tc.wantFedoraVer {
				t.Errorf("fedoraVersion = %q, want %q", fedoraVer, tc.wantFedoraVer)
			}
			if arch != tc.wantArch {
				t.Errorf("arch = %q, want %q", arch, tc.wantArch)
			}
		})
	}
}
