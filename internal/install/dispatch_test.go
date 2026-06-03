package install

import (
	"context"
	"errors"
	"testing"

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

func TestDispatchingInstaller_DelegatesToFlatcar(t *testing.T) {
	flatcar := &stubInstaller{}
	fcos := &stubInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flatcar.called {
		t.Error("expected Flatcar installer to be called")
	}
	if fcos.called {
		t.Error("FCOS installer should not be called for Flatcar OS")
	}
}

func TestDispatchingInstaller_DelegatesToFCOS(t *testing.T) {
	flatcar := &stubInstaller{}
	fcos := &stubInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar, FCOS: fcos}

	cfg := &model.InstallConfig{OS: model.OSFCOS}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fcos.called {
		t.Error("expected FCOS installer to be called")
	}
	if flatcar.called {
		t.Error("Flatcar installer should not be called for FCOS")
	}
}

func TestDispatchingInstaller_EmptyOSDefaultsToFlatcar(t *testing.T) {
	flatcar := &stubInstaller{}
	d := &DispatchingInstaller{Flatcar: flatcar}

	cfg := &model.InstallConfig{OS: ""}
	if err := d.Install(context.Background(), cfg, func(string) {}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flatcar.called {
		t.Error("expected Flatcar installer for empty OS")
	}
}

func TestDispatchingInstaller_UnsupportedOS(t *testing.T) {
	d := &DispatchingInstaller{
		Flatcar: &stubInstaller{},
		FCOS:    &stubInstaller{},
	}

	cfg := &model.InstallConfig{OS: "windows"}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
	if got := err.Error(); got != `unsupported OS "windows"` {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestDispatchingInstaller_NilFCOS(t *testing.T) {
	d := &DispatchingInstaller{Flatcar: &stubInstaller{}}

	cfg := &model.InstallConfig{OS: model.OSFCOS}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when FCOS installer is nil")
	}
}

func TestDispatchingInstaller_NilFlatcar(t *testing.T) {
	d := &DispatchingInstaller{FCOS: &stubInstaller{}}

	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	err := d.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected error when Flatcar installer is nil")
	}
}

func TestDispatchingInstaller_PropagatesError(t *testing.T) {
	want := errors.New("disk full")
	flatcar := &stubInstaller{err: want}
	d := &DispatchingInstaller{Flatcar: flatcar}

	cfg := &model.InstallConfig{OS: model.OSFlatcar}
	err := d.Install(context.Background(), cfg, func(string) {})
	if !errors.Is(err, want) {
		t.Errorf("expected %v, got %v", want, err)
	}
}
