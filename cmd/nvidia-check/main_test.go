package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestExtractDriverSeries_Empty(t *testing.T) {
	result := extractDriverSeries("")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestExtractDriverSeries_Single(t *testing.T) {
	doc := "Install with nvidia-drivers-latest and reboot."
	result := extractDriverSeries(doc)
	if len(result) != 1 || result[0] != "nvidia-drivers-latest" {
		t.Errorf("expected [nvidia-drivers-latest], got %v", result)
	}
}

func TestExtractDriverSeries_Multiple(t *testing.T) {
	doc := `Available driver series:
- SYSEXTNAME=nvidia-drivers-550
- SYSEXTNAME=nvidia-drivers-535
- SYSEXTNAME=nvidia-drivers-470`
	result := extractDriverSeries(doc)
	if len(result) != 3 {
		t.Errorf("expected 3 results, got %d: %v", len(result), result)
	}
	expected := []string{"nvidia-drivers-550", "nvidia-drivers-535", "nvidia-drivers-470"}
	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("result[%d] = %q, want %q", i, result[i], exp)
		}
	}
}

func TestExtractDriverSeries_Dedup(t *testing.T) {
	doc := "nvidia-drivers-latest and nvidia-drivers-latest again"
	result := extractDriverSeries(doc)
	if len(result) != 1 || result[0] != "nvidia-drivers-latest" {
		t.Errorf("expected deduped [nvidia-drivers-latest], got %v", result)
	}
}

func TestExtractDriverSeries_Production(t *testing.T) {
	// Production-like examples (open-source and proprietary variants)
	doc := `Example: SYSEXTNAME=nvidia-drivers-latest SYSEXTOPTS="--strip"
For older GPUs: SYSEXTNAME=nvidia-drivers-550
Also available: nvidia-drivers-production nvidia-drivers-470`
	result := extractDriverSeries(doc)
	if len(result) != 4 {
		t.Errorf("expected 4 results, got %d: %v", len(result), result)
	}
	seen := make(map[string]bool)
	for _, r := range result {
		seen[r] = true
	}
	for _, want := range []string{"nvidia-drivers-latest", "nvidia-drivers-550", "nvidia-drivers-production", "nvidia-drivers-470"} {
		if !seen[want] {
			t.Errorf("missing expected driver series: %s", want)
		}
	}
}

// --- fetchNvidiaDocs tests ---

func fakeGHContent(content string) []byte {
	resp := ghContentResponse{
		Content:  base64.StdEncoding.EncodeToString([]byte(content)),
		Encoding: "base64",
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestFetchNvidiaDocs_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(fakeGHContent("nvidia-drivers-550 nvidia-drivers-latest")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	content, err := fetchNvidiaDocs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "nvidia-drivers-550 nvidia-drivers-latest" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestFetchNvidiaDocs_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestFetchNvidiaDocs_BadEncoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ghContentResponse{Content: "plain text", Encoding: "utf-8"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for non-base64 encoding")
	}
}

func TestFetchNvidiaDocs_InvalidBase64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ghContentResponse{Content: "!!!invalid-base64!!!", Encoding: "base64"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestFetchNvidiaDocs_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("not json")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchNvidiaDocs_ConnectionRefused(t *testing.T) {
	orig := docsURL
	docsURL = "http://127.0.0.1:1" // unreachable port
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

// --- compareDriverSeries tests ---

func TestCompareDriverSeries_AllMatch(t *testing.T) {
	docSeries := []string{"nvidia-drivers-550", "nvidia-drivers-latest"}
	modelIDs := []string{"550", "latest"}

	missing, extra := compareDriverSeries(docSeries, modelIDs)
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra, got %v", extra)
	}
}

func TestCompareDriverSeries_MissingInModel(t *testing.T) {
	docSeries := []string{"nvidia-drivers-550", "nvidia-drivers-535", "nvidia-drivers-latest"}
	modelIDs := []string{"550", "latest"}

	missing, extra := compareDriverSeries(docSeries, modelIDs)
	if len(missing) != 1 || missing[0] != "535" {
		t.Errorf("expected missing=[535], got %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra, got %v", extra)
	}
}

func TestCompareDriverSeries_ExtraInModel(t *testing.T) {
	docSeries := []string{"nvidia-drivers-latest"}
	modelIDs := []string{"550", "latest"}

	missing, extra := compareDriverSeries(docSeries, modelIDs)
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
	if len(extra) != 1 || extra[0] != "550" {
		t.Errorf("expected extra=[550], got %v", extra)
	}
}

func TestCompareDriverSeries_Empty(t *testing.T) {
	missing, extra := compareDriverSeries(nil, nil)
	if len(missing) != 0 || len(extra) != 0 {
		t.Errorf("expected empty results, got missing=%v extra=%v", missing, extra)
	}
}

// --- run() integration tests ---

// buildFakeDocsContent constructs a markdown string containing nvidia-drivers-X
// entries matching the given IDs.
func buildFakeDocsContent(ids []string) string {
	var b strings.Builder
	b.WriteString("# NVIDIA Drivers\n\n")
	for _, id := range ids {
		b.WriteString("SYSEXTNAME=nvidia-drivers-" + id + "\n")
	}
	return b.String()
}

// serveFakeDocs starts a test server that returns the given content as a
// GitHub file API response. Caller must defer srv.Close().
func serveFakeDocs(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(fakeGHContent(content)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
}

func TestRun_Consistent(t *testing.T) {
	// Serve doc content that exactly matches model.go driver IDs.
	modelIDs := model.DriverSeriesMap()
	ids := make([]string, 0, len(modelIDs))
	for k := range modelIDs {
		ids = append(ids, k)
	}
	srv := serveFakeDocs(t, buildFakeDocsContent(ids))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	code := run()
	if code != 0 {
		t.Errorf("run() = %d, want 0 (consistent)", code)
	}
}

func TestRun_MissingInModel(t *testing.T) {
	// Serve doc content that includes an ID NOT in model.go.
	modelIDs := model.DriverSeriesMap()
	ids := make([]string, 0, len(modelIDs)+1)
	for k := range modelIDs {
		ids = append(ids, k)
	}
	ids = append(ids, "999-fake") // not in model
	srv := serveFakeDocs(t, buildFakeDocsContent(ids))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	code := run()
	if code != 1 {
		t.Errorf("run() = %d, want 1 (missing in model)", code)
	}
}

func TestRun_FetchError(t *testing.T) {
	orig := docsURL
	docsURL = "http://127.0.0.1:1" // unreachable
	defer func() { docsURL = orig }()

	code := run()
	if code != 2 {
		t.Errorf("run() = %d, want 2 (fetch error)", code)
	}
}

func TestRun_EmptyDocSeries(t *testing.T) {
	// Doc has no nvidia-drivers-* mentions — model has extras, but nothing missing.
	srv := serveFakeDocs(t, "No drivers mentioned here at all.")
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	code := run()
	if code != 0 {
		t.Errorf("run() = %d, want 0 (no missing = consistent)", code)
	}
}

func TestRun_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	code := run()
	if code != 2 {
		t.Errorf("run() = %d, want 2 (HTTP error → fetch error)", code)
	}
}
