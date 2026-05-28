package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/wizard"
)

type startInstallErrorInstaller struct {
	err error
}

func (i *startInstallErrorInstaller) Install(_ context.Context, _ *model.InstallConfig, _ func(string)) error {
	return i.err
}

type startInstallPanicInstaller struct{}

func (i *startInstallPanicInstaller) Install(_ context.Context, _ *model.InstallConfig, _ func(string)) error {
	panic("installer panic")
}

// startInstallProgressInstaller calls progress once then succeeds.
type startInstallProgressInstaller struct {
	step string
}

func (i *startInstallProgressInstaller) Install(_ context.Context, _ *model.InstallConfig, progress func(string)) error {
	progress(i.step)
	return nil
}

func TestStartInstall_ReturnsInstallDoneMsgOnError(t *testing.T) {
	w := wizard.New(nil, nil, &startInstallErrorInstaller{err: errors.New("install failed")})
	m := New(w)

	cmd := m.startInstall()
	if cmd == nil {
		t.Fatal("startInstall() returned nil cmd")
	}
	if m.progressCh == nil {
		t.Fatal("startInstall() should initialize progressCh")
	}
	if m.installCancel == nil {
		t.Fatal("startInstall() should initialize installCancel")
	}

	msg := cmd()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("expected installDoneMsg, got %T", msg)
	}
	if done.err == nil {
		t.Fatal("expected install error")
	}
	if !strings.Contains(done.err.Error(), "install failed") {
		t.Fatalf("expected install error to include installer failure, got %v", done.err)
	}
}

func TestStartInstall_ReturnsInstallDoneMsgOnPanic(t *testing.T) {
	w := wizard.New(nil, nil, &startInstallPanicInstaller{})
	m := New(w)

	cmd := m.startInstall()
	if cmd == nil {
		t.Fatal("startInstall() returned nil cmd")
	}

	msg := cmd()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("expected installDoneMsg, got %T", msg)
	}
	if done.err == nil {
		t.Fatal("expected panic error")
	}
	if !strings.Contains(done.err.Error(), "PANIC: installer panic") {
		t.Fatalf("expected panic error to include panic marker, got %v", done.err)
	}
}

func TestStartInstall_DeliversProgressMsgBeforeDone(t *testing.T) {
	w := wizard.New(nil, nil, &startInstallProgressInstaller{step: "partitioning disk"})
	m := New(w)

	cmd := m.startInstall()
	if cmd == nil {
		t.Fatal("startInstall() returned nil cmd")
	}

	// Drain all messages until installDoneMsg.
	gotProgress := false
	for {
		msg := cmd()
		switch v := msg.(type) {
		case installProgressMsg:
			if string(v) == "partitioning disk" {
				gotProgress = true
			}
			// Get the next waitForProgress cmd from the model update cycle.
			cmd = m.waitForProgress()
		case installDoneMsg:
			if v.err != nil {
				t.Fatalf("expected successful install, got error: %v", v.err)
			}
			if !gotProgress {
				t.Error("expected at least one installProgressMsg before installDoneMsg")
			}
			return
		default:
			t.Fatalf("unexpected message type %T", msg)
		}
	}
}
