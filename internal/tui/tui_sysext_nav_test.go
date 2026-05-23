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
