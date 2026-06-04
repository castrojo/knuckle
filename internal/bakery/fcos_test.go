package bakery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseFCOSTagName(t *testing.T) {
	tests := []struct {
		tag     string
		want    FCOSTagParsed
		wantErr bool
	}{
		{
			tag:  "tailscale-0-1.98.3-1-44-x86-64",
			want: FCOSTagParsed{Name: "tailscale", Version: "0-1.98.3-1", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "docker-ce-3-29.4.0-1.fc43-43-x86-64",
			want: FCOSTagParsed{Name: "docker-ce", Version: "3-29.4.0-1.fc43", FedoraVersion: "43", Arch: "x86-64"},
		},
		{
			tag:  "vscodium-1.121.03429-el8-44-arm64",
			want: FCOSTagParsed{Name: "vscodium", Version: "1.121.03429-el8", FedoraVersion: "44", Arch: "arm64"},
		},
		{
			tag:  "1password-gui-8.12.6-1-43-x86-64",
			want: FCOSTagParsed{Name: "1password-gui", Version: "8.12.6-1", FedoraVersion: "43", Arch: "x86-64"},
		},
		{
			tag:  "google-chrome-147.0.7727.55-1-44-x86-64",
			want: FCOSTagParsed{Name: "google-chrome", Version: "147.0.7727.55-1", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "ghostty-1.3.1-2.fc44-44-x86-64",
			want: FCOSTagParsed{Name: "ghostty", Version: "1.3.1-2.fc44", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "cloud-hypervisor-51.0.0-39.24-43-x86-64",
			want: FCOSTagParsed{Name: "cloud-hypervisor", Version: "51.0.0-39.24", FedoraVersion: "43", Arch: "x86-64"},
		},
		{
			tag:  "incus-6.18-1.fc42-42-x86-64",
			want: FCOSTagParsed{Name: "incus", Version: "6.18-1.fc42", FedoraVersion: "42", Arch: "x86-64"},
		},
		{
			tag:  "nordvpn-gui-0-4.2.3-1-43-x86-64",
			want: FCOSTagParsed{Name: "nordvpn-gui", Version: "0-4.2.3-1", FedoraVersion: "43", Arch: "x86-64"},
		},
		{
			tag:  "openconnect-9.12.git.265.a7e7514-0.fc43-43-x86-64",
			want: FCOSTagParsed{Name: "openconnect", Version: "9.12.git.265.a7e7514-0.fc43", FedoraVersion: "43", Arch: "x86-64"},
		},
		{
			tag:  "vscode-1.122.1-1780040898.el8-44-x86-64",
			want: FCOSTagParsed{Name: "vscode", Version: "1.122.1-1780040898.el8", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "dnclient-0.9.3-44-x86-64",
			want: FCOSTagParsed{Name: "dnclient", Version: "0.9.3", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "mullvad-vpn-2026.2-1-44-x86-64",
			want: FCOSTagParsed{Name: "mullvad-vpn", Version: "2026.2-1", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "glab-1.92.1-1-44-x86-64",
			want: FCOSTagParsed{Name: "glab", Version: "1.92.1-1", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "microsoft-edge-147.0.3912.72-1-44-x86-64",
			want: FCOSTagParsed{Name: "microsoft-edge", Version: "147.0.3912.72-1", FedoraVersion: "44", Arch: "x86-64"},
		},
		{
			tag:  "bitwarden-2026.3.1-1-44-x86-64",
			want: FCOSTagParsed{Name: "bitwarden", Version: "2026.3.1-1", FedoraVersion: "44", Arch: "x86-64"},
		},
		// Umbrella tags (no arch suffix) should fail
		{
			tag:     "vscode",
			wantErr: true,
		},
		{
			tag:     "tailscale",
			wantErr: true,
		},
		{
			tag:     "latest",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, err := ParseFCOSTagName(tt.tag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseFCOSTagName(%q) expected error, got %+v", tt.tag, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFCOSTagName(%q) unexpected error: %v", tt.tag, err)
			}
			if got != tt.want {
				t.Errorf("ParseFCOSTagName(%q) = %+v, want %+v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestFetchCatalogFCOS_FiltersCorrectly(t *testing.T) {
	releases := []githubRelease{
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Tailscale for Fedora 44",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/tailscale-0-1.98.3-1-44-x86-64.raw"},
				{Name: "SHA256SUMS", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/SHA256SUMS"},
			},
		},
		{
			TagName: "tailscale-0-1.98.3-1-44-arm64",
			Body:    "Tailscale for Fedora 44 arm64",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-44-arm64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-arm64/tailscale-0-1.98.3-1-44-arm64.raw"},
			},
		},
		{
			TagName: "tailscale-0-1.98.3-1-43-x86-64",
			Body:    "Tailscale for Fedora 43",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-43-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-43-x86-64/tailscale-0-1.98.3-1-43-x86-64.raw"},
			},
		},
		{
			TagName: "docker-ce-3-29.4.0-1.fc43-44-x86-64",
			Body:    "Docker CE for Fedora 44",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "docker-ce-3-29.4.0-1.fc43-44-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-29.4.0-1.fc43-44-x86-64/docker-ce-3-29.4.0-1.fc43-44-x86-64.raw"},
			},
		},
		// Umbrella tag — should be skipped
		{
			TagName: "tailscale",
			Body:    "Latest tailscale",
			Assets:  nil,
		},
	}

	body, err := json.Marshal(releases)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	client := &FCOSClient{
		HTTPClient:    NewHTTPClientWithURL(srv.URL),
		FedoraVersion: 44,
	}

	entries, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get tailscale (44-x86-64) and docker-ce (44-x86-64).
	// NOT tailscale 43, NOT tailscale arm64, NOT umbrella tag.
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(entries), entries)
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["tailscale"] {
		t.Error("expected tailscale in results")
	}
	if !names["docker-ce"] {
		t.Error("expected docker-ce in results")
	}
}

func TestFetchCatalogFCOS_Deduplication(t *testing.T) {
	releases := []githubRelease{
		{
			TagName: "glab-1.95.0-1-44-x86-64",
			Body:    "Newer glab",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "glab-1.95.0-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/glab-1.95.0-1-44-x86-64/glab-1.95.0-1-44-x86-64.raw"},
			},
		},
		{
			TagName: "glab-1.92.1-1-44-x86-64",
			Body:    "Older glab",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "glab-1.92.1-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/glab-1.92.1-1-44-x86-64/glab-1.92.1-1-44-x86-64.raw"},
			},
		},
	}

	body, _ := json.Marshal(releases)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	client := &FCOSClient{
		HTTPClient:    NewHTTPClientWithURL(srv.URL),
		FedoraVersion: 44,
	}

	entries, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (dedup)", len(entries))
	}
	if entries[0].Version != "1.95.0-1" {
		t.Errorf("version = %q, want newest (1.95.0-1)", entries[0].Version)
	}
}

