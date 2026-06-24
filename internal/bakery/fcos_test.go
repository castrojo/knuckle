package bakery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeFCOSRelease builds a minimal JSON release object for the test server.
func makeFCOSRelease(tagName, body string, assets []struct{ Name, URL string }) map[string]any {
	assetList := make([]map[string]any, len(assets))
	for i, a := range assets {
		assetList[i] = map[string]any{"name": a.Name, "browser_download_url": a.URL}
	}
	return map[string]any{"tag_name": tagName, "body": body, "assets": assetList}
}

// fcosCatalogServer returns an httptest.Server that serves a single-page
// FCOS catalog containing the given releases.  The server's URL is returned
// as baseURL so callers can point FCOSClient or HTTPClient at it.
func fcosCatalogServer(t *testing.T, releases []any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("fcosCatalogServer: encode error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fcosSingleRelease constructs one release that passes all filters.
func fcosSingleRelease(t *testing.T, name, version string, fedora int, arch string, rawURL, sha256URL string) map[string]any {
	t.Helper()
	var archSuffix string
	if arch == "amd64" {
		archSuffix = "x86-64"
	} else {
		archSuffix = arch
	}
	tag := fmt.Sprintf("%s-%s-%d-%s", name, version, fedora, archSuffix)
	return makeFCOSRelease(tag, "release body", []struct{ Name, URL string }{
		{Name: tag + ".raw", URL: rawURL},
		{Name: "SHA256SUMS", URL: sha256URL},
	})
}

// ── FetchCatalogFCOS ──────────────────────────────────────────────────────────

func TestFetchCatalogFCOS_Success(t *testing.T) {
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"

	sha256Body := "aabbccdd" + strings.Repeat("0", 56) + "  tailscale-0-1.2.3-44-x86-64.raw\n"

	// Two servers: one for the catalog, one for SHA256SUMS.
	sha256Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sha256Body))
	}))
	defer sha256Srv.Close()

	// Repoint the SHA256SUMS URL to our test server.
	sha256TestURL := sha256Srv.URL + "/sha256sums"
	rawRel := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "amd64", rawURL, sha256TestURL)

	srv := fcosCatalogServer(t, []any{rawRel})

	c := NewHTTPClientWithURL(srv.URL)
	c.HTTP = srv.Client()

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Name != "tailscale" {
		t.Errorf("name: got %q, want tailscale", e.Name)
	}
	if e.Version != "0-1.2.3" {
		t.Errorf("version: got %q, want 0-1.2.3", e.Version)
	}
	if e.URL != rawURL {
		t.Errorf("URL: got %q, want %q", e.URL, rawURL)
	}
}

func TestFetchCatalogFCOS_BadArch(t *testing.T) {
	c := NewHTTPClientWithURL("http://localhost:1")
	_, err := c.FetchCatalogFCOS(context.Background(), "riscv64", 44)
	if err == nil || !strings.Contains(err.Error(), "unsupported architecture") {
		t.Errorf("expected unsupported architecture error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_NetworkError(t *testing.T) {
	c := &HTTPClient{
		CatalogURL: "http://127.0.0.1:1",
		HTTP:       &http.Client{},
	}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil {
		t.Fatal("expected network error")
	}
	if !strings.Contains(err.Error(), "fetching FCOS catalog") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFetchCatalogFCOS_HTTPNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Errorf("expected HTTP 429 error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil || !strings.Contains(err.Error(), "parsing FCOS catalog JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_EmptyPageStops(t *testing.T) {
	var pageCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&pageCount, 1)
		w.Header().Set("Content-Type", "application/json")
		// Return empty JSON array — pagination should stop.
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	if pageCount != 1 {
		t.Errorf("expected exactly 1 page fetch, got %d", pageCount)
	}
}

func TestFetchCatalogFCOS_Deduplication(t *testing.T) {
	// Two releases for "tailscale", same arch and fedora version — only first should appear.
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"
	rawURL2 := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.2-44-x86-64/tailscale-0-1.2.2-44-x86-64.raw"

	rel1 := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "amd64", rawURL, "")
	rel2 := fcosSingleRelease(t, "tailscale", "0-1.2.2", 44, "amd64", rawURL2, "")

	srv := fcosCatalogServer(t, []any{rel1, rel2})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (deduplication), got %d", len(entries))
	}
	if entries[0].Version != "0-1.2.3" {
		t.Errorf("expected newest version 0-1.2.3, got %q", entries[0].Version)
	}
}

func TestFetchCatalogFCOS_FedoraVersionFiltering(t *testing.T) {
	rawURL43 := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-43-x86-64/tailscale-0-1.2.3-43-x86-64.raw"
	rawURL44 := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"

	rel43 := fcosSingleRelease(t, "tailscale", "0-1.2.3", 43, "amd64", rawURL43, "")
	rel44 := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "amd64", rawURL44, "")

	srv := fcosCatalogServer(t, []any{rel43, rel44})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (version filter), got %d", len(entries))
	}
	if entries[0].URL != rawURL44 {
		t.Errorf("expected fedora44 URL, got %q", entries[0].URL)
	}
}

