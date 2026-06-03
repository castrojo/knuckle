package install

import (
	"context"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestNewFCOSInstaller(t *testing.T) {
	spy := runner.NewSpyRunner()
	i := NewFCOSInstaller(spy, testLogger())
	if i == nil {
		t.Fatal("NewFCOSInstaller returned nil")
	}
	if i.Runner != spy {
		t.Error("Runner not set correctly")
	}
	if i.Generator == nil {
		t.Error("Generator should be initialized")
	}
}

func TestFCOSInstaller_NilConfig(t *testing.T) {
	spy := runner.NewSpyRunner()
	i := NewFCOSInstaller(spy, testLogger())
	err := i.Install(context.Background(), nil, func(string) {})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config cannot be nil") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFCOSInstaller_StubReturnsNotImplemented(t *testing.T) {
	spy := runner.NewSpyRunner()
	i := NewFCOSInstaller(spy, testLogger())
	cfg := &model.InstallConfig{OS: model.OSFCOS, Hostname: "fcos-test"}
	err := i.Install(context.Background(), cfg, func(string) {})
	if err == nil {
		t.Fatal("expected not-implemented error")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected not-implemented message, got: %v", err)
	}
}

func TestFCOSInstaller_ImplementsInterface(t *testing.T) {
	spy := runner.NewSpyRunner()
	var _ Installer = NewFCOSInstaller(spy, testLogger())
}
