package fcos

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseFedoraVersion(t *testing.T) {
	tests := []struct {
		release string
		want    int
		wantErr bool
	}{
		{"44.20260510.3.1", 44, false},
		{"41.20250223.3.0", 41, false},
		{"43.20260101.1.0", 43, false},
		{"", 0, true},
		{"notanumber.123", 0, true},
		{"0.123", 0, true},
		{"-1.123", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.release, func(t *testing.T) {
			got, err := parseFedoraVersion(tt.release)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %d", tt.release, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.release, err)
			}
			if got != tt.want {
				t.Errorf("parseFedoraVersion(%q) = %d, want %d", tt.release, got, tt.want)
			}
		})
	}
}

func TestFetchStreamFedoraVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/streams/stable.json":
			fmt.Fprint(w, `{
				"architectures": {
					"x86_64": {
						"artifacts": {
							"metal": {"release": "44.20260510.3.1"}
						}
					}
				}
			}`)
		case "/streams/testing.json":
			fmt.Fprint(w, `{
				"architectures": {
					"aarch64": {
						"artifacts": {
							"metal": {"release": "45.20260601.1.0"}
						}
					}
				}
			}`)
		case "/streams/empty.json":
			fmt.Fprint(w, `{"architectures": {}}`)
		case "/streams/badjson.json":
			fmt.Fprint(w, `not json`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Override the base URL for testing.
	origBase := streamBaseURL
	defer func() {
		// streamBaseURL is a const, so we test via the internal helper.
		_ = origBase
	}()

	client := srv.Client()

	t.Run("stable stream x86_64", func(t *testing.T) {
		got, err := fetchStreamFedoraVersionWithClient(
			context.Background(), "stable",
			&http.Client{Transport: rewriteTransport{base: srv.URL + "/streams/", rt: client.Transport}},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 44 {
			t.Errorf("got %d, want 44", got)
		}
	})

	t.Run("testing stream aarch64 fallback", func(t *testing.T) {
		got, err := fetchStreamFedoraVersionWithClient(
			context.Background(), "testing",
			&http.Client{Transport: rewriteTransport{base: srv.URL + "/streams/", rt: client.Transport}},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 45 {
			t.Errorf("got %d, want 45", got)
		}
	})

	t.Run("empty architectures", func(t *testing.T) {
		_, err := fetchStreamFedoraVersionWithClient(
			context.Background(), "empty",
			&http.Client{Transport: rewriteTransport{base: srv.URL + "/streams/", rt: client.Transport}},
		)
		if err == nil {
			t.Fatal("expected error for empty architectures")
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		_, err := fetchStreamFedoraVersionWithClient(
			context.Background(), "badjson",
			&http.Client{Transport: rewriteTransport{base: srv.URL + "/streams/", rt: client.Transport}},
		)
		if err == nil {
			t.Fatal("expected error for bad JSON")
		}
	})

	t.Run("HTTP 404", func(t *testing.T) {
		_, err := fetchStreamFedoraVersionWithClient(
			context.Background(), "nonexistent",
			&http.Client{Transport: rewriteTransport{base: srv.URL + "/streams/", rt: client.Transport}},
		)
		if err == nil {
			t.Fatal("expected error for 404")
		}
	})
}

// rewriteTransport rewrites the request URL to point at the test server while
// preserving the path suffix, so fetchStreamFedoraVersionWithClient hits the
// test server instead of builds.coreos.fedoraproject.org.
type rewriteTransport struct {
	base string
	rt   http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the const base URL with the test server base.
	orig := req.URL.String()
	newURL := t.base + req.URL.Path[len("/streams/"):]
	req2, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("rewriting %s → %s: %w", orig, newURL, err)
	}
	req2.Header = req.Header
	return t.rt.RoundTrip(req2)
}
