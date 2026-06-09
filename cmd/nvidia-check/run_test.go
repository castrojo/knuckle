package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// serveDocContent starts an httptest server returning the given doc content
// as a base64-encoded GitHub API response.
func serveDocContent(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ghContentResponse{
			Content:  base64.StdEncoding.EncodeToString([]byte(content)),
			Encoding: "base64",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
}

// withDocsURL overrides docsURL for the duration of the test.
func withDocsURL(t *testing.T, url string) {
	t.Helper()
	orig := docsURL
	docsURL = url
	t.Cleanup(func() { docsURL = orig })
}

func TestRun_Consistent(t *testing.T) {
	// Build doc content that matches all model driver series so run() reports consistent.
	modelIDs := model.DriverSeriesMap()
	var docLines []string
	for id := range modelIDs {
		docLines = append(docLines, "nvidia-drivers-"+id)
	}
	docContent := strings.Join(docLines, "\n")

	srv := serveDocContent(t, docContent)
	defer srv.Close()
	withDocsURL(t, srv.URL)

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}

	exitCode := run(stdout, stderr)
	if exitCode != 0 {
		// Read stderr for debug
		_ = stderr.Sync()
		_, _ = stderr.Seek(0, 0)
		buf := make([]byte, 4096)
		n, _ := stderr.Read(buf)
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, string(buf[:n]))
	}

	// Verify stdout mentions consistency
	_ = stdout.Sync()
	_, _ = stdout.Seek(0, 0)
	buf := make([]byte, 8192)
	n, _ := stdout.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "consistent") {
		t.Errorf("expected 'consistent' in output, got:\n%s", output)
	}
}

func TestRun_MissingInModel(t *testing.T) {
	// Include a driver series that is NOT in model.go
	modelIDs := model.DriverSeriesMap()
	var docLines []string
	for id := range modelIDs {
		docLines = append(docLines, "nvidia-drivers-"+id)
	}
	// Add a fake series that won't be in the model
	docLines = append(docLines, "nvidia-drivers-999-fake")
	docContent := strings.Join(docLines, "\n")

	srv := serveDocContent(t, docContent)
	defer srv.Close()
	withDocsURL(t, srv.URL)

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}

	exitCode := run(stdout, stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 (missing drivers), got %d", exitCode)
	}

	_ = stdout.Sync()
	_, _ = stdout.Seek(0, 0)
	buf := make([]byte, 8192)
	n, _ := stdout.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "MISSING IN MODEL") {
		t.Errorf("expected 'MISSING IN MODEL' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "999-fake") {
		t.Errorf("expected '999-fake' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ACTION REQUIRED") {
		t.Errorf("expected 'ACTION REQUIRED' in output, got:\n%s", output)
	}
}

func TestRun_FetchError(t *testing.T) {
	// Point to unreachable server
	withDocsURL(t, "http://127.0.0.1:1")

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}

	exitCode := run(stdout, stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 (fetch error), got %d", exitCode)
	}

	_ = stderr.Sync()
	_, _ = stderr.Seek(0, 0)
	buf := make([]byte, 4096)
	n, _ := stderr.Read(buf)
	errOutput := string(buf[:n])
	if !strings.Contains(errOutput, "ERROR") {
		t.Errorf("expected 'ERROR' in stderr, got:\n%s", errOutput)
	}
}

func TestRun_EmptyDocs(t *testing.T) {
	// Docs with no nvidia-drivers-* patterns — all model entries become "extra"
	srv := serveDocContent(t, "This doc has no driver patterns at all.")
	defer srv.Close()
	withDocsURL(t, srv.URL)

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}

	exitCode := run(stdout, stderr)
	// No missing = exit 0 (extra items are just notes, not failures)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 (no missing), got %d", exitCode)
	}

	_ = stdout.Sync()
	_, _ = stdout.Seek(0, 0)
	buf := make([]byte, 8192)
	n, _ := stdout.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "none found") {
		t.Errorf("expected 'none found' in output, got:\n%s", output)
	}
}

func TestRun_HTTPServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	withDocsURL(t, srv.URL)

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}

	exitCode := run(stdout, stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
}

func TestRun_ExtraInModel(t *testing.T) {
	// Docs only mention a single series (less than what model has)
	srv := serveDocContent(t, "nvidia-drivers-latest")
	defer srv.Close()
	withDocsURL(t, srv.URL)

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}

	exitCode := run(stdout, stderr)
	// "extra in model" is not a failure — exit 0
	if exitCode != 0 && exitCode != 1 {
		t.Fatalf("expected exit code 0 or 1, got %d", exitCode)
	}

	_ = stdout.Sync()
	_, _ = stdout.Seek(0, 0)
	buf := make([]byte, 8192)
	n, _ := stdout.Read(buf)
	output := string(buf[:n])

	// If model has "latest" then no missing, exit 0, with NOTE about extra
	if exitCode == 0 {
		if !strings.Contains(output, "NOTE:") {
			// Only expect NOTE if model has other entries besides "latest"
			modelIDs := model.DriverSeriesMap()
			if len(modelIDs) > 1 {
				t.Errorf("expected 'NOTE:' in output for extra drivers, got:\n%s", output)
			}
		}
	}
}
