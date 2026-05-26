package probe

import (
	"context"
	"os"
	"testing"

	"github.com/NVIDIA/go-nvlib/pkg/nvpci"

	"github.com/projectbluefin/knuckle/internal/runner"
)

func BenchmarkListDisks(b *testing.B) {
	fixture, err := os.ReadFile("../../testdata/lsblk.json")
	if err != nil {
		b.Fatalf("reading fixture: %v", err)
	}

	spy := runner.NewSpyRunner()
	spy.StubResponse("lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT", &runner.Result{
		Stdout: string(fixture),
	})

	prober := NewSystemProber(spy)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := prober.ListDisks(ctx)
		if err != nil {
			b.Fatalf("ListDisks() error: %v", err)
		}
	}
}

func BenchmarkListNetworkInterfaces(b *testing.B) {
	fixture, err := os.ReadFile("../../testdata/ip_addr.json")
	if err != nil {
		b.Fatalf("reading fixture: %v", err)
	}

	spy := runner.NewSpyRunner()
	spy.StubResponse("ip -j addr show", &runner.Result{
		Stdout: string(fixture),
	})

	prober := NewSystemProber(spy)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := prober.ListNetworkInterfaces(ctx)
		if err != nil {
			b.Fatalf("ListNetworkInterfaces() error: %v", err)
		}
	}
}

func BenchmarkDetectNvidiaGPUs(b *testing.B) {
	mock := &nvpci.InterfaceMock{
		GetGPUsFunc: func() ([]*nvpci.NvidiaPCIDevice, error) {
			return []*nvpci.NvidiaPCIDevice{
				{
					Address:    "0000:01:00.0",
					Class:      0x030200,
					Device:     0x2204,
					DeviceName: "GA102 [GeForce RTX 3080]",
				},
				{
					Address:    "0000:02:00.0",
					Class:      0x030200,
					Device:     0x2208,
					DeviceName: "AD102 [GeForce RTX 4090]",
				},
			}, nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gpus := nvidiaGPUsFromClient(mock)
		if len(gpus) != 2 {
			b.Fatalf("expected 2 GPUs, got %d", len(gpus))
		}
	}
}

func BenchmarkHumanSize(b *testing.B) {
	sizes := []uint64{
		512,                           // bytes
		2048,                          // KB
		5 * 1024 * 1024,               // MB
		100 * 1024 * 1024,             // MB
		1024 * 1024 * 1024,            // GB
		500 * 1024 * 1024 * 1024,      // GB
		2 * 1024 * 1024 * 1024 * 1024, // TB
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range sizes {
			_ = humanSize(size)
		}
	}
}

func BenchmarkResolveByIDPath(b *testing.B) {
	devPath := "/dev/sda"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resolveByIDPath(devPath)
	}
}
