package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/projectbluefin/knuckle/internal/runner"
)

const (
	serialConsolePath = "/dev/console"
	serialRows        = "40"
	serialColumns     = "80"
)

// terminalSizeFn and activeConsoleFn are injectable so serial-console startup
// behavior can be tested without requiring a real TTY or sysfs.
var (
	terminalSizeFn = func(fd uintptr) (int, int, error) {
		return term.GetSize(fd)
	}
	activeConsoleFn = func() (string, error) {
		data, err := os.ReadFile("/sys/class/tty/console/active")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
)

// prepareSerialTerminal gives hardware serial consoles a usable geometry
// before Bubble Tea queries the terminal size. Hardware serial TTYs commonly
// report 0x0 because they do not negotiate rows and columns like a PTY.
func prepareSerialTerminal(ctx context.Context, cmdRunner runner.Runner) error {
	width, height, err := terminalSizeFn(os.Stdout.Fd())
	if err != nil || width != 0 || height != 0 {
		return nil
	}

	activeConsole, err := activeConsoleFn()
	if err != nil {
		return fmt.Errorf("detecting active console: %w", err)
	}
	if !isSerialConsole(activeConsole) {
		return nil
	}

	if _, err := cmdRunner.Run(ctx, "stty", "-F", serialConsolePath,
		"rows", serialRows, "cols", serialColumns); err != nil {
		return fmt.Errorf("setting serial console geometry: %w", err)
	}
	return nil
}

func isSerialConsole(active string) bool {
	fields := strings.Fields(active)
	if len(fields) == 0 {
		return false
	}

	// /sys/class/tty/console/active lists active consoles in registration
	// order; the last one is the console selected by the final console= kernel
	// argument and therefore the device /dev/console refers to.
	name := fields[len(fields)-1]
	for _, prefix := range []string{"ttyS", "ttyUSB", "ttyACM", "ttyAMA"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
