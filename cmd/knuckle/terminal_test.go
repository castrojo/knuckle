package main

import (
	"context"
	"errors"
	"testing"

	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestPrepareSerialTerminalSetsFixedGeometry(t *testing.T) {
	setTerminalHooks(t,
		func(uintptr) (int, int, error) { return 0, 0, nil },
		func() (string, error) { return "ttyS0", nil },
	)

	spy := runner.NewSpyRunner()
	if err := prepareSerialTerminal(context.Background(), spy); err != nil {
		t.Fatalf("prepareSerialTerminal() error = %v", err)
	}

	if len(spy.Calls) != 1 {
		t.Fatalf("expected one command, got %d: %+v", len(spy.Calls), spy.Calls)
	}
	want := []string{"-F", "/dev/console", "rows", "40", "cols", "80"}
	call := spy.Calls[0]
	if call.Name != "stty" {
		t.Errorf("command = %q, want stty", call.Name)
	}
	if len(call.Args) != len(want) {
		t.Fatalf("args = %v, want %v", call.Args, want)
	}
	for i := range want {
		if call.Args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, call.Args[i], want[i])
		}
	}
}

func TestPrepareSerialTerminalSkipsNonSerialConsole(t *testing.T) {
	setTerminalHooks(t,
		func(uintptr) (int, int, error) { return 0, 0, nil },
		func() (string, error) { return "tty0", nil },
	)

	spy := runner.NewSpyRunner()
	if err := prepareSerialTerminal(context.Background(), spy); err != nil {
		t.Fatalf("prepareSerialTerminal() error = %v", err)
	}
	if len(spy.Calls) != 0 {
		t.Fatalf("expected no command, got %+v", spy.Calls)
	}
}

func TestPrepareSerialTerminalSkipsSizedTerminal(t *testing.T) {
	setTerminalHooks(t,
		func(uintptr) (int, int, error) { return 80, 24, nil },
		func() (string, error) { return "ttyS0", nil },
	)

	spy := runner.NewSpyRunner()
	if err := prepareSerialTerminal(context.Background(), spy); err != nil {
		t.Fatalf("prepareSerialTerminal() error = %v", err)
	}
	if len(spy.Calls) != 0 {
		t.Fatalf("expected no command, got %+v", spy.Calls)
	}
}

func TestPrepareSerialTerminalSkipsSizeProbeErrors(t *testing.T) {
	setTerminalHooks(t,
		func(uintptr) (int, int, error) { return 0, 0, errors.New("not a TTY") },
		func() (string, error) { return "ttyS0", nil },
	)

	spy := runner.NewSpyRunner()
	if err := prepareSerialTerminal(context.Background(), spy); err != nil {
		t.Fatalf("prepareSerialTerminal() error = %v", err)
	}
	if len(spy.Calls) != 0 {
		t.Fatalf("expected no command, got %+v", spy.Calls)
	}
}

func TestPrepareSerialTerminalReportsConsoleDetectionError(t *testing.T) {
	wantErr := errors.New("sysfs unavailable")
	setTerminalHooks(t,
		func(uintptr) (int, int, error) { return 0, 0, nil },
		func() (string, error) { return "", wantErr },
	)

	spy := runner.NewSpyRunner()
	err := prepareSerialTerminal(context.Background(), spy)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(spy.Calls) != 0 {
		t.Fatalf("expected no command, got %+v", spy.Calls)
	}
}

func TestPrepareSerialTerminalReportsSttyError(t *testing.T) {
	setTerminalHooks(t,
		func(uintptr) (int, int, error) { return 0, 0, nil },
		func() (string, error) { return "ttyUSB0", nil },
	)

	wantErr := errors.New("stty failed")
	spy := runner.NewSpyRunner()
	spy.StubError("stty -F /dev/console rows 40 cols 80", wantErr)
	err := prepareSerialTerminal(context.Background(), spy)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestIsSerialConsole(t *testing.T) {
	tests := []struct {
		name   string
		active string
		want   bool
	}{
		{name: "serial", active: "ttyS0", want: true},
		{name: "usb serial", active: "ttyUSB0", want: true},
		{name: "acm serial", active: "ttyACM0", want: true},
		{name: "arm serial", active: "ttyAMA0", want: true},
		{name: "vga", active: "ttyS0 tty0", want: false},
		{name: "empty", active: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSerialConsole(tt.active); got != tt.want {
				t.Errorf("isSerialConsole(%q) = %v, want %v", tt.active, got, tt.want)
			}
		})
	}
}

func setTerminalHooks(t *testing.T,
	sizeFn func(uintptr) (int, int, error),
	consoleFn func() (string, error),
) {
	t.Helper()
	originalSizeFn := terminalSizeFn
	originalConsoleFn := activeConsoleFn
	terminalSizeFn = sizeFn
	activeConsoleFn = consoleFn
	t.Cleanup(func() {
		terminalSizeFn = originalSizeFn
		activeConsoleFn = originalConsoleFn
	})
}
