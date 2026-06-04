package bakery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

func TestFCOSClient_FetchCatalogArch(t *testing.T) {
	releases := `[
		{
			"tag_name": "tailscale-0-1.98.4-1-44-x86-64",
			"body": "Tailscale VPN",
			"assets": [
				{"name": "SHA256SUMS", "browser_download_url": "https://example.com/SHA256SUMS"},
				{"name": "tailscale-0-1.98.4-1-44-x86-64.raw", "browser_download_url": "https://example.com/tailscale.raw"}
			]
		},
		{
			"tag_name": "tailscale-0-1.98.4-1-44-arm64",
			"body": "Tailscale VPN arm64",
			"assets": [
				{"name": "tailscale-0-1.98.4-1-44-arm64.raw", "browser_download_url": "https://example.com/tailscale-arm64.raw"}
			]
		},
		{
			"tag_name": "tailscale-0-1.98.4-1-43-x86-64",
			"body": "Tailscale VPN Fedora 43",
			"assets": [
				{"name": "tailscale-0-1.98.4-1-43-x86-64.raw", "browser_download_url": "https://example.com/tailscale-f43.raw"}
			]
		},
		{
			"tag_name": "docker-ce-3-29.5.2-1.fc44-44-x86-64",
			"body": "Docker CE",
			"assets": [
				{"name": "docker-ce-3-29.5.2-1.fc44-44-x86-64.raw", "browser_download_url": "https://example.com/docker.raw"}
			]
		},
		{
			"tag_name": "tailscale",
			"body": "Index tag",
			"assets": []
		},
		{
			"tag_name": "latest",
			"body": "Latest index",
			"assets": []
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, releases)
	}))
	defer srv.Close()

	client := &bakery.FCOSClient{
		CatalogURL:    srv.URL,
		HTTP:          srv.Client(),
		FedoraVersion: 44,
	}

	t.Run("amd64 filters by arch and fedora version", func(t *testing.T) {
		entries, err := client.FetchCatalogArch(context.Background(), "amd64")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries (tailscale + docker-ce), got %d: %v", len(entries), entryNames(entries))
		}
		names := entryNames(entries)
		if !strings.Contains(names, "tailscale") || !strings.Contains(names, "docker-ce") {
			t.Errorf("expected tailscale and docker-ce, got %s", names)
		}
	})

	t.Run("arm64 filters correctly", func(t *testing.T) {
		entries, err := client.FetchCatalogArch(context.Background(), "arm64")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 || entries[0].Name != "tailscale" {
			t.Fatalf("expected 1 tailscale entry, got %d: %v", len(entries), entryNames(entries))
		}
	})

	t.Run("unsupported arch", func(t *testing.T) {
		_, err := client.FetchCatalogArch(context.Background(), "mips")
		if err == nil {
			t.Fatal("expected error for unsupported arch")
		}
	})

	t.Run("FetchCatalog defaults to amd64", func(t *testing.T) {
		entries, err := client.FetchCatalog(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
	})
}

func TestFCOSClient_DeduplicatesByName(t *testing.T) {
	releases := `[
		{
			"tag_name": "tailscale-0-1.98.4-1-44-x86-64",
			"body": "newer",
			"assets": [{"name": "tailscale.raw", "browser_download_url": "https://example.com/new.raw"}]
		},
		{
			"tag_name": "tailscale-0-1.98.3-1-44-x86-64",
			"body": "older",
			"assets": [{"name": "tailscale.raw", "browser_download_url": "https://example.com/old.raw"}]
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, releases)
	}))
	defer srv.Close()

	client := &bakery.FCOSClient{
		CatalogURL:    srv.URL,
		HTTP:          srv.Client(),
		FedoraVersion: 44,
	}

	entries, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 deduplicated entry, got %d", len(entries))
	}
	if entries[0].Version != "0-1.98.4-1" {
		t.Errorf("expected newest version 0-1.98.4-1, got %s", entries[0].Version)
	}
}

func TestFCOSClient_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := &bakery.FCOSClient{
		CatalogURL:    srv.URL,
		HTTP:          srv.Client(),
		FedoraVersion: 44,
	}

	_, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestFCOSClient_AppliesCuratedDescriptions(t *testing.T) {
	releases := `[
		{
			"tag_name": "tailscale-0-1.98.4-1-44-x86-64",
			"body": "raw body text",
			"assets": [{"name": "tailscale.raw", "browser_download_url": "https://example.com/ts.raw"}]
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, releases)
	}))
	defer srv.Close()

	client := &bakery.FCOSClient{
		CatalogURL:    srv.URL,
		HTTP:          srv.Client(),
		FedoraVersion: 44,
	}

	entries, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description == "raw body text" {
		t.Error("expected curated description, got raw body")
	}
	if entries[0].Category != "Networking" {
		t.Errorf("expected category Networking, got %q", entries[0].Category)
	}
}

func entryNames(entries []model.SysextEntry) string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return strings.Join(names, ", ")
}