func TestFetchCatalogFCOS_ArchFiltering(t *testing.T) {
	rawURLx86 := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"
	rawURLarm := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-arm64/tailscale-0-1.2.3-44-arm64.raw"

	relx86 := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "amd64", rawURLx86, "")
	relarm := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "arm64", rawURLarm, "")

	srv := fcosCatalogServer(t, []any{relx86, relarm})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "arm64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 arm64 entry, got %d", len(entries))
	}
	if !strings.Contains(entries[0].URL, "arm64") {
		t.Errorf("expected arm64 URL, got %q", entries[0].URL)
	}
}

func TestFetchCatalogFCOS_SkipBadTags(t *testing.T) {
	// Bare pointer tag (no arch suffix) — should be silently skipped.
	rel := makeFCOSRelease("tailscale", "pointer", nil)

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (bad tag skipped), got %d", len(entries))
	}
}

func TestFetchCatalogFCOS_SkipMissingDownloadURL(t *testing.T) {
	tag := "tailscale-0-1.2.3-44-x86-64"
	// Release with only a non-.raw asset — should be skipped.
	rel := makeFCOSRelease(tag, "body", []struct{ Name, URL string }{
		{Name: "README.md", URL: "https://github.com/x/README.md"},
	})

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (no .raw asset), got %d", len(entries))
	}
}

func TestFetchCatalogFCOS_SkipURLTooLong(t *testing.T) {
	longURL := "https://github.com/" + strings.Repeat("a", maxSysextURLLen) + ".raw"
	tag := "tailscale-0-1.2.3-44-x86-64"
	rel := makeFCOSRelease(tag, "body", []struct{ Name, URL string }{
		{Name: tag + ".raw", URL: longURL},
	})

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (URL too long), got %d", len(entries))
	}
}

func TestFetchCatalogFCOS_SHA256SumsURLTooLong(t *testing.T) {
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"
	longSHA256URL := "https://github.com/" + strings.Repeat("b", maxSysextURLLen) + "/SHA256SUMS"
	tag := "tailscale-0-1.2.3-44-x86-64"
	rel := makeFCOSRelease(tag, "body", []struct{ Name, URL string }{
		{Name: tag + ".raw", URL: rawURL},
		{Name: "SHA256SUMS", URL: longSHA256URL},
	})

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	// Should succeed — the SHA256SUMS URL is sanitized (dropped), not fatal.
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty sha256 (URL sanitized), got %q", entries[0].Sha256)
	}
}

