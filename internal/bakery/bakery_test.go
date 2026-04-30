package bakery_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/castrojo/knuckle/internal/bakery"
	"github.com/castrojo/knuckle/internal/model"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestFetchCatalogSuccess(t *testing.T) {
	fixture, err := os.ReadFile(testdataPath("bakery_catalog.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify first entry
	if entries[0].Name != "docker" {
		t.Errorf("expected name 'docker', got %q", entries[0].Name)
	}
	if entries[0].Description != "Docker container runtime" {
		t.Errorf("expected description 'Docker container runtime', got %q", entries[0].Description)
	}
	if entries[0].Version != "24.0.7" {
		t.Errorf("expected version '24.0.7', got %q", entries[0].Version)
	}
	if entries[0].URL != "https://bakery.flatcar.org/sysext/docker-24.0.7.raw" {
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
