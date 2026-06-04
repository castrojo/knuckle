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
	"github.com/projectbluefin/knuckle/internal/model"
)

// fcosMockRelease is a minimal GitHub release as returned by the API.
type fcosMockRelease struct {
	TagName string          `json:"tag_name"`
	Body    string          `json:"body"`
	Assets  []fcosMockAsset `json:"assets"`
}

type fcosMockAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func makeFCOSReleasesJSON(releases []fcosMockRelease) string {
	b, _ := json.Marshal(releases)
	return string(b)
}

// buildFCOSClient creates a bakery.HTTPClient with the CatalogURL pointed at the
// test server and no auth token.
func buildFCOSClient(serverURL string) *bakery.HTTPClient {
	c := bakery.NewFCOSHTTPClient()
	c.CatalogURL = serverURL
	c.AuthToken = ""
	return c
}

// TestFetchCatalogFCOS_BasicFiltering verifies that only releases matching the
// requested arch AND fedoraVersion are returned, and that deduplication (newest
// wins) is applied.
func TestFetchCatalogFCOS_BasicFiltering(t *testing.T) {
	releases := []fcosMockRelease{
		// Match: docker-ce, x86-64, fedora 44 (newest)
		{
			TagName: "docker-ce-3-29.5.1-1.fc44-44-x86-64",
			Body:    "Docker CE for Fedora 44",
			Assets: []fcosMockAsset{
				{Name: "docker-ce-29.5.1-1.fc44.x86_64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-29.5.1-1.fc44-44-x86-64/docker-ce-29.5.1-1.fc44.x86_64.raw"},
			},
		},
		// Should be deduped (older docker-ce for same arch+fedora)
		{
			TagName: "docker-ce-3-28.0.0-1.fc44-44-x86-64",
			Body:    "Older Docker CE",
			Assets: []fcosMockAsset{
				{Name: "docker-ce-28.0.0-1.fc44.x86_64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-28.0.0-1.fc44-44-x86-64/docker-ce-28.0.0-1.fc44.x86_64.raw"},
			},
		},
		// Wrong fedora version (45 vs 44 requested)
		{
			TagName: "tailscale-0-1.98.3-1-45-x86-64",
			Body:    "Tailscale for Fedora 45",
			Assets: []fcosMockAsset{
				{Name: "tailscale-1.98.3-1.x86_64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-45-x86-64/tailscale-1.98.3-1.x86_64.raw"},
			},
		},
		// Wrong arch (arm64 vs amd64 requested)
		{
			TagName: "tailscale-0-1.98.3-1-44-arm64",
			Body:    "Tailscale for Fedora 44 arm64",
			Assets: []fcosMockAsset{
				{Name: "tailscale-1.98.3-1.aarch64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-arm64/tailscale-1.98.3-1.aarch64.raw"},
			},
		},
		// Match: tailscale x86-64 fedora 44
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Tailscale for Fedora 44",
			Assets: []fcosMockAsset{
				{Name: "tailscale-1.98.3-1.x86_64.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/tailscale-1.98.3-1.x86_64.raw"},
			},
		},
		// Alias tag — should be skipped
		{
			TagName: "tailscale",
			Body:    "alias",
			Assets:  []fcosMockAsset{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeFCOSReleasesJSON(releases))
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (docker-ce + tailscale), got %d: %v", len(entries), entries)
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["docker-ce"] {
		t.Error("expected docker-ce in entries")
	}
	if !names["tailscale"] {
		t.Error("expected tailscale in entries")
	}
}

func TestFetchCatalogFCOS_EnrichesWithCuratedMetadata(t *testing.T) {
	releases := []fcosMockRelease{
		{
			TagName: "docker-ce-3-29.5.1-1.fc44-44-x86-64",
			Body:    "raw body",
			Assets: []fcosMockAsset{
				{Name: "docker-ce.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-29.5.1-1.fc44-44-x86-64/docker-ce.raw"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeFCOSReleasesJSON(releases))
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Category == "" {
		t.Error("expected curated category for docker-ce, got empty")
	}
	if e.SupportTier == "" {
		t.Error("expected curated support tier for docker-ce, got empty")
	}
	if strings.Contains(e.Description, "raw body") {
		t.Errorf("expected curated description, got raw body: %q", e.Description)
	}
}

func TestFetchCatalogFCOS_UnknownExtensionUsesRawBody(t *testing.T) {
	releases := []fcosMockRelease{
		{
			TagName: "unknowntool-0-1.0.0-1-44-x86-64",
			Body:    "An unknown tool description",
			Assets: []fcosMockAsset{
				{Name: "unknowntool.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/unknowntool-0-1.0.0-1-44-x86-64/unknowntool.raw"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeFCOSReleasesJSON(releases))
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description == "" {
		t.Error("expected non-empty description for unknown tool")
	}
}

func TestFetchCatalogFCOS_InvalidArch(t *testing.T) {
	c := &bakery.HTTPClient{CatalogURL: "http://unused", HTTP: http.DefaultClient}
	_, err := c.FetchCatalogFCOS(context.Background(), "riscv64", 44)
	if err == nil {
		t.Fatal("expected error for unsupported arch")
	}
	if !strings.Contains(err.Error(), "unsupported architecture") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchCatalogFCOS_InvalidFedoraVersion(t *testing.T) {
	c := &bakery.HTTPClient{CatalogURL: "http://unused", HTTP: http.DefaultClient}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 0)
	if err == nil {
		t.Fatal("expected error for fedoraVersion=0")
	}
	if !strings.Contains(err.Error(), "invalid fedoraVersion") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchCatalogFCOS_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]")
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty response, got %d", len(entries))
	}
}

func TestFetchCatalogFCOS_Pagination(t *testing.T) {
	page1 := []fcosMockRelease{
		{
			TagName: "docker-ce-3-29.5.1-1.fc44-44-x86-64",
			Body:    "Docker CE",
			Assets: []fcosMockAsset{
				{Name: "docker-ce.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-29.5.1-1.fc44-44-x86-64/docker-ce.raw"},
			},
		},
	}
	page2 := []fcosMockRelease{
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Tailscale",
			Assets: []fcosMockAsset{
				{Name: "tailscale.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/tailscale.raw"},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "page=2") {
			fmt.Fprint(w, makeFCOSReleasesJSON(page2))
			return
		}
		// First page — add Link header pointing to page 2.
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, "http://"+r.Host+r.URL.Path))
		fmt.Fprint(w, makeFCOSReleasesJSON(page1))
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries across 2 pages, got %d: %v", len(entries), entries)
	}
}

func TestFetchCatalogFCOS_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	c := buildFCOSClient(srv.URL)
	_, err := c.FetchCatalogFCOS(ctx, "amd64", 44)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchCatalogFCOS_Arm64Filtering(t *testing.T) {
	releases := []fcosMockRelease{
		{
			TagName: "tailscale-0-1.98.3-1-44-arm64",
			Body:    "Tailscale arm64",
			Assets: []fcosMockAsset{
				{Name: "tailscale.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-arm64/tailscale.raw"},
			},
		},
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Tailscale x86",
			Assets: []fcosMockAsset{
				{Name: "tailscale.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/tailscale.raw"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeFCOSReleasesJSON(releases))
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "arm64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 arm64 entry, got %d", len(entries))
	}
}

func TestFetchCatalogFCOS_NoRawAssetSkipped(t *testing.T) {
	releases := []fcosMockRelease{
		{
			TagName: "docker-ce-3-29.5.1-1.fc44-44-x86-64",
			Body:    "Docker CE",
			Assets:  []fcosMockAsset{}, // no .raw asset
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeFCOSReleasesJSON(releases))
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when no .raw asset, got %d", len(entries))
	}
}

// TestFetchCatalogFCOS_ImplementsFCOSClientInterface verifies compile-time.
var _ bakery.FCOSClient = (*bakery.HTTPClient)(nil)

// TestFetchCatalogFCOS_SetsVersionField verifies that the parsed version is stored.
func TestFetchCatalogFCOS_SetsVersionField(t *testing.T) {
	releases := []fcosMockRelease{
		{
			TagName: "tailscale-0-1.98.3-1-44-x86-64",
			Body:    "Tailscale",
			Assets: []fcosMockAsset{
				{Name: "tailscale.raw", BrowserDownloadURL: "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.98.3-1-44-x86-64/tailscale.raw"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeFCOSReleasesJSON(releases))
	}))
	defer srv.Close()

	c := buildFCOSClient(srv.URL)
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("FetchCatalogFCOS: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Version == "" {
		t.Error("expected non-empty version field")
	}
}

// Ensure that HTTPClient.CatalogURL and HTTPClient.AuthToken fields are accessible.
var _ = (*bakery.HTTPClient)(nil)

// TestNewFCOSHTTPClient verifies the constructor sets a non-empty catalog URL.
func TestNewFCOSHTTPClient(t *testing.T) {
	c := bakery.NewFCOSHTTPClient()
	if c.CatalogURL == "" {
		t.Error("expected non-empty CatalogURL")
	}
	if c.HTTP == nil {
		t.Error("expected non-nil HTTP client")
	}
}

// TestLookupFCOS_KnownName verifies curated metadata lookup.
func TestLookupFCOS_KnownName(t *testing.T) {
	meta, ok := bakery.LookupFCOS("docker-ce")
	if !ok {
		t.Fatal("expected docker-ce to be in FCOS catalog")
	}
	if meta.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if meta.Category == "" {
		t.Error("expected non-empty Category")
	}
	if meta.SupportTier == "" {
		t.Error("expected non-empty SupportTier")
	}
}

func TestLookupFCOS_UnknownName(t *testing.T) {
	_, ok := bakery.LookupFCOS("notareal-extension")
	if ok {
		t.Fatal("expected unknown extension to return ok=false")
	}
}

// TestFCOSCatalogURL_HasExpectedHost verifies the catalog URL points at GitHub.
func TestFCOSCatalogURL_HasExpectedHost(t *testing.T) {
	if !strings.Contains(bakery.FCOSCatalogURL, "api.github.com") {
		t.Errorf("expected github API URL, got %q", bakery.FCOSCatalogURL)
	}
	if !strings.Contains(bakery.FCOSCatalogURL, "fedora-sysexts") {
		t.Errorf("expected fedora-sysexts in URL, got %q", bakery.FCOSCatalogURL)
	}
}

// modelSysextEntry is a compile-time sanity check.
var _ model.SysextEntry = model.SysextEntry{}
