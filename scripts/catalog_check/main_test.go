package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// mockFetcher implements CatalogFetcher for tests.
type mockFetcher struct {
	entries []model.SysextEntry
	err     error
}

func (m *mockFetcher) FetchCatalogArch(_ context.Context, _ string) ([]model.SysextEntry, error) {
	return m.entries, m.err
}

func TestRun_AllCovered(t *testing.T) {
	fetcher := &mockFetcher{
		entries: []model.SysextEntry{
			{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
			{Name: "tailscale", Version: "1.56.1", URL: "https://example.com/tailscale.raw"},
		},
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), &out, &errOut, fetcher, "amd64", false)
	if err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "✓ All live bakery extensions have curated descriptions") {
		t.Error("expected success message in output")
	}
	if !strings.Contains(output, "2 extensions found") {
		t.Error("expected '2 extensions found' in output")
	}
	if !strings.Contains(output, "Result: 2 covered, 0 missing") {
		t.Errorf("expected result summary, got: %s", output)
	}
	if errOut.Len() > 0 {
		t.Errorf("unexpected stderr: %s", errOut.String())
	}
}

func TestRun_SomeMissing_NoStrict(t *testing.T) {
	fetcher := &mockFetcher{
		entries: []model.SysextEntry{
			{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
			{Name: "unknown-ext", Version: "1.0.0", URL: "https://example.com/unknown.raw"},
		},
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), &out, &errOut, fetcher, "amd64", false)
	if err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Result: 1 covered, 1 missing") {
		t.Errorf("expected result summary, got: %s", output)
	}
	if !strings.Contains(output, "MISSING") {
		t.Error("expected MISSING in output")
	}
	if !strings.Contains(output, "─── Missing entries") {
		t.Error("expected missing entries section")
	}
	if !strings.Contains(output, "─── Checklist") {
		t.Error("expected checklist section")
	}
	if !strings.Contains(output, `"unknown-ext"`) {
		t.Error("expected template with extension name")
	}
	if !strings.Contains(output, "⚠ 1 extension(s) are missing curated descriptions") {
		t.Error("expected warning message")
	}
}

func TestRun_SomeMissing_Strict(t *testing.T) {
	fetcher := &mockFetcher{
		entries: []model.SysextEntry{
			{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
			{Name: "unknown-ext", Version: "1.0.0", URL: "https://example.com/unknown.raw"},
		},
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), &out, &errOut, fetcher, "amd64", true)
	if !errors.Is(err, errStrictViolation) {
		t.Fatalf("expected errStrictViolation, got: %v", err)
	}

	if !strings.Contains(errOut.String(), "FAIL: 1 extension(s) missing curated descriptions (--strict)") {
		t.Errorf("expected FAIL message on stderr, got: %s", errOut.String())
	}
}

func TestRun_FetchError(t *testing.T) {
	fetcher := &mockFetcher{
		err: errors.New("network timeout"),
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), &out, &errOut, fetcher, "amd64", false)
	if err == nil {
		t.Fatal("expected error from run()")
	}
	if !strings.Contains(err.Error(), "fetching catalog") {
		t.Errorf("expected wrapped fetch error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("expected original error in chain, got: %v", err)
	}
}

func TestRun_EmptyCatalog(t *testing.T) {
	fetcher := &mockFetcher{
		entries: []model.SysextEntry{},
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), &out, &errOut, fetcher, "arm64", false)
	if err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "0 extensions found") {
		t.Errorf("expected '0 extensions found', got: %s", output)
	}
	if !strings.Contains(output, "Result: 0 covered, 0 missing") {
		t.Errorf("expected zero result, got: %s", output)
	}
	if !strings.Contains(output, "✓ All live bakery extensions have curated descriptions") {
		t.Error("expected success message for empty catalog")
	}
}

func TestRun_SortsEntries(t *testing.T) {
	fetcher := &mockFetcher{
		entries: []model.SysextEntry{
			{Name: "tailscale", Version: "1.56.1", URL: "https://example.com/tailscale.raw"},
			{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"},
		},
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), &out, &errOut, fetcher, "amd64", false)
	if err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	output := out.String()
	dockerIdx := strings.Index(output, "docker")
	tailscaleIdx := strings.Index(output, "tailscale")
	if dockerIdx > tailscaleIdx {
		t.Error("expected entries sorted alphabetically (docker before tailscale)")
	}
}

func TestRun_ArchPassedToFetcher(t *testing.T) {
	var capturedArch string
	fetcher := &archCaptureFetcher{archPtr: &capturedArch}

	var out, errOut bytes.Buffer
	_ = run(context.Background(), &out, &errOut, fetcher, "arm64", false)

	if capturedArch != "arm64" {
		t.Errorf("expected arch 'arm64' passed to fetcher, got %q", capturedArch)
	}
}

type archCaptureFetcher struct {
	archPtr *string
}

func (f *archCaptureFetcher) FetchCatalogArch(_ context.Context, arch string) ([]model.SysextEntry, error) {
	*f.archPtr = arch
	return nil, nil
}
