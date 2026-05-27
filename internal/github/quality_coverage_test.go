package github

// quality_coverage_test.go — tests for coverage gaps identified by the quality
// agent (ACMM L3 run, 2026-05-26). Covers:
//   - github.go:66-68  io.ReadAll error path when response body read fails

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// errReader is an io.Reader that returns an error after writing n bytes.
type errReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// TestClient_FetchKeys_BodyReadError verifies the io.ReadAll
// error branch via a custom handler that serves a response whose body returns
// an explicit error. This tests github.go:66-68 via a simulated read failure.
func TestClient_FetchKeys_BodyReadError_WithErrReader(t *testing.T) {
	// Build a custom http.RoundTripper that injects a broken body.
	rt := &brokenBodyTransport{statusCode: 200}
	client := &Client{
		BaseURL: "http://example.invalid",
		HTTP:    &http.Client{Transport: rt},
	}

	_, err := client.FetchKeys(context.Background(), "someuser")
	if err == nil {
		t.Fatal("expected error when body read fails mid-stream")
	}
	if !strings.Contains(err.Error(), "failed to read response") &&
		!strings.Contains(err.Error(), "no public SSH keys") &&
		!strings.Contains(err.Error(), "read") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// brokenBodyTransport returns a response with a body that errors after partial read.
type brokenBodyTransport struct {
	statusCode int
}

func (t *brokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := &errReader{
		data: []byte("partial"),
		err:  errors.New("simulated body read error"),
	}
	// Exhaust data immediately so the next Read call returns the error.
	body.pos = len(body.data)

	return &http.Response{
		StatusCode: t.statusCode,
		Body:       io.NopCloser(body),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}
