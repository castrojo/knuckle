package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeBase64Response(content string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	body, _ := json.Marshal(ghContentResponse{Content: encoded, Encoding: "base64"})
	return string(body)
}

func TestFetchNvidiaDocs_NetworkError(t *testing.T) {
	orig := docsURL
	docsURL = "http://127.0.0.1:0/unreachable"
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestFetchNvidiaDocs_Happy(t *testing.T) {
	want := "nvidia-drivers-latest something something"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(makeBase64Response(want)))
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	got, err := fetchNvidiaDocs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetchNvidiaDocs_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestFetchNvidiaDocs_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
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

func TestFetchNvidiaDocs_UnexpectedEncoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, _ := json.Marshal(ghContentResponse{Content: "aGVsbG8=", Encoding: "utf-8"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	orig := docsURL
	docsURL = srv.URL
	defer func() { docsURL = orig }()

	_, err := fetchNvidiaDocs()
	if err == nil {
		t.Fatal("expected error for unexpected encoding")
	}
}

func TestFetchNvidiaDocs_BadBase64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, _ := json.Marshal(ghContentResponse{Content: "!!!not-valid-base64!!!", Encoding: "base64"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
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
