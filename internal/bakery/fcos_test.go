package bakery_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
)

// --- ParseFCOSTagName tests ---

func TestParseFCOSTagName(t *testing.T) {
	tests := []struct {
		tag             string
		wantName        string
		wantVersion     string
		wantFedoraVer   string
		wantArch        string
		wantErrContains string
	}{
		// Standard cases
		{
			tag:      "tailscale-0-1.98.4-1-44-x86-64",
			wantName: "tailscale", wantVersion: "0-1.98.4-1", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "docker-ce-3-29.5.3-1.fc44-44-x86-64",
			wantName: "docker-ce", wantVersion: "3-29.5.3-1.fc44", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "1password-gui-8.12.22-1-44-x86-64",
			wantName: "1password-gui", wantVersion: "8.12.22-1", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag:      "vscode-1.100.3-1-44-arm64",
			wantName: "vscode", wantVersion: "1.100.3-1", wantFedoraVer: "44", wantArch: "arm64",
		},
		{
			tag:      "netbird-ui-0.35.1-1-44-arm64",
			wantName: "netbird-ui", wantVersion: "0.35.1-1", wantFedoraVer: "44", wantArch: "arm64",
		},
		{
			tag:      "cloud-hypervisor-1-39.1-1-43-x86-64",
			wantName: "cloud-hypervisor", wantVersion: "1-39.1-1", wantFedoraVer: "43", wantArch: "x86-64",
		},
		// Error cases
		{
			tag:             "tailscale",
			wantErrContains: "no recognized arch suffix",
		},
		{
			tag:             "latest",
			wantErrContains: "no recognized arch suffix",
		},
		{
			tag:             "vscode",
			wantErrContains: "no recognized arch suffix",
		},
		{
			tag:             "noversion-x86-64",
			wantErrContains: "no fedora version segment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			name, version, fedoraVer, arch, err := bakery.ParseFCOSTagName(tt.tag)
			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrContains)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErrContains, err)
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

// --- FetchCatalogFCOS tests ---

type fcosRelease struct {
	TagName string      `json:"tag_name"`
	Body    string      `json:"body"`
	Assets  []fcosAsset `json:"assets"`
}

type fcosAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func makeFCOSTestServer(t *testing.T, releases []fcosRelease) *httptest.Server {
	t.Helper()
	body, err := json.Marshal(releases)
	if err != nil {
		t.Fatalf("marshaling test releases: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func TestFetchCatalogFCOS_BasicFiltering(t *testing.T) {
	releases := []fcosRelease{
		{
			TagName: "tailscale-0-1.98.4-1-44-x86-64",
			Body:    "Tailscale VPN",
			Assets: []fcosAsset{
				{Name: "tailscale-0-1.98.4-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/org/repo/releases/download/tailscale-0-1.98.4-1-44-x86-64/tailscale-0-1.98.4-1-44-x86-64.raw"},
				{Name: "SHA256SUMS", BrowserDownloadURL: "https://github.com/org/repo/releases/download/tailscale-0-1.98.4-1-44-x86-64/SHA256SUMS"},
			},
		},
		{
			// Wrong arch — should be filtered out.
			TagName: "tailscale-0-1.98.4-1-44-arm64",
			Body:    "Tailscale VPN arm64",
			Assets: []fcosAsset{
				{Name: "tailscale-0-1.98.4-1-44-arm64.raw", BrowserDownloadURL: "https://github.com/org/repo/releases/download/tailscale-0-1.98.4-1-44-arm64/tailscale-0-1.98.4-1-44-arm64.raw"},
			},
		},
		{
			// Wrong fedora version — should be filtered out.
			TagName: "docker-ce-3-29.5.3-1.fc43-43-x86-64",
			Body:    "Docker CE for F43",
			Assets: []fcosAsset{
				{Name: "docker-ce-3-29.5.3-1.fc43-43-x86-64.raw", BrowserDownloadURL: "https://github.com/org/repo/releases/download/docker-ce-3-29.5.3-1.fc43-43-x86-64/docker-ce-3-29.5.3-1.fc43-43-x86-64.raw"},
			},
		},
	}

	srv := makeFCOSTestServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), entries)
	}
	if entries[0].Name != "tailscale" {
		t.Errorf("expected 'tailscale', got %q", entries[0].Name)
	}
}

