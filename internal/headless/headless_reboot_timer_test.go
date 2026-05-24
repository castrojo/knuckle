package headless

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/projectbluefin/knuckle/internal/model"
)

// TestRun_RebootTimerFires covers the happy-path reboot branch (lines 477-485)
// where Reboot=true, DryRun=false, and the timer fires before context cancellation.
// Uses the rebootDelay variable to avoid a 3-second wait.
func TestRun_RebootTimerFires(t *testing.T) {
	// Shorten reboot delay for test speed
	origDelay := rebootDelay
	rebootDelay = 10 * time.Millisecond
	defer func() { rebootDelay = origDelay }()

	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(_ context.Context, _ string) ([]string, error) {
		return nil, nil
	}

	cfg := &Config{
		Channel:     "stable",
		Hostname:    "node01",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign", // bypass butane generation
		Reboot:      true,
		DryRun:      false, // enters the reboot timer select
	}

	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Run(context.Background(), cfg, installer, logger)
	if err != nil {
		t.Fatalf("expected nil return after reboot timer fires, got: %v", err)
	}
}

// TestRun_RebootTimerContextCancelled verifies that context cancellation during
// the reboot wait returns ctx.Err() (complementary test to the timer-fires case).
func TestRun_RebootTimerContextCancelled(t *testing.T) {
	// Use a long delay so context cancellation wins the race
	origDelay := rebootDelay
	rebootDelay = 10 * time.Second
	defer func() { rebootDelay = origDelay }()

	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(_ context.Context, _ string) ([]string, error) {
		return nil, nil
	}

	cfg := &Config{
		Channel:     "stable",
		Hostname:    "node01",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign",
		Reboot:      true,
		DryRun:      false,
	}

	// Cancel immediately — context cancellation should win over the long timer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Run(ctx, cfg, installer, logger)
	if err == nil {
		t.Fatal("expected context error during reboot wait, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context-related error, got: %v", err)
	}
}

// TestRun_NoReboot_DryRunFalse verifies that when Reboot=false and DryRun=false,
// Run prints the success message and returns nil without entering the timer.
func TestRun_NoReboot_DryRunFalse(t *testing.T) {
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(_ context.Context, _ string) ([]string, error) {
		return nil, nil
	}

	cfg := &Config{
		Channel:     "stable",
		Hostname:    "node01",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign",
		Reboot:      false,
		DryRun:      false,
	}

	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Run(context.Background(), cfg, installer, logger)
	if err != nil {
		t.Fatalf("expected nil return when Reboot=false, got: %v", err)
	}
}
