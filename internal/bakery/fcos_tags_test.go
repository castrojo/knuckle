package bakery

import (
	"testing"
)

func TestParseFCOSTagName(t *testing.T) {
	tests := []struct {
		tag            string
		wantName       string
		wantVersion    string
		wantFedoraVer  string
		wantArch       string
		wantEmpty      bool
	}{
		{
			tag:           "tailscale-0-1.98.3-1-44-x86-64",
			wantName:      "tailscale",
			wantVersion:   "0-1.98.3-1",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		{
			tag:           "tailscale-0-1.98.3-1-43-arm64",
			wantName:      "tailscale",
			wantVersion:   "0-1.98.3-1",
			wantFedoraVer: "43",
			wantArch:      "arm64",
		},
		{
			tag:           "docker-ce-3-29.5.1-1.fc44-44-x86-64",
			wantName:      "docker-ce",
			wantVersion:   "3-29.5.1-1.fc44",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		{
			tag:           "vscodium-1.121.03429-el8-44-arm64",
			wantName:      "vscodium",
			wantVersion:   "1.121.03429-el8",
			wantFedoraVer: "44",
			wantArch:      "arm64",
		},
		{
			tag:           "1password-gui-8.12.21-1-43-x86-64",
			wantName:      "1password-gui",
			wantVersion:   "8.12.21-1",
			wantFedoraVer: "43",
			wantArch:      "x86-64",
		},
		{
			tag:           "cloud-hypervisor-51.0.0-39.32-43-arm64",
			wantName:      "cloud-hypervisor",
			wantVersion:   "51.0.0-39.32",
			wantFedoraVer: "43",
			wantArch:      "arm64",
		},
		{
			tag:           "glab-1.97.0-1-43-arm64",
			wantName:      "glab",
			wantVersion:   "1.97.0-1",
			wantFedoraVer: "43",
			wantArch:      "arm64",
		},
		{
			tag:           "ghostty-1.3.0-1.fc43-43-x86-64",
			wantName:      "ghostty",
			wantVersion:   "1.3.0-1.fc43",
			wantFedoraVer: "43",
			wantArch:      "x86-64",
		},
		{
			tag:           "bitwarden-2026.4.0-1-43-x86-64",
			wantName:      "bitwarden",
			wantVersion:   "2026.4.0-1",
			wantFedoraVer: "43",
			wantArch:      "x86-64",
		},
		{
			tag:           "vscode-1.122.1-1780040898.el8-44-x86-64",
			wantName:      "vscode",
			wantVersion:   "1.122.1-1780040898.el8",
			wantFedoraVer: "44",
			wantArch:      "x86-64",
		},
		// Meta-releases (no version info) → all empty
		{tag: "tailscale", wantEmpty: true},
		{tag: "latest", wantEmpty: true},
		{tag: "vscode", wantEmpty: true},
		{tag: "docker-ce", wantEmpty: true},
		// Edge cases
		{tag: "", wantEmpty: true},
		{tag: "just-a-name", wantEmpty: true},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			name, version, fedoraVer, arch, err := ParseFCOSTagName(tt.tag)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantEmpty {
				if name != "" || version != "" || fedoraVer != "" || arch != "" {
					t.Errorf("expected all empty for %q, got name=%q version=%q fedora=%q arch=%q",
						tt.tag, name, version, fedoraVer, arch)
				}
				return
			}

			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
			if fedoraVer != tt.wantFedoraVer {
				t.Errorf("fedoraVersion = %q, want %q", fedoraVer, tt.wantFedoraVer)
			}
			if arch != tt.wantArch {
				t.Errorf("arch = %q, want %q", arch, tt.wantArch)
			}
		})
	}
}
