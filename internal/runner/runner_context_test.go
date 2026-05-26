package runner

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRealRunner_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := NewRealRunner(slog.Default())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := r.Run(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}

func TestRealRunner_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := NewRealRunner(slog.Default())
	result, err := r.Run(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}
	if result == nil {
		t.Fatal("expected result on timeout, got nil")
		return
	}
	if result.ExitCode == 0 {
		t.Errorf("expected non-zero exit code on timeout, got 0")
	}
}

func TestRealRunner_AlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := NewRealRunner(slog.Default())
	_, err := r.Run(ctx, "echo", "should not run")
	if err == nil {
		t.Fatal("expected error with pre-cancelled context")
	}
}

func TestRealRunner_RunWithInput_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := NewRealRunner(slog.Default())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := r.RunWithInput(ctx, "hello", "sh", "-c", "cat >/dev/null; sleep 10")
	if time.Since(start) > 2*time.Second {
		t.Fatal("RunWithInput did not return promptly after context cancellation")
	}
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}
