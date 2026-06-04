package bakery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
)

// --- ParseFCOSTagName tests ---

func TestParseFCOSTagName(t *testing.T) {
	tests := []struct {
		tag            string
		wantName       string
		wantVersion    string
		wantFedoraVer  string
		wantArch       string
		wantErr        bool
	}{
		// Real community repo tag formats
		{
			tag: "tailscale-0-1.98.3-1-44-x86-64",
			wantName: "tailscale", wantVersion: "0-1.98.3-1", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag: "docker-ce-3-29.5.3-1.fc44-44-x86-64",
			wantName: "docker-ce", wantVersion: "3-29.5.3-1.fc44", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag: "vscode-1.123.0-1780481629.el8-44-arm64",
			wantName: "vscode", wantVersion: "1.123.0-1780481629.el8", wantFedoraVer: "44", wantArch: "arm64",
		},
		{
			tag: "vscodium-1.121.03429-el8-44-x86-64",
			wantName: "vscodium", wantVersion: "1.121.03429-el8", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag: "dnclient-0.9.4-44-x86-64",
			wantName: "dnclient", wantVersion: "0.9.4", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag: "nordvpn-gui-5.0.0-1-44-x86-64",
			wantName: "nordvpn-gui", wantVersion: "5.0.0-1", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag: "microsoft-edge-148.0.3967.96-1-44-x86-64",
			wantName: "microsoft-edge", wantVersion: "148.0.3967.96-1", wantFedoraVer: "44", wantArch: "x86-64",
		},
		{
			tag: "openconnect-9.12.git.270.549fd2d-0.fc44-44-x86-64",
			wantName: "openconnect", wantVersion: "9.12.git.270.549fd2d-0.fc44", wantFedoraVer: "44", wantArch: "x86-64",
		},
		// Arm64 variant
		{
			tag: "tailscale-0-1.98.3-1-44-arm64",
			wantName: "tailscale", wantVersion: "0-1.98.3-1", wantFedoraVer: "44", wantArch: "arm64",
		},
		// Index-only tags must return error
		{tag: "tailscale", wantErr: true},
		{tag: "latest", wantErr: true},
		{tag: "docker-ce", wantErr: true},
		{tag: "vscode", wantErr: true},
		// Malformed: no arch suffix
		{tag: "something-1.0-44", wantErr: true},
		// Malformed: non-digit fedora version
		{tag: "foo-1.0-rc1-x86-64", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			name, version, fedoraVer, arch, err := bakery.ParseFCOSTagName(tc.tag)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for tag %q, got name=%q ver=%q fed=%q arch=%q", tc.tag, name, version, fedoraVer, arch)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for tag %q: %v", tc.tag, err)
			}
			if name != tc.wantName {
				t.Errorf("name: got %q, want %q", name, tc.wantName)
			}
			if version != tc.wantVersion {
				t.Errorf("version: got %q, want %q", version, tc.wantVersion)
			}
			if fedoraVer != tc.wantFedoraVer {
				t.Errorf("fedoraVersion: got %q, want %q", fedoraVer, tc.wantFedoraVer)
			}
			if arch != tc.wantArch {
				t.Errorf("arch: got %q, want %q", arch, tc.wantArch)
			}
		})
	}
}

// --- FetchCatalogFCOS tests ---

func fcosCatalogServer(t *testing.T, releases []map[string]any) *httptest.Server {
	t.Helper()
	body, _ := json.Marshal(releases)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func TestFetchCatalogFCOS_Basic(t *testing.T) {
	releases := []map[string]any{
		{
			"tag_name": "tailscale-0-1.98.3-1-44-x86-64",
			"body":     "Tailscale",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-44-x86-64.raw", "browser_download_url": "https://example.com/tailscale-0-1.98.3-1-44-x86-64.raw"},
				{"name": "SHA256SUMS", "browser_download_url": "https://example.com/SHA256SUMS"},
			},
		},
		{
			"tag_name": "docker-ce-3-29.5.3-1.fc44-44-x86-64",
			"body":     "Docker CE",
			"assets": []map[string]any{
				{"name": "docker-ce-3-29.5.3-1.fc44-44-x86-64.raw", "browser_download_url": "https://example.com/docker-ce-3-29.5.3-1.fc44-44-x86-64.raw"},
			},
		},
		// Arm64 entry — should be filtered out for amd64 request
		{
			"tag_name": "tailscale-0-1.98.3-1-44-arm64",
			"body":     "Tailscale arm64",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-44-arm64.raw", "browser_download_url": "https://example.com/tailscale-0-1.98.3-1-44-arm64.raw"},
			},
		},
		// Index-only tag — should be skipped
		{
			"tag_name": "tailscale",
			"body":     "index",
			"assets":   []map[string]any{},
		},
	}

	srv := fcosCatalogServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	if entries[0].Name != "tailscale" {
		t.Errorf("first entry: got %q, want %q", entries[0].Name, "tailscale")
	}
	if entries[1].Name != "docker-ce" {
		t.Errorf("second entry: got %q, want %q", entries[1].Name, "docker-ce")
	}
}

