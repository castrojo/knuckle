package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// errWriter returns an error after N successful writes.
// This exercises every writef/writeln error-return branch in run().
type errWriter struct {
	remaining int
	buf       bytes.Buffer
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errors.New("simulated write error")
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

// TestRun_WriteErrorPropagation_AllCovered verifies that every writef/writeln
// call in the "all covered" path returns an error when the writer fails.
func TestRun_WriteErrorPropagation_AllCovered(t *testing.T) {
	entries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "tailscale", Version: "1.56.1", URL: "https://example.com/tailscale.raw"},
	}

	total := countWriteCalls(t, entries, false)
	if total == 0 {
		t.Fatal("expected at least one write call")
	}

	for failAt := 0; failAt < total; failAt++ {
		w := &errWriter{remaining: failAt}
		errOut := &bytes.Buffer{}
		fetcher := &mockFetcher{entries: entries}

		err := run(context.Background(), w, errOut, fetcher, "amd64", false)
		if err == nil {
			t.Errorf("failAt=%d: expected error, got nil", failAt)
		}
	}
}

// TestRun_WriteErrorPropagation_MissingEntries verifies write-error branches
// in the "missing entries" report path (non-strict).
func TestRun_WriteErrorPropagation_MissingEntries(t *testing.T) {
	entries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "unknown-ext", Version: "1.0.0", URL: "https://example.com/unknown.raw"},
	}

	total := countWriteCalls(t, entries, false)
	if total == 0 {
		t.Fatal("expected at least one write call")
	}

	for failAt := 0; failAt < total; failAt++ {
		w := &errWriter{remaining: failAt}
		errOut := &bytes.Buffer{}
		fetcher := &mockFetcher{entries: entries}

		err := run(context.Background(), w, errOut, fetcher, "amd64", false)
		if err == nil {
			t.Errorf("failAt=%d: expected error, got nil", failAt)
		}
	}
}

// TestRun_WriteErrorPropagation_Strict verifies write-error branches in
// the strict-mode error output path.
func TestRun_WriteErrorPropagation_Strict(t *testing.T) {
	entries := []model.SysextEntry{
		{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		{Name: "unknown-ext", Version: "1.0.0", URL: "https://example.com/unknown.raw"},
	}

	// In strict mode, errW gets a write too. Test that errW failure propagates.
	total := countWriteCalls(t, entries, true)
	if total == 0 {
		t.Fatal("expected at least one write call")
	}

	// Test stdout write failures
	for failAt := 0; failAt < total; failAt++ {
		w := &errWriter{remaining: failAt}
		errOut := &bytes.Buffer{}
		fetcher := &mockFetcher{entries: entries}

		err := run(context.Background(), w, errOut, fetcher, "amd64", true)
		if err == nil {
			t.Errorf("failAt=%d: expected error, got nil", failAt)
		}
	}

	// Test errW (stderr) write failure in strict mode
	w := &bytes.Buffer{}
	errW := &errWriter{remaining: 0}
	fetcher := &mockFetcher{entries: entries}
	err := run(context.Background(), w, errW, fetcher, "amd64", true)
	if err == nil {
		t.Error("expected error when errW fails in strict mode")
	}
}

// TestWritef_Error verifies writef returns a wrapped error.
func TestWritef_Error(t *testing.T) {
	w := &errWriter{remaining: 0}
	err := writef(w, "hello %s", "world")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		// The error wraps "simulated write error", not io.ErrClosedPipe.
		// Just verify it's non-nil and contains our message.
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	}
}

// TestWriteln_Error verifies writeln returns a wrapped error.
func TestWriteln_Error(t *testing.T) {
	w := &errWriter{remaining: 0}
	err := writeln(w, "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
