package bakery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
)

// TestFetchCatalogSkipsInvalidSysextName verifies that releases whose parsed
// name fails validate.SysextName are silently skipped (coverage: line 173).
func TestFetchCatalogSkipsInvalidSysextName(t *testing.T) {
	// A tag like ".hidden-v1.0" parses to name=".hidden" which fails SysextName
	// (must start with alphanumeric). The valid entry should still be returned.
	payload := `[
		{"tag_name":".hidden-v1.0","body":"bad name","assets":[
			{"name":".hidden-1.0-x86-64.raw","browser_download_url":"https://example.com/.hidden.raw"}
		]},
		{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
			{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://example.com/docker.raw"}
		]}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only "docker" should appear — ".hidden" is skipped.
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "docker" {
		t.Errorf("expected name 'docker', got %q", entries[0].Name)
	}
}

// TestFetchCatalogEmptyPageStopsPagination verifies that an empty releases
// page terminates pagination early (coverage: line 134 break).
func TestFetchCatalogEmptyPageStopsPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if page == 0 {
			page++
			// First page has data; include Link header to force a second request.
			next := fmt.Sprintf("http://%s/page2", r.Host)
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, next))
			_, _ = w.Write([]byte(`[{"tag_name":"k3s-v1.28.0","body":"k3s","assets":[
				{"name":"k3s-1.28.0-x86-64.raw","browser_download_url":"https://example.com/k3s.raw"}
			]}]`))
		} else {
			// Second page is empty — should trigger the break.
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry from first page only, got %d", len(entries))
	}
	if entries[0].Name != "k3s" {
		t.Errorf("expected name 'k3s', got %q", entries[0].Name)
	}
}

// TestFetchSHA256MalformedLines verifies that SHA256SUMS lines with fewer
// than 2 fields are safely skipped (coverage: line 277).
func TestFetchSHA256MalformedLines(t *testing.T) {
	const wantHash = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	// Include malformed lines: empty, comment, single-field, and one valid line.
	sha256Content := `
# comment line

singlefieldnospaces
` + wantHash + `  myext-1.0-x86-64.raw
`
	payload := `[{"tag_name":"myext-v1.0","body":"","assets":[
		{"name":"myext-1.0-x86-64.raw","browser_download_url":"https://BASEURL/myext-1.0-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/SHA256SUMS" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(sha256Content))
			return
		}
		catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
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
	if entries[0].Sha256 != wantHash {
		t.Errorf("expected hash %q, got %q", wantHash, entries[0].Sha256)
	}
}

// TestFetchSHA256HTTPError verifies that a non-200 response from the
// SHA256SUMS endpoint is handled gracefully (soft failure, no hash).
func TestFetchSHA256HTTPError(t *testing.T) {
	payload := `[{"tag_name":"nginx-v1.25.0","body":"","assets":[
		{"name":"nginx-1.25.0-x86-64.raw","browser_download_url":"https://BASEURL/nginx-1.25.0-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/SHA256SUMS" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
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
	// SHA256 should be empty — soft failure on HTTP 404.
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty Sha256 on HTTP error, got %q", entries[0].Sha256)
	}
}

// TestFetchCatalogReadBodyError verifies that an error reading the response
// body is propagated (coverage: line 122).
func TestFetchCatalogReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Set Content-Length to a large value but close the connection early.
		w.Header().Set("Content-Length", "99999")
		w.WriteHeader(http.StatusOK)
		// Write partial data then close — causes io.ReadAll to return an error.
		_, _ = w.Write([]byte(`[{"tag_name"`))
		// Hijack connection to force close without completing response.
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			if conn != nil {
				_ = conn.Close()
			}
		}
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	_, err := client.FetchCatalog(context.Background())
	// Should get either a read error or a JSON parse error (partial body).
	if err == nil {
		t.Fatal("expected error from body read failure, got nil")
	}
}

// TestFetchSHA256ReadBodyError verifies that an error reading the SHA256SUMS
// response body results in empty hash (soft failure, coverage: line 266).
func TestFetchSHA256ReadBodyError(t *testing.T) {
	payload := `[{"tag_name":"etcd-v3.5.0","body":"","assets":[
		{"name":"etcd-3.5.0-x86-64.raw","browser_download_url":"https://BASEURL/etcd-3.5.0-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/SHA256SUMS" {
			// Return a response with a body that errors on read.
			w.Header().Set("Content-Length", "99999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("partial"))
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				if conn != nil {
					_ = conn.Close()
				}
			}
			return
		}
		catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
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
	// SHA256 should be empty — soft failure on read error.
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty Sha256 on read error, got %q", entries[0].Sha256)
	}
}

// TestFetchCatalogSkipsUnparseableTagName verifies that releases with tag
// names that cannot be parsed by ParseTagName are silently skipped
// (coverage: line 149 — name == "").
func TestFetchCatalogSkipsUnparseableTagName(t *testing.T) {
	payload := `[
		{"tag_name":"nodashes","body":"unparseable","assets":[
			{"name":"nodashes-x86-64.raw","browser_download_url":"https://example.com/nodashes.raw"}
		]},
		{"tag_name":"valid-v2.0.0","body":"valid ext","assets":[
			{"name":"valid-2.0.0-x86-64.raw","browser_download_url":"https://example.com/valid.raw"}
		]}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	client := bakery.NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only "valid" should appear — "nodashes" tag is unparseable.
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "valid" {
		t.Errorf("expected name 'valid', got %q", entries[0].Name)
	}
}
