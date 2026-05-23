package bakery_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
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

	// Verify first entry — description is now overridden by curated catalog.
	if entries[0].Name != "docker" {
		t.Errorf("expected name 'docker', got %q", entries[0].Name)
	}
	// The curated Short description must override the raw GitHub body.
	if entries[0].Description == "Docker container runtime sysext for Flatcar" {
		t.Error("description should be curated text, not the raw GitHub release body")
	}
	if entries[0].Description == "" {
		t.Error("description must not be empty after catalog enrichment")
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

	// Verify third entry — description is overridden by curated catalog.
	if entries[2].Name != "tailscale" {
		t.Errorf("expected name 'tailscale', got %q", entries[2].Name)
	}
	if entries[2].Description == "Tailscale mesh VPN" {
		t.Error("tailscale description should be curated text, not raw GitHub body")
	}
	if entries[2].Description == "" {
		t.Error("tailscale description must not be empty")
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

func TestFetchCatalogResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Send more than 5MB — triggers the size limit guard
		payload := make([]byte, (5<<20)+1)
		for i := range payload {
			payload[i] = 'x'
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	_, err := client.FetchCatalog(context.Background())
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !strings.Contains(err.Error(), "5MB") {
		t.Errorf("expected size-limit error message, got: %v", err)
	}
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

func TestFetchCatalogArch_Arm64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockGitHubReleasesJSON))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalogArch(context.Background(), "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// docker has an arm64 asset in the fixture — should appear
	found := false
	for _, e := range entries {
		if e.Name == "docker" {
			found = true
			if !strings.Contains(e.URL, "arm64") {
				t.Errorf("docker entry URL should contain arm64, got %q", e.URL)
			}
		}
	}
	if !found {
		t.Error("docker should appear in arm64 catalog (has arm64.raw asset)")
	}

	// tailscale only has x86-64 — should be absent from arm64 catalog
	for _, e := range entries {
		if e.Name == "tailscale" {
			t.Error("tailscale should not appear in arm64 catalog (no arm64.raw asset in fixture)")
		}
	}
}

func TestFetchCatalogArch_InvalidArch(t *testing.T) {
	client := bakery.NewHTTPClientWithURL("http://unused")
	_, err := client.FetchCatalogArch(context.Background(), "mips")
	if err == nil {
		t.Fatal("expected error for unsupported arch")
	}
}

func TestMockClientFetchCatalogArch(t *testing.T) {
	entries := []model.SysextEntry{
		{Name: "docker", URL: "https://example.com/docker-x86-64.raw"},
		{Name: "containerd", URL: "https://example.com/containerd-arm64.raw"},
		{Name: "plain", URL: "https://example.com/plain.raw"}, // no arch suffix
	}

	m := &bakery.MockClient{Entries: entries}

	// amd64: gets x86-64 entries and arch-neutral entries
	amd64, err := m.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("amd64: unexpected error: %v", err)
	}
	names := make(map[string]bool)
	for _, e := range amd64 {
		names[e.Name] = true
	}
	if !names["docker"] {
		t.Error("amd64 catalog should contain docker (x86-64 URL)")
	}
	if names["containerd"] {
		t.Error("amd64 catalog should not contain containerd (arm64 URL)")
	}
	if !names["plain"] {
		t.Error("amd64 catalog should contain plain (no arch suffix)")
	}

	// arm64: gets arm64 entries and arch-neutral entries
	arm64, err := m.FetchCatalogArch(context.Background(), "arm64")
	if err != nil {
		t.Fatalf("arm64: unexpected error: %v", err)
	}
	names = make(map[string]bool)
	for _, e := range arm64 {
		names[e.Name] = true
	}
	if !names["containerd"] {
		t.Error("arm64 catalog should contain containerd (arm64 URL)")
	}
	if names["docker"] {
		t.Error("arm64 catalog should not contain docker (x86-64 URL)")
	}
	if !names["plain"] {
		t.Error("arm64 catalog should contain plain (no arch suffix in URL)")
	}

	// error path
	merr := &bakery.MockClient{Err: errors.New("boom")}
	_, err = merr.FetchCatalogArch(context.Background(), "amd64")
	if err == nil {
		t.Error("expected error from MockClient with Err set")
	}

	// empty entries — FetchCatalogArch should return nil, nil
	empty := &bakery.MockClient{}
	got, err := empty.FetchCatalogArch(context.Background(), "amd64")
	if err != nil || len(got) != 0 {
		t.Errorf("empty mock: got %v %v, want nil nil", got, err)
	}
}

func TestFetchCatalogPagination(t *testing.T) {
	page1 := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://example.com/docker.raw"}]}]`
	page2 := `[{"tag_name":"btop-1.4.0","body":"btop monitor","assets":[{"name":"btop-1.4.0-x86-64.raw","browser_download_url":"https://example.com/btop.raw"}]}]`

	var mu sync.Mutex
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		call := callCount
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			// Return page 1 with a Link header pointing to page 2.
			nextURL := fmt.Sprintf("http://%s%s?page=2", r.Host, r.URL.Path)
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next", <%s>; rel="last"`, nextURL, nextURL))
			_, _ = w.Write([]byte(page1))
		} else {
			// Page 2 — no Link header, no more pages.
			_, _ = w.Write([]byte(page2))
		}
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (one per page), got %d", callCount)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (1 per page), got %d", len(entries))
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["docker"] {
		t.Error("expected docker entry from page 1")
	}
	if !names["btop"] {
		t.Error("expected btop entry from page 2")
	}
}

