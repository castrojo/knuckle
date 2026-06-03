package bakery

import (
	"context"
	"errors"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

func TestDispatchingClient_FetchCatalogForOS_Flatcar(t *testing.T) {
	flatcar := &MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	fcos := &MockClient{Entries: []model.SysextEntry{{Name: "podman"}}}
	d := &DispatchingClient{Flatcar: flatcar, FCOS: fcos}

	entries, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFlatcar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Errorf("expected Flatcar entries, got %v", entries)
	}
}

func TestDispatchingClient_FetchCatalogForOS_FCOS(t *testing.T) {
	flatcar := &MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	fcos := &MockClient{Entries: []model.SysextEntry{{Name: "podman"}}}
	d := &DispatchingClient{Flatcar: flatcar, FCOS: fcos}

	entries, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFCOS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "podman" {
		t.Errorf("expected FCOS entries, got %v", entries)
	}
}

func TestDispatchingClient_FetchCatalogForOS_EmptyDefaultsFlatcar(t *testing.T) {
	flatcar := &MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	d := &DispatchingClient{Flatcar: flatcar}

	entries, err := d.FetchCatalogForOS(context.Background(), "amd64", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Errorf("expected Flatcar entries for empty OS, got %v", entries)
	}
}

func TestDispatchingClient_FetchCatalogForOS_UnsupportedOS(t *testing.T) {
	d := &DispatchingClient{
		Flatcar: &MockClient{},
		FCOS:    &MockClient{},
	}

	_, err := d.FetchCatalogForOS(context.Background(), "amd64", "windows")
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
}

func TestDispatchingClient_FetchCatalogForOS_NilFCOS(t *testing.T) {
	d := &DispatchingClient{Flatcar: &MockClient{}}

	_, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFCOS)
	if err == nil {
		t.Fatal("expected error when FCOS client is nil")
	}
}

func TestDispatchingClient_FetchCatalogForOS_PropagatesError(t *testing.T) {
	want := errors.New("network down")
	flatcar := &MockClient{Err: want}
	d := &DispatchingClient{Flatcar: flatcar}

	_, err := d.FetchCatalogForOS(context.Background(), "amd64", model.OSFlatcar)
	if !errors.Is(err, want) {
		t.Errorf("expected %v, got %v", want, err)
	}
}

func TestDispatchingClient_FetchCatalog_DelegatesToFlatcar(t *testing.T) {
	flatcar := &MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	d := &DispatchingClient{Flatcar: flatcar}

	entries, err := d.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Errorf("expected Flatcar entries, got %v", entries)
	}
}

func TestDispatchingClient_FetchCatalogArch_DelegatesToFlatcar(t *testing.T) {
	flatcar := &MockClient{Entries: []model.SysextEntry{{Name: "docker"}}}
	d := &DispatchingClient{Flatcar: flatcar}

	entries, err := d.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docker" {
		t.Errorf("expected Flatcar entries, got %v", entries)
	}
}
