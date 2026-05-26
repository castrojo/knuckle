package probe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"testing"

	"github.com/NVIDIA/go-nvlib/pkg/nvpci"

	"github.com/projectbluefin/knuckle/internal/model"
	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestListDisks(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/lsblk.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	spy := runner.NewSpyRunner()
	spy.StubResponse("lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT", &runner.Result{
		Stdout: string(fixture),
	})

	prober := NewSystemProber(spy)
	disks, err := prober.ListDisks(context.Background())
	if err != nil {
		t.Fatalf("ListDisks() error: %v", err)
	}

	// sr0 is type "rom" (filtered), sda is boot disk with / mounted (filtered)
	// Only nvme0n1 should remain as an installable target
	if got := len(disks); got != 1 {
		t.Fatalf("expected 1 disk (boot disk and rom filtered), got %d", got)
	}

	// Verify the only disk is nvme
	nvme := disks[0]
	if nvme.DevPath != "/dev/nvme0n1" {
		t.Errorf("nvme.DevPath = %q, want /dev/nvme0n1", nvme.DevPath)
	}
	if nvme.SizeHuman != "931.5 GB" {
		t.Errorf("nvme.SizeHuman = %q, want 931.5 GB", nvme.SizeHuman)
	}
	if len(nvme.Partitions) != 0 {
		t.Errorf("nvme partitions: got %d, want 0", len(nvme.Partitions))
	}
}

func TestListNetworkInterfaces(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/ip_addr.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	spy := runner.NewSpyRunner()
	spy.StubResponse("ip -j addr show", &runner.Result{
		Stdout: string(fixture),
	})

	prober := NewSystemProber(spy)
	ifaces, err := prober.ListNetworkInterfaces(context.Background())
	if err != nil {
		t.Fatalf("ListNetworkInterfaces() error: %v", err)
	}

	// lo should be filtered out — expect 2 interfaces
	if got := len(ifaces); got != 2 {
		t.Fatalf("expected 2 interfaces, got %d", got)
	}

	// Verify eth0
	eth0 := ifaces[0]
	if eth0.Name != "eth0" {
		t.Errorf("eth0.Name = %q, want eth0", eth0.Name)
	}
	if eth0.MAC != "52:54:00:12:34:56" {
		t.Errorf("eth0.MAC = %q, want 52:54:00:12:34:56", eth0.MAC)
	}
	if eth0.State != "UP" {
		t.Errorf("eth0.State = %q, want UP", eth0.State)
	}
	if eth0.Driver != "ether" {
		t.Errorf("eth0.Driver = %q, want ether", eth0.Driver)
	}
	if len(eth0.IPv4Addrs) != 1 || eth0.IPv4Addrs[0] != "192.168.1.100/24" {
		t.Errorf("eth0.IPv4Addrs = %v, want [192.168.1.100/24]", eth0.IPv4Addrs)
	}
	if len(eth0.IPv6Addrs) != 1 || eth0.IPv6Addrs[0] != "fe80::5054:ff:fe12:3456/64" {
		t.Errorf("eth0.IPv6Addrs = %v, want [fe80::5054:ff:fe12:3456/64]", eth0.IPv6Addrs)
	}

	// Verify wlan0
	wlan0 := ifaces[1]
	if wlan0.Name != "wlan0" {
		t.Errorf("wlan0.Name = %q, want wlan0", wlan0.Name)
	}
	if wlan0.State != "DOWN" {
		t.Errorf("wlan0.State = %q, want DOWN", wlan0.State)
	}
	if len(wlan0.IPv4Addrs) != 0 {
		t.Errorf("wlan0.IPv4Addrs = %v, want empty", wlan0.IPv4Addrs)
	}
}

func TestListDisksError(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubError("lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT", errors.New("command not found"))

	prober := NewSystemProber(spy)
	_, err := prober.ListDisks(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "lsblk: command not found" {
		t.Errorf("error = %q, want %q", got, "lsblk: command not found")
	}
}

func TestListNetworkInterfacesInvalidJSON(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubResponse("ip -j addr show", &runner.Result{
		Stdout: "not valid json{{{",
	})

	prober := NewSystemProber(spy)
	_, err := prober.ListNetworkInterfaces(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); len(got) == 0 {
		t.Error("expected non-empty error message")
	}
}

func TestResolveByIDPathFallback(t *testing.T) {
	// When /dev/disk/by-id doesn't exist (CI, containers), falls back to devPath
	got := resolveByIDPath("/dev/sda")
	// In CI there's no /dev/disk/by-id, so it should return the input path
	if got != "/dev/sda" && got == "" {
		t.Errorf("resolveByIDPath() = %q, want non-empty fallback", got)
	}
	// The function must never return empty string
	if got == "" {
		t.Error("resolveByIDPath() returned empty string, should fallback to devPath")
	}
}

func TestResolveByIDPathLogsWarning(t *testing.T) {
	// Capture slog output to verify warning is emitted on fallback
	var records []slog.Record
	handler := &testLogHandler{records: &records}
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(original) })

	// Use a path that definitely won't have a by-id symlink
	got := resolveByIDPath("/dev/nonexistent-test-device")
	if got != "/dev/nonexistent-test-device" {
		t.Errorf("resolveByIDPath() = %q, want /dev/nonexistent-test-device", got)
	}

	// Verify at least one warning was logged
	var foundWarning bool
	for _, r := range records {
		if r.Level == slog.LevelWarn {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("resolveByIDPath() did not emit slog.Warn on fallback")
	}
}

