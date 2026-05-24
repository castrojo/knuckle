package runner

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRealRunner_ContextCancellation_KillsProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := NewRealRunner(slog.Default())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := r.Run(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
}

func TestRealRunner_ContextTimeout_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r := NewRealRunner(slog.Default())

	start := time.Now()
	_, err := r.Run(ctx, "sleep", "10")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error on context timeout, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("command took %v; expected cancellation within ~100ms", elapsed)
	}
}

func TestRealRunner_RunWithInput_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := NewRealRunner(slog.Default())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := r.RunWithInput(ctx, "ignored", "sleep", "10")
	if err == nil {
		t.Fatal("expected error when context is cancelled during RunWithInput")
	}
}

func TestRealRunner_ContextCancellation_NonZeroExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r := NewRealRunner(slog.Default())
	result, err := r.Run(ctx, "sleep", "10")

	if err == nil {
		t.Fatal("expected error on context timeout")
	}
	// When a process is killed by signal, ExitCode should be non-zero (typically -1).
	// exec.CommandContext sends SIGKILL which means ExitError with code -1.
	if result != nil && result.ExitCode == 0 {
		t.Errorf("expected non-zero ExitCode for killed process, got %d", result.ExitCode)
	}
}
