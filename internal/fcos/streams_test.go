package fcos_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projectbluefin/knuckle/internal/fcos"
)

func TestFetchStreamFedoraVersion(t *testing.T) {
	const sampleStreamJSON = `{
		"architectures": {
			"x86_64": {
				"artifacts": {
					"metal": {"release": "44.20260510.3.1"},
					"qemu": {"release": "44.20260510.3.1"}
				}
			},
			"aarch64": {
				"artifacts": {
					"metal": {"release": "44.20260510.3.1"}
				}
			}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleStreamJSON))
	}))
	defer srv.Close()

	// Override the base URL for the test.
	origBase := fcos.StreamsBaseURL
	fcos.StreamsBaseURL = srv.URL
	defer func() { fcos.StreamsBaseURL = origBase }()

	ver, err := fcos.FetchStreamFedoraVersion(context.Background(), "stable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != 44 {
		t.Errorf("expected Fedora version 44, got %d", ver)
	}
}

func TestFetchStreamFedoraVersion_InvalidStream(t *testing.T) {
	_, err := fcos.FetchStreamFedoraVersion(context.Background(), "nightly")
	if err == nil {
		t.Fatal("expected error for unsupported stream")
	}
}

func TestFetchStreamFedoraVersion_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	origBase := fcos.StreamsBaseURL
	fcos.StreamsBaseURL = srv.URL
	defer func() { fcos.StreamsBaseURL = origBase }()

	_, err := fcos.FetchStreamFedoraVersion(context.Background(), "stable")
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestFetchStreamFedoraVersion_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"architectures":{}}`))
	}))
	defer srv.Close()

	origBase := fcos.StreamsBaseURL
	fcos.StreamsBaseURL = srv.URL
	defer func() { fcos.StreamsBaseURL = origBase }()

	_, err := fcos.FetchStreamFedoraVersion(context.Background(), "testing")
	if err == nil {
		t.Fatal("expected error when no release fields found")
	}
}

func TestFetchStreamFedoraVersion_MalformedRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"architectures": {
				"x86_64": {
					"artifacts": {
						"metal": {"release": "not-a-number.20260510.3.1"},
						"qemu": {"release": "44.20260510.3.1"}
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	origBase := fcos.StreamsBaseURL
	fcos.StreamsBaseURL = srv.URL
	defer func() { fcos.StreamsBaseURL = origBase }()

	// Should still succeed using the valid artifact.
	ver, err := fcos.FetchStreamFedoraVersion(context.Background(), "next")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != 44 {
		t.Errorf("expected 44, got %d", ver)
	}
}
