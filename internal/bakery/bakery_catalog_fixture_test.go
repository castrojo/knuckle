package bakery_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

func TestMockClientFetchCatalogArch_UsesRepositoryFixture(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "bakery_catalog.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", fixturePath, err)
	}

	var entries []model.SysextEntry
	if err := json.Unmarshal(fixture, &entries); err != nil {
		t.Fatalf("Unmarshal(%q): %v", fixturePath, err)
	}

	client := &bakery.MockClient{Entries: entries}
	got, err := client.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("FetchCatalogArch: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries from fixture, got %d", len(got))
	}

	wants := []struct {
		name    string
		version string
		url     string
	}{
		{name: "docker", version: "24.0.7", url: "https://bakery.flatcar.org/sysext/docker-24.0.7.raw"},
		{name: "wasmcloud", version: "0.82.0", url: "https://bakery.flatcar.org/sysext/wasmcloud-0.82.0.raw"},
		{name: "tailscale", version: "1.56.1", url: "https://bakery.flatcar.org/sysext/tailscale-1.56.1.raw"},
	}
	for i, want := range wants {
		if got[i].Name != want.name {
			t.Fatalf("entry %d name = %q, want %q", i, got[i].Name, want.name)
		}
		if got[i].Version != want.version {
			t.Fatalf("entry %d version = %q, want %q", i, got[i].Version, want.version)
		}
		if got[i].URL != want.url {
			t.Fatalf("entry %d url = %q, want %q", i, got[i].URL, want.url)
		}
	}
}
