package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestRun_InjectsWizardAndRebootFn verifies that Run() accepts wizard+rebootFn
// and the injected runner is called (no TTY required).
func TestRun_InjectsWizardAndRebootFn(t *testing.T) {
	w := newTestWizard()
	rebootFn := func(_ context.Context) error { return nil }

	origRunner := programRunner
	programRunner = func(_ *tea.Program) error { return nil }
	defer func() { programRunner = origRunner }()

	if err := Run(w, rebootFn); err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
}

// TestRun_PropagatesRunnerError verifies that an error from the injected runner
// is returned from Run().
func TestRun_PropagatesRunnerError(t *testing.T) {
	w := newTestWizard()
	wantErr := errors.New("injected runner error")

	origRunner := programRunner
	programRunner = func(_ *tea.Program) error { return wantErr }
	defer func() { programRunner = origRunner }()

	err := Run(w, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

// TestRun_NilRebootFn verifies Run() accepts a nil rebootFn without panicking.
func TestRun_NilRebootFn(t *testing.T) {
	w := newTestWizard()

	origRunner := programRunner
	programRunner = func(_ *tea.Program) error { return nil }
	defer func() { programRunner = origRunner }()

	if err := Run(w, nil); err != nil {
		t.Fatalf("Run(nil rebootFn) returned unexpected error: %v", err)
	}
}
