package fcos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchStreamFedoraVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
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
		}`))
	}))
	defer srv.Close()

	ver, err := fetchStreamFedoraVersionFromURL(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != 44 {
		t.Errorf("version = %d, want 44", ver)
	}
}

func TestFetchStreamFedoraVersion_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchStreamFedoraVersionFromURL(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestFetchStreamFedoraVersion_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	_, err := fetchStreamFedoraVersionFromURL(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractFedoraVersion(t *testing.T) {
	tests := []struct {
		name    string
		release string
		want    int
		wantErr bool
	}{
		{name: "fedora 44", release: "44.20260510.3.1", want: 44},
		{name: "fedora 41", release: "41.20250223.3.0", want: 41},
		{name: "fedora 43", release: "43.20260101.1.0", want: 43},
		{name: "empty release", release: "", wantErr: true},
		{name: "no dot", release: "44", wantErr: true},
		{name: "non-numeric", release: "abc.20260510.3.1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := streamMetadata{
				Architectures: map[string]struct {
					Artifacts map[string]struct {
						Release string `json:"release"`
					} `json:"artifacts"`
				}{
					"x86_64": {
						Artifacts: map[string]struct {
							Release string `json:"release"`
						}{
							"metal": {Release: tt.release},
						},
					},
				},
			}
			got, err := extractFedoraVersion(meta)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExtractFedoraVersion_MissingArch(t *testing.T) {
	meta := streamMetadata{
		Architectures: map[string]struct {
			Artifacts map[string]struct {
				Release string `json:"release"`
			} `json:"artifacts"`
		}{},
	}
	_, err := extractFedoraVersion(meta)
	if err == nil {
		t.Fatal("expected error for missing x86_64")
	}
}

func TestExtractFedoraVersion_MissingMetal(t *testing.T) {
	meta := streamMetadata{
		Architectures: map[string]struct {
			Artifacts map[string]struct {
				Release string `json:"release"`
			} `json:"artifacts"`
		}{
			"x86_64": {
				Artifacts: map[string]struct {
					Release string `json:"release"`
				}{},
			},
		},
	}
	_, err := extractFedoraVersion(meta)
	if err == nil {
		t.Fatal("expected error for missing metal artifact")
	}
}