func TestFetchCatalogFCOS_UnsupportedArch(t *testing.T) {
	client := &FCOSClient{
		HTTPClient:    NewHTTPClient(),
		FedoraVersion: 44,
	}
	_, err := client.FetchCatalogArch(context.Background(), "s390x")
	if err == nil {
		t.Fatal("expected error for unsupported arch")
	}
}

func TestFetchCatalogFCOS_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := &FCOSClient{
		HTTPClient:    NewHTTPClientWithURL(srv.URL),
		FedoraVersion: 44,
	}

	_, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestFetchCatalogFCOS_CuratedDescriptions(t *testing.T) {
	releases := []githubRelease{
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Raw body from GitHub",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/tailscale-0-1.98.3-1-44-x86-64.raw"},
			},
		},
	}

	body, _ := json.Marshal(releases)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	client := &FCOSClient{
		HTTPClient:    NewHTTPClientWithURL(srv.URL),
		FedoraVersion: 44,
	}

	entries, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	// Should use curated description, not raw body
	if entries[0].Description == "Raw body from GitHub" {
		t.Error("description should be curated, not raw GitHub body")
	}
	if entries[0].Category != "Networking" {
		t.Errorf("category = %q, want Networking", entries[0].Category)
	}
}

func TestFCOSLookup(t *testing.T) {
	tests := []struct {
		name    string
		wantOK  bool
		wantCat string
	}{
		{"docker-ce", true, "Container Runtime"},
		{"tailscale", true, "Networking"},
		{"vscode", true, "Development"},
		{"vscodium", true, "Development"},
		{"google-chrome", true, "Desktop"},
		{"1password-gui", true, "Security"},
		{"nonexistent-ext", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := FCOSLookup(tt.name)
			if ok != tt.wantOK {
				t.Fatalf("FCOSLookup(%q) ok=%v, want %v", tt.name, ok, tt.wantOK)
			}
			if ok && meta.Category != tt.wantCat {
				t.Errorf("FCOSLookup(%q).Category = %q, want %q", tt.name, meta.Category, tt.wantCat)
			}
		})
	}
}

func TestMaxFCOSCatalogPages(t *testing.T) {
	requestCount := 0
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		releases := []githubRelease{
			{
				TagName: fmt.Sprintf("test-ext-%d-1.0-44-x86-64", requestCount),
				Body:    "test",
				Assets: []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				}{
					{
						Name:               fmt.Sprintf("test-ext-%d-1.0-44-x86-64.raw", requestCount),
						BrowserDownloadURL: fmt.Sprintf("https://example.com/test-ext-%d.raw", requestCount),
					},
				},
			},
		}
		body, _ := json.Marshal(releases)
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d>; rel="next"`, srvURL, requestCount+1))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	srvURL = srv.URL

	client := &FCOSClient{
		HTTPClient:    NewHTTPClientWithURL(srv.URL),
		FedoraVersion: 44,
	}

	_, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount > maxFCOSCatalogPages {
		t.Errorf("fetched %d pages, want at most %d", requestCount, maxFCOSCatalogPages)
	}
}
