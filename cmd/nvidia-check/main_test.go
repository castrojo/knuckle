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

func TestRun_Success_AllMatch(t *testing.T) {
	// Serve docs that exactly match model.go entries
	modelIDs := model.DriverSeriesMap()
	var docContent string
	for id := range modelIDs {
		docContent += "nvidia-drivers-" + id + "\n"
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeGHContent(docContent))
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	var stdout, stderr strings.Builder
	code := run(&stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "✓ model.go NvidiaDriverOptions appears consistent") {
		t.Errorf("expected success message in stdout, got: %s", stdout.String())
	}
}

func TestRun_MissingInModel(t *testing.T) {
	// Serve docs with an extra driver series not in model.go
	modelIDs := model.DriverSeriesMap()
	var docContent string
	for id := range modelIDs {
		docContent += "nvidia-drivers-" + id + "\n"
	}
	docContent += "nvidia-drivers-999-fake\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeGHContent(docContent))
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	var stdout, stderr strings.Builder
	code := run(&stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "MISSING IN MODEL") {
		t.Errorf("expected MISSING IN MODEL in output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ACTION REQUIRED") {
		t.Errorf("expected ACTION REQUIRED in output, got: %s", stdout.String())
	}
}

func TestRun_FetchFailure(t *testing.T) {
	orig := docsURL
	docsURL = "http://127.0.0.1:1" // unreachable
	defer func() { docsURL = orig }()

	var stdout, stderr strings.Builder
	code := run(&stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "ERROR") {
		t.Errorf("expected ERROR in stderr, got: %s", stderr.String())
	}
}

func TestRun_EmptyDocSeries(t *testing.T) {
	// Serve docs with no nvidia-drivers-* patterns
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeGHContent("No driver series mentioned here at all."))
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	var stdout, stderr strings.Builder
	code := run(&stdout, &stderr)
	// model.go has entries not in docs → they show as "extra" but exit is still 0
	if code != 0 {
		t.Errorf("expected exit 0 (extra in model is not an error), got %d", code)
	}
	if !strings.Contains(stdout.String(), "(none found") {
		t.Errorf("expected '(none found' message, got: %s", stdout.String())
	}
}

func TestRun_ExtraInModel(t *testing.T) {
	// Serve docs with only one model entry, so the rest show as extras
	modelIDs := model.DriverSeriesMap()
	var oneID string
	for id := range modelIDs {
		oneID = id
		break
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeGHContent("nvidia-drivers-" + oneID))
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	var stdout, stderr strings.Builder
	code := run(&stdout, &stderr)
	// All doc entries are in model, extras are informational → exit 0
	if code != 0 {
		t.Errorf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "NOTE:") {
		t.Errorf("expected NOTE about extra entries, got: %s", stdout.String())
	}
}
