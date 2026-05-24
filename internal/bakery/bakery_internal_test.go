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
	if len([]rune(got)) != 80 {
		t.Errorf("truncated rune count = %d, want 80", len([]rune(got)))
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

// ── truncateDescription: UTF-8 safety ────────────────────────────────────────

func TestTruncateDescription_UTF8_DoesNotSplitRune(t *testing.T) {
	// 10 runes of 3-byte chars (30 bytes) — truncate to 8 runes
	input := strings.Repeat("日", 10) // each 日 = 3 bytes
	got := truncateDescription(input, 8)
	// Should be 5 runes ("日日日日日") + "..." = 8 runes total
	wantRunes := 8
	if runeCount := len([]rune(got)); runeCount != wantRunes {
		t.Errorf("UTF-8 truncate: rune count = %d, want %d; got %q", runeCount, wantRunes, got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("UTF-8 truncated string should end with ...: %q", got)
	}
	// Must be valid UTF-8 — no partial rune at the boundary
	for i, r := range got {
		if r == '\uFFFD' {
			t.Errorf("invalid UTF-8 at byte %d in %q", i, got)
		}
	}
}

func TestTruncateDescription_UTF8_ExactLength(t *testing.T) {
	// Exactly maxLen runes → no truncation
	input := strings.Repeat("é", 10) // each é = 2 bytes
	got := truncateDescription(input, 10)
	if got != input {
		t.Errorf("exact-length input should not be truncated, got %q", got)
	}
}

func TestTruncateDescription_UTF8_MixedASCIIAndMultibyte(t *testing.T) {
	// "Hello 世界！" = 8 runes — truncate to 7 should give "Hell..."
	input := "Hello 世界！"
	got := truncateDescription(input, 7)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("mixed truncation should end with ...: %q", got)
	}
	if runeCount := len([]rune(got)); runeCount != 7 {
		t.Errorf("mixed truncation rune count = %d, want 7; got %q", runeCount, got)
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
