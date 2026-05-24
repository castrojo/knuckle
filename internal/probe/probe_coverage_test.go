package probe

import (
	"context"
	"fmt"
	"testing"

	"github.com/projectbluefin/knuckle/internal/runner"
)

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{102400, "100.0 KB"},
		{1024*1024 - 1, "1024.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{512 * 1024 * 1024, "512.0 MB"},
		{2 * 1024 * 1024 * 1024, "2.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{2 * 1024 * 1024 * 1024 * 1024, "2.0 TB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := humanSize(tt.bytes); got != tt.want {
				t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestListDisks_InvalidJSON(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubResponse("lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT",
		&runner.Result{Stdout: "not valid json{{{"})
	_, err := NewSystemProber(spy).ListDisks(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestListDisks_SmallDiskFiltered(t *testing.T) {
	// Disks < 8 GiB are filtered out (too small for Flatcar)
	js := buildSingleDiskJSON("/dev/sdb", 4*1024*1024*1024, "disk", false, nil)
	spy := runner.NewSpyRunner()
	spy.StubResponse("lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT",
		&runner.Result{Stdout: js})
	disks, err := NewSystemProber(spy).ListDisks(context.Background())
	if err != nil {
		t.Fatalf("ListDisks: %v", err)
	}
	if len(disks) != 0 {
		t.Errorf("expected 0 disks (<8 GiB filtered), got %d", len(disks))
	}
}

func TestListDisks_RemovableDiskFiltered(t *testing.T) {
	js := buildSingleDiskJSON("/dev/sdc", 64*1024*1024*1024, "disk", true, nil)
	spy := runner.NewSpyRunner()
	spy.StubResponse("lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT",
		&runner.Result{Stdout: js})
	disks, err := NewSystemProber(spy).ListDisks(context.Background())
	if err != nil {
		t.Fatalf("ListDisks: %v", err)
	}
	if len(disks) != 0 {
		t.Errorf("expected 0 disks (removable filtered), got %d", len(disks))
	}
}

func TestListDisks_PartitionsIncluded(t *testing.T) {
	// Unmounted data partition — not filtered as a boot disk
	child := `{"name":"sda1","path":"/dev/sda1","model":null,"serial":null,"size":"512000000","tran":null,"rm":false,"type":"part","fstype":"ext4","label":null,"mountpoint":null,"children":null}`
	js := buildSingleDiskJSON("/dev/sda", 500*1024*1024*1024, "disk", false, []string{child})
	spy := runner.NewSpyRunner()
	spy.StubResponse("lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT",
		&runner.Result{Stdout: js})
	disks, err := NewSystemProber(spy).ListDisks(context.Background())
	if err != nil {
		t.Fatalf("ListDisks: %v", err)
	}
	if len(disks) == 0 {
		t.Fatal("expected disk, got none")
	}
	if len(disks[0].Partitions) == 0 {
		t.Error("expected partition to be parsed")
	}
	if disks[0].Partitions[0].Path != "/dev/sda1" {
		t.Errorf("partition Path = %q, want /dev/sda1", disks[0].Partitions[0].Path)
	}
}

func TestListNetworkInterfaces_RunnerError(t *testing.T) {
	spy := runner.NewSpyRunner()
	spy.StubError("ip -j addr show", fmt.Errorf("ip: command not found"))
	_, err := NewSystemProber(spy).ListNetworkInterfaces(context.Background())
	if err == nil {
		t.Fatal("expected error when runner fails, got nil")
	}
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func buildSingleDiskJSON(path string, size uint64, devType string, removable bool, children []string) string {
	rm := "false"
	if removable {
		rm = "true"
	}
	ch := ""
	for i, c := range children {
		if i > 0 {
			ch += ","
		}
		ch += c
	}
	return fmt.Sprintf(`{"blockdevices":[{"name":"testdisk","path":%q,"model":"Test","serial":"001","size":"%d","tran":"sata","rm":%s,"type":%q,"fstype":null,"label":null,"mountpoint":null,"children":[%s]}]}`,
		path, size, rm, devType, ch)
}
