package bakery_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

func TestDispatchingClient_FlatcarDefault(t *testing.T) {
	flatcar := &bakery.MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	fcos := &bakery.MockClient{Entries: []model.SysextEntry{{Name: "podman"}}}
	d := &bakery.DispatchingClient{Flatcar: flatcar, FCOS: fcos}

	entries, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFlatcar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Fatalf("expected flatcar entries, got %v", entries)
	}
}

func TestDispatchingClient_EmptyOSDefaultsToFlatcar(t *testing.T) {
	flatcar := &bakery.MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	d := &bakery.DispatchingClient{Flatcar: flatcar, FCOS: &bakery.MockClient{}}

	entries, err := d.FetchCatalogForOS(context.Background(), "amd64", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Fatalf("expected flatcar entries for empty OS, got %v", entries)
	}
}

func TestDispatchingClient_FCOS(t *testing.T) {
	flatcar := &bakery.MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	fcos := &bakery.MockClient{Entries: []model.SysextEntry{{Name: "podman"}}}
	d := &bakery.DispatchingClient{Flatcar: flatcar, FCOS: fcos}

	entries, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFCOS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "podman" {
		t.Fatalf("expected fcos entries, got %v", entries)
	}
}

func TestDispatchingClient_UnsupportedOS(t *testing.T) {
	d := &bakery.DispatchingClient{
		Flatcar: &bakery.MockClient{},
		FCOS:    &bakery.MockClient{},
	}
	_, err := d.FetchCatalogForOS(context.Background(), "amd64", "nixos")
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
}

func TestDispatchingClient_NilFCOS(t *testing.T) {
	d := &bakery.DispatchingClient{Flatcar: &bakery.MockClient{}}
	_, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFCOS)
	if err == nil {
		t.Fatal("expected error when FCOS client is nil")
	}
}

func TestDispatchingClient_PropagatesError(t *testing.T) {
	flatcar := &bakery.MockClient{Err: fmt.Errorf("network error")}
	d := &bakery.DispatchingClient{Flatcar: flatcar}

	_, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFlatcar)
	if err == nil || err.Error() != "network error" {
		t.Fatalf("expected 'network error', got: %v", err)
	}
}

func TestDispatchingClient_FetchCatalogDelegatesToFlatcar(t *testing.T) {
	flatcar := &bakery.MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	d := &bakery.DispatchingClient{Flatcar: flatcar}

	entries, err := d.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Fatalf("expected flatcar entries from FetchCatalog, got %v", entries)
	}
}

func TestDispatchingClient_FetchCatalogArchDelegatesToFlatcar(t *testing.T) {
	flatcar := &bakery.MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	d := &bakery.DispatchingClient{Flatcar: flatcar}

	entries, err := d.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Fatalf("expected flatcar entries from FetchCatalogArch, got %v", entries)
	}
}

var _ bakery.Client = (*bakery.DispatchingClient)(nil)
var _ bakery.FCOSClient = (*bakery.DispatchingClient)(nil)

func TestDispatchingClient_FetchCatalogFCOS_DelegatesToFCOSClient(t *testing.T) {
	wantEntries := []model.SysextEntry{{Name: "docker-ce"}}
	fcosClient := &bakery.MockFCOSClient{FCOSEntries: wantEntries}
	d := &bakery.DispatchingClient{
		Flatcar: &bakery.MockClient{},
		FCOS:    fcosClient,
	}

	entries, err := d.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker-ce" {
		t.Fatalf("expected fcos entries, got %v", entries)
	}
	if fcosClient.CalledArch != "amd64" {
		t.Errorf("expected arch amd64, got %q", fcosClient.CalledArch)
	}
	if fcosClient.CalledFedoraVer != 44 {
		t.Errorf("expected fedoraVersion 44, got %d", fcosClient.CalledFedoraVer)
	}
}

func TestDispatchingClient_FetchCatalogFCOS_FallsBackWhenNotFCOSClient(t *testing.T) {
	flatEntries := []model.SysextEntry{{Name: "docker"}}
	fcosGeneric := &bakery.MockClient{Entries: flatEntries}
	d := &bakery.DispatchingClient{
		Flatcar: &bakery.MockClient{},
		FCOS:    fcosGeneric,
	}

	// MockClient does not implement FCOSClient; should fall back to FetchCatalogArch.
	entries, err := d.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Fatalf("expected fallback entries, got %v", entries)
	}
}

func TestDispatchingClient_FetchCatalogFCOS_NilFCOS(t *testing.T) {
	d := &bakery.DispatchingClient{Flatcar: &bakery.MockClient{}}
	_, err := d.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil {
		t.Fatal("expected error when FCOS client is nil")
	}
}

func TestDispatchingClient_FetchCatalogFCOS_PropagatesError(t *testing.T) {
	fcosClient := &bakery.MockFCOSClient{FCOSErr: fmt.Errorf("catalog error")}
	d := &bakery.DispatchingClient{
		Flatcar: &bakery.MockClient{},
		FCOS:    fcosClient,
	}
	_, err := d.FetchCatalogFCOS(context.Background(), "amd64", 44)
	if err == nil || err.Error() != "catalog error" {
		t.Fatalf("expected 'catalog error', got %v", err)
	}
}
