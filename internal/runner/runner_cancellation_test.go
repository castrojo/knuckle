package runner_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestRealRunner_ContextCancellation(t *testing.T) {
	r := newRealRunner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := r.Run(ctx, "sleep", "30")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if result == nil {
		t.Fatal("expected result from cancelled context")
	}
	assertContextError(t, ctx, err, context.Canceled)
}

func TestRealRunner_ContextDeadlineExceeded(t *testing.T) {
	r := newRealRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := r.Run(ctx, "sleep", "30")
	if err == nil {
		t.Fatal("expected error from deadline exceeded")
	}
	if result == nil {
		t.Fatal("expected result from deadline exceeded")
	}
	assertContextError(t, ctx, err, context.DeadlineExceeded)
}

func TestRealRunner_RunWithInput_ContextCancellation(t *testing.T) {
	r := newRealRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	input := strings.Repeat("x", 8<<20)
	result, err := r.RunWithInput(ctx, input, "sleep", "30")
	if err == nil {
		t.Fatal("expected error from cancelled context in RunWithInput")
	}
	if result == nil {
		t.Fatal("expected result from cancelled context in RunWithInput")
	}
	assertContextError(t, ctx, err, context.DeadlineExceeded)
}

func TestRealRunner_CancellationKillsProcessGroup(t *testing.T) {
	r := newRealRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := r.Run(ctx, "sh", "-c", "sleep 30 & child=$!; echo $child; wait $child")
	if err == nil {
		t.Fatal("expected error from cancelled process group")
	}
	if result == nil {
		t.Fatal("expected result from cancelled process group")
	}
	assertContextError(t, ctx, err, context.DeadlineExceeded)

	fields := strings.Fields(result.Stdout)
	if len(fields) == 0 {
		t.Fatalf("expected child pid in stdout, got %q", result.Stdout)
	}

	childPID, convErr := strconv.Atoi(fields[0])
	if convErr != nil {
		t.Fatalf("expected numeric child pid, got %q: %v", fields[0], convErr)
	}

	if killErr := syscall.Kill(childPID, 0); killErr == nil {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
		t.Fatalf("expected child process %d to be terminated", childPID)
	} else if !errors.Is(killErr, syscall.ESRCH) {
		t.Fatalf("unexpected error checking child process %d: %v", childPID, killErr)
	}
}

func newRealRunner() *runner.RealRunner {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return runner.NewRealRunner(logger)
}

func assertContextError(t *testing.T, ctx context.Context, err error, want error) {
	t.Helper()

	if errors.Is(err, want) || strings.Contains(err.Error(), want.Error()) {
		return
	}
	if ctxErr := ctx.Err(); errors.Is(ctxErr, want) {
		return
	}

	t.Fatalf("expected context-related error %q, got %v", want, err)
}