func TestFetchCatalogFCOS_PageCap(t *testing.T) {
	// Serve fcosMaxCatalogPages pages of one release each (with a next-page link).
	// After the last page the server would serve more — expect an error.
	var pageCount int32
	var capSrv *httptest.Server
	capSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := atomic.AddInt32(&pageCount, 1)
		tag := fmt.Sprintf("dummy-%d-0-1.0.0-44-x86-64", page)
		rawURL := fmt.Sprintf("https://github.com/dummy/releases/%s.raw", tag)
		rel := makeFCOSRelease(tag, "", []struct{ Name, URL string }{
			{Name: tag + ".raw", URL: rawURL},
		})
		releases := []any{rel}

		// Always set a Link: next header to simulate an infinite catalog.
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d>; rel="next"`, capSrv.URL, page+1))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("encode error: %v", err)
		}
	}))
	defer capSrv.Close()

	c := &HTTPClient{CatalogURL: capSrv.URL, HTTP: capSrv.Client()}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil {
		t.Fatal("expected page cap error")
	}
	if !strings.Contains(err.Error(), "page cap") {
		t.Errorf("expected page cap error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_Pagination(t *testing.T) {
	// Serve two pages: page 1 has tailscale, page 2 has docker-ce.  Page 2 has no next link.
	rawURLTailscale := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"
	rawURLDocker := "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-29.5.3-1.fc44-44-x86-64/docker-ce-3-29.5.3-1.fc44-44-x86-64.raw"

	relTailscale := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "amd64", rawURLTailscale, "")
	relDocker := fcosSingleRelease(t, "docker-ce", "3-29.5.3-1.fc44", 44, "amd64", rawURLDocker, "")

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if page == "2" {
			if err := json.NewEncoder(w).Encode([]any{relDocker}); err != nil {
				t.Errorf("encode error: %v", err)
			}
			return
		}
		// page 1 — set Link: next
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, srv.URL))
		if err := json.NewEncoder(w).Encode([]any{relTailscale}); err != nil {
			t.Errorf("encode error: %v", err)
		}
	}))
	defer srv.Close()

	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}
	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (paginated), got %d", len(entries))
	}
}

func TestFetchCatalogFCOS_FCOSLookupApplied(t *testing.T) {
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/docker-ce-3-29.5.3-1.fc44-44-x86-64/docker-ce-3-29.5.3-1.fc44-44-x86-64.raw"
	rel := fcosSingleRelease(t, "docker-ce", "3-29.5.3-1.fc44", 44, "amd64", rawURL, "")

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// docker-ce is in fcosCatalog — curated description should be applied.
	if entries[0].Category == "" {
		t.Errorf("expected category from FCOSLookup, got empty")
	}
	if entries[0].SupportTier == "" {
		t.Errorf("expected support tier from FCOSLookup, got empty")
	}
	if !strings.Contains(entries[0].Description, "Docker") {
		t.Errorf("expected curated description containing Docker, got %q", entries[0].Description)
	}
}

func TestFetchCatalogFCOS_UnknownExtensionFallsBackToBody(t *testing.T) {
	// Use a name not in fcosCatalog.
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/mypkg-1.2.3-44-x86-64/mypkg-1.2.3-44-x86-64.raw"
	rel := makeFCOSRelease("mypkg-1.2.3-44-x86-64", "This is the release body", []struct{ Name, URL string }{
		{Name: "mypkg-1.2.3-44-x86-64.raw", URL: rawURL},
	})

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description != "This is the release body" {
		t.Errorf("expected release body as description, got %q", entries[0].Description)
	}
}

func TestFetchCatalogFCOS_SHA256Fetch(t *testing.T) {
	rawFilename := "tailscale-0-1.2.3-44-x86-64.raw"
	expectedHash := strings.Repeat("a", 64)
	sha256Body := expectedHash + "  " + rawFilename + "\n"

	sha256Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sha256Body))
	}))
	defer sha256Srv.Close()

	rawURL := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/" + rawFilename
	tag := "tailscale-0-1.2.3-44-x86-64"
	rel := makeFCOSRelease(tag, "body", []struct{ Name, URL string }{
		{Name: tag + ".raw", URL: rawURL},
		{Name: "SHA256SUMS", URL: sha256Srv.URL + "/sha256"},
	})

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}
	// Allow SHA256 fetches to reach sha256Srv (different server).
	c.HTTP = &http.Client{Transport: &dualTransport{a: srv.Client().Transport, aHost: srv.Listener.Addr().String(), b: sha256Srv.Client().Transport}}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Sha256 != expectedHash {
		t.Errorf("expected sha256 %q, got %q", expectedHash, entries[0].Sha256)
	}
}

// dualTransport routes requests to one of two transports based on host.
type dualTransport struct {
	a     http.RoundTripper
	aHost string
	b     http.RoundTripper
}

func (d *dualTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == d.aHost {
		return d.a.RoundTrip(req)
	}
	return d.b.RoundTrip(req)
}

func TestFetchCatalogFCOS_SHA256FetchError_SoftFail(t *testing.T) {
	// SHA256SUMS server returns 500 — should be a soft fail, not fatal.
	sha256Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer sha256Srv.Close()

	rawURL := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"
	tag := "tailscale-0-1.2.3-44-x86-64"
	rel := makeFCOSRelease(tag, "body", []struct{ Name, URL string }{
		{Name: tag + ".raw", URL: rawURL},
		{Name: "SHA256SUMS", URL: sha256Srv.URL + "/sha256"},
	})

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}
	c.HTTP = &http.Client{Transport: &dualTransport{a: srv.Client().Transport, aHost: srv.Listener.Addr().String(), b: sha256Srv.Client().Transport}}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("expected soft fail on SHA256 error, got fatal error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry despite SHA256 error, got %d", len(entries))
	}
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty sha256 after soft fail, got %q", entries[0].Sha256)
	}
}

func TestFetchCatalogFCOS_InvalidCatalogURL(t *testing.T) {
	// A URL with a control character causes http.NewRequestWithContext to fail.
	c := &HTTPClient{CatalogURL: "http://\n", HTTP: &http.Client{}}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil || !strings.Contains(err.Error(), "creating request") {
		t.Errorf("expected 'creating request' error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_ResponseBodyReadError(t *testing.T) {
	c := &HTTPClient{
		CatalogURL: "http://placeholder",
		HTTP:       &http.Client{Transport: &errorBodyTransport{}},
	}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil || !strings.Contains(err.Error(), "reading FCOS catalog response") {
		t.Errorf("expected read error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_ResponseBodyTooLarge(t *testing.T) {
	// Serve exactly maxResponseSize bytes so the limit-exceeded check triggers.
	c := &HTTPClient{
		CatalogURL: "http://placeholder",
		HTTP:       &http.Client{Transport: &bigBodyTransport{}},
	}
	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil || !strings.Contains(err.Error(), "5MB") {
		t.Errorf("expected 5MB size limit error, got: %v", err)
	}
}

func TestFetchCatalogFCOS_InvalidSysextName(t *testing.T) {
	// Tag that parses fine but whose name fails validate.SysextName (starts with '!').
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/!pkg-1.0.0-44-x86-64/!pkg-1.0.0-44-x86-64.raw"
	rel := makeFCOSRelease("!pkg-1.0.0-44-x86-64", "body", []struct{ Name, URL string }{
		{Name: "!pkg-1.0.0-44-x86-64.raw", URL: rawURL},
	})

	srv := fcosCatalogServer(t, []any{rel})
	c := &HTTPClient{CatalogURL: srv.URL, HTTP: srv.Client()}

	entries, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (invalid sysext name skipped), got %d", len(entries))
	}
}

// ── transport helpers for error injection ─────────────────────────────────────

// errorBodyTransport returns HTTP 200 responses whose body errors on Read.
type errorBodyTransport struct{}

func (t *errorBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &errorReadBody{},
		Header:     make(http.Header),
	}, nil
}

type errorReadBody struct{}

func (e *errorReadBody) Read(_ []byte) (int, error) { return 0, fmt.Errorf("simulated read error") }
func (e *errorReadBody) Close() error               { return nil }

// bigBodyTransport returns HTTP 200 responses with exactly maxResponseSize bytes
// of data, triggering the ≥5MB size limit check.
type bigBodyTransport struct{}

func (t *bigBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const maxResponseSize = 5 << 20
	body := make([]byte, maxResponseSize)
	// Fill with '[' so the limit reader reads the maximum and hits the check.
	for i := range body {
		body[i] = '['
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Body:          readCloser{r: fixedReader{data: body, pos: 0}},
		Header:        make(http.Header),
		ContentLength: int64(len(body)),
	}, nil
}

type readCloser struct{ r fixedReader }

func (rc readCloser) Read(p []byte) (int, error) { return rc.r.read(p) }
func (rc readCloser) Close() error               { return nil }

type fixedReader struct {
	data []byte
	pos  int
}

func (f *fixedReader) read(p []byte) (int, error) {
	if f.pos >= len(f.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, f.data[f.pos:])
	f.pos += n
	return n, nil
}

// ── FCOSClient ────────────────────────────────────────────────────────────────

func TestNewFCOSClient(t *testing.T) {
	c := NewFCOSClient(44)
	if c.FedoraVersion != 44 {
		t.Errorf("FedoraVersion: got %d, want 44", c.FedoraVersion)
	}
	if c.CatalogURL != FCOSCatalogURL {
		t.Errorf("CatalogURL: got %q, want %q", c.CatalogURL, FCOSCatalogURL)
	}
	if c.HTTP == nil {
		t.Error("HTTP client should not be nil")
	}
}

func TestNewFCOSClientWithURL(t *testing.T) {
	c := NewFCOSClientWithURL("http://example.com", 43)
	if c.FedoraVersion != 43 {
		t.Errorf("FedoraVersion: got %d, want 43", c.FedoraVersion)
	}
	if c.CatalogURL != "http://example.com" {
		t.Errorf("CatalogURL: got %q", c.CatalogURL)
	}
}

func TestFCOSClient_FetchCatalog_DelegatesToAmd64(t *testing.T) {
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-x86-64/tailscale-0-1.2.3-44-x86-64.raw"
	rel := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "amd64", rawURL, "")

	srv := fcosCatalogServer(t, []any{rel})

	c := NewFCOSClientWithURL(srv.URL, 44)
	c.HTTP = srv.Client()

	entries, err := c.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestFCOSClient_FetchCatalogArch(t *testing.T) {
	rawURL := "https://github.com/fedora-sysexts/community/releases/download/tailscale-0-1.2.3-44-arm64/tailscale-0-1.2.3-44-arm64.raw"
	rel := fcosSingleRelease(t, "tailscale", "0-1.2.3", 44, "arm64", rawURL, "")

	srv := fcosCatalogServer(t, []any{rel})

	c := NewFCOSClientWithURL(srv.URL, 44)
	c.HTTP = srv.Client()

	entries, err := c.FetchCatalogArch(context.Background(), "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 arm64 entry, got %d", len(entries))
	}
}

// ── FCOSMockClient compile-time assertion is tested by building ───────────────

func TestFCOSMockClient_ImplementsClient(t *testing.T) {
	mock := &FCOSMockClient{
		MockClient: &MockClient{
			Entries: []model.SysextEntry{{Name: "test", URL: "https://example.com/test.raw"}},
		},
		FedoraVersion: 44,
	}
	// Verify the interface is satisfied by calling through the interface.
	var c Client = mock
	entries, err := c.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected entries from mock")
	}
	if mock.FedoraVersion != 44 {
		t.Errorf("FedoraVersion: got %d, want 44", mock.FedoraVersion)
	}
}

func TestFetchCatalogFCOS_AuthTokenSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := NewHTTPClientWithURL(srv.URL)
	c.HTTP = srv.Client()
	c.AuthToken = "test-token-abc"

	_, err := c.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer test-token-abc" {
		t.Errorf("Authorization header: got %q, want %q", gotAuth, "Bearer test-token-abc")
	}
}
