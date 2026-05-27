package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

var errInjectedWrite = errors.New("injected write error")

// errWriter returns an error after N successful writes.
// This exercises writef/writeln error-return branches in run().
type errWriter struct {
	remaining int
	err       error
	buf       bytes.Buffer
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, w.err
	}
	w.remaining--
	return w.buf.Write(p)
}

// countWrites counts how many io.Writer calls run() makes before completing
// successfully. We use this to know how many write-failure points exist.
func countWriteCalls(t *testing.T, entries []model.SysextEntry, strict bool) int {
	t.Helper()
	cw := &countingWriter{}
	errOut := &bytes.Buffer{}
	fetcher := &mockFetcher{entries: entries}
	_ = run(context.Background(), cw, errOut, fetcher, "amd64", strict)
	return cw.count
}

type countingWriter struct {
	count int
	buf   bytes.Buffer
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.count++
	return w.buf.Write(p)
}

func TestRun_WriteErrorPropagation_AllOutputWrites(t *testing.T) {
	t.Parallel()

	knownEntries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "tailscale", Version: "1.56.1", URL: "https://example.com/tailscale.raw"},
	}
	mixedEntries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "unknown-ext", Version: "1.0.0", URL: "https://example.com/unknown.raw"},
	}

	tests := []struct {
		name    string
		entries []model.SysextEntry
		strict  bool
	}{
		{name: "all covered path", entries: knownEntries, strict: false},
		{name: "missing entries non-strict path", entries: mixedEntries, strict: false},
		{name: "missing entries strict path stdout", entries: mixedEntries, strict: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			total := countWriteCalls(t, tc.entries, tc.strict)
			if total == 0 {
				t.Fatal("expected at least one write call")
			}

			for failAt := 0; failAt < total; failAt++ {
				w := &errWriter{remaining: failAt, err: errInjectedWrite}
				errOut := &bytes.Buffer{}
				fetcher := &mockFetcher{entries: tc.entries}

				err := run(context.Background(), w, errOut, fetcher, "amd64", tc.strict)
				if err == nil {
					t.Fatalf("failAt=%d: expected write error, got nil", failAt)
				}
				if !errors.Is(err, errInjectedWrite) {
					t.Fatalf("failAt=%d: expected injected write error, got %v", failAt, err)
				}
				if !strings.Contains(err.Error(), "writing report") {
					t.Fatalf("failAt=%d: expected wrapped report error, got %v", failAt, err)
				}
			}
		})
	}
}

func TestRun_WriteErrorPropagation_StrictErrWriter(t *testing.T) {
	t.Parallel()

	entries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "unknown-ext", Version: "1.0.0", URL: "https://example.com/unknown.raw"},
	}

	stdout := &bytes.Buffer{}
	stderr := &errWriter{remaining: 0, err: errInjectedWrite}
	fetcher := &mockFetcher{entries: entries}
	err := run(context.Background(), stdout, stderr, fetcher, "amd64", true)
	if err == nil {
		t.Fatal("expected error when strict-mode stderr write fails")
	}
	if !errors.Is(err, errInjectedWrite) {
		t.Fatalf("expected injected write error, got %v", err)
	}
	if !strings.Contains(err.Error(), "writing report") {
		t.Fatalf("expected wrapped report error, got %v", err)
	}
}

func TestWritef_Error(t *testing.T) {
	t.Parallel()

	w := &errWriter{remaining: 0, err: errInjectedWrite}
	err := writef(w, "hello %s", "world")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errInjectedWrite) {
		t.Fatalf("expected injected write error, got %v", err)
	}
	if !strings.Contains(err.Error(), "writing report") {
		t.Fatalf("expected wrapped report error, got %v", err)
	}
}

func TestWriteln_Error(t *testing.T) {
	t.Parallel()

	w := &errWriter{remaining: 0, err: errInjectedWrite}
	err := writeln(w, "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errInjectedWrite) {
		t.Fatalf("expected injected write error, got %v", err)
	}
	if !strings.Contains(err.Error(), "writing report") {
		t.Fatalf("expected wrapped report error, got %v", err)
	}
}
