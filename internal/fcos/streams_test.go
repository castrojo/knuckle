package fcos_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projectbluefin/knuckle/internal/fcos"
)

func TestFetchStreamFedoraVersion(t *testing.T) {
	makeStreamJSON := func(release string) []byte {
		data := map[string]any{
			"stream": "stable",
			"architectures": map[string]any{
				"x86_64": map[string]any{
					"artifacts": map[string]any{
						"metal": map[string]any{
							"release": release,
						},
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		stream      string
		wantVersion int
		wantErr     bool
	}{
		{
			name:   "stable stream Fedora 44",
			stream: "stable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/streams/stable.json" {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write(makeStreamJSON("44.20260510.3.1"))
			},
			wantVersion: 44,
		},
		{
			name:   "testing stream Fedora 43",
			stream: "testing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(makeStreamJSON("43.20250131.1.0"))
			},
			wantVersion: 43,
		},
		{
			name:   "next stream Fedora 45",
			stream: "next",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(makeStreamJSON("45.20260101.0.0"))
			},
			wantVersion: 45,
		},
		{
			name:   "HTTP 404 returns error",
			stream: "stable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			},
			wantErr: true,
		},
		{
			name:   "invalid JSON returns error",
			stream: "stable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("not json"))
			},
			wantErr: true,
		},
		{
			name:   "missing architectures returns error",
			stream: "stable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"stream":"stable","architectures":{}}`))
			},
			wantErr: true,
		},
		{
			name:   "aarch64 fallback when x86_64 missing",
			stream: "stable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				data := map[string]any{
					"architectures": map[string]any{
						"aarch64": map[string]any{
							"artifacts": map[string]any{
								"metal": map[string]any{
									"release": "44.20260510.3.1",
								},
							},
						},
					},
				}
				b, _ := json.Marshal(data)
				_, _ = w.Write(b)
			},
			wantVersion: 44,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			// Override the stream base URL to point at the test server.
			ver, err := fcos.FetchStreamFedoraVersionFromURL(context.Background(), tc.stream, srv.URL+"/streams/%s.json")

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got version %d", ver)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ver != tc.wantVersion {
				t.Errorf("version = %d, want %d", ver, tc.wantVersion)
			}
		})
	}
}

func TestFetchStreamFedoraVersion_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never responds
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fcos.FetchStreamFedoraVersionFromURL(ctx, "stable", srv.URL+"/streams/%s.json")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
