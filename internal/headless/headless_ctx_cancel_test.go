package headless

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

// TestRun_RebootDelay_ContextCancellation exercises the ctx.Done() arm of the
// reboot-delay select (headless.go lines 479-482).
//
// It uses the OverrideRebootDelay hook with a 5 ms delay so the timer does not
// fire first, then supplies a pre-cancelled context and verifies that Run
// returns exactly context.Canceled via errors.Is.
func TestRun_RebootDelay_ContextCancellation(t *testing.T) {
	// Shorten the reboot delay so the timer cannot win the select race.
	cleanup := OverrideRebootDelay(nil)
	defer cleanup()
	// Override with an explicit non-zero but very short duration so context
	// cancellation reliably wins.
	rebootDelay = 5e6 // 5 ms in nanoseconds (time.Duration)

	// Bypass block-device validation (no real disk required).
	cleanupBD := OverrideValidateBlockDevice(func(string) error { return nil })
	defer cleanupBD()

	// Override GitHub key fetch (no network).
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(_ context.Context, _ string) ([]string, error) {
		return nil, nil
	}

	cfg := &Config{
		Channel:     "stable",
		Hostname:    "node01",
		Disk:        "/dev/vda",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign", // bypass butane generation
		Reboot:      true,
		DryRun:      false, // must be false to enter the reboot-delay select
	}

	// Pre-cancel the context so ctx.Done() is already closed when Run reaches
	// the select, guaranteeing the ctx.Done() arm fires.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Run(ctx, cfg, installer, logger)
	if err == nil {
		t.Fatal("expected context.Canceled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected errors.Is(err, context.Canceled) to be true, got: %v", err)
	}
}