func TestFetchCatalogFCOS_Arm64(t *testing.T) {
	releases := []map[string]any{
		{
			"tag_name": "tailscale-0-1.98.3-1-44-arm64",
			"body":     "Tailscale arm64",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-44-arm64.raw", "browser_download_url": "https://example.com/tailscale-0-1.98.3-1-44-arm64.raw"},
			},
		},
		// x86-64 entry — should be filtered out for arm64 request
		{
			"tag_name": "tailscale-0-1.98.3-1-44-x86-64",
			"body":     "Tailscale x86-64",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-44-x86-64.raw", "browser_download_url": "https://example.com/tailscale-0-1.98.3-1-44-x86-64.raw"},
			},
		},
	}

	srv := fcosCatalogServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "arm64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 arm64 entry, got %d", len(entries))
	}
	if entries[0].Name != "tailscale" {
		t.Errorf("expected tailscale, got %q", entries[0].Name)
	}
}

func TestFetchCatalogFCOS_DeduplicatesByName(t *testing.T) {
	// Same extension appears twice (different Fedora versions) — only the first (newest) should be kept.
	releases := []map[string]any{
		{
			"tag_name": "tailscale-0-1.98.3-1-44-x86-64",
			"body":     "Tailscale v1.98",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-44-x86-64.raw", "browser_download_url": "https://example.com/tailscale-1.98.raw"},
			},
		},
		{
			"tag_name": "tailscale-0-1.97.0-1-44-x86-64",
			"body":     "Tailscale v1.97 (older)",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.97.0-1-44-x86-64.raw", "browser_download_url": "https://example.com/tailscale-1.97.raw"},
			},
		},
	}

	srv := fcosCatalogServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 deduplicated entry, got %d", len(entries))
	}
	if entries[0].Version != "0-1.98.3-1" {
		t.Errorf("expected newest version, got %q", entries[0].Version)
	}
}

func TestFetchCatalogFCOS_FedoraVersionFiltering(t *testing.T) {
	releases := []map[string]any{
		{
			"tag_name": "tailscale-0-1.98.3-1-44-x86-64",
			"body":     "for Fedora 44",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-44-x86-64.raw", "browser_download_url": "https://example.com/44.raw"},
			},
		},
		{
			"tag_name": "tailscale-0-1.98.3-1-43-x86-64",
			"body":     "for Fedora 43",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-43-x86-64.raw", "browser_download_url": "https://example.com/43.raw"},
			},
		},
	}

	srv := fcosCatalogServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)

	// Request Fedora 44 only
	entries44, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries44) != 1 || entries44[0].URL != "https://example.com/44.raw" {
		t.Fatalf("expected Fedora 44 entry, got %v", entries44)
	}

	// Request Fedora 43 only
	entries43, err := client.FetchCatalogFCOS(context.Background(), "amd64", 43)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries43) != 1 || entries43[0].URL != "https://example.com/43.raw" {
		t.Fatalf("expected Fedora 43 entry, got %v", entries43)
	}

	// fedoraVersion=0 disables version filtering — should return both
	entriesAll, err := client.FetchCatalogFCOS(context.Background(), "amd64", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entriesAll) != 1 {
		// Both have the same name → deduplication keeps first (44)
		t.Fatalf("expected 1 deduplicated entry for version=0, got %d", len(entriesAll))
	}
}

func TestFetchCatalogFCOS_InvalidArch(t *testing.T) {
	client := bakery.NewHTTPClientWithURL("http://localhost:0")
	_, err := client.FetchCatalogFCOS(context.Background(), "mips", 44)
	if err == nil {
		t.Fatal("expected error for unsupported arch")
	}
}

func TestFetchCatalogFCOS_AppliesCuratedMetadata(t *testing.T) {
	releases := []map[string]any{
		{
			"tag_name": "tailscale-0-1.98.3-1-44-x86-64",
			"body":     "raw description",
			"assets": []map[string]any{
				{"name": "tailscale-0-1.98.3-1-44-x86-64.raw", "browser_download_url": "https://example.com/tailscale.raw"},
			},
		},
	}

	srv := fcosCatalogServer(t, releases)
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Curated metadata should override raw body
	if entries[0].Description == "raw description" {
		t.Error("expected curated description to override raw body")
	}
	if entries[0].Category == "" {
		t.Error("expected non-empty category from curated metadata")
	}
}

// Compile-time check: DispatchingClient implements Client (already present in dispatch_test.go,
// but also verify HTTPClient and MockClient include FetchCatalogFCOS).
var _ bakery.Client = (*bakery.MockClient)(nil)
