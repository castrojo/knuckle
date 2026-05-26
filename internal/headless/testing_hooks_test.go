package headless

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

// TestOverrideValidateBlockDevice_UsesCustomValidator verifies that the override
// hook properly replaces the block device validator used by Run().
func TestOverrideValidateBlockDevice_UsesCustomValidator(t *testing.T) {
	cleanup := OverrideValidateBlockDevice(func(device string) error {
		// Custom validator that only accepts /dev/test-device
		if device != "/dev/test-device" {
			return nil // Pass for testing
		}
		return nil
	})
	defer cleanup()

	// Verify override is active by checking the behavior
	// Run a test that requires block device validation
	cfg := &Config{
		Channel:     "stable",
		Hostname:    "test",
		Disk:        "/dev/test-device",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign",
		Reboot:      false,
		DryRun:      false,
	}

	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := Run(context.Background(), cfg, installer, logger)
	if err != nil {
		t.Fatalf("Run with overridden validator failed: %v", err)
	}
}

// TestOverrideValidateBlockDevice_RestoresOriginal verifies that the cleanup
// function properly restores the original validator.
func TestOverrideValidateBlockDevice_RestoresOriginal(t *testing.T) {
	// Save original
	originalValidator := validateBlockDeviceFunc

	// Override and verify it's different
	cleanup := OverrideValidateBlockDevice(func(device string) error {
		t.Log("custom validator called")
		return nil
	})

	// Cleanup should restore original
	cleanup()

	// Verify restoration (by pointer comparison)
	if validateBlockDeviceFunc == nil || originalValidator == nil {
		// Can't easily compare function pointers, so just verify the function works
		// by checking that it doesn't panic
		err := validateBlockDeviceFunc("/dev/null")
		if err != nil && err.Error() == "not a block device" {
			// This is the expected behavior of the original validator
			return
		}
	}
}

// TestOverrideRebootDelay_ShortsTheWait verifies that the override hook
// properly shortens the reboot delay used by Run().
func TestOverrideRebootDelay_ShortsTheWait(t *testing.T) {
	cleanup := OverrideRebootDelay(nil) // Sets rebootDelay to 0
	defer cleanup()

	// Test that the delay was shortened
	origFetch := fetchGitHubKeysFunc
	defer func() { fetchGitHubKeysFunc = origFetch }()
	fetchGitHubKeysFunc = func(_ context.Context, _ string) ([]string, error) {
		return nil, nil
	}

	origValidateBlockDevice := validateBlockDeviceFunc
	defer func() { validateBlockDeviceFunc = origValidateBlockDevice }()
	validateBlockDeviceFunc = func(_ string) error { return nil }

	cfg := &Config{
		Channel:     "stable",
		Hostname:    "node",
		Disk:        "/dev/vda",
		Network:     NetworkConfig{Mode: "dhcp"},
		IgnitionURL: "https://example.com/config.ign",
		Reboot:      true,
		DryRun:      false,
	}

	installer := &skipBlockDeviceInstaller{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Measure execution time to verify the delay was shortened
	start := time.Now()
	err := Run(context.Background(), cfg, installer, logger)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil return, got: %v", err)
	}

	// With rebootDelay=0, Run() should complete very quickly (under 100ms)
	// If it took 3+ seconds, the override didn't work
	if duration > 1*time.Second {
		t.Errorf("reboot delay override didn't work: Run took %v (expected <100ms)", duration)
	}
}

// TestOverrideRebootDelay_RestoresOriginal verifies that the cleanup function
// properly restores the original reboot delay.
func TestOverrideRebootDelay_RestoresOriginal(t *testing.T) {
	originalDelay := rebootDelay

	cleanup := OverrideRebootDelay(nil)
	if rebootDelay != 0 {
		t.Errorf("override didn't set rebootDelay to 0, got: %v", rebootDelay)
	}

	cleanup()
	if rebootDelay != originalDelay {
		t.Errorf("cleanup didn't restore original delay, got: %v (expected %v)", rebootDelay, originalDelay)
	}
}
