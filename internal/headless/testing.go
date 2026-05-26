package headless

// OverrideValidateBlockDevice replaces the block-device validation function
// used during headless.Run. The caller must restore the original via the
// returned cleanup function. Intended for integration tests in cmd/knuckle
// that need non-dry-run paths without requiring real block devices.
func OverrideValidateBlockDevice(fn func(string) error) (cleanup func()) {
	orig := validateBlockDeviceFunc
	validateBlockDeviceFunc = fn
	return func() { validateBlockDeviceFunc = orig }
}

// OverrideRebootDelay replaces the reboot delay duration used during
// headless.Run. Returns a cleanup function to restore the original value.
func OverrideRebootDelay(fn func()) (cleanup func()) {
	// Use zero delay for tests — just set the package var directly.
	// We expose it via a no-op setter so callers don't import time.
	orig := rebootDelay
	rebootDelay = 0
	return func() { rebootDelay = orig }
}
