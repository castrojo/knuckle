package runner

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestRealRunner_CommandNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	_, err := NewRealRunner(logger).Run(context.Background(), "/no-such-cmd-xyz-abc")
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

func TestRealRunner_NonZeroExit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	result, err := NewRealRunner(logger).Run(context.Background(), "sh", "-c", "exit 2")
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	if result == nil || result.ExitCode != 2 {
		t.Errorf("ExitCode = %v, want 2", result)
	}
}

func TestRealRunner_RunWithInput_NonZeroExit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	result, err := NewRealRunner(logger).RunWithInput(context.Background(), "ignored", "sh", "-c", "exit 3")
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	if result == nil || result.ExitCode != 3 {
		t.Errorf("ExitCode = %v, want 3", result)
	}
}

func TestSpyRunner_AllError_RunWithInput(t *testing.T) {
	spy := NewSpyRunner()
	spy.AllError = errors.New("all commands fail")
	_, err := spy.RunWithInput(context.Background(), "input", "any-cmd")
	if err == nil {
		t.Fatal("expected AllError to be returned")
	}
}

func TestSpyRunner_KeyedError_RunWithInput(t *testing.T) {
	spy := NewSpyRunner()
	spy.StubError("mycommand --flag", errors.New("specific failure"))
	_, err := spy.RunWithInput(context.Background(), "stdin", "mycommand", "--flag")
	if err == nil || err.Error() != "specific failure" {
		t.Errorf("expected keyed error, got %v", err)
	}
}

func TestSpyRunner_KeyedResponse_RunWithInput(t *testing.T) {
	spy := NewSpyRunner()
	spy.StubResponse("mycommand arg1", &Result{Stdout: "hello", ExitCode: 0})
	res, err := spy.RunWithInput(context.Background(), "stdin", "mycommand", "arg1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "hello" {
		t.Errorf("Stdout = %q, want hello", res.Stdout)
	}
}

func TestRealRunner_RunWithInput_CommandNotFound(t *testing.T) {
	// A missing binary causes cmd.Run() to return *exec.Error (not *exec.ExitError),
	// so the type assertion fails and RunWithInput falls through to the generic
	// "command %q failed" error path.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	_, err := NewRealRunner(logger).RunWithInput(context.Background(), "stdin-data", "/no-such-binary-xyz-abc")
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

func TestSpyRunner_AllError_Run(t *testing.T) {
	// AllError is tested via RunWithInput elsewhere; this covers SpyRunner.Run.
	spy := NewSpyRunner()
	spy.AllError = errors.New("all-run error")
	_, err := spy.Run(context.Background(), "any-cmd")
	if err == nil || err.Error() != "all-run error" {
		t.Errorf("expected AllError from Run, got %v", err)
	}
}