func TestHumanSize_SpecifiedBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "below kilobyte", bytes: 1023, want: "1023 B"},
		{name: "kilobyte boundary", bytes: 1024, want: "1.0 KB"},
		{name: "megabyte boundary", bytes: 1024 * 1024, want: "1.0 MB"},
		{name: "terabyte boundary", bytes: 1024 * 1024 * 1024 * 1024, want: "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanSize(tt.bytes); got != tt.want {
				t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestDefaultNvidiaDriverSeries_Invariant(t *testing.T) {
	got := model.DefaultNvidiaDriverSeries
	if got == "" {
		t.Fatal("DefaultNvidiaDriverSeries must not be empty")
	}
	if !regexp.MustCompile(`^\d{3}(?:-open)?$`).MatchString(got) {
		t.Fatalf("DefaultNvidiaDriverSeries = %q, want NNN or NNN-open format", got)
	}
}

// testLogHandler captures log records for test assertions.
type testLogHandler struct {
	records *[]slog.Record
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	*h.records = append(*h.records, r)
	return nil
}
func (h *testLogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *testLogHandler) WithGroup(_ string) slog.Handler      { return h }

// GPU detection tests use nvidiaGPUsFromClient with nvpci.InterfaceMock,
// eliminating the need for fake /sys directories.

func TestDetectNvidiaGPUs_NoDevices(t *testing.T) {
	mock := &nvpci.InterfaceMock{
		GetGPUsFunc: func() ([]*nvpci.NvidiaPCIDevice, error) {
			return nil, nil
		},
	}
	gpus := nvidiaGPUsFromClient(mock)
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs, got %d", len(gpus))
	}
}

func TestDetectNvidiaGPUs_NvidiaPresent(t *testing.T) {
	mock := &nvpci.InterfaceMock{
		GetGPUsFunc: func() ([]*nvpci.NvidiaPCIDevice, error) {
			return []*nvpci.NvidiaPCIDevice{
				{
					Address:    "0000:01:00.0",
					Class:      0x030200,
					Device:     0x2204, // RTX 3080
					DeviceName: "GA102 [GeForce RTX 3080]",
				},
			}, nil
		},
	}
	gpus := nvidiaGPUsFromClient(mock)
	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}
	if gpus[0].PCIAddress != "0000:01:00.0" {
		t.Errorf("unexpected PCI address: %q", gpus[0].PCIAddress)
	}
	if gpus[0].PCIClass != "0x030200" {
		t.Errorf("unexpected class: %q", gpus[0].PCIClass)
	}
	if gpus[0].DeviceName != "GA102 [GeForce RTX 3080]" {
		t.Errorf("expected DeviceName from PCI DB, got %q", gpus[0].DeviceName)
	}
}

func TestDetectNvidiaGPUs_MultipleGPUs(t *testing.T) {
	mock := &nvpci.InterfaceMock{
		GetGPUsFunc: func() ([]*nvpci.NvidiaPCIDevice, error) {
			return []*nvpci.NvidiaPCIDevice{
				{Address: "0000:01:00.0", Class: 0x030200, DeviceName: "GA102 [GeForce RTX 3080]"},
				{Address: "0000:02:00.0", Class: 0x030200, DeviceName: "AD102 [GeForce RTX 4090]"},
			}, nil
		},
	}
	gpus := nvidiaGPUsFromClient(mock)
	if len(gpus) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(gpus))
	}
	if gpus[1].DeviceName != "AD102 [GeForce RTX 4090]" {
		t.Errorf("unexpected DeviceName for second GPU: %q", gpus[1].DeviceName)
	}
}

func TestDetectNvidiaGPUs_UnknownDevice_FallsBackToDeviceID(t *testing.T) {
	mock := &nvpci.InterfaceMock{
		GetGPUsFunc: func() ([]*nvpci.NvidiaPCIDevice, error) {
			return []*nvpci.NvidiaPCIDevice{
				{
					Address:    "0000:03:00.0",
					Class:      0x030200,
					Device:     0xabcd,
					DeviceName: nvpci.UnknownDeviceString,
				},
			}, nil
		},
	}
	gpus := nvidiaGPUsFromClient(mock)
	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}
	// Should fall back to "NVIDIA GPU (device 0xabcd)"
	if gpus[0].DeviceName == nvpci.UnknownDeviceString {
		t.Error("DeviceName should not be UNKNOWN_DEVICE — should fall back to device ID")
	}
	if gpus[0].DeviceName == "" {
		t.Error("DeviceName must not be empty")
	}
}

func TestDetectNvidiaGPUs_ErrorFromClient(t *testing.T) {
	mock := &nvpci.InterfaceMock{
		GetGPUsFunc: func() ([]*nvpci.NvidiaPCIDevice, error) {
			return nil, fmt.Errorf("sysfs unavailable")
		},
	}
	gpus := nvidiaGPUsFromClient(mock)
	if gpus != nil {
		t.Errorf("expected nil on error, got %v", gpus)
	}
}

func TestDetectNvidiaGPUs_Smoke(t *testing.T) {
	// On CI (no NVIDIA hardware) this returns nil — that is a valid result.
	// Test only verifies it does not panic.
	result := DetectNvidiaGPUs()
	_ = result
}
