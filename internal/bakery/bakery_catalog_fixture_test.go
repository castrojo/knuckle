package bakery_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

func loadBakeryCatalogFixture(t *testing.T) []model.SysextEntry {
	t.Helper()

	fixturePath := filepath.Join("..", "..", "testdata", "bakery_catalog.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", fixturePath, err)
	}

	var entries []model.SysextEntry
	if err := json.Unmarshal(fixture, &entries); err != nil {
		t.Fatalf("Unmarshal(%q): %v", fixturePath, err)
	}

	return entries
}

func TestMockClientFetchCatalogArch_UsesRepositoryFixture(t *testing.T) {
	entries := loadBakeryCatalogFixture(t)
	client := &bakery.MockClient{Entries: entries}

	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			got, err := client.FetchCatalogArch(context.Background(), arch)
			if err != nil {
				t.Fatalf("FetchCatalogArch(%q): %v", arch, err)
			}
			if !reflect.DeepEqual(got, entries) {
				t.Fatalf("FetchCatalogArch(%q) = %#v, want %#v", arch, got, entries)
			}
		})
	}
}
