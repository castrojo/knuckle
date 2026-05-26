package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// ── sysextListLookup ──────────────────────────────────────────────────────────

func TestSysextListLookup_Found(t *testing.T) {
	w := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated},
		{Name: "tailscale", SupportTier: bakery.TierMaintained},
	}
	m := New(w)
	m.Wizard.State.CurrentStep = model.StepSysext
	m.initSysextList() // populates m.sysextList

	// Look up item at sysexts index 1 (tailscale)
	listIdx := m.sysextListLookup(1)
	if listIdx < 0 || listIdx >= len(m.sysextList.Items()) {
		t.Fatalf("sysextListLookup(1) = %d, out of bounds", listIdx)
	}
	item := m.sysextList.Items()[listIdx]
	si, ok := item.(sysextItem)
	if !ok || si.idx != 1 {
		t.Errorf("sysextListLookup(1) returned item with idx %d, want 1", si.idx)
	}
}

func TestSysextListLookup_NotFound_ReturnsZero(t *testing.T) {
	w := newTestWizard()
	w.State.Sysexts = []model.SysextEntry{
		{Name: "docker", SupportTier: bakery.TierIntegrated},
	}
	m := New(w)
	m.Wizard.State.CurrentStep = model.StepSysext
	m.initSysextList() // populates m.sysextList

	// Index 99 doesn't exist in the list -> should return 0
	if got := m.sysextListLookup(99); got != 0 {
		t.Errorf("sysextListLookup(99) = %d, want 0", got)
	}
}

// ── getChannelMeta ────────────────────────────────────────────────────────────

func TestGetChannelMeta_ReturnsAllChannels(t *testing.T) {
	m := New(newTestWizard())
	metas := m.getChannelMeta()
	if len(metas) == 0 {
		t.Fatal("getChannelMeta() returned empty slice")
	}
	names := make(map[string]bool)
	for _, meta := range metas {
		names[meta.name] = true
	}
	for _, want := range []string{"stable", "beta", "alpha"} {
		if !names[want] {
			t.Errorf("getChannelMeta() missing channel %q", want)
		}
	}
}

func TestGetChannelMeta_PopulatesVersionFromState(t *testing.T) {
	w := newTestWizard()
	w.State.Channels = []bakery.ChannelInfo{
		{Channel: "stable", Version: "4593.2.0", Kernel: "6.12.81"},
	}
	m := New(w)
	metas := m.getChannelMeta()
	var stableMeta channelMeta
	for _, meta := range metas {
		if meta.name == "stable" {
			stableMeta = meta
			break
		}
	}
	if stableMeta.version != "4593.2.0" {
		t.Errorf("stable version = %q, want 4593.2.0", stableMeta.version)
	}
	if stableMeta.kernel != "6.12.81" {
		t.Errorf("stable kernel = %q, want 6.12.81", stableMeta.kernel)
	}
}

func TestGetChannelMeta_FillsAllKnownChannelVersions(t *testing.T) {
	w := newTestWizard()
	w.State.Channels = []bakery.ChannelInfo{
		{Channel: "stable", Version: "4600.1.0", Kernel: "6.12.90", Systemd: "257.10", Docker: "28.1.1"},
		{Channel: "lts", Version: "4081.4.2", Kernel: "5.15.180", Systemd: "255.22", Docker: "27.5.1"},
		{Channel: "beta", Version: "4610.0.0", Kernel: "6.13.1", Systemd: "258.1", Docker: "29.0.0"},
	}

	got := make(map[string]channelMeta)
	for _, meta := range New(w).getChannelMeta() {
		got[meta.name] = meta
	}

	for _, want := range []struct {
		name    string
		version string
		kernel  string
		systemd string
		docker  string
	}{
		{name: "stable", version: "4600.1.0", kernel: "6.12.90", systemd: "257.10", docker: "28.1.1"},
		{name: "lts", version: "4081.4.2", kernel: "5.15.180", systemd: "255.22", docker: "27.5.1"},
		{name: "beta", version: "4610.0.0", kernel: "6.13.1", systemd: "258.1", docker: "29.0.0"},
	} {
		meta, ok := got[want.name]
		if !ok {
			t.Fatalf("missing channel %q in getChannelMeta() output", want.name)
		}
		if meta.version != want.version || meta.kernel != want.kernel || meta.systemd != want.systemd || meta.docker != want.docker {
			t.Fatalf("channel %q = %+v, want version=%q kernel=%q systemd=%q docker=%q", want.name, meta, want.version, want.kernel, want.systemd, want.docker)
		}
	}

	if alpha := got["alpha"]; alpha.version != "" || alpha.kernel != "" || alpha.systemd != "" || alpha.docker != "" {
		t.Fatalf("alpha metadata should remain empty when not fetched, got %+v", alpha)
	}
}

func TestGetChannelMeta_HasDescriptions(t *testing.T) {
	m := New(newTestWizard())
	for _, meta := range m.getChannelMeta() {
		if meta.desc == "" {
			t.Errorf("channel %q has empty description", meta.name)
		}
	}
}

// ── reviewSummary branches ────────────────────────────────────────────────────

func TestReviewSummary_Version(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Version = "3510.2.8"
	s := New(w).reviewSummary()
	if !strings.Contains(s, "v3510.2.8") {
		t.Errorf("reviewSummary should show version: %q", s)
	}
}

func TestReviewSummary_SwapEnabled(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Swap = model.SwapConfig{Enabled: true, SizeMB: 4096}
	s := New(w).reviewSummary()
	if !strings.Contains(s, "4096") {
		t.Errorf("reviewSummary should show swap size: %q", s)
	}
}
