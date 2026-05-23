package tui

import (
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/bakery"
)

func TestViewChannelCardsLongVersionNoPanic(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Channels = []bakery.ChannelInfo{
		{
			Channel: "stable",
			Version: "99999.99.99-very-long-prerelease-tag-exceeding-width",
			Kernel:  "6.99.0",
		},
	}
	m := New(w)
	m.cursor = 0

	// This should not panic even with a very long version string
	// Regression test for #248 (negative padding fix)
	out := m.viewChannelCards()
	if len(out) == 0 {
		t.Error("viewChannelCards should render non-empty output")
	}
	if !strings.Contains(out, "stable") && !strings.Contains(out, "Stable") {
		t.Errorf("viewChannelCards should contain channel name, got: %q", out)
	}
}

func TestViewChannelCardsSelectsCurrent(t *testing.T) {
	w := newTestWizard()
	w.State.Config.Channel = "stable"
	w.State.Channels = []bakery.ChannelInfo{
		{Channel: "stable", Version: "1.0.0", Kernel: "6.1.0"},
		{Channel: "edge", Version: "2.0.0", Kernel: "6.2.0"},
	}
	m := New(w)
	m.cursor = 0

	out := m.viewChannelCards()
	if !strings.Contains(out, "▸") {
		t.Error("viewChannelCards should show cursor indicator for selected item")
	}
}
