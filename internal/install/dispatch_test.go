package install

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

type spyInstaller struct {
	called bool
	cfg    *model.InstallConfig
	err    error
}

func (s *spyInstaller) Install(_ context.Context, cfg *model.InstallConfig, _ func(string)) error {
	s.called = true
	s.cfg = cfg
	return s.err
}

func TestDispatchingInstaller_DelegatesToFlatcar(t *testing.T) {
	flatcar := &spyInstaller{}
	fcos := &spyInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFlatcar, Hostname: "flatcar-node"}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flatcar.called {
		t.Error("expected Flatcar installer to be called")
	}
	if fcos.called {
		t.Error("FCOS installer should not be called for Flatcar OS")
	}
	if flatcar.cfg.Hostname != "flatcar-node" {
		t.Errorf("expected hostname 'flatcar-node', got %q", flatcar.cfg.Hostname)
	}
}

func TestDispatchingInstaller_DelegatesToFCOS(t *testing.T) {
	flatcar := &spyInstaller{}
	fcos := &spyInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFCOS, Hostname: "fcos-node"}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fcos.called {
		t.Error("expected FCOS installer to be called")
	}
	if flatcar.called {
		t.Error("Flatcar installer should not be called for FCOS OS")
	}
}

func TestDispatchingInstaller_DefaultsToFlatcar(t *testing.T) {
	flatcar := &spyInstaller{}
	fcos := &spyInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: "", Hostname: "default-node"}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flatcar.called {
		t.Error("expected Flatcar installer for empty OS (default)")
	}
}

func TestDispatchingInstaller_NilConfig(t *testing.T) {
	d := &DispatchingInstaller{
		Flatcar: &spyInstaller{},
		FCOS:    &spyInstaller{},
	}
	err := d.Install(context.Background(), nil, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config cannot be nil") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDispatchingInstaller_NilFCOSInstaller(t *testing.T) {
	d := &DispatchingInstaller{
		Flatcar: &spyInstaller{},
		FCOS:    nil,
	}
	cfg := &model.InstallConfig{OS: model.OSFCOS}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil FCOS installer")
	}
	if !strings.Contains(err.Error(), "FCOS installer not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDispatchingInstaller_NilFlatcarInstaller(t *testing.T) {
	d := &DispatchingInstaller{
		Flatcar: nil,
		FCOS:    &spyInstaller{},
	}
	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil Flatcar installer")
	}
	if !strings.Contains(err.Error(), "Flatcar installer not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDispatchingInstaller_PropagatesFlatcarError(t *testing.T) {
	flatcar := &spyInstaller{err: fmt.Errorf("flatcar install failed")}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: &spyInstaller{}}

	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "flatcar install failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDispatchingInstaller_PropagatesFCOSError(t *testing.T) {
	fcos := &spyInstaller{err: fmt.Errorf("fcos install failed")}
	d := &DispatchingInstaller{Flatcar: &spyInstaller{}, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFCOS}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "fcos install failed") {
		t.Errorf("unexpected error: %v", err)
	}
}
