package bakery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCatalogFCOS_Basic(t *testing.T) {
	releases := []githubRelease{
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Tailscale sysext",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-44-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/tailscale-0-1.98.3-1-44-x86-64.raw"},
				{Name: "SHA256SUMS", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/SHA256SUMS"},
			},
		},
		{
			TagName: "tailscale-0-1.98.3-1-43-x86-64",
			Body:    "Tailscale sysext for f43",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-43-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-43-x86-64/tailscale-0-1.98.3-1-43-x86-64.raw"},
			},
		},
		{
			TagName: "docker-ce-3-29.5.1-1.fc44-44-x86-64",
			Body:    "Docker CE",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "docker-ce-3-29.5.1-1.fc44-44-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-29.5.1-1.fc44-44-x86-64/docker-ce-3-29.5.1-1.fc44-44-x86-64.raw"},
			},
		},
		// Meta-release (no version) — should be skipped
		{
			TagName: "tailscale",
			Body:    "Latest tailscale",
			Assets:  nil,
		},
		// Wrong Fedora version — should be filtered out
		{
			TagName: "glab-1.97.0-1-43-x86-64",
			Body:    "glab",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "glab-1.97.0-1-43-x86-64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/glab-1.97.0-1-43-x86-64/glab-1.97.0-1-43-x86-64.raw"},
			},
		},
		// arm64 — should be filtered out when requesting amd64
		{
			TagName: "tailscale-0-1.98.3-1-44-arm64",
			Body:    "Tailscale arm64",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-44-arm64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-arm64/tailscale-0-1.98.3-1-44-arm64.raw"},
			},
		},
	}

	data, _ := json.Marshal(releases)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	client := NewFCOSClientWithURL(srv.URL, 44)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (tailscale + docker-ce for f44/x86-64)", len(entries))
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

func TestFetchCatalogFCOS_Dedup(t *testing.T) {
	releases := []githubRelease{
		{
			TagName: "glab-1.97.0-1-44-x86-64",
			Body:    "newer",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "glab-1.97.0-1-44-x86-64.raw", BrowserDownloadURL: "https://example.com/glab-new.raw"},
			},
		},
		{
			TagName: "glab-1.96.0-1-44-x86-64",
			Body:    "older",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "glab-1.96.0-1-44-x86-64.raw", BrowserDownloadURL: "https://example.com/glab-old.raw"},
			},
		},
	}

	data, _ := json.Marshal(releases)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	client := NewFCOSClientWithURL(srv.URL, 44)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (deduplicated by name)", len(entries))
	}
	if entries[0].Version != "1.97.0-1" {
		t.Errorf("version = %q, want newer version 1.97.0-1", entries[0].Version)
	}
}

func TestFetchCatalogFCOS_UnsupportedArch(t *testing.T) {
	client := NewFCOSClient(44)
	_, err := client.FetchCatalogFCOS(context.Background(), "mips", 44)
	if err == nil {
		t.Fatal("expected error for unsupported architecture")
	}
}

func TestFetchCatalogFCOS_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewFCOSClientWithURL(srv.URL, 44)
	_, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestFetchCatalogFCOS_CuratedDescriptions(t *testing.T) {
	releases := []githubRelease{
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Raw body text",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "tailscale-0-1.98.3-1-44-x86-64.raw", BrowserDownloadURL: "https://example.com/tailscale.raw"},
			},
		},
	}

	data, _ := json.Marshal(releases)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	client := NewFCOSClientWithURL(srv.URL, 44)
	entries, err := client.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	// Should use curated description, not raw body text
	if entries[0].Description == "Raw body text" {
		t.Error("expected curated description to override raw body")
	}
	if entries[0].Category != "Networking" {
		t.Errorf("category = %q, want Networking", entries[0].Category)
	}
}

func TestFCOSLookup(t *testing.T) {
	meta, ok := FCOSLookup("docker-ce")
	if !ok {
		t.Fatal("expected docker-ce to be in FCOS catalog")
	}
	if meta.Category != "Container Runtime" {
		t.Errorf("category = %q, want Container Runtime", meta.Category)
	}

	_, ok = FCOSLookup("nonexistent-extension")
	if ok {
		t.Error("expected nonexistent extension to not be in catalog")
	}
}

func TestMaxFCOSCatalogPages(t *testing.T) {
	if maxFCOSCatalogPages < maxCatalogPages {
		t.Errorf("maxFCOSCatalogPages (%d) must be >= maxCatalogPages (%d)", maxFCOSCatalogPages, maxCatalogPages)
	}
}
