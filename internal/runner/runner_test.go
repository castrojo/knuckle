package runner

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestDryRunnerRecordsHistory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dr := NewDryRunner(logger)
	ctx := context.Background()

	_, err := dr.Run(ctx, "mount", "/dev/sda1", "/mnt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = dr.RunWithInput(ctx, "yes", "fdisk", "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(dr.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(dr.History))
	}

	if dr.History[0].Command != "mount" {
		t.Errorf("expected command 'mount', got %q", dr.History[0].Command)
	}
	if len(dr.History[0].Args) != 2 || dr.History[0].Args[0] != "/dev/sda1" {
		t.Errorf("unexpected args: %v", dr.History[0].Args)
	}
	if dr.History[1].Command != "fdisk" {
		t.Errorf("expected command 'fdisk', got %q", dr.History[1].Command)
	}
}

func TestDryRunnerButaneReturnsIgnitionJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dr := NewDryRunner(logger)
	ctx := context.Background()

	result, err := dr.RunWithInput(ctx, "some butane yaml", "butane", "--strict")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"ignition":{"version":"3.4.0"}}`
	if result.Stdout != expected {
		t.Errorf("expected butane stub output %q, got %q", expected, result.Stdout)
	}

	// Non-butane commands should still return empty stdout
	result2, err := dr.RunWithInput(ctx, "input", "cat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.Stdout != "" {
		t.Errorf("expected empty stdout for non-butane command, got %q", result2.Stdout)
	}
}

func TestSpyRunnerReturnsStubbed(t *testing.T) {
	spy := NewSpyRunner()
	ctx := context.Background()

	spy.StubResponse("lsblk --json", &Result{
		Command: "lsblk",
		Args:    []string{"--json"},
		Stdout:  `{"blockdevices":[]}`,
	})
	spy.StubError("mount /dev/sda1 /mnt", errors.New("permission denied"))

	// Test stubbed response
	res, err := spy.Run(ctx, "lsblk", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != `{"blockdevices":[]}` {
		t.Errorf("unexpected stdout: %q", res.Stdout)
	}

	// Test stubbed error
	_, err = spy.Run(ctx, "mount", "/dev/sda1", "/mnt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "permission denied" {
		t.Errorf("unexpected error: %v", err)
	}

	// Test unstubbed command returns empty success
	res, err = spy.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
}

func TestSpyRunnerRecordsCalls(t *testing.T) {
	spy := NewSpyRunner()
	ctx := context.Background()

	_, _ = spy.Run(ctx, "echo", "hello")
	_, _ = spy.RunWithInput(ctx, "payload", "tee", "/tmp/out")

	if len(spy.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(spy.Calls))
	}

	if spy.Calls[0].Name != "echo" || spy.Calls[0].Args[0] != "hello" {
		t.Errorf("unexpected first call: %+v", spy.Calls[0])
	}
	if spy.Calls[0].Input != "" {
		t.Errorf("expected empty input for Run, got %q", spy.Calls[0].Input)
	}

	if spy.Calls[1].Name != "tee" || spy.Calls[1].Input != "payload" {
		t.Errorf("unexpected second call: %+v", spy.Calls[1])
	}
}

func TestRealRunnerExecutes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rr := NewRealRunner(logger)
	ctx := context.Background()

	res, err := rr.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("expected stdout %q, got %q", "hello\n", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if res.Duration == 0 {
		t.Error("expected non-zero duration")
	}

	// Test RunWithInput
	res, err = rr.RunWithInput(ctx, "from stdin", "cat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "from stdin" {
		t.Errorf("expected stdout %q, got %q", "from stdin", res.Stdout)
	}
}
