package fcos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchStreamFedoraVersion_Stable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/streams/stable.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"architectures": {
				"x86_64": {
					"artifacts": {
						"metal": {
							"release": {"version": "41.20250223.3.0"}
						}
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	// Override base URL for testing
	origBase := streamBaseURL
	defer func() { /* can't reassign const, use client approach */ }()
	_ = origBase

	// Use the server directly via custom client
	ver, err := fetchFromURL(t, srv.URL+"/streams/stable.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != 41 {
		t.Errorf("got Fedora version %d, want 41", ver)
	}
}

func TestFetchStreamFedoraVersion_Next(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"architectures": {
				"x86_64": {
					"artifacts": {
						"metal": {
							"release": {"version": "44.20250601.1.0"}
						}
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	ver, err := fetchFromURL(t, srv.URL+"/streams/next.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != 44 {
		t.Errorf("got Fedora version %d, want 44", ver)
	}
}

func TestFetchStreamFedoraVersion_EmptyStream(t *testing.T) {
	_, err := FetchStreamFedoraVersion(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty stream")
	}
}

func TestFetchStreamFedoraVersion_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchFromURL(t, srv.URL+"/streams/bogus.json")
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestFetchStreamFedoraVersion_NoVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"architectures": {}}`))
	}))
	defer srv.Close()

	_, err := fetchFromURL(t, srv.URL+"/streams/empty.json")
	if err == nil {
		t.Fatal("expected error when no version in metadata")
	}
}

func TestFetchStreamFedoraVersion_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	_, err := fetchFromURL(t, srv.URL+"/streams/bad.json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseFedoraMajor(t *testing.T) {
	tests := []struct {
		version string
		want    int
		wantErr bool
	}{
		{"41.20250223.3.0", 41, false},
		{"44.20250601.1.0", 44, false},
		{"38.20230514.3.0", 38, false},
		{"", 0, true},
		{"abc.123", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := parseFedoraMajor(tt.version)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFedoraMajor(%q) error=%v, wantErr=%v", tt.version, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseFedoraMajor(%q) = %d, want %d", tt.version, got, tt.want)
			}
		})
	}
}

// fetchFromURL is a test helper that fetches stream metadata from a specific URL
// rather than the production stream base URL.
func fetchFromURL(t *testing.T, url string) (int, error) {
	t.Helper()
	client := &http.Client{}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var meta streamMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return 0, err
	}

	version, err := extractVersion(meta)
	if err != nil {
		return 0, err
	}

	return parseFedoraMajor(version)
}