func TestFetchCatalogFCOS_Deduplication(t *testing.T) {
	releases := []fcosRelease{
		{
			TagName: "tailscale-0-1.99.0-1-44-x86-64",
			Body:    "Newer",
			Assets: []fcosAsset{
				{Name: "tailscale-0-1.99.0-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/r/r/releases/download/t/tailscale-0-1.99.0-1-44-x86-64.raw"},
			},
		},
		{
			// Older release of same extension — should be deduped.
			TagName: "tailscale-0-1.98.4-1-44-x86-64",
			Body:    "Older",
			Assets: []fcosAsset{
				{Name: "tailscale-0-1.98.4-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/r/r/releases/download/t2/tailscale-0-1.98.4-1-44-x86-64.raw"},
			},
		},
	}

	srv := makeFCOSTestServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 deduplicated entry, got %d", len(entries))
	}
	if entries[0].Version != "0-1.99.0-1" {
		t.Errorf("expected newer version, got %q", entries[0].Version)
	}
}

func TestFetchCatalogFCOS_SkipsFloatingTags(t *testing.T) {
	releases := []fcosRelease{
		// Floating tag — no arch/version, should be skipped.
		{TagName: "tailscale", Body: "floating", Assets: []fcosAsset{
			{Name: "tailscale.raw", BrowserDownloadURL: "https://github.com/r/r/releases/download/tailscale/tailscale.raw"},
		}},
		// Valid entry.
		{TagName: "vscode-1.100.3-1-44-x86-64", Body: "VS Code", Assets: []fcosAsset{
			{Name: "vscode-1.100.3-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/r/r/releases/download/vscode-1.100.3-1-44-x86-64/vscode-1.100.3-1-44-x86-64.raw"},
		}},
	}

	srv := makeFCOSTestServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "vscode" {
		t.Fatalf("expected only 'vscode', got %v", entries)
	}
}

func TestFetchCatalogFCOS_CuratedDescriptions(t *testing.T) {
	releases := []fcosRelease{
		{
			TagName: "tailscale-0-1.98.4-1-44-x86-64",
			Body:    "raw github body",
			Assets: []fcosAsset{
				{Name: "tailscale-0-1.98.4-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/r/r/releases/download/t/tailscale-0-1.98.4-1-44-x86-64.raw"},
			},
		},
	}

	srv := makeFCOSTestServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry")
	}
	// Curated description should override the raw GitHub body.
	if entries[0].Description == "raw github body" {
		t.Errorf("expected curated description, got raw body %q", entries[0].Description)
	}
	if entries[0].Category != "Networking" {
		t.Errorf("expected category Networking, got %q", entries[0].Category)
	}
}

func TestFetchCatalogFCOS_UnsupportedArch(t *testing.T) {
	client := bakery.NewHTTPClientWithURL("http://unused")
	_, err := client.FetchCatalogFCOS(context.Background(), "riscv64", 44)
	if err == nil {
		t.Fatal("expected error for unsupported arch")
	}
}

func TestFetchCatalogFCOS_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	_, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestFetchCatalogFCOS_Pagination(t *testing.T) {
	// Page 1 → returns one release + Link: next header
	page1Release := []fcosRelease{{
		TagName: "vscode-1.100.3-1-44-x86-64",
		Body:    "VS Code",
		Assets: []fcosAsset{
			{Name: "vscode-1.100.3-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/r/r/releases/download/v/vscode-1.100.3-1-44-x86-64.raw"},
		},
	}}
	page1Body, _ := json.Marshal(page1Release)

	page2Release := []fcosRelease{{
		TagName: "tailscale-0-1.98.4-1-44-x86-64",
		Body:    "Tailscale",
		Assets: []fcosAsset{
			{Name: "tailscale-0-1.98.4-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/r/r/releases/download/t/tailscale-0-1.98.4-1-44-x86-64.raw"},
		},
	}}
	page2Body, _ := json.Marshal(page2Release)

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery == "" || strings.Contains(r.URL.RawQuery, "page=1") {
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, srvURL))
			_, _ = w.Write(page1Body)
		} else {
			_, _ = w.Write(page2Body)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries from 2 pages, got %d", len(entries))
	}
}
