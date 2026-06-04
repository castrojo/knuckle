package bakery

import "testing"

func TestParseFCOSTagName(t *testing.T) {
	tests := []struct {
		tag           string
		wantName      string
		wantVersion   string
		wantFedora    string
		wantArch      string
		wantOK        bool
	}{
		{
			tag: "tailscale-0-1.98.4-1-44-x86-64", wantName: "tailscale",
			wantVersion: "0-1.98.4-1", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "tailscale-0-1.98.4-1-44-arm64", wantName: "tailscale",
			wantVersion: "0-1.98.4-1", wantFedora: "44", wantArch: "arm64", wantOK: true,
		},
		{
			tag: "vscode-1.122.1-1780040915.el8-44-arm64", wantName: "vscode",
			wantVersion: "1.122.1-1780040915.el8", wantFedora: "44", wantArch: "arm64", wantOK: true,
		},
		{
			tag: "vscodium-1.121.03429-el8-44-x86-64", wantName: "vscodium",
			wantVersion: "1.121.03429-el8", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "docker-ce-3-29.5.2-1.fc44-44-x86-64", wantName: "docker-ce",
			wantVersion: "3-29.5.2-1.fc44", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "cilium-cli-0.19.4-44-x86-64", wantName: "cilium-cli",
			wantVersion: "0.19.4", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "virtctl-1.5.0-44-x86-64", wantName: "virtctl",
			wantVersion: "1.5.0", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "openconnect-9.12.git.270.549fd2d-0.fc44-44-x86-64", wantName: "openconnect",
			wantVersion: "9.12.git.270.549fd2d-0.fc44", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "nvidia-driver-cuda-580.95.05-3-580.95.05-1.fc42-42-x86-64", wantName: "nvidia-driver-cuda",
			wantVersion: "580.95.05-3-580.95.05-1.fc42", wantFedora: "42", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "1password-gui-8.12.22-1-44-x86-64", wantName: "1password-gui",
			wantVersion: "8.12.22-1", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "nordvpn-gui-5.0.0-1-44-x86-64", wantName: "nordvpn-gui",
			wantVersion: "5.0.0-1", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "netbird-ui-0.71.4-1-44-x86-64", wantName: "netbird-ui",
			wantVersion: "0.71.4-1", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "bitwarden-2026.5.0-1-44-x86-64", wantName: "bitwarden",
			wantVersion: "2026.5.0-1", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "cloud-hypervisor-51.0.0-39.41-43-x86-64", wantName: "cloud-hypervisor",
			wantVersion: "51.0.0-39.41", wantFedora: "43", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "glab-1.101.0-1-44-x86-64", wantName: "glab",
			wantVersion: "1.101.0-1", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},
		{
			tag: "microsoft-edge-148.0.3967.96-1-44-x86-64", wantName: "microsoft-edge",
			wantVersion: "148.0.3967.96-1", wantFedora: "44", wantArch: "x86-64", wantOK: true,
		},

		// Index/bare tags — should return ok=false.
		{tag: "tailscale", wantOK: false},
		{tag: "vscode", wantOK: false},
		{tag: "latest", wantOK: false},
		{tag: "", wantOK: false},
		{tag: "only-name-no-version-x86-64", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			name, version, fedora, arch, ok := ParseFCOSTagName(tt.tag)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (name=%q ver=%q fedora=%q arch=%q)", ok, tt.wantOK, name, version, fedora, arch)
			}
			if !ok {
				return
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
			if fedora != tt.wantFedora {
				t.Errorf("fedoraVersion = %q, want %q", fedora, tt.wantFedora)
			}
			if arch != tt.wantArch {
				t.Errorf("arch = %q, want %q", arch, tt.wantArch)
			}
		})
	}
}
