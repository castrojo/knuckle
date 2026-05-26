package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

type captureFetcher struct {
	archs   []string
	entries []model.SysextEntry
	err     error
}

func (f *captureFetcher) FetchCatalogArch(_ context.Context, arch string) ([]model.SysextEntry, error) {
	f.archs = append(f.archs, arch)
	return f.entries, f.err
}

func TestMainWithArgs_ParsesFlagsAndUsesFetcher(t *testing.T) {
	fetcher := &captureFetcher{
		entries: []model.SysextEntry{{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"}},
	}

	oldFetcher := newCatalogFetcher
	newCatalogFetcher = func() CatalogFetcher { return fetcher }
	defer func() { newCatalogFetcher = oldFetcher }()

	var stdout, stderr bytes.Buffer
	code := mainWithArgs([]string{"--arch", "arm64"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mainWithArgs() exit code = %d, want 0", code)
	}
	if len(fetcher.archs) != 1 || fetcher.archs[0] != "arm64" {
		t.Fatalf("FetchCatalogArch archs = %v, want [arm64]", fetcher.archs)
	}
	if !strings.Contains(stdout.String(), "Fetching live bakery catalog (arm64)") {
		t.Fatalf("expected arm64 fetch banner, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestMainWithArgs_StrictViolationReturnsOne(t *testing.T) {
	fetcher := &captureFetcher{
		entries: []model.SysextEntry{{Name: "missing-ext", Version: "1.0.0", URL: "https://example.com/missing.raw"}},
	}

	oldFetcher := newCatalogFetcher
	newCatalogFetcher = func() CatalogFetcher { return fetcher }
	defer func() { newCatalogFetcher = oldFetcher }()

	var stdout, stderr bytes.Buffer
	code := mainWithArgs([]string{"--strict"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("mainWithArgs() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "FAIL: 1 extension(s) missing curated descriptions (--strict)") {
		t.Fatalf("expected strict failure on stderr, got %q", stderr.String())
	}
}

func TestMainWithArgs_FetchErrorReturnsTwo(t *testing.T) {
	fetcher := &captureFetcher{err: errors.New("catalog offline")}

	oldFetcher := newCatalogFetcher
	newCatalogFetcher = func() CatalogFetcher { return fetcher }
	defer func() { newCatalogFetcher = oldFetcher }()

	var stdout, stderr bytes.Buffer
	code := mainWithArgs(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("mainWithArgs() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "ERROR: fetching catalog: catalog offline") {
		t.Fatalf("expected wrapped fetch error on stderr, got %q", stderr.String())
	}
}

func TestMainWithArgs_InvalidFlagReturnsTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := mainWithArgs([]string{"--nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("mainWithArgs() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("expected flag parse error, got %q", stderr.String())
	}
}

func TestMain_UsesExitFunc(t *testing.T) {
	fetcher := &captureFetcher{
		entries: []model.SysextEntry{{Name: "docker", Version: "28.0.0", URL: "https://example.com/docker.raw"}},
	}

	oldArgs := os.Args
	oldExit := exitFunc
	oldFetcher := newCatalogFetcher
	defer func() {
		os.Args = oldArgs
		exitFunc = oldExit
		newCatalogFetcher = oldFetcher
	}()

	os.Args = []string{"catalog_check", "--arch", "arm64"}
	newCatalogFetcher = func() CatalogFetcher { return fetcher }

	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 0 {
		t.Fatalf("main() exit code = %d, want 0", exitCode)
	}
	if len(fetcher.archs) != 1 || fetcher.archs[0] != "arm64" {
		t.Fatalf("FetchCatalogArch archs = %v, want [arm64]", fetcher.archs)
	}
}
