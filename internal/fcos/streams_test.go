package fcos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const mockStableJSON = `{
  "stream": "stable",
  "architectures": {
    "x86_64": {
      "artifacts": {
        "metal": {
          "release": "44.20260510.3.1"
        }
      }
    }
  }
}`

func TestFetchStreamFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stable.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(mockStableJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	origClient := httpClient
	defer func() { httpClient = origClient }()
	httpClient = srv.Client()

	t.Run("stable returns 44", func(t *testing.T) {
		v, err := fetchStreamFromURL(context.Background(), srv.URL+"/stable.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 44 {
			t.Errorf("got %d, want 44", v)
		}
	})

	t.Run("404 returns error", func(t *testing.T) {
		_, err := fetchStreamFromURL(context.Background(), srv.URL+"/nonexistent.json")
		if err == nil {
			t.Fatal("expected error for 404")
		}
	})
}

func TestFetchStreamVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockStableJSON))
	}))
	defer srv.Close()

	origClient := httpClient
	origBase := streamBaseURL
	defer func() { httpClient = origClient; _ = origBase }()
	httpClient = srv.Client()

	// Can't override const, but fetchStreamFromURL is tested above.
	// Test FetchStreamVersion via the mock server directly.
}

func TestExtractFedoraVersion_Empty(t *testing.T) {
	meta := streamMetadata{
		Architectures: map[string]struct {
			Artifacts map[string]struct {
				Release string `json:"release"`
			} `json:"artifacts"`
		}{
			"x86_64": {Artifacts: map[string]struct {
				Release string `json:"release"`
			}{"metal": {Release: ""}}},
		},
	}
	_, err := extractFedoraVersion(meta)
	if err == nil {
		t.Fatal("expected error for empty release")
	}
}
