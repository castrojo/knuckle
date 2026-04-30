package bakery_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/castrojo/knuckle/internal/bakery"
	"github.com/castrojo/knuckle/internal/model"
)

const mockGitHubReleasesJSON = `[
  {
    "tag_name": "docker-v24.0.7",
    "body": "Docker container runtime sysext for Flatcar",
    "assets": [
      {"name": "docker-24.0.7-x86-64.raw", "browser_download_url": "https://github.com/flatcar/sysext-bakery/releases/download/docker-v24.0.7/docker-24.0.7-x86-64.raw"},
      {"name": "docker-24.0.7-arm64.raw", "browser_download_url": "https://github.com/flatcar/sysext-bakery/releases/download/docker-v24.0.7/docker-24.0.7-arm64.raw"}
    ]
  },
  {
    "tag_name": "wasmcloud-v0.82.0",
    "body": "wasmCloud runtime for WebAssembly workloads",
    "assets": [
      {"name": "wasmcloud-0.82.0-x86-64.raw", "browser_download_url": "https://github.com/flatcar/sysext-bakery/releases/download/wasmcloud-v0.82.0/wasmcloud-0.82.0-x86-64.raw"}
    ]
  },
  {
    "tag_name": "tailscale-v1.56.1",
    "body": "Tailscale mesh VPN",
    "assets": [
      {"name": "tailscale-1.56.1-x86-64.raw", "browser_download_url": "https://github.com/flatcar/sysext-bakery/releases/download/tailscale-v1.56.1/tailscale-1.56.1-x86-64.raw"}
    ]
  },
  {
    "tag_name": "docker-v23.0.0",
    "body": "Older docker release (should be deduplicated)",
    "assets": [
      {"name": "docker-23.0.0-x86-64.raw", "browser_download_url": "https://github.com/flatcar/sysext-bakery/releases/download/docker-v23.0.0/docker-23.0.0-x86-64.raw"}
    ]
  }
]`

func TestFetchCatalogSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "knuckle/1.0" {
			t.Errorf("expected User-Agent 'knuckle/1.0', got %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockGitHubReleasesJSON))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be 3 (docker deduplicated, older release skipped)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify first entry
	if entries[0].Name != "docker" {
		t.Errorf("expected name 'docker', got %q", entries[0].Name)
	}
	if entries[0].Description != "Docker container runtime sysext for Flatcar" {
		t.Errorf("unexpected description: %q", entries[0].Description)
	}
	if entries[0].Version != "24.0.7" {
		t.Errorf("expected version '24.0.7', got %q", entries[0].Version)
	}
	if entries[0].URL != "https://github.com/flatcar/sysext-bakery/releases/download/docker-v24.0.7/docker-24.0.7-x86-64.raw" {
		t.Errorf("unexpected URL: %q", entries[0].URL)
	}
	if entries[0].Selected != false {
		t.Errorf("expected Selected=false")
	}

	// Verify second entry
	if entries[1].Name != "wasmcloud" {
		t.Errorf("expected name 'wasmcloud', got %q", entries[1].Name)
	}
	if entries[1].Version != "0.82.0" {
		t.Errorf("expected version '0.82.0', got %q", entries[1].Version)
	}

	// Verify third entry
	if entries[2].Name != "tailscale" {
		t.Errorf("expected name 'tailscale', got %q", entries[2].Name)
	}
	if entries[2].Description != "Tailscale mesh VPN" {
		t.Errorf("expected description 'Tailscale mesh VPN', got %q", entries[2].Description)
	}
}

func TestFetchCatalogSkipsReleasesWithoutX86Asset(t *testing.T) {
	payload := `[{
		"tag_name": "myext-v1.0.0",
		"body": "No x86 asset here",
		"assets": [{"name": "myext-1.0.0-arm64.raw", "browser_download_url": "https://example.com/arm64.raw"}]
	}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (no x86-64 asset), got %d", len(entries))
	}
}

func TestFetchCatalogHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	_, err := client.FetchCatalog(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if got := err.Error(); got != "catalog returned status 500" {
		t.Errorf("unexpected error message: %q", got)
	}
}

func TestFetchCatalogInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not valid json at all`))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	_, err := client.FetchCatalog(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFetchCatalogTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.FetchCatalog(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestMockClient(t *testing.T) {
	t.Run("returns entries", func(t *testing.T) {
		expected := []model.SysextEntry{
			{Name: "docker", Description: "Docker", Version: "24.0.7", URL: "https://example.com/docker.raw", Selected: true},
		}
		mock := &bakery.MockClient{Entries: expected}

		entries, err := mock.FetchCatalog(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Name != "docker" {
			t.Errorf("expected 'docker', got %q", entries[0].Name)
		}
	})

	t.Run("returns error", func(t *testing.T) {
		expectedErr := errors.New("network failure")
		mock := &bakery.MockClient{Err: expectedErr}

		_, err := mock.FetchCatalog(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err != expectedErr {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})
}

func TestParseTagName(t *testing.T) {
	tests := []struct {
		tag     string
		name    string
		version string
	}{
		{"docker-v24.0.7", "docker", "24.0.7"},
		{"tailscale-v1.56.1", "tailscale", "1.56.1"},
		{"my-ext-v2.0.0", "my-ext", "2.0.0"},
		{"simple-1.0.0", "simple", "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			name, version := bakery.ParseTagName(tt.tag)
			if name != tt.name {
				t.Errorf("name: got %q, want %q", name, tt.name)
			}
			if version != tt.version {
				t.Errorf("version: got %q, want %q", version, tt.version)
			}
		})
	}
}
