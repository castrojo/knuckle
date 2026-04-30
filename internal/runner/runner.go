// Package runner provides a command execution abstraction with dry-run support
// and test spy capabilities for the knuckle installer.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Result holds the output of a command execution.
type Result struct {
	Command  string
	Args     []string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Runner is the interface for executing system commands.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (*Result, error)
	RunWithInput(ctx context.Context, input string, name string, args ...string) (*Result, error)
}

// RealRunner executes commands on the real system.
type RealRunner struct {
	Logger *slog.Logger
}

// NewRealRunner creates a RealRunner with the given logger.
func NewRealRunner(logger *slog.Logger) *RealRunner {
	return &RealRunner{Logger: logger}
}

func (r *RealRunner) Run(ctx context.Context, name string, args ...string) (*Result, error) {
	r.Logger.Debug("executing command", "cmd", name, "args", args)

	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Command:  name,
		Args:     args,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, fmt.Errorf("command %q exited with code %d: %w", name, result.ExitCode, err)
		}
		return result, fmt.Errorf("command %q failed: %w", name, err)
	}

	return result, nil
}

func (r *RealRunner) RunWithInput(ctx context.Context, input string, name string, args ...string) (*Result, error) {
	r.Logger.Debug("executing command with input", "cmd", name, "args", args)

	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)

	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Command:  name,
		Args:     args,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, fmt.Errorf("command %q exited with code %d: %w", name, result.ExitCode, err)
		}
		return result, fmt.Errorf("command %q failed: %w", name, err)
	}

	return result, nil
}

// DryRunner logs commands without executing them.
type DryRunner struct {
	Logger  *slog.Logger
	History []Result
}

// NewDryRunner creates a DryRunner with the given logger.
func NewDryRunner(logger *slog.Logger) *DryRunner {
	return &DryRunner{Logger: logger}
}

func (r *DryRunner) Run(ctx context.Context, name string, args ...string) (*Result, error) {
	r.Logger.Info("dry-run", "cmd", name, "args", args)
	result := &Result{Command: name, Args: args, ExitCode: 0}
	r.History = append(r.History, *result)
	return result, nil
}

func (r *DryRunner) RunWithInput(ctx context.Context, input string, name string, args ...string) (*Result, error) {
	r.Logger.Info("dry-run with input", "cmd", name, "args", args)
	result := &Result{Command: name, Args: args, ExitCode: 0}
	// When butane is invoked in dry-run, return a minimal valid Ignition JSON
	// so downstream steps can proceed without error.
	if name == "butane" {
		result.Stdout = `{"ignition":{"version":"3.4.0"}}`
	}
	r.History = append(r.History, *result)
	return result, nil
}

// SpyRunner records commands and returns preconfigured responses (for tests).
type SpyRunner struct {
	Calls     []SpyCall
	Responses map[string]*Result
	Errors    map[string]error
}

// SpyCall records a single invocation of the runner.
type SpyCall struct {
	Name  string
	Args  []string
	Input string
}

// NewSpyRunner creates a SpyRunner ready for use.
func NewSpyRunner() *SpyRunner {
	return &SpyRunner{
		Responses: make(map[string]*Result),
		Errors:    make(map[string]error),
	}
}

// StubResponse sets up a canned response for a command pattern.
func (r *SpyRunner) StubResponse(command string, result *Result) {
	r.Responses[command] = result
}

// StubError sets up a canned error for a command pattern.
func (r *SpyRunner) StubError(command string, err error) {
	r.Errors[command] = err
}

func (r *SpyRunner) Run(ctx context.Context, name string, args ...string) (*Result, error) {
	r.Calls = append(r.Calls, SpyCall{Name: name, Args: args})
	key := strings.Join(append([]string{name}, args...), " ")

	if err, ok := r.Errors[key]; ok {
		return &Result{Command: name, Args: args, ExitCode: 1}, err
	}
	if resp, ok := r.Responses[key]; ok {
		return resp, nil
	}
	return &Result{Command: name, Args: args, ExitCode: 0}, nil
}

func (r *SpyRunner) RunWithInput(ctx context.Context, input string, name string, args ...string) (*Result, error) {
	r.Calls = append(r.Calls, SpyCall{Name: name, Args: args, Input: input})
	key := strings.Join(append([]string{name}, args...), " ")

	if err, ok := r.Errors[key]; ok {
		return &Result{Command: name, Args: args, ExitCode: 1}, err
	}
	if resp, ok := r.Responses[key]; ok {
		return resp, nil
	}
	return &Result{Command: name, Args: args, ExitCode: 0}, nil
}
