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

	// sr0 is type "rom", should be filtered out — expect 2 disks
	if got := len(disks); got != 2 {
		t.Fatalf("expected 2 disks, got %d", got)
	}

	// Verify first disk (sda)
	sda := disks[0]
	if sda.DevPath != "/dev/sda" {
		t.Errorf("sda.DevPath = %q, want /dev/sda", sda.DevPath)
	}
	if sda.Model != "Samsung SSD 870" {
		t.Errorf("sda.Model = %q, want Samsung SSD 870", sda.Model)
	}
	if sda.Serial != "S5PXNG0R312345" {
		t.Errorf("sda.Serial = %q, want S5PXNG0R312345", sda.Serial)
	}
	if sda.Size != 500107862016 {
		t.Errorf("sda.Size = %d, want 500107862016", sda.Size)
	}
	if sda.SizeHuman != "465.8 GB" {
		t.Errorf("sda.SizeHuman = %q, want 465.8 GB", sda.SizeHuman)
	}
	if sda.Transport != "sata" {
		t.Errorf("sda.Transport = %q, want sata", sda.Transport)
	}
	if sda.Removable {
		t.Error("sda.Removable = true, want false")
	}

	// Verify partitions
	if got := len(sda.Partitions); got != 2 {
		t.Fatalf("sda partitions: got %d, want 2", got)
	}
	p1 := sda.Partitions[0]
	if p1.Path != "/dev/sda1" {
		t.Errorf("partition[0].Path = %q, want /dev/sda1", p1.Path)
	}
	if p1.Label != "EFI" {
		t.Errorf("partition[0].Label = %q, want EFI", p1.Label)
	}
	if p1.FSType != "vfat" {
		t.Errorf("partition[0].FSType = %q, want vfat", p1.FSType)
	}
	if p1.MountPoint != "/boot/efi" {
		t.Errorf("partition[0].MountPoint = %q, want /boot/efi", p1.MountPoint)
	}

	// Verify second disk (nvme)
	nvme := disks[1]
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
