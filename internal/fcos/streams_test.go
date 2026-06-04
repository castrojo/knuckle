package fcos_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projectbluefin/knuckle/internal/fcos"
)

func serveFCOSStream(t *testing.T, release string) *httptest.Server {
	t.Helper()
	payload := map[string]any{
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
	body, _ := json.Marshal(payload)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func TestFetchStreamFedoraVersion(t *testing.T) {
	srv := serveFCOSStream(t, "44.20260510.3.1")
	defer srv.Close()

	ver, err := fcos.FetchStreamFedoraVersionFromURL(context.Background(), srv.URL+"/streams/stable.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != 44 {
		t.Fatalf("expected fedora version 44, got %d", ver)
	}
}

func TestFetchStreamFedoraVersion_41(t *testing.T) {
	srv := serveFCOSStream(t, "41.20241109.3.0")
	defer srv.Close()

	ver, err := fcos.FetchStreamFedoraVersionFromURL(context.Background(), srv.URL+"/streams/testing.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != 41 {
		t.Fatalf("expected fedora version 41, got %d", ver)
	}
}

func TestFetchStreamFedoraVersion_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fcos.FetchStreamFedoraVersionFromURL(context.Background(), srv.URL+"/streams/bogus.json")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFetchStreamFedoraVersion_EmptyArchitectures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"architectures":{}}`))
	}))
	defer srv.Close()

	_, err := fcos.FetchStreamFedoraVersionFromURL(context.Background(), srv.URL+"/streams/stable.json")
	if err == nil {
		t.Fatal("expected error for empty architectures")
	}
}

func TestFetchStreamFedoraVersion_ContextCancelled(t *testing.T) {
	srv := serveFCOSStream(t, "44.20260510.3.1")
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fcos.FetchStreamFedoraVersionFromURL(ctx, srv.URL+"/streams/stable.json")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
