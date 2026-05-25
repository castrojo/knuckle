//go:build integration
// +build integration

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRealRunnerRunCancelsProcessGroup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux")
	}
	requireCommands(t, "sh", "sleep")

	rr := newIntegrationRealRunner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := rr.Run(ctx, "sh", "-c", `sleep 30 & child=$!; printf '%s\n' "$child"; wait "$child"`)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Duration >= 5*time.Second {
		t.Fatalf("expected command to be cancelled quickly, ran for %s", result.Duration)
	}
	if !isContextCancellationError(err, context.Canceled) {
		t.Fatalf("expected context-cancellation related error, got %v", err)
	}

	childPID, parseErr := strconv.Atoi(strings.TrimSpace(result.Stdout))
	if parseErr != nil {
		t.Fatalf("expected child pid in stdout, got %q: %v", result.Stdout, parseErr)
	}

	waitForProcessGone(t, childPID, 3*time.Second)
}

func TestRealRunnerRunHonorsContextDeadline(t *testing.T) {
	requireCommands(t, "sleep")

	rr := newIntegrationRealRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := rr.Run(ctx, "sleep", "30")
	if err == nil {
		t.Fatal("expected deadline error, got nil")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Duration >= 5*time.Second {
		t.Fatalf("expected command to stop at deadline, ran for %s", result.Duration)
	}
	if !isContextCancellationError(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline-related error, got %v", err)
	}
}

func TestRealRunnerRunWithInputCancelMidWrite(t *testing.T) {
	requireCommands(t, "sh", "sleep")

	rr := newIntegrationRealRunner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	input := strings.Repeat("knuckle", 1<<20)
	result, err := rr.RunWithInput(ctx, input, "sh", "-c", "sleep 30")
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Duration >= 5*time.Second {
		t.Fatalf("expected command to be cancelled quickly, ran for %s", result.Duration)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("expected stdin-pipe failure path, got exit error: %v", err)
	}
	if !hasAnySubstring(err.Error(), "broken pipe", "file already closed", "closed pipe", context.Canceled.Error()) {
		t.Fatalf("expected stdin-pipe or cancellation error, got %v", err)
	}
}

func newIntegrationRealRunner() *RealRunner {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewRealRunner(logger)
}

func requireCommands(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("required command %q not available: %v", name, err)
		}
	}
}

func isContextCancellationError(err error, want error) bool {
	if errors.Is(err, want) {
		return true
	}

	message := err.Error()
	if strings.Contains(message, want.Error()) {
		return true
	}

	return strings.Contains(message, "signal: killed")
}

func hasAnySubstring(value string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(value, sub) {
			return true
		}
	}
	return false
}

func waitForProcessGone(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		exists, state, err := processState(pid)
		if err != nil {
			t.Fatalf("checking child process %d: %v", pid, err)
		}
		if !exists {
			return
		}
		if state != "Z" {
			t.Logf("waiting for child process %d to exit, current state=%s", pid, state)
		}
		time.Sleep(25 * time.Millisecond)
	}

	exists, state, err := processState(pid)
	if err != nil {
		t.Fatalf("re-checking child process %d: %v", pid, err)
	}
	if !exists {
		return
	}

	t.Fatalf("child process %d still present after %s (state=%s)", pid, timeout, state)
}

func processState(pid int) (exists bool, state string, err error) {
	procStatus := filepath.Join("/proc", strconv.Itoa(pid), "status")
	data, err := os.ReadFile(procStatus)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, syscall.ESRCH) {
			return false, "", nil
		}
		return false, "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "State:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return true, fields[1], nil
			}
			return true, "", fmt.Errorf("unexpected process state line: %q", line)
		}
	}

	return true, "", fmt.Errorf("process state not found in %s", procStatus)
}
