package probe

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/castrojo/knuckle/internal/runner"
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
