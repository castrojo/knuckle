package install_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/projectbluefin/knuckle/internal/install"
	"github.com/projectbluefin/knuckle/internal/model"
)

type stubInstaller struct {
	called bool
	err    error
}

func (s *stubInstaller) Install(_ context.Context, _ *model.InstallConfig, _ func(string)) error {
	s.called = true
	return s.err
}

func TestDispatchingInstaller_Flatcar(t *testing.T) {
	flatcar := &stubInstaller{}
	fcos := &stubInstaller{}
	d := &install.DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flatcar.called {
		t.Fatal("expected Flatcar installer to be called")
	}
	if fcos.called {
		t.Fatal("FCOS installer should not be called")
	}
}

func TestDispatchingInstaller_EmptyOSDefaultsToFlatcar(t *testing.T) {
	flatcar := &stubInstaller{}
	d := &install.DispatchingInstaller{Flatcar: flatcar, FCOS: &stubInstaller{}}

	cfg := &model.InstallConfig{OS: ""}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flatcar.called {
		t.Fatal("expected Flatcar installer to be called for empty OS")
	}
}

func TestDispatchingInstaller_FCOS(t *testing.T) {
	flatcar := &stubInstaller{}
	fcos := &stubInstaller{}
	d := &install.DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFCOS}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fcos.called {
		t.Fatal("expected FCOS installer to be called")
	}
	if flatcar.called {
		t.Fatal("Flatcar installer should not be called")
	}
}

func TestDispatchingInstaller_UnsupportedOS(t *testing.T) {
	d := &install.DispatchingInstaller{
		Flatcar: &stubInstaller{},
		FCOS:    &stubInstaller{},
	}
	cfg := &model.InstallConfig{OS: "nixos"}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
}

func TestDispatchingInstaller_NilFCOS(t *testing.T) {
	d := &install.DispatchingInstaller{Flatcar: &stubInstaller{}}
	cfg := &model.InstallConfig{OS: model.OSFCOS}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when FCOS installer is nil")
	}
}

func TestDispatchingInstaller_NilFCOSDryRun(t *testing.T) {
	// Dry-run only validates config; nil FCOS installer must not block it.
	d := &install.DispatchingInstaller{Flatcar: &stubInstaller{}}
	cfg := &model.InstallConfig{OS: model.OSFCOS, DryRun: true}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error in dry-run with nil FCOS installer: %v", err)
	}
}

func TestDispatchingInstaller_NilFlatcar(t *testing.T) {
	d := &install.DispatchingInstaller{FCOS: &stubInstaller{}}
	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when Flatcar installer is nil")
	}
}

func TestDispatchingInstaller_PropagatesError(t *testing.T) {
	flatcar := &stubInstaller{err: fmt.Errorf("disk full")}
	d := &install.DispatchingInstaller{Flatcar: flatcar, FCOS: &stubInstaller{}}

	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil || err.Error() != "disk full" {
		t.Fatalf("expected 'disk full' error, got: %v", err)
	}
}

var _ install.Installer = (*install.DispatchingInstaller)(nil)