func TestFetchCatalogEnrichment(t *testing.T) {
	// The curated catalog must override the GitHub release body for known extensions.
	payload := `[{"tag_name":"kubernetes-v1.36.1","body":"raw github body text","assets":[{"name":"kubernetes-v1.36.1-x86-64.raw","browser_download_url":"https://example.com/k8s.raw"}]}]`

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
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Description == "raw github body text" {
		t.Error("description should be overridden by curated catalog, not raw body")
	}
	if e.Description == "" {
		t.Error("description must not be empty after enrichment")
	}
	if e.Category == "" {
		t.Error("category must be populated from curated catalog")
	}
	if e.SupportTier == "" {
		t.Error("support tier must be populated from curated catalog")
	}
	if e.SupportTier != bakery.TierIntegrated {
		t.Errorf("kubernetes should be %q, got %q", bakery.TierIntegrated, e.SupportTier)
	}
}

func TestParseLinkNext(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		wantURL string
		wantOK  bool
	}{
		{
			name:    "empty header",
			header:  "",
			wantURL: "", wantOK: false,
		},
		{
			name:    "next and last",
			header:  `<https://api.github.com/repos/x/y/releases?page=2>; rel="next", <https://api.github.com/repos/x/y/releases?page=5>; rel="last"`,
			wantURL: "https://api.github.com/repos/x/y/releases?page=2", wantOK: true,
		},
		{
			name:    "last only — no next",
			header:  `<https://api.github.com/repos/x/y/releases?page=5>; rel="last"`,
			wantURL: "", wantOK: false,
		},
		{
			name:    "next only",
			header:  `<https://api.github.com/repos/x/y/releases?page=3>; rel="next"`,
			wantURL: "https://api.github.com/repos/x/y/releases?page=3", wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// parseLinkNext is unexported; test it indirectly via pagination behaviour.
			// We verify the URL and ok by checking that a paginated fetch follows the
			// correct URL — done in TestFetchCatalogPagination above.
			// This test documents expected behaviour for future maintenance.
			_ = tt.header
			_ = tt.wantURL
			_ = tt.wantOK
		})
	}
}

func TestFetchSHA256ForAsset(t *testing.T) {
	sha256Content := `e5336201eedf0c5e7620c6947c821009c362231f7c9023174b9c4f99a1f0ad1b  wasmtime-v44.0.1-x86-64.raw
b9f7ded92635349c405f833d51d7d268cc81062d081960d3c15870c9262e6df2  wasmtime-v44.0.1-arm64.raw
`
	sha256Payload := `[{
		"tag_name": "wasmtime-v44.0.1",
		"body": "wasmtime release",
		"assets": [
			{"name": "wasmtime-v44.0.1-x86-64.raw", "browser_download_url": "https://BASEURL/wasmtime-v44.0.1-x86-64.raw"},
			{"name": "SHA256SUMS", "browser_download_url": "https://BASEURL/SHA256SUMS"}
		]
	}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if r.URL.Path == "/SHA256SUMS" {
			_, _ = w.Write([]byte(sha256Content))
			return
		}
		// Main catalog endpoint — replace placeholder with real server URL
		catalog := strings.ReplaceAll(sha256Payload, "https://BASEURL", "http://"+r.Host)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(catalog))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Sha256 == "" {
		t.Error("SHA256 hash should be populated from SHA256SUMS file")
	}
	const wantHash = "e5336201eedf0c5e7620c6947c821009c362231f7c9023174b9c4f99a1f0ad1b"
	if e.Sha256 != wantHash {
		t.Errorf("expected SHA256 %q, got %q", wantHash, e.Sha256)
	}
}

func TestFetchSHA256MissingAsset(t *testing.T) {
	// Release without a SHA256SUMS asset — entry should be returned without hash.
	payload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://example.com/docker.raw"}]}]`

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
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Sha256 must be empty when no SHA256SUMS asset is present (soft fail).
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty Sha256 when no SHA256SUMS asset, got %q", entries[0].Sha256)
	}
}

func TestNewHTTPClient(t *testing.T) {
	c := bakery.NewHTTPClient()
	if c == nil {
		t.Fatal("NewHTTPClient() returned nil")
	}
	if c.CatalogURL != bakery.DefaultCatalogURL {
		t.Errorf("CatalogURL = %q, want %q", c.CatalogURL, bakery.DefaultCatalogURL)
	}
	if c.HTTP == nil {
		t.Error("HTTP client field is nil")
	}
}

func TestParseTagName_NoMatch(t *testing.T) {
	// A tag with no "-vN" and no "-N" pattern → both loops exhaust → ("", "")
	name, version := bakery.ParseTagName("nodashes")
	if name != "" || version != "" {
		t.Errorf("ParseTagName(%q) = (%q, %q), want (\"\", \"\")", "nodashes", name, version)
	}
}
