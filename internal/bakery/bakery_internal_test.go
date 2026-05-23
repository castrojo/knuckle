package bakery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTruncateDescription_Short(t *testing.T) {
	if got := truncateDescription("hello", 80); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncateDescription_TooLong(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := truncateDescription(long, 80)
	if len(got) != 80 {
		t.Errorf("truncated len = %d, want 80", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated string should end with ...: %q", got)
	}
}

func TestTruncateDescription_MultilineUsesFirstLine(t *testing.T) {
	got := truncateDescription("first line\nsecond line\nthird", 80)
	if got != "first line" {
		t.Errorf("got %q, want %q", got, "first line")
	}
}

func TestTruncateDescription_Empty(t *testing.T) {
	if got := truncateDescription("", 80); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ── parseLinkNext: missing branches ──────────────────────────────────────────

func TestParseLinkNext_NoSemicolon_Skipped(t *testing.T) {
	// A part with no semicolon has len(segments) < 2 → continue; the
	// function then exhausts all parts and returns ("", false).
	url, ok := parseLinkNext(`<https://api.github.com/page=2>`)
	if ok || url != "" {
		t.Errorf("parseLinkNext(no-semicolon) = (%q, %v), want (\"\", false)", url, ok)
	}
}

func TestParseLinkNext_NonEmpty_NoNext(t *testing.T) {
	// Non-empty header with rel="last" only — no rel="next" match → ("", false).
	url, ok := parseLinkNext(`<https://api.github.com/page=5>; rel="last"`)
	if ok || url != "" {
		t.Errorf("parseLinkNext(last-only) = (%q, %v), want (\"\", false)", url, ok)
	}
}

func TestFetchSHA256ForAsset_MalformedHash_WhiteBox(t *testing.T) {
	// Test fetchSHA256ForAsset directly with a malformed hash — the reSHA256
	// regex rejects it, returning "malformed SHA256 hash" error.
	const assetName = "target-1.0-x86-64.raw"
	sha256Content := "NOTAHEXHASH  " + assetName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sha256Content))
	}))
	defer srv.Close()

	c := &HTTPClient{HTTP: srv.Client()}
	_, err := c.fetchSHA256ForAsset(context.Background(), srv.URL+"/SHA256SUMS", assetName)
	if err == nil {
		t.Fatal("expected error for malformed SHA256 hash, got nil")
	}
	if !strings.Contains(err.Error(), "malformed SHA256") {
		t.Errorf("error should mention 'malformed SHA256', got: %v", err)
	}
}
