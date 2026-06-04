package fcos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── parseFedoraVersionFromRelease ─────────────────────────────────────────────

func TestParseFedoraVersionFromRelease_Standard(t *testing.T) {
	v, err := parseFedoraVersionFromRelease("stable", "44.20260510.3.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 44 {
		t.Errorf("got %d, want 44", v)
	}
}

func TestParseFedoraVersionFromRelease_OlderVersion(t *testing.T) {
	v, err := parseFedoraVersionFromRelease("testing", "41.20250223.3.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 41 {
		t.Errorf("got %d, want 41", v)
	}
}

func TestParseFedoraVersionFromRelease_NoDot(t *testing.T) {
	v, err := parseFedoraVersionFromRelease("next", "44")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 44 {
		t.Errorf("got %d, want 44", v)
	}
}

func TestParseFedoraVersionFromRelease_Empty(t *testing.T) {
	_, err := parseFedoraVersionFromRelease("stable", "")
	if err == nil {
		t.Fatal("expected error for empty release")
	}
}

func TestParseFedoraVersionFromRelease_NonNumeric(t *testing.T) {
	_, err := parseFedoraVersionFromRelease("stable", "notanumber.20260510.3.1")
	if err == nil {
		t.Fatal("expected error for non-numeric major version")
	}
}

func TestParseFedoraVersionFromRelease_EmptyAfterDot(t *testing.T) {
	_, err := parseFedoraVersionFromRelease("stable", ".20260510.3.1")
	if err == nil {
		t.Fatal("expected error for empty major version before dot")
	}
}

// ── parseFedoraVersionFromStreamJSON ─────────────────────────────────────────

func TestParseFedoraVersionFromStreamJSON_X86_64Metal(t *testing.T) {
	body := []byte(`{"architectures":{"x86_64":{"artifacts":{"metal":{"release":"44.20260510.3.1"}}}}}`)
	v, err := parseFedoraVersionFromStreamJSON("stable", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 44 {
		t.Errorf("got %d, want 44", v)
	}
}

func TestParseFedoraVersionFromStreamJSON_FallbackToAarch64(t *testing.T) {
	// No x86_64, only aarch64
	body := []byte(`{"architectures":{"aarch64":{"artifacts":{"metal":{"release":"44.20260510.3.1"}}}}}`)
	v, err := parseFedoraVersionFromStreamJSON("stable", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 44 {
		t.Errorf("got %d, want 44", v)
	}
}

func TestParseFedoraVersionFromStreamJSON_FallbackNonMetal(t *testing.T) {
	// x86_64 present but no metal artifact; falls back to another artifact
	body := []byte(`{"architectures":{"x86_64":{"artifacts":{"qemu":{"release":"44.20260510.3.1"}}}}}`)
	v, err := parseFedoraVersionFromStreamJSON("stable", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 44 {
		t.Errorf("got %d, want 44", v)
	}
}

func TestParseFedoraVersionFromStreamJSON_NoArchitectures(t *testing.T) {
	body := []byte(`{"architectures":{}}`)
	_, err := parseFedoraVersionFromStreamJSON("stable", body)
	if err == nil {
		t.Fatal("expected error when architectures map is empty")
	}
}

func TestParseFedoraVersionFromStreamJSON_InvalidJSON(t *testing.T) {
	_, err := parseFedoraVersionFromStreamJSON("stable", []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseFedoraVersionFromStreamJSON_NoReleaseField(t *testing.T) {
	body := []byte(`{"architectures":{"x86_64":{"artifacts":{"metal":{"release":""}}}}}`)
	_, err := parseFedoraVersionFromStreamJSON("stable", body)
	if err == nil {
		t.Fatal("expected error when release is empty")
	}
}

// ── fetchStreamFedoraVersion (HTTP path) ─────────────────────────────────────

func TestFetchStreamFedoraVersion_EmptyStream(t *testing.T) {
	_, err := fetchStreamFedoraVersion(context.Background(), "http://localhost", "", http.DefaultClient)
	if err == nil {
		t.Fatal("expected error for empty stream")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestFetchStreamFedoraVersion_HTTP200(t *testing.T) {
	body := `{"architectures":{"x86_64":{"artifacts":{"metal":{"release":"44.20260510.3.1"}}}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "knuckle/1.0" {
			t.Errorf("expected User-Agent knuckle/1.0, got %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	v, err := fetchStreamFedoraVersion(context.Background(), srv.URL, "stable", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 44 {
		t.Errorf("got %d, want 44", v)
	}
}

func TestFetchStreamFedoraVersion_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := fetchStreamFedoraVersion(context.Background(), srv.URL, "unknown", srv.Client())
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestFetchStreamFedoraVersion_NetworkError(t *testing.T) {
	_, err := fetchStreamFedoraVersion(context.Background(), "http://127.0.0.1:1", "stable", &http.Client{})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestFetchStreamFedoraVersion_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := fetchStreamFedoraVersion(context.Background(), srv.URL, "stable", srv.Client())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchStreamFedoraVersion_CorrectURLPath(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		body := `{"architectures":{"x86_64":{"artifacts":{"metal":{"release":"44.20260510.3.1"}}}}}`
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	_, err := fetchStreamFedoraVersion(context.Background(), srv.URL, "testing", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPath != "/testing.json" {
		t.Errorf("got path %q, want %q", capturedPath, "/testing.json")
	}
}

func TestFetchStreamFedoraVersion_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// serve nothing — just block
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := fetchStreamFedoraVersion(ctx, srv.URL, "stable", srv.Client())
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchStreamFedoraVersion_InvalidBaseURL(t *testing.T) {
	// A URL with a control character causes http.NewRequestWithContext to fail.
	_, err := fetchStreamFedoraVersion(context.Background(), "http://\n", "stable", http.DefaultClient)
	if err == nil || !strings.Contains(err.Error(), "creating FCOS stream request") {
		t.Errorf("expected 'creating FCOS stream request' error, got: %v", err)
	}
}

func TestFetchStreamFedoraVersion_BodyReadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send 200 OK but close the connection immediately (causes read error).
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijack", 500)
			return
		}
		conn, _, _ := hj.Hijack()
		// Send a malformed response that will cause Read to fail.
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000000\r\n\r\n"))
		_ = conn.Close()
	}))
	defer srv.Close()

	_, err := fetchStreamFedoraVersion(context.Background(), srv.URL, "stable", srv.Client())
	if err == nil {
		t.Fatal("expected read error for abruptly closed connection")
	}
}

// ── FetchStreamFedoraVersion (public, uses package-level client) ──────────────

func TestFetchStreamFedoraVersion_PublicFunction_EmptyStream(t *testing.T) {
	_, err := FetchStreamFedoraVersion(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty stream name")
	}
}
