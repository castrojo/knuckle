package demo_test

import (
	"context"
	"testing"

	"github.com/projectbluefin/knuckle/internal/demo"
)

func TestDemoProber_ReturnsFakeDisks(t *testing.T) {
	p := &demo.Prober{}
	disks, err := p.ListDisks(context.Background())
	if err != nil {
		t.Fatalf("ListDisks: %v", err)
	}
	if len(disks) == 0 {
		t.Fatal("expected at least one fake disk")
	}
	for _, d := range disks {
		if d.DevPath == "" {
			t.Error("disk DevPath must not be empty")
		}
		if d.Size == 0 {
			t.Error("disk Size must be non-zero")
		}
	}
}

func TestDemoProber_ReturnsFakeInterfaces(t *testing.T) {
	p := &demo.Prober{}
	ifaces, err := p.ListNetworkInterfaces(context.Background())
	if err != nil {
		t.Fatalf("ListNetworkInterfaces: %v", err)
	}
	if len(ifaces) == 0 {
		t.Fatal("expected at least one fake interface")
	}
}

func TestDemoBakery_ReturnsSysexts(t *testing.T) {
	b := &demo.Bakery{}
	entries, err := b.FetchCatalogArch(context.Background(), "amd64")
	if err != nil {
		t.Fatalf("FetchCatalogArch: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected demo sysext entries")
	}
	for _, e := range entries {
		if e.Name == "" || e.URL == "" {
			t.Errorf("sysext entry missing Name or URL: %+v", e)
		}
	}
}

func TestDemoInstaller_CompletesWithProgress(t *testing.T) {
	inst := &demo.Installer{}
	var msgs []string
	err := inst.Install(context.Background(), nil, func(msg string) {
		msgs = append(msgs, msg)
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected progress messages from demo installer")
	}
}

func TestDemoChannels_ReturnsFourChannels(t *testing.T) {
	channels := demo.Channels()
	if len(channels) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(channels))
	}
	names := make(map[string]bool)
	for _, c := range channels {
		names[c.Channel] = true
		if c.Version == "" {
			t.Errorf("channel %q has empty Version", c.Channel)
		}
	}
	for _, want := range []string{"stable", "beta", "alpha", "lts"} {
		if !names[want] {
			t.Errorf("missing channel %q", want)
		}
	}
}
