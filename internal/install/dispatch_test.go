package install

import (
	"context"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

// mockInstaller records whether Install was called and with what OS.
type mockInstaller struct {
	called bool
	osUsed string
}

func (m *mockInstaller) Install(_ context.Context, cfg *model.InstallConfig, _ func(string)) error {
	m.called = true
	m.osUsed = cfg.OS
	return nil
}

func TestDispatchingInstaller_DelegatesToFlatcar(t *testing.T) {
	flatcar := &mockInstaller{}
	fcos := &mockInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !flatcar.called {
		t.Error("expected Flatcar installer to be called")
	}
	if fcos.called {
		t.Error("FCOS installer should not be called for Flatcar target")
	}
}

func TestDispatchingInstaller_DelegatesToFCOS(t *testing.T) {
	flatcar := &mockInstaller{}
	fcos := &mockInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFCOS}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fcos.called {
		t.Error("expected FCOS installer to be called")
	}
	if flatcar.called {
		t.Error("Flatcar installer should not be called for FCOS target")
	}
}

func TestDispatchingInstaller_DefaultsToFlatcar(t *testing.T) {
	flatcar := &mockInstaller{}
	fcos := &mockInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: ""}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !flatcar.called {
		t.Error("empty OS should default to Flatcar installer")
	}
	if fcos.called {
		t.Error("FCOS installer should not be called for empty OS")
	}
}
